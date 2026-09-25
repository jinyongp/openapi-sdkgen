import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  writeFileSync,
  statfsSync,
  realpathSync,
} from "node:fs";
import { dirname, resolve, relative } from "node:path";
import { fileURLToPath } from "node:url";
import { parseArgs } from "node:util";
import { performance } from "node:perf_hooks";
import { cpus, platform, arch } from "node:os";
import { transformWithOxc } from "vite";
import {
  loadCatalog,
  repositoryRoot,
  inputFor,
  inspectGenerated,
  sha256,
  containedPath,
} from "./catalog.mjs";
import { assertSameContract, assertSameSdkContract, pairedSummary, median } from "./contracts.mjs";

const { values } = parseArgs({
  options: {
    baseline: { type: "string" },
    candidate: { type: "string" },
    input: { type: "string", multiple: true, default: [] },
    fixtures: { type: "string" },
    pairs: { type: "string", default: "10" },
    "aa-pairs": { type: "string", default: "5" },
    "memory-pairs": { type: "string", default: "3" },
    "candidate-label": { type: "string", default: "candidate" },
  },
});
assert.ok(values.baseline && values.candidate, "--baseline BINARY --candidate BINARY are required");
function count(value) {
  const n = Number(value);
  assert.ok(Number.isSafeInteger(n) && n >= 1 && n <= 100);
  return n;
}
const pairs = count(values.pairs),
  aaPairs = count(values["aa-pairs"]),
  memoryPairs = count(values["memory-pairs"]);
const binaries = {
  baseline: realpathSync(resolve(repositoryRoot, values.baseline)),
  candidate: realpathSync(resolve(repositoryRoot, values.candidate)),
};
const inputs = {};
for (const item of values.input) {
  const i = item.indexOf("=");
  assert.ok(i > 0 && item.length > i + 1, "--input requires catalog-id=local-path");
  const id = item.slice(0, i);
  assert.equal(Object.hasOwn(inputs, id), false, `duplicate input: ${id}`);
  Object.defineProperty(inputs, id, {
    value: realpathSync(resolve(repositoryRoot, item.slice(i + 1))),
    enumerable: true,
  });
}
const catalog = loadCatalog();
for (const id of Object.keys(inputs))
  assert.ok(
    catalog.external.some((f) => f.id === id),
    `unknown external corpus ${id}`,
  );
const chosen = values.fixtures ? new Set(values.fixtures.split(",")) : null;
const fixtures = [...catalog.local, ...catalog.external].filter(
  (f) =>
    f.profiles.includes("preimplementation") &&
    (!chosen || chosen.has(f.id)) &&
    (f.input || inputs[f.id]),
);
if (chosen)
  for (const id of chosen)
    assert.ok(
      fixtures.some((f) => f.id === id),
      `missing/unknown fixture or external input: ${id}`,
    );
assert.ok(fixtures.length > 0);
const prepared = fixtures.map((entry) => ({ entry, input: inputFor(entry, inputs) }));
const disk = statfsSync(repositoryRoot);
const minimumFree = Math.max(
  512 * 1024 * 1024,
  prepared.reduce((n, f) => n + f.input.bytes * 32, 0),
);
assert.ok(
  disk.bavail * disk.bsize > minimumFree && disk.ffree > 25000,
  "insufficient workspace space/inodes; no generation started",
);
const outputParent = resolve(repositoryRoot, ".tmp/preimplementation");
mkdirSync(outputParent, { recursive: true });
const output = mkdtempSync(resolve(outputParent, "run-"));
const worker = resolve(dirname(fileURLToPath(import.meta.url)), "worker.mjs");
const reportPath = resolve(output, "report.json");
const hashes = Object.fromEntries(
  Object.entries(binaries).map(([key, path]) => [key, sha256(readFileSync(path))]),
);
const git = spawnSync("git", ["rev-parse", "HEAD"], { cwd: repositoryRoot, encoding: "utf8" });
const report = {
  version: 1,
  status: "running",
  candidateLabel: values["candidate-label"],
  startedAt: new Date().toISOString(),
  repositoryRevision: git.stdout?.trim(),
  binaries,
  binarySha256: hashes,
  comparisonKind:
    hashes.baseline === hashes.candidate ? "same-binary-control" : "different-binaries",
  harnessSha256: Object.fromEntries(
    ["catalog.mjs", "contracts.mjs", "scenarios.mjs", "worker.mjs", "preimplementation.mjs"].map(
      (name) => [name, sha256(readFileSync(resolve(dirname(worker), name)))],
    ),
  ),
  catalogSha256: sha256(
    readFileSync(resolve(repositoryRoot, "test/typescript/fixtures/catalog.json")),
  ),
  environment: {
    node: process.version,
    platform: platform(),
    arch: arch(),
    cpu: cpus()[0]?.model,
    logicalCPUs: cpus().length,
    vite: JSON.parse(
      readFileSync(
        resolve(repositoryRoot, "test/typescript/node_modules/vite/package.json"),
        "utf8",
      ),
    ).version,
    lockfileSha256: sha256(readFileSync(resolve(repositoryRoot, "test/typescript/pnpm-lock.yaml"))),
  },
  executionMode: "unbundled ES2022 ESM, Oxc TypeScript erasure, no minification",
  finalProductionApproval: false,
  policy: {
    pairs,
    aaPairs,
    memoryPairs,
    freshProcesses: true,
    intermediateGCInTiming: false,
    memorySeparate: true,
    absoluteFloorMs: {
      importMs: 2,
      firstClientMs: 0.25,
      importThroughFirstClientMs: 2,
      repeatedClientMs: 0.25,
    },
    relativeReviewPercent: 5,
    noiseMultiplier: 2,
    retentionLateGrowthReviewBytes: 1048576,
    note: "Review flags are not silently waived; A/A noise and absolute floors accompany every timing comparison.",
  },
  fixtures: [],
};
const save = () => writeFileSync(reportPath, JSON.stringify(report, null, 2) + "\n");
save();
console.log(`report: ${relative(repositoryRoot, reportPath)}`);

async function transpile(sourceRoot, destination, manifest) {
  mkdirSync(destination, { recursive: true });
  const names = Object.keys(manifest.files)
    .filter((f) => f.endsWith(".ts"))
    .sort();
  let cursor = 0;
  await Promise.all(
    Array.from({ length: 4 }, async () => {
      while (cursor < names.length) {
        const name = names[cursor++];
        const source = readFileSync(containedPath(sourceRoot, name), "utf8");
        const result = await transformWithOxc(source, resolve(sourceRoot, name), {
          target: "es2022",
          sourcemap: false,
        });
        const path = containedPath(destination, name.replace(/\.ts$/, ".js"));
        mkdirSync(dirname(path), { recursive: true });
        writeFileSync(path, result.code);
      }
    }),
  );
  writeFileSync(resolve(destination, "package.json"), '{"type":"module","private":true}\n');
  writeFileSync(
    resolve(destination, "verification-files.json"),
    JSON.stringify(names.map((f) => f.replace(/\.ts$/, ".js"))),
  );
  return {
    files: names.length,
    helper: existsSync(resolve(destination, "internal/runtime/wire-properties.js")),
    esmTreeSha256: sha256(
      JSON.stringify(
        names.map((f) => [
          f,
          sha256(readFileSync(resolve(destination, f.replace(/\.ts$/, ".js")))),
        ]),
      ),
    ),
  };
}

function runWorker(dir, scenario, mode, helper = false) {
  const result = spawnSync(
    process.execPath,
    ["--expose-gc", worker, dir, scenario, mode, String(helper)],
    { cwd: repositoryRoot, encoding: "utf8", timeout: 180000, maxBuffer: 32 * 1024 * 1024 },
  );
  if (result.error) throw result.error;
  assert.equal(
    result.status,
    0,
    `worker ${scenario}/${mode} failed:\n${result.stderr}\n${result.stdout}`,
  );
  return JSON.parse(result.stdout);
}

try {
  for (const { entry, input } of prepared) {
    const row = {
      id: entry.id,
      characteristics: entry.characteristics,
      input,
      generated: {},
      contracts: {},
      timings: { aa: [], ab: [] },
      memory: [],
    };
    report.fixtures.push(row);
    const dirs = {};
    for (const variant of ["baseline", "candidate"]) {
      const generated = resolve(output, entry.id, variant, "source");
      const args = [
        "generate",
        "--input",
        input.path,
        "--target",
        "typescript",
        "--output",
        generated,
      ];
      for (const addon of entry.addons ?? []) args.push("--with", addon);
      const t = performance.now();
      const process = spawnSync(binaries[variant], args, {
        cwd: repositoryRoot,
        encoding: "utf8",
        timeout: 180000,
        maxBuffer: 4 * 1024 * 1024,
      });
      writeFileSync(
        resolve(output, `${entry.id}-${variant}-generate.log`),
        (process.stdout ?? "") + (process.stderr ?? ""),
      );
      if (process.error) throw process.error;
      assert.equal(process.status, 0, `generation ${entry.id}/${variant}: ${process.stderr}`);
      const audit = inspectGenerated(generated);
      dirs[variant] = resolve(output, entry.id, variant, "esm");
      row.generated[variant] = {
        ...audit,
        files: undefined,
        singleGenerationMs: performance.now() - t,
        ...(await transpile(generated, dirs[variant], audit)),
      };
    }
    const baselineContract = runWorker(dirs.baseline, entry.scenario, "contract");
    const candidateContract = runWorker(dirs.candidate, entry.scenario, "contract");
    const permittedInternalAdditions = assertSameSdkContract(baselineContract, candidateContract);
    // A/A confirms the actual generated comparison path, not only the unit oracle.
    assertSameContract(baselineContract, runWorker(dirs.baseline, entry.scenario, "contract"));
    row.contracts = {
      status: "pass",
      fingerprint: baselineContract.contract,
      moduleExportCount: Object.keys(baselineContract.exports).length,
      rootExports: baselineContract.rootExports,
      requests: baselineContract.traces.length,
      plainWebhookMaps: baselineContract.plainWebhookMaps,
      permittedInternalAdditions,
    };
    if (entry.id === "representation") {
      const baselineLeaf = runWorker(dirs.baseline, entry.scenario, "leaf");
      const candidateLeaf = runWorker(dirs.candidate, entry.scenario, "leaf");
      assertSameContract(baselineLeaf.shape, candidateLeaf.shape);
      row.leaf = { baseline: baselineLeaf, candidate: candidateLeaf, status: "pass" };
    }
    for (let i = 0; i < aaPairs; i++)
      row.timings.aa.push({
        baseline: runWorker(dirs.baseline, entry.scenario, "timing"),
        candidate: runWorker(dirs.baseline, entry.scenario, "timing"),
      });
    for (let i = 0; i < pairs; i++) {
      const sample = {};
      for (const variant of i % 2 ? ["candidate", "baseline"] : ["baseline", "candidate"])
        sample[variant] = runWorker(dirs[variant], entry.scenario, "timing");
      row.timings.ab.push(sample);
    }
    row.timingSummary = {};
    for (const field of Object.keys(report.policy.absoluteFloorMs)) {
      const aa = pairedSummary(row.timings.aa, field);
      const ab = pairedSummary(row.timings.ab, field);
      const noise = Math.max(
        report.policy.absoluteFloorMs[field],
        2 * median(row.timings.aa.map((p) => Math.abs(p.candidate[field] - p.baseline[field]))),
      );
      row.timingSummary[field] = {
        aa,
        ab,
        noiseFloorMs: noise,
        review: ab.pairedPercent > 5 && ab.pairedDelta > noise,
      };
    }
    if (entry.profiles.includes("lifetime")) {
      for (let i = 0; i < memoryPairs; i++) {
        const pair = {};
        for (const variant of i % 2 ? ["candidate", "baseline"] : ["baseline", "candidate"])
          pair[variant] = runWorker(
            dirs[variant],
            entry.scenario,
            "lifetime",
            row.generated[variant].helper,
          );
        row.memory.push(pair);
      }
      row.memoryReview =
        median(row.memory.map((p) => p.candidate.lateGrowthBytes - p.baseline.lateGrowthBytes)) >
        report.policy.retentionLateGrowthReviewBytes;
    }
    assert.equal(
      sha256(readFileSync(input.path)),
      input.sha256,
      "input changed during verification",
    );
    save();
    console.log(
      `ok ${entry.id}: unbundled contract + ${aaPairs} A/A + ${pairs} A/B pairs${row.memory.length ? " + lifetime/isolation" : ""}`,
    );
  }
  for (const [key, path] of Object.entries(binaries))
    assert.equal(sha256(readFileSync(path)), hashes[key], "binary changed during verification");
  report.correctness = "pass";
  report.review = report.fixtures
    .filter((r) => Object.values(r.timingSummary).some((m) => m.review) || r.memoryReview)
    .map((r) => r.id);
  report.status = report.review.length ? "review" : "pass";
  if (report.status === "review") process.exitCode = 2;
} catch (error) {
  report.status = "failed";
  report.error = { message: error.message, stack: error.stack };
  process.exitCode = 1;
} finally {
  report.finishedAt = new Date().toISOString();
  save();
}
console.log(
  JSON.stringify(
    {
      status: report.status,
      correctness: report.correctness,
      review: report.review,
      error: report.error?.message,
      report: relative(repositoryRoot, reportPath),
    },
    null,
    2,
  ),
);
