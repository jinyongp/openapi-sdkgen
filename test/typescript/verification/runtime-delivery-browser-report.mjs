// Cross-check actual HTTP responses against the independently built module inventory.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const manifest = JSON.parse(
  fs.readFileSync(path.join(root, ".tmp/runtime-delivery/browser-latest.json"), "utf8"),
);
const directory = path.resolve(root, manifest.directory);
assert(directory.startsWith(path.join(root, ".tmp/runtime-delivery") + path.sep));
const runtime = JSON.parse(fs.readFileSync(path.resolve(root, manifest.report), "utf8"));
const requests = fs
  .readFileSync(path.join(directory, "requests.jsonl"), "utf8")
  .trim()
  .split("\n")
  .filter(Boolean)
  .map((line) => JSON.parse(line));
const observed =
  process.argv[2] === undefined
    ? undefined
    : JSON.parse(fs.readFileSync(path.resolve(root, process.argv[2]), "utf8"));
const groups = new Map();
for (const entry of manifest.cases) {
  const build = runtime.workloads.find(
    (row) => row.kind === entry.kind && row.names.join("+") === entry.names.join("+"),
  );
  assert(build, `Missing build for ${entry.id}`);
  const rows = requests.filter((row) => row.caseID === entry.id);
  const sdk = rows.filter((row) => row.category === "sdk");
  const actual = sdk.map((row) => row.path.split("/").slice(3).join("/")).sort();
  assert.deepEqual(
    actual,
    [...build.native.inventory].sort(),
    `Incomplete, duplicate or unexpected SDK requests in ${entry.id}`,
  );
  const encodings = [...new Set(sdk.map((row) => row.encoding))];
  assert.equal(encodings.length, 1);
  const field = { br: "brotli5", gzip: "gzip6", identity: "raw" }[encodings[0]];
  assert(field);
  const body = sdk.reduce((sum, row) => sum + row.bodyBytes, 0);
  assert.equal(body, build.native[field], `HTTP body and static inventory disagree in ${entry.id}`);
  assert.equal(
    rows.filter((row) => row.category === "api").length,
    entry.names.length,
    `Not all API requests reached the local server in ${entry.id}`,
  );
  const key = entry.kind + ":" + entry.names.join("+");
  const group = groups.get(key) ?? {
    kind: entry.kind,
    names: entry.names,
    cases: 0,
    encoding: encodings[0],
    bodySamples: [],
    requestSamples: [],
  };
  group.cases++;
  group.bodySamples.push(body);
  group.requestSamples.push(sdk.length);
  groups.set(key, group);
}
const median = (values) => {
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2;
};
if (observed !== undefined) {
  assert.equal(observed.runID, manifest.runID);
  assert.equal(observed.browserRun, manifest.browserRun);
  assert.equal(observed.cases, manifest.cases.length);
  assert.equal(observed.passed, manifest.cases.length);
  for (const group of groups.values()) {
    const found = observed.results.find(
      (row) => row.kind === group.kind && row.names.join("+") === group.names.join("+"),
    );
    assert(found, "Missing browser observation group");
    assert.equal(found.passed, group.cases);
    assert.deepEqual(found.bodyBytes, group.bodySamples);
    assert.deepEqual(found.requests, group.requestSamples);
    group.elapsedSamplesMS = found.elapsedMS;
    group.medianElapsedMS = median(found.elapsedMS);
    group.madElapsedMS = median(
      found.elapsedMS.map((value) => Math.abs(value - group.medianElapsedMS)),
    );
  }
}
const comparisons = [];
for (const baseline of [...groups.values()].filter((group) => group.kind === "baseline")) {
  const candidate = groups.get("candidate:" + baseline.names.join("+"));
  assert(candidate);
  const baselineBytes = median(baseline.bodySamples),
    candidateBytes = median(candidate.bodySamples);
  assert(candidateBytes < baselineBytes, `${baseline.names.join("+")} native body regression`);
  comparisons.push({
    names: baseline.names,
    pairs: baseline.cases,
    baselineBytes,
    candidateBytes,
    reductionPercent: (1 - candidateBytes / baselineBytes) * 100,
    baselineRequests: median(baseline.requestSamples),
    candidateRequests: median(candidate.requestSamples),
    baselineMedianMS: baseline.medianElapsedMS,
    candidateMedianMS: candidate.medianElapsedMS,
  });
}
const report = {
  runID: manifest.runID,
  browserRun: manifest.browserRun,
  status: observed ? "pass" : "http-inventory-pass-browser-observation-required",
  cases: manifest.cases.length,
  excludedWorkloads: manifest.excludedWorkloads ?? [],
  scope:
    "Actual native HTTP runtime/module bodies cross-checked with the compiled inventory. Browser semantic/Resource Timing checks require an explicitly captured console summary. Loopback timing is not a public latency SLA. This is not the complete operation-reference loader.",
  groups: [...groups.values()],
  comparisons,
};
fs.writeFileSync(
  path.join(directory, "browser-report.json"),
  JSON.stringify(report, null, 2) + "\n",
);
console.log(JSON.stringify({ ...report, groups: undefined }, null, 2));
