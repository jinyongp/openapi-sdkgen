import assert from "node:assert/strict";
import { copyFileSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { basename, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { corpusNames, publishCompatibilityDocuments, readCompatibilityResults } from "./build-compatibility-results.mjs";

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

test("server-required documents publish the successful generation counts and receiving code", () => {
  const data = readCompatibilityResults(fileURLToPath(sourceDirectory));
  const holdout = data.find((corpus) => corpus.id === "holdout");
  assert.equal(holdout.generatedOperations, 2829);
  const listenNotes = holdout.results.find((document) => document.id === "listennotes.com");
  assert.equal(listenNotes.generatedOperations, 24);
  assert.deepEqual(listenNotes.receiving, ["document.webhooks"]);
  const uniCourt = holdout.results.find((document) => document.id === "unicourt.com");
  assert.equal(uniCourt.generatedOperations, 158);
  assert.deepEqual(uniCourt.receiving, ["operation.callbacks"]);
  const modern = data.find((corpus) => corpus.id === "modern");
  assert.equal(modern.generatedOperations, 136);
  const webhookOnly = modern.results.find((document) => document.id === "server-webhook");
  assert.equal(webhookOnly.generatedOperations, 0);
  assert.deepEqual(webhookOnly.receiving, ["document.webhooks"]);
});

test("server success requires recorded generation, typecheck and emission evidence", (t) => {
  for (const field of ["generation", "typecheck", "operationEmission"]) {
    const directory = fixture(t);
    change(directory, "holdout-results.json", (report) => {
      const profile = report.documents.find((document) => document.id === "listennotes.com").supportProfiles[0];
      if (field === "operationEmission") profile[field].available = false;
      else profile[field].status = "fail";
    });
    assert.throws(() => readCompatibilityResults(directory), /server success without/);
  }
});

test("an unsuccessful generation shows no call count or receiving code", (t) => {
  const directory = fixture(t);
  change(directory, "holdout-results.json", (report) => {
    const document = report.documents.find((document) => document.id === "listennotes.com");
    document.supportProfiles[0].success = false;
    document.supportProfiles[0].generation.status = "fail";
    document.capabilityAdjustedSuccess = false;
    report.overall.capabilityAdjustedDocuments--;
  });
  const document = readCompatibilityResults(directory)[0].results.find((document) => document.id === "listennotes.com");
  assert.equal(document.generatedOperations, null);
  assert.deepEqual(document.receiving, []);
});

test("all document links open the measured source rather than the standard or provider homepage", () => {
  const data = readCompatibilityResults(fileURLToPath(sourceDirectory));
  assert.equal(data.flatMap((corpus) => corpus.results).length, 32);
  for (const document of data[0].results) {
    assert.match(document.sourceUrl, /^https:\/\/raw\.githubusercontent\.com\/APIs-guru\/openapi-directory\/[a-f0-9]{40}\/APIs\/.+\/openapi\.(yaml|json)$/);
  }
  for (const corpus of data.slice(1)) for (const document of corpus.results) {
    assert.match(document.sourceUrl, /^\/compatibility-results\/documents\/.+\.json$/);
    assert.ok(document.sourceUrl.includes(`/${document.id}/`));
  }
});

test("published multi-file examples retain the exact source and relative reference targets", (t) => {
  const directory = fixture(t);
  publishCompatibilityDocuments(fileURLToPath(sourceDirectory), directory);
  for (const filename of ["root.json", "source.json", "target.json"]) {
    const published = readFileSync(resolve(directory, "documents/external-link", filename));
    const original = readFileSync(new URL(`../fixtures/support-gaps/links/${filename}`, sourceDirectory));
    assert.ok(published.equals(original));
  }
  for (const [id,filename] of [["zenith-merchant-v2","zenith-merchant-v2.json"],["resend-3.1","resend.json"]]) {
    assert.equal(typeof JSON.parse(readFileSync(resolve(directory, "documents", id, filename))).openapi, "string");
  }
});

test("changed source bytes cannot be published as the measured document", (t) => {
  const directory = fixture(t);
  const manifest = JSON.parse(readFileSync(resolve(directory, "production32.json")));
  const first = manifest.corpora[0];
  const original = first.input;
  first.input = `${basename(directory)}/changed.json`;
  writeFileSync(resolve(directory, "changed.json"), readFileSync(new URL(`../${original}`, sourceDirectory)) + "\n");
  writeFileSync(resolve(directory, "production32.json"), JSON.stringify(manifest));
  assert.throws(() => publishCompatibilityDocuments(directory, directory), /published document hash mismatch/);
});

test("generation time follows the same successful server profile as API calls", (t) => {
  const directory = fixture(t);
  change(directory, "holdout-results.json", (report) => {
    const document = report.documents.find((document) => document.id === "listennotes.com");
    document.generation.durationMillis = 999;
    document.supportProfiles[0].generation.durationMillis = 7.25;
  });
  const document = readCompatibilityResults(directory)[0].results.find((document) => document.id === "listennotes.com");
  assert.equal(document.generatedOperations, 24);
  assert.equal(document.generationDurationMillis, 7.25);
});

test("missing generation timings remain unavailable, including the summary total", (t) => {
  const directory = fixture(t);
  change(directory, "holdout-results.json", (report) => {
    delete report.measurement;
    for (const document of report.documents) {
      delete document.generation.durationMillis;
      for (const profile of document.supportProfiles ?? []) delete profile.generation.durationMillis;
    }
  });
  const corpus = readCompatibilityResults(directory)[0];
  assert.equal(corpus.generationDurationMillis, null);
  assert.ok(corpus.results.every((document) => document.generationDurationMillis === null));
});

test("invalid or differently scoped durations cannot be published as generation time", (t) => {
  for (const invalid of ["negative", "scope"]) {
    const directory = fixture(t);
    change(directory, "holdout-results.json", (report) => {
      const document = report.documents.find((document) => document.documentSuccess);
      if (invalid === "negative") document.generation.durationMillis = -1;
      else report.measurement.generationScope = "compile-only";
    });
    assert.throws(() => readCompatibilityResults(directory), /generation timing is invalid/);
  }
});
