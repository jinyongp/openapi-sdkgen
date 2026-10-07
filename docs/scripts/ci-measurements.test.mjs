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
  const { readCompatibilityResults } = await import("./build-compatibility-results.mjs");
  const source = resolve(import.meta.dirname, "../../test/compatibility");
  const directory = resolve(source, "ci");
  const ci = readCIMeasurements(directory);
  const data = readCompatibilityResults(source, { reportDirectory: directory, compressed: true });
  assert.equal(data.length, corpusNames.length);
  assert.ok(data.every(corpus => corpus.measurement.sourceCommit === ci.provenance.sourceCommit));
});
