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
