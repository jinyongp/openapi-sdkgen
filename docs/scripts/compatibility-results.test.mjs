import assert from "node:assert/strict";
import { copyFileSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import { test } from "node:test";
import { corpusNames, readCompatibilityResults } from "./build-compatibility-results.mjs";

const sourceDirectory = new URL("../../test/compatibility/", import.meta.url);

function fixture(t) {
  const directory = mkdtempSync(resolve(tmpdir(), "sdkgen-docs-results-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  for (const id of corpusNames) {
    for (const name of [`${id}.json`, `${id}-results.json`]) {
      copyFileSync(new URL(name, sourceDirectory), resolve(directory, name));
    }
  }
  return directory;
}

function change(directory, name, mutate) {
  const path = resolve(directory, name);
  const data = JSON.parse(readFileSync(path));
  mutate(data);
  writeFileSync(path, JSON.stringify(data));
}

test("a changed selection cannot retain a previous measurement", (t) => {
  const directory = fixture(t);
  change(directory, "holdout.json", (manifest) => manifest.corpora.pop());
  assert.throws(() => readCompatibilityResults(directory), /report\/manifest hash mismatch/);
});

test("a stale aggregate cannot publish a higher emitted-operation count", (t) => {
  const directory = fixture(t);
  change(directory, "holdout-results.json", (report) => report.overall.operationEmission.count++);
  assert.throws(() => readCompatibilityResults(directory), /summary does not match/);
});

test("a result for a different input cannot inherit the pinned document identity", (t) => {
  const directory = fixture(t);
  change(directory, "production32-results.json", (report) => report.documents[0].inputSha256 = "0".repeat(64));
  assert.throws(() => readCompatibilityResults(directory), /document provenance mismatch/);
});

test("a report without emission measurement cannot publish zero as measured coverage", (t) => {
  const directory = fixture(t);
  change(directory, "modern-results.json", (report) => report.schemaVersion = 1);
  assert.throws(() => readCompatibilityResults(directory), /emission-aware report is required/);
});
