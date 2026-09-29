// Audit real server delivery, browser execution and HTTP cache reuse separately.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const manifest = JSON.parse(
  fs.readFileSync(
    path.resolve(root, process.argv[3] ?? ".tmp/runtime-delivery/session-latest.json"),
    "utf8",
  ),
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
const median = (values) => {
  const sorted = [...values].sort((a, b) => a - b),
    mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2;
};
const expected = new Map();
for (const kind of ["baseline", "candidate"]) {
  const seen = new Set(),
    stages = [];
  for (const names of manifest.workloads) {
    const build = runtime.workloads.find(
      (row) => row.kind === kind && row.names.join("+") === names.join("+"),
    );
    assert(build);
    const additions = build.native.inventory.filter((name) => !seen.has(name)).sort();
    for (const name of additions) seen.add(name);
    stages.push({
      names,
      files: additions.length,
      body: additions.reduce((n, name) => n + manifest.assets[name].brotli5, 0),
      paths: additions,
    });
  }
  expected.set(kind, {
    stages,
    inventory: [...seen].sort(),
    bytes: stages.reduce((n, s) => n + s.body, 0),
  });
}
const groups = new Map();
for (const entry of manifest.cases) {
  const cold = entry.phase === "cold" || entry.phase === "new-url-version";
  const rows = requests.filter((row) => row.caseID === entry.id && row.category === "sdk");
  const inventory = expected.get(entry.kind);
  assert.deepEqual(
    rows.map((row) => row.asset).sort(),
    cold ? inventory.inventory : [],
    `HTTP cache/graph mismatch at ${entry.kind}:${entry.phase}:${entry.repetition}`,
  );
  for (const row of rows) {
    assert.equal(row.status, 200);
    assert.equal(row.method, "GET");
    assert.equal(row.bodyBytes, manifest.assets[row.asset].brotli5);
  }
  assert(
    !requests.some((row) => row.caseID === entry.id && row.path.startsWith("/api/")),
    "API requests must terminate at injected fetch",
  );
  const key = entry.kind + ":" + entry.phase;
  const group = groups.get(key) ?? {
    kind: entry.kind,
    phase: entry.phase,
    cases: 0,
    networkBytes: [],
    networkRequests: [],
  };
  group.cases++;
  group.networkBytes.push(rows.reduce((n, row) => n + row.bodyBytes, 0));
  group.networkRequests.push(rows.length);
  groups.set(key, group);
}
function verifyBrowserObservation(observed) {
  assert.equal(observed.runID, manifest.runID);
  assert.equal(observed.sessionRun, manifest.sessionRun);
  assert.equal(observed.cases, manifest.cases.length);
  assert.equal(observed.passed, manifest.cases.length);
  for (const group of groups.values()) {
    const found = observed.results.find(
      (row) => row.kind === group.kind && row.phase === group.phase,
    );
    assert(found);
    assert.equal(found.passed, group.cases);
    assert.equal(found.signatures.length, group.cases);
    assert.equal(found.elapsedMS.length, group.cases);
    assert.deepEqual(found.errors, []);
    const stages = expected.get(group.kind).stages;
    for (const signature of found.signatures) {
      const values = signature.split(",").map((stage) => stage.split("/").map(Number));
      assert.equal(values.length, stages.length);
      values.forEach(([files, body, transfer], index) => {
        assert.equal(files, stages[index].files, "Sequential module deduplication");
        assert.equal(body, stages[index].body, "Browser encoded body and static inventory");
        if (group.phase === "revisit" || group.phase === "retained-old-url")
          assert.equal(transfer, 0, "Cached response must not be counted as new transfer");
        else assert(transfer >= body, "Fresh response transfer must include its body");
      });
    }
    group.browserSignatures = found.signatures;
    group.elapsedMS = found.elapsedMS;
    group.medianMS = median(found.elapsedMS);
    group.madMS = median(found.elapsedMS.map((value) => Math.abs(value - group.medianMS)));
  }
}
let negativeControls = 0;
if (observed !== undefined) {
  for (const corrupt of [
    (value) => {
      value.runID = "different-run";
    },
    (value) => {
      value.passed--;
    },
    (value) => {
      value.results[0].signatures.pop();
    },
    (value) => {
      value.results[0].signatures[0] = "0/0/0";
    },
    (value) => {
      value.results[0].signatures[0] = value.results[0].signatures[0].replace(/[0-9]+$/, "0");
    },
    (value) => {
      const row = value.results.find((row) => row.phase === "revisit");
      row.signatures[0] = row.signatures[0].replace(/[0-9]+$/, "1");
    },
    (value) => {
      value.results[0].errors.push("application failure");
    },
  ]) {
    const invalid = structuredClone(observed);
    corrupt(invalid);
    assert.throws(
      () => verifyBrowserObservation(invalid),
      "Malformed browser evidence was accepted",
    );
    negativeControls++;
  }
  verifyBrowserObservation(observed);
}
const base = expected.get("baseline"),
  candidate = expected.get("candidate");
assert(candidate.bytes < base.bytes, "Cumulative mixed runtime regression");
const report = {
  runID: manifest.runID,
  sessionRun: manifest.sessionRun,
  status: observed ? "pass" : "http-inventory-pass-browser-observation-required",
  cases: manifest.cases.length,
  scope: manifest.scope,
  apiTransport: "injected-fetch",
  expected: Object.fromEntries(expected),
  groups: [...groups.values()],
  cumulative: { baseline: base.bytes, candidate: candidate.bytes },
  observedSource: observed?.source,
  negativeControls,
};
fs.writeFileSync(path.join(directory, "report.json"), JSON.stringify(report, null, 2) + "\n");
console.log(
  JSON.stringify(
    {
      ...report,
      expected: undefined,
      groups: report.groups.map((g) => ({
        ...g,
        browserSignatures: undefined,
        elapsedMS: undefined,
      })),
    },
    null,
    2,
  ),
);
