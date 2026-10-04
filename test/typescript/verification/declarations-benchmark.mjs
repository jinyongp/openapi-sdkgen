import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { performance } from "node:perf_hooks";
import { cpus, platform, release, arch } from "node:os";
import { repositoryRoot, fixtureRoot, loadCatalog, containedPath, sha256 } from "./catalog.mjs";
import { sourceFiles, generatedFiles } from "./declarations.mjs";
import { sdkDeliveryDocument } from "./sdk-delivery-fixture.mjs";

const [binary, reportPath] = process.argv.slice(2);
assert.ok(
  binary && reportPath && process.argv.length === 4,
  "usage: declarations-benchmark.mjs BINARY REPORT_JSON",
);
const root = resolve(repositoryRoot, ".tmp/declaration-benchmark");
mkdirSync(root, { recursive: true });
const runtime = resolve(repositoryRoot, "internal/target/typescript/runtime");
const catalogRoots = loadCatalog()
  .local.filter((entry) => entry.profiles.includes("conformance") && !entry.expectedFailure)
  .map((entry) => containedPath(resolve(fixtureRoot, "generated"), entry.output));
const groups = [
  { name: "runtime", args: ["--runtime", runtime], files: sourceFiles(runtime) },
  {
    name: "representative",
    args: ["--runtime", runtime, ...catalogRoots.flatMap((path) => ["--generated", path])],
    files: [...sourceFiles(runtime), ...catalogRoots.flatMap(generatedFiles)],
  },
];
const toolFiles = Object.fromEntries(
  ["declarations.mjs", "type-reuse.mjs"].map((name) => [
    name,
    sha256(readFileSync(new URL(name, import.meta.url))),
  ]),
);
const report = {
  version: 1,
  repetitions: 5,
  environment: {
    node: process.version,
    platform: platform(),
    release: release(),
    arch: arch(),
    cpu: cpus()[0]?.model,
  },
  commit: spawnSync("git", ["rev-parse", "HEAD"], {
    cwd: repositoryRoot,
    encoding: "utf8",
  }).stdout.trim(),
  dirty:
    spawnSync("git", ["status", "--porcelain"], {
      cwd: repositoryRoot,
      encoding: "utf8",
    }).stdout.trim().length > 0,
  toolFiles,
  toolSha256: sha256(JSON.stringify(toolFiles)),
  generatorSha256: sha256(readFileSync(binary)),
  build: {
    command: "devtools run build:dev",
    millis: Number(process.env.SDKGEN_DECLARATION_BUILD_MILLIS),
    includedInMeasurements: false,
  },
  measurement:
    "Fresh declaration CLI process per repetition; manifest validation, IO, hashing and startup included. CPU and peak RSS use GNU time for that process; generation and the benchmark coordinator are excluded.",
  generation: [],
  groups: [],
};
const save = () => writeFileSync(reportPath, JSON.stringify(report, null, 2) + "\n");
for (const count of [100, 1000, 10000]) {
  const input = containedPath(root, `scale-${count}.json`);
  writeFileSync(input, JSON.stringify(sdkDeliveryDocument(count)));
  const output = containedPath(root, `scale-${count}`);
  rmSync(output, { recursive: true, force: true });
  const started = performance.now();
  const result = spawnSync(
    resolve(binary),
    [
      "generate",
      "--input",
      input,
      "--target",
      "typescript",
      "--output",
      output,
      "--typecheck=true",
    ],
    { cwd: repositoryRoot, encoding: "utf8", timeout: 180000, maxBuffer: 4 * 1024 * 1024 },
  );
  assert.equal(result.error, undefined);
  assert.equal(result.status, 0, result.stderr);
  report.generation.push({
    operations: count,
    millis: performance.now() - started,
    inputSha256: sha256(readFileSync(input)),
  });
  groups.push({
    name: `scale-${count}`,
    args: ["--generated", output],
    files: generatedFiles(output),
  });
  save();
}
const median = (values) => values.toSorted((a, b) => a - b)[Math.floor(values.length / 2)];
for (const group of groups) {
  const samples = [];
  let identity;
  for (let index = 0; index < report.repetitions; index++) {
    const output = containedPath(root, `${group.name}-${index}.json`);
    const costs = containedPath(root, `${group.name}-${index}.time`);
    const started = performance.now();
    const result = spawnSync(
      "/usr/bin/time",
      [
        "-f",
        "%U %S %M",
        "-o",
        costs,
        process.execPath,
        fileURLToPath(new URL("./declarations.mjs", import.meta.url)),
        ...group.args,
        "--json",
        output,
      ],
      { cwd: repositoryRoot, encoding: "utf8", timeout: 180000, maxBuffer: 4 * 1024 * 1024 },
    );
    const wallMillis = performance.now() - started;
    assert.equal(result.error, undefined);
    assert.equal(result.status, 0, result.stderr);
    const scan = JSON.parse(readFileSync(output, "utf8"));
    assert.ok(scan.ok && scan.errors === 0 && Object.keys(scan.violations).length === 0);
    assert.equal(scan.files, group.files.length);
    const current = JSON.stringify([
      scan.inputSha256,
      scan.files,
      scan.bytes,
      scan.exceptions,
      scan.parser,
      scan.rulesVersion,
    ]);
    if (identity)
      assert.equal(current, identity, `${group.name}: input/rules changed between repetitions`);
    identity = current;
    const [user, system, peakKiB] = readFileSync(costs, "utf8").trim().split(/\s+/).map(Number);
    assert.ok([user, system, peakKiB].every(Number.isFinite));
    samples.push({
      wallMillis,
      toolMillis: scan.toolMillis,
      inventoryMillis: scan.inventoryMillis,
      scanMillis: scan.scanMillis,
      cpuMillis: (user + system) * 1000,
      peakRSSBytes: peakKiB * 1024,
      files: scan.files,
      bytes: scan.bytes,
      inputSha256: scan.inputSha256,
      parser: scan.parser,
      rulesVersion: scan.rulesVersion,
      exceptions: scan.exceptions,
      violations: scan.violations,
      errors: scan.errors,
    });
  }
  report.groups.push({
    name: group.name,
    samples,
    median: {
      wallMillis: median(samples.map((sample) => sample.wallMillis)),
      toolMillis: median(samples.map((sample) => sample.toolMillis)),
      scanMillis: median(samples.map((sample) => sample.scanMillis)),
      cpuMillis: median(samples.map((sample) => sample.cpuMillis)),
      peakRSSBytes: median(samples.map((sample) => sample.peakRSSBytes)),
    },
  });
  save();
  console.log(`ok declaration benchmark ${group.name}: ${group.files.length} files, 5 repetitions`);
}
report.ok =
  report.groups.length === 5 && report.groups.every((group) => group.samples.length === 5);
save();
