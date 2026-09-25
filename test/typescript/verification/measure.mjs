import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readFileSync, writeFileSync, rmSync, existsSync } from "node:fs";
import { resolve, relative } from "node:path";
import { parseArgs } from "node:util";
import { performance } from "node:perf_hooks";
import { repositoryRoot, containedPath, sha256 } from "./catalog.mjs";
import { pairedSummary } from "./contracts.mjs";

const { values } = parseArgs({
  options: {
    report: { type: "string" },
    artifacts: { type: "string" },
    pairs: { type: "string", default: "10" },
  },
});
assert.ok(
  values.report && values.artifacts,
  "--report RUNTIME_REPORT --artifacts ARTIFACT_REPORT required",
);
const sourcePath = containedPath(repositoryRoot, values.report),
  artifactPath = containedPath(repositoryRoot, values.artifacts);
const source = JSON.parse(readFileSync(sourcePath, "utf8")),
  artifacts = JSON.parse(readFileSync(artifactPath, "utf8"));
assert.equal(source.correctness, "pass");
assert.equal(
  artifacts.sourceReportSha256,
  sha256(readFileSync(sourcePath)),
  "reports describe different executions",
);
assert.ok(existsSync("/usr/bin/time"), "GNU time required for per-child CPU and peak RSS");
const pairs = Number(values.pairs);
assert.ok(Number.isInteger(pairs) && pairs >= 5 && pairs <= 50);
const fixtures = source.fixtures.filter((f) => ["github", "stripe"].includes(f.id));
assert.equal(fixtures.length, 2, "both pinned real-world inputs required");
const parent = resolve(repositoryRoot, ".tmp/representation-measure");
mkdirSync(parent, { recursive: true });
const output = mkdtempSync(resolve(parent, "run-"));
const reportPath = resolve(output, "report.json");
const tsc = resolve(repositoryRoot, "test/typescript/node_modules/typescript/lib/tsc.js");
const result = {
  version: 1,
  status: "running",
  pairs,
  node: process.version,
  baseline: source.binarySha256.baseline,
  candidate: source.binarySha256.candidate,
  runtimeReport: relative(repositoryRoot, sourcePath),
  runtimeReportSha256: sha256(readFileSync(sourcePath)),
  artifactReportSha256: sha256(readFileSync(artifactPath)),
  method:
    "GNU time per child, paired independent processes, one unmeasured generation warmup, local input and warm filesystem, build/network excluded",
  finalProductionApproval: false,
  fixtures: [],
};
const save = () => writeFileSync(reportPath, JSON.stringify(result, null, 2) + "\n");
function timed(executable, args, log) {
  const start = performance.now();
  const child = spawnSync(
    "/usr/bin/time",
    ["-f", "__VERIFY_TIME__ %U %S %M", executable, ...args],
    { cwd: repositoryRoot, encoding: "utf8", timeout: 240000, maxBuffer: 8 * 1024 * 1024 },
  );
  const wallMs = performance.now() - start;
  writeFileSync(log, (child.stdout ?? "") + (child.stderr ?? ""));
  if (child.error) throw child.error;
  assert.equal(child.status, 0, `child failed: ${log}\n${child.stderr}\n${child.stdout}`);
  const match = /__VERIFY_TIME__ ([\d.]+) ([\d.]+) (\d+)/.exec(child.stderr);
  assert.ok(match, `missing per-process time metrics: ${log}`);
  return {
    wallMs,
    cpuMs: (Number(match[1]) + Number(match[2])) * 1000,
    peakRssBytes: Number(match[3]) * 1024,
  };
}
function generation(f, variant, iteration) {
  const destination = resolve(output, f.id, `${variant}-${iteration}`);
  const metric = timed(
    source.binaries[variant],
    ["generate", "--input", f.input.path, "--target", "typescript", "--output", destination],
    resolve(output, `${f.id}-${variant}-${iteration}-generation.log`),
  );
  rmSync(destination, { recursive: true }); // Only this invocation's newly created output.
  return metric;
}
save();
console.log(`measurement report: ${relative(repositoryRoot, reportPath)}`);
try {
  for (const f of fixtures) {
    assert.equal(sha256(readFileSync(f.input.path)), f.input.sha256);
    const art = artifacts.fixtures.find((a) => a.id === f.id);
    assert.equal(art.status, "pass", "do not benchmark failing typechecks");
    for (const variant of ["baseline", "candidate"]) {
      assert.equal(sha256(readFileSync(source.binaries[variant])), source.binarySha256[variant]);
      generation(f, variant, "warmup");
    }
    const row = { id: f.id, generation: [], strict: [], consumer: [], summary: {} };
    result.fixtures.push(row);
    for (let pair = 0; pair < pairs; pair++) {
      const generated = {},
        strict = {},
        consumer = {};
      for (const variant of pair % 2 ? ["candidate", "baseline"] : ["baseline", "candidate"]) {
        generated[variant] = generation(f, variant, pair);
        const v = art.variants[variant];
        containedPath(repositoryRoot, relative(repositoryRoot, v.strictConfig));
        containedPath(repositoryRoot, relative(repositoryRoot, v.consumerConfig));
        strict[variant] = timed(
          process.execPath,
          [tsc, "--project", v.strictConfig],
          resolve(output, `${f.id}-${variant}-${pair}-strict.log`),
        );
        consumer[variant] = timed(
          process.execPath,
          [tsc, "--project", v.consumerConfig],
          resolve(output, `${f.id}-${variant}-${pair}-consumer.log`),
        );
      }
      row.generation.push(generated);
      row.strict.push(strict);
      row.consumer.push(consumer);
      save();
    }
    for (const kind of ["generation", "strict", "consumer"])
      row.summary[kind] = Object.fromEntries(
        ["wallMs", "cpuMs", "peakRssBytes"].map((field) => [
          field,
          pairedSummary(row[kind], field),
        ]),
      );
    console.log(JSON.stringify({ id: row.id, summary: row.summary }));
    save();
  }
  for (const variant of ["baseline", "candidate"])
    assert.equal(sha256(readFileSync(source.binaries[variant])), source.binarySha256[variant]);
  result.status = "pass";
} catch (error) {
  result.status = "failed";
  result.error = { message: error.message, stack: error.stack };
  process.exitCode = 1;
} finally {
  save();
}
console.log(
  JSON.stringify({
    status: result.status,
    report: relative(repositoryRoot, reportPath),
    error: result.error?.message,
  }),
);
