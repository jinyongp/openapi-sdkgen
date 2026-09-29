// Correlate independently observed browser console summaries with the HTTP log.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const [directoryArgument, observedArgument] = process.argv.slice(2);
assert(
  directoryArgument && observedArgument,
  "Pass the browser run directory and observed console JSON",
);
const directory = fs.realpathSync(path.resolve(root, directoryArgument));
assert(directory.startsWith(path.join(root, ".tmp/sdk-browser") + path.sep));
const observedFile = fs.realpathSync(path.resolve(root, observedArgument));
assert(observedFile.startsWith(directory + path.sep));
const manifestBytes = fs.readFileSync(path.join(directory, "manifest.json"));
const observedBytes = fs.readFileSync(observedFile);
const logBytes = fs.readFileSync(path.join(directory, "requests.jsonl"));
const manifest = JSON.parse(manifestBytes),
  observed = JSON.parse(observedBytes);
const requests = logBytes
  .toString()
  .trim()
  .split(/\r?\n/)
  .filter(Boolean)
  .map((line) => JSON.parse(line));
const hash = (bytes) => createHash("sha256").update(bytes).digest("hex");
const median = (values) => {
  const sorted = [...values].sort((a, b) => a - b);
  return sorted[Math.floor(sorted.length / 2)];
};
function expectedStages(entry, source) {
  const seen = new Set(),
    result = [];
  const add = (name, files) => {
    const fresh = files.filter((file) => !seen.has(file));
    for (const file of files) seen.add(file);
    result.push({
      name,
      files: fresh.length,
      body: fresh.reduce((sum, file) => sum + source.assets[file].brotli5, 0),
    });
  };
  add("initial", entry.kind === "baseline" ? source.baseline : source.bootstrap);
  for (const stage of source.stages)
    add(stage.name, entry.kind === "baseline" ? [] : stage.selected);
  return { stages: result, files: [...seen].sort() };
}
export function validateReport(m, o, records) {
  assert.equal(o.summary.runID, m.runID);
  assert.equal(o.summary.cases, m.cases.length);
  assert.equal(o.summary.passed, m.cases.length, "Browser cases did not all pass");
  assert.notEqual(m.versions.v1.identity, m.versions.v2.identity);
  const expectedGroups = new Map();
  for (const entry of m.cases) {
    const key = entry.kind + ":" + entry.phase;
    const group = expectedGroups.get(key) ?? [];
    group.push(entry);
    expectedGroups.set(key, group);
  }
  assert.equal(o.groups.length, expectedGroups.size);
  const seenGroups = new Set(),
    groups = [];
  for (const group of o.groups) {
    const key = group.kind + ":" + group.phase,
      entries = expectedGroups.get(key);
    assert(entries && !seenGroups.has(key), "Unknown or duplicate observation group");
    seenGroups.add(key);
    assert.equal(group.passed, entries.length);
    assert.deepEqual(group.errors, []);
    assert.equal(group.signatures.length, entries.length);
    assert.equal(group.elapsedMS.length, entries.length);
    assert(group.elapsedMS.every((value) => Number.isFinite(value) && value >= 0));
    const serverSamples = [];
    for (let index = 0; index < entries.length; index++) {
      const entry = entries[index],
        version = m.versions[entry.version];
      const rows = records.filter((row) => row.caseID === entry.id && row.category === "sdk");
      const negative = [
        "missing",
        "mime",
        "mixed",
        "disconnect",
        "cors-denied",
        "csp-denied",
      ].includes(entry.variant);
      if (negative) {
        const stage =
          entry.variant === "mixed"
            ? "IDENTITY"
            : ["cors-denied", "csp-denied"].includes(entry.variant)
              ? "null"
              : "MODULE_LOAD";
        assert.equal(group.signatures[index], "expected:" + stage);
        if (entry.variant === "missing")
          assert(
            rows.some((row) => row.status === 404),
            "Missing-module case never observed a missing module",
          );
        if (entry.variant === "disconnect")
          assert(
            rows.some((row) => row.disconnected === true),
            "Network-interruption case did not disconnect",
          );
        if (entry.variant === "csp-denied")
          assert.equal(rows.length, 0, "CSP did not block the remote script before serving");
        serverSamples.push({
          requests: rows.length,
          body: rows.reduce((sum, row) => sum + row.bodyBytes, 0),
        });
        continue;
      }
      const expected = expectedStages(entry, version);
      const signature = group.signatures[index]
        .split(",")
        .map((value) => value.split("/").map(Number));
      assert.equal(signature.length, expected.stages.length);
      const cached = ["revisit", "retained-old"].includes(entry.phase);
      for (let stage = 0; stage < signature.length; stage++) {
        const [files, body, transfer] = signature[stage],
          wanted = expected.stages[stage];
        assert([files, body, transfer].every((value) => Number.isFinite(value) && value >= 0));
        if (entry.variant !== "preload") {
          assert.equal(files, wanted.files, `Wrong observed files at ${key}/${wanted.name}`);
          if (!cached)
            assert.equal(body, wanted.body, `Wrong observed body at ${key}/${wanted.name}`);
          else
            assert(
              body === 0 || body === wanted.body,
              "Cached body metadata is neither zero nor the verified asset size",
            );
        }
        if (cached) assert.equal(transfer, 0, "Cache phase transferred module bytes");
      }
      if (cached)
        assert.equal(rows.length, 0, "Server received SDK requests during a retained cache phase");
      else {
        const names = rows.map((row) => row.asset).sort();
        assert.equal(new Set(names).size, names.length, "Duplicate asset request");
        const expectedFiles =
          entry.variant === "preload"
            ? [...expected.files, "browser/all.js"].sort()
            : expected.files;
        assert.deepEqual(names, expectedFiles, `Server asset graph differs in ${key}`);
        for (const row of rows) {
          assert.equal(row.status, 200);
          assert.equal(row.bodyBytes, version.assets[row.asset].brotli5);
          assert.equal(row.method, "GET");
        }
      }
      serverSamples.push({
        requests: rows.length,
        body: rows.reduce((sum, row) => sum + row.bodyBytes, 0),
      });
    }
    const center = median(group.elapsedMS);
    groups.push({
      ...group,
      http: serverSamples,
      medianMS: center,
      madMS: median(group.elapsedMS.map((value) => Math.abs(value - center))),
    });
  }
  assert(
    records.every((row) => row.method === "GET"),
    "Harness sent an unexpected HTTP method",
  );
  for (const phase of ["cold", "new-generation"]) {
    const selected = groups.find((row) => row.kind === "selected" && row.phase === phase),
      full = groups.find((row) => row.kind === "baseline" && row.phase === phase);
    for (let i = 0; i < selected.http.length; i++)
      assert(
        selected.http[i].body < full.http[i].body,
        "Selected session did not reduce actual HTTP bytes",
      );
  }
  return groups;
}
const groups = validateReport(manifest, observed, requests);
assert.equal(observed.https?.id, manifest.tlsCase.id, "Missing independent HTTPS observation");
assert.equal(observed.https.pass, true);
assert.equal(observed.https.protocol, "https:");
assert.equal(observed.https.secureContext, true);
assert.equal(new URL(observed.https.origin).protocol, "https:");
const tlsExpected = expectedStages(manifest.tlsCase, manifest.versions.v1);
assert.equal(observed.https.files, tlsExpected.files.length);
assert.equal(observed.https.stages, tlsExpected.stages.length);
const tlsRequests = requests.filter(
  (row) => row.caseID === manifest.tlsCase.id && row.category === "sdk",
);
assert.deepEqual(tlsRequests.map((row) => row.asset).sort(), tlsExpected.files);
assert(tlsRequests.every((row) => row.method === "GET" && row.status === 200));
const clone = (value) => JSON.parse(JSON.stringify(value));
const negatives = [];
const reject = (name, mutate) => {
  const m = clone(manifest),
    o = clone(observed),
    r = clone(requests);
  mutate(m, o, r);
  assert.throws(() => validateReport(m, o, r), name);
  negatives.push(name);
};
reject("wrong-run", (_m, o) => {
  o.summary.runID = "wrong";
});
reject("missing-group", (_m, o) => {
  o.groups.pop();
});
reject("failed-case", (_m, o) => {
  o.summary.passed--;
});
reject("same-generation", (m) => {
  m.versions.v2.identity = m.versions.v1.identity;
});
reject("corrupt-byte-log", (_m, _o, r) => {
  r.find((row) => row.category === "sdk" && row.status === 200).bodyBytes++;
});
reject("cache-network-regression", (m, _o, r) => {
  const entry = m.cases.find((row) => row.phase === "revisit");
  r.push({ ...r.find((row) => row.category === "sdk" && row.status === 200), caseID: entry.id });
});
reject("duplicate-observation", (_m, o) => {
  o.groups[1] = clone(o.groups[0]);
});
const report = {
  status: "pass",
  runID: manifest.runID,
  cases: manifest.cases.length,
  groups,
  negativeControls: negatives,
  manifestSHA256: hash(manifestBytes),
  observedSHA256: hash(observedBytes),
  requestLogSHA256: hash(logBytes),
  https: {
    origin: observed.https.origin,
    pass: true,
    requests: tlsRequests.length,
    scope:
      "Actual HTTPS module execution through a temporary preview; not a public CDN performance benchmark.",
  },
  scope: manifest.scope,
};
fs.writeFileSync(path.join(directory, "report.json"), JSON.stringify(report, null, 2) + "\n");
console.log(
  JSON.stringify({
    status: report.status,
    runID: report.runID,
    cases: report.cases,
    negativeControls: negatives.length,
    groups: groups.map((row) => ({
      kind: row.kind,
      phase: row.phase,
      passed: row.passed,
      http: row.http,
      medianMS: row.medianMS,
      madMS: row.madMS,
    })),
    report: path.relative(root, path.join(directory, "report.json")),
  }),
);
