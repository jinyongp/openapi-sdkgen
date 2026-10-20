import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import { test } from "node:test";
import { gunzipSync, gzipSync } from "node:zlib";
import { corpusNames, graphNames, exportCIMeasurements, readCIMeasurements } from "./ci-measurements.mjs";

function fixture(t) {
  const directory = mkdtempSync(resolve(tmpdir(), "sdkgen-ci-measurements-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  const options = { input: directory, output: resolve(directory, "public"),
    sourceCommit: "a".repeat(40), runUrl: "https://github.com/jinyongp/openapi-sdkgen/actions/runs/123",
    runner: "ubuntu-24.04" };
  for (const id of [...corpusNames, ...graphNames]) {
    writeFileSync(resolve(directory, `${id}-results.json`), JSON.stringify({
      measurement: { sourceCommit: options.sourceCommit, sourceDirty: false,
        measuredAt: "2026-10-02T00:00:00Z", cpu: "private-cpu", hostname: "private-host" },
      environment: { totalMemoryBytes: 123456, release: "private-kernel" },
      documents: [{ generation: { durationMillis: 123, artifactBytes: 456 } }],
    }));
  }
  return options;
}

test("CI export preserves measured values and removes machine identifiers", t => {
  const options = fixture(t);
  exportCIMeasurements(options);
  const data = readCIMeasurements(options.output);
  for (const { report } of Object.values(data.reports)) {
    assert.deepEqual(report.documents, [{ generation: { durationMillis: 123, artifactBytes: 456 } }]);
    assert.equal(report.measurement.sourceCommit, options.sourceCommit);
    assert.equal(report.measurement.cpu, undefined);
    assert.equal(report.measurement.hostname, undefined);
    assert.deepEqual(report.environment, {});
  }
});

test("dirty or mismatched source measurements cannot become CI evidence", t => {
  for (const change of [report => { report.measurement.sourceDirty = true; },
    report => { report.measurement.sourceCommit = "b".repeat(40); },
    report => { report.documentMeasurements = { other: { ...report.measurement, sourceCommit: "b".repeat(40) } }; }]) {
    const options = fixture(t);
    const path = resolve(options.input, "holdout-results.json");
    const report = JSON.parse(readFileSync(path));
    change(report);
    writeFileSync(path, JSON.stringify(report));
    assert.throws(() => exportCIMeasurements(options), /benchmark commit/);
  }
});

test("later validation retains the measured source and records its own Actions provenance", t => {
  const options = { ...fixture(t), validationRunUrl: "https://github.com/jinyongp/openapi-sdkgen/actions/runs/456", validationSourceCommit: "b".repeat(40) };
  exportCIMeasurements(options);
  const { provenance } = readCIMeasurements(options.output);
  assert.equal(provenance.sourceCommit, options.sourceCommit);
  assert.equal(provenance.validationSourceCommit, options.validationSourceCommit);
  assert.equal(provenance.validationRunUrl, options.validationRunUrl);
  assert.throws(() => exportCIMeasurements({ ...options, validationSourceCommit: undefined }), /validation provenance/);
  assert.throws(() => exportCIMeasurements({ ...options, validationRunUrl: "https://example.test/456" }), /validation provenance/);
});

test("CI snapshots reject altered reports and invalid run provenance", t => {
  const options = fixture(t);
  exportCIMeasurements(options);
  const path = resolve(options.output, "regression-results.json.gz");
  const report = JSON.parse(gunzipSync(readFileSync(path)));
  report.documents[0].generation.durationMillis++;
  writeFileSync(path, gzipSync(JSON.stringify(report)));
  assert.throws(() => readCIMeasurements(options.output), /integrity/);
  for (const override of [{ runUrl: "https://example.test/run/123" },
    { sourceCommit: "main" }, { runner: "personal-workstation" }]) {
    assert.throws(() => exportCIMeasurements({ ...fixture(t), ...override }), /provenance/);
  }
});

test("published CI snapshots also pass document and input integrity validation", async () => {
  const { readCompatibilityResults, readGraphCIMeasurements } = await import("./build-compatibility-results.mjs");
  const source = resolve(import.meta.dirname, "../../test/compatibility");
  const directory = resolve(source, "ci");
  const ci = readCIMeasurements(directory);
  const data = readCompatibilityResults(source, { reportDirectory: directory, compressed: true });
  assert.equal(data.length, corpusNames.length);
  assert.ok(data.every(corpus => corpus.measurement.sourceCommit === ci.provenance.sourceCommit));
  if (ci.reports["graph-full"]) {
    const graph = readGraphCIMeasurements(source, ci);
    assert.equal(graph.measurement.sourceCommit, ci.provenance.sourceCommit);
    assert.equal(graph.full.typecheck.status, "not-run");
    assert.equal(graph.selected.typecheck.status, "not-run");
  }
});

test("Graph CI publication binds both scopes to the pinned input and skips verification", async () => {
  const { readGraphCIMeasurements } = await import("./build-compatibility-results.mjs");
  const directory = resolve(import.meta.dirname, "../../test/compatibility");
  const full = JSON.parse(readFileSync(resolve(directory, "regression-results.json")));
  const baseline = JSON.parse(readFileSync(resolve(directory, "graph-full-baseline.json")));
  const item = full.documents.find(item => item.id === "microsoft-graph-beta");
  Object.assign(item, baseline.document, { generationAddons: [], documentSuccess: false,
    capabilityAdjustedSuccess: false, typecheck: { status: "not-run" } });
  full.documents = [item];
  const selected = JSON.parse(readFileSync(resolve(directory, "graph-selected-results.json")));
  const ci = { provenance: { runUrl: "https://github.com/jinyongp/openapi-sdkgen/actions/runs/123" }, reports: {
    "graph-full": { report: full, bytes: Buffer.from(JSON.stringify(full)) },
    "graph-selected": { report: selected, bytes: Buffer.from(JSON.stringify(selected)) },
  } };
  const result = readGraphCIMeasurements(directory, ci);
  assert.equal(result.selected.operationEmission.count, 9);
  assert.equal(result.full.operationEmission.count, item.operationRetention.total);
  assert.equal(result.ciRunUrl, ci.provenance.runUrl);
  for (const change of [item => { item.inputSha256 = "wrong"; },
    item => { item.typecheck.status = "pass"; }, item => { item.documentSuccess = true; }]) {
    const invalid = structuredClone(ci);
    change(invalid.reports["graph-full"].report.documents[0]);
    assert.throws(() => readGraphCIMeasurements(directory, invalid), /Graph full CI/);
  }
  const missing = { ...ci, reports: { "graph-selected": ci.reports["graph-selected"] } };
  assert.throws(() => readGraphCIMeasurements(directory, missing), /both generation scopes/);
});

test("independent Go evidence is optional and bound to the same Actions commit", t => {
  const options = fixture(t);
  const path = resolve(options.input, "golang-results.json");
  const report = { measurement: { sourceCommit: options.sourceCommit, sourceDirty: false, measuredAt: "2026-10-20T00:00:00Z" },
    documents: [{ id: "provider", pass: false, failure: "generation blocked" }] };
  writeFileSync(path, JSON.stringify(report));
  exportCIMeasurements(options);
  const ci = readCIMeasurements(options.output);
  assert.deepEqual(ci.reports.golang.report.documents, report.documents);
  report.measurement.sourceCommit = "f".repeat(40);
  writeFileSync(path, JSON.stringify(report));
  assert.throws(() => exportCIMeasurements(options), /benchmark commit/);
});

test("Go publication keeps failed outcomes and rejects missing, stale or altered identities", async () => {
  const { readGoCIMeasurements } = await import("./build-compatibility-results.mjs");
  const directory = resolve(import.meta.dirname, "../../test/compatibility");
  const sourceCommit = "a".repeat(40);
  const documents = corpusNames.flatMap(corpus => JSON.parse(readFileSync(resolve(directory, `${corpus}.json`))).corpora
    .filter(entry => entry.id !== "microsoft-graph-beta").map(entry => ({ corpus, id: entry.id, input: entry.input, inputSha256: entry.sha256 ?? "c".repeat(64),
      ...(!entry.sha256 ? { inputPin: { kind: "git-blob", value: entry.gitBlob, sha256: "c".repeat(64) } } : {}),
      sourceCommit, sourceDirty: false, verificationRequested: true, mode: "golang-generation", addons: ["server"], pass: true,
      golang: { blocked: false, compilation: { status: "pass" } },
      ...(entry.preparation ? { inputPreparation: { ...entry.preparation, sourceSha256: entry.sha256 ?? "c".repeat(64), contractsVerified: true } } : {}),
    })));
  const ci = { provenance: { sourceCommit, runUrl: "https://github.com/jinyongp/openapi-sdkgen/actions/runs/123" }, reports: { golang: { report: { documents } } } };
  documents[0].golang.inbound = [{ identity: "#/webhooks/notice/post" }];
  const published = readGoCIMeasurements(directory, ci);
  assert.equal(published.documents.length, documents.length);
  assert.equal(published.documents[0].routes, 0);
  assert.equal(published.documents[0].handlers, 1);
  const blocked = structuredClone(ci);
  Object.assign(blocked.reports.golang.report.documents[0], { pass: false, failure: "blocked", golang: { blocked: true, generationError: "invalid pattern" } });
  assert.equal(readGoCIMeasurements(directory, blocked).documents[0].compilation, "blocked");
  const unavailable = structuredClone(ci);
  Object.assign(unavailable.reports.golang.report.documents[0], {
    pass: false, failure: "compiler evidence unavailable", generationCheckpoint: true,
    compilerEvidenceUnavailable: { runId: 123, jobId: 456, jobConclusion: "failure", sourceCommit },
    golang: { blocked: false, compilation: { status: "unavailable", detail: "No compiler result was recovered." } },
  });
  assert.equal(readGoCIMeasurements(directory, unavailable).documents[0].compilation, "unavailable");
  for (const change of [{ runId: 999 }, { sourceCommit: "f".repeat(40) }, { jobConclusion: "success" }]) {
    const invalid = structuredClone(unavailable);
    Object.assign(invalid.reports.golang.report.documents[0].compilerEvidenceUnavailable, change);
    assert.throws(() => readGoCIMeasurements(directory, invalid), /Go compiler evidence/);
  }
  for (const mutate of [list => list.pop(), list => list.push(list[0]), list => { list[0].inputSha256 = "bad"; },
    list => { list[0].sourceDirty = true; }, list => { list[0].verificationRequested = false; },
    list => { list.find(item => item.inputPreparation).inputPreparation.contractsVerified = false; }]) {
    const invalid = structuredClone(ci);
    mutate(invalid.reports.golang.report.documents);
    assert.throws(() => readGoCIMeasurements(directory, invalid), /Go measurement/);
  }
  assert.equal(readGoCIMeasurements(directory, { reports: {} }), null);
});
