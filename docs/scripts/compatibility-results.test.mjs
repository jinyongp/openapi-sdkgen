import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { copyFileSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { basename, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { corpusNames, publishCompatibilityDocuments, readCompatibilityResults, readGraphSelection, readMetadataComparison } from "./build-compatibility-results.mjs";

const sourceDirectory = new URL("../../test/compatibility/", import.meta.url);

function fixture(t) {
  const directory = mkdtempSync(resolve(tmpdir(), "sdkgen-docs-results-"));
  t.after(() => rmSync(directory, { recursive: true, force: true }));
  for (const id of corpusNames) {
    for (const name of [`${id}.json`, `${id}-results.json`]) {
      copyFileSync(new URL(name, sourceDirectory), resolve(directory, name));
    }
  }
  mkdirSync(resolve(directory, "selections"));
  for (const name of ["graph-selected-results.json", "graph-selected-ci-results.json", "graph-metadata-results.json", "metadata-comparison-results.json", "selections/microsoft-graph-beta.toml", "selections/microsoft-graph-beta.mjs"]) {
    copyFileSync(new URL(name, sourceDirectory), resolve(directory, name));
  }
  return directory;
}

test("Graph selection publishes separate measured evidence and rejects stale or inflated claims", (t) => {
  const directory = fixture(t);
  const data = readGraphSelection(directory);
  assert.equal(data.full.operationEmission.count, 29581);
  assert.equal(data.full.generation.durationMillis, 276562.598521);
  assert.equal(data.selected.operationEmission.count, 9);
  assert.equal(data.selected.generationSelection.runtime.status, "pass");
  const ci = readGraphSelection(directory, "graph-selected-ci-results.json");
  assert.deepEqual(ci.selected.generationSelection.requested, data.selected.generationSelection.requested);
  assert.equal(ci.selected.generationSelection.runtime.status, "pass");
  // Each environment retains the version that produced its own measurement.
  change(directory, "graph-selected-ci-results.json", report => report.measurement.sourceCommit = "1".repeat(40));
  assert.equal(readGraphSelection(directory, "graph-selected-ci-results.json").measurement.sourceCommit, "1".repeat(40));
  assert.equal(readGraphSelection(directory).measurement.sourceCommit, data.measurement.sourceCommit);
  for (const mutate of [
    report => report.documents[0].generationScope = "full",
    report => report.documents[0].generationSelection.fixtureSha256 = "wrong",
    report => report.documents[0].generationSelection.runtime.status = "fail",
    report => report.documents[0].operationEmission.count++,
    report => report.documents[0].generationSelection.excludedOperations++,
    report => report.measurement.sourceDirty = true,
    report => {
      report.documents[0].generationSelection.routes.pop();
      report.documents[0].generationSelection.requested.routes.pop();
      report.documents[0].operationEmission.count--;
      report.documents[0].generationSelection.excludedOperations++;
    },
  ]) {
    const directory = fixture(t);
    change(directory, "graph-selected-results.json", mutate);
    assert.throws(() => readGraphSelection(directory), /Graph selection/);
  }
});

test("metadata comparisons retain matched source, settings, API and schema scope", (t) => {
  const directory = fixture(t);
  const comparison = readMetadataComparison(directory);
  assert.equal(comparison.cases.length, 4);
  assert.ok(comparison.cases.every(item => item.default.generation.artifactBytes < item.metadata.generation.artifactBytes));
  for (const mutate of [
    data => data.cases[0].default.generationAddons.push("metadata"),
    data => data.cases[0].metadata.measurement.sourceCommit = "0".repeat(40),
    data => data.cases[1].default.generation.schemaArtifactCount--,
    data => data.cases[1].default.generation.artifactBytes--,
    data => data.cases[2].default.selection.routes.pop(),
    data => data.cases[2].inputSha256 = "0".repeat(64),
    data => data.cases[3].metadata.typecheck.status = "fail",
  ]) {
    const directory = fixture(t);
    change(directory, "metadata-comparison-results.json", mutate);
    assert.throws(() => readMetadataComparison(directory), /Metadata comparison/);
  }
  change(directory, "graph-selected-results.json", data => delete data.documents[0].generationAddons);
  assert.throws(() => readGraphSelection(directory), /metadata setting/);
});

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

test("an internal generation failure publishes unavailable counts and times while other inputs remain visible", (t) => {
  const directory = fixture(t);
  change(directory, "holdout-results.json", (report) => {
    const document = report.documents.find((document) => document.documentSuccess);
    report.overall.successfulDocuments--;
    report.overall.capabilityAdjustedDocuments--;
    report.overall.operationEmission.availableDocuments--;
    report.overall.operationEmission.count -= document.operationEmission.count;
    report.overall.operationEmission.operationOmissions -= document.operationEmission.operationOmissions;
    report.overall.operationEmission.helperOmissions -= document.operationEmission.helperOmissions;
    document.documentSuccess = false;
    document.capabilityAdjustedSuccess = false;
    document.generation = { status: "fail", detail: "internal target preparation failure" };
    document.typecheck = { status: "not-run" };
    document.operationEmission = { available: false, count: 0, operationOmissions: 0, helperOmissions: 0 };
  });
  const corpus = readCompatibilityResults(directory).find((corpus) => corpus.id === "holdout");
  assert.equal(corpus.results.length, 20);
  const failed = corpus.results.find((document) => !document.adjustedSuccess);
  assert.equal(failed.generatedOperations, null);
  assert.equal(failed.generationDurationMillis, null);
  assert.deepEqual(failed.receiving, []);
  assert.ok(corpus.results.some((document) => document.adjustedSuccess && document.generatedOperations > 0));
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
  const document = readCompatibilityResults(directory).find((corpus) => corpus.id === "holdout").results.find((document) => document.id === "listennotes.com");
  assert.equal(document.generatedOperations, null);
  assert.deepEqual(document.receiving, []);
});

test("generated SDK counts and timings remain visible when typechecking does not pass", (t) => {
  const directory = fixture(t);
  let expectedCount, expectedTime;
  change(directory, "holdout-results.json", (report) => {
    const document = report.documents.find((document) => document.documentSuccess);
    expectedCount = document.operationEmission.count;
    expectedTime = document.generation.durationMillis;
    document.documentSuccess = false;
    document.capabilityAdjustedSuccess = false;
    document.typecheck = { status: "timeout" };
    report.overall.successfulDocuments--;
    report.overall.capabilityAdjustedDocuments--;
  });
  const corpus = readCompatibilityResults(directory).find((corpus) => corpus.id === "holdout");
  const document = corpus.results.find((document) => document.clientGenerated && !document.defaultSuccess);
  assert.equal(document.generatedOperations, expectedCount);
  assert.equal(document.generationDurationMillis, expectedTime);
  assert.equal(corpus.generatedOperations, 2829);
  assert.equal(corpus.adjustedSuccess, 19);
  assert.ok(corpus.generationDurationMillis >= expectedTime);
});

test("generated server handlers remain visible when server typechecking does not pass", (t) => {
  const directory = fixture(t);
  change(directory, "holdout-results.json", (report) => {
    const document = report.documents.find((document) => document.id === "listennotes.com");
    document.supportProfiles[0].success = false;
    document.supportProfiles[0].typecheck.status = "timeout";
    document.capabilityAdjustedSuccess = false;
    report.overall.capabilityAdjustedDocuments--;
  });
  const document = readCompatibilityResults(directory).find((corpus) => corpus.id === "holdout").results.find((document) => document.id === "listennotes.com");
  assert.equal(document.adjustedSuccess, false);
  assert.equal(document.serverGenerated, true);
  assert.equal(document.generatedOperations, 24);
  assert.deepEqual(document.receiving, ["document.webhooks"]);
});

test("all document links open the measured source rather than the standard or provider homepage", () => {
  const data = readCompatibilityResults(fileURLToPath(sourceDirectory));
  assert.equal(data.flatMap((corpus) => corpus.results).length, 39);
  const regression = data.find((corpus) => corpus.id === "regression");
  const manifest = JSON.parse(readFileSync(new URL("regression.json", sourceDirectory)));
  assert.equal(data[0].id, "regression");
  assert.deepEqual(regression.results.map((document) => document.name), [
    "GitHub", "Stripe", "Cloudflare", "GitLab", "Microsoft Graph beta", "DigitalOcean", "Twilio",
  ]);
  for (const document of regression.results) {
    const entry = manifest.corpora.find((entry) => entry.id === document.id);
    assert.equal(document.sourceUrl, entry.sourceUrl);
    assert.ok(document.sourceUrl.includes(`/${entry.revision}/`));
  }
  for (const document of data.find((corpus) => corpus.id === "holdout").results) {
    assert.match(document.sourceUrl, /^https:\/\/raw\.githubusercontent\.com\/APIs-guru\/openapi-directory\/[a-f0-9]{40}\/APIs\/.+\/openapi\.(yaml|json)$/);
  }
  for (const corpus of data.filter((corpus) => ["modern", "production32"].includes(corpus.id))) for (const document of corpus.results) {
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

test("upstream links require the measured input hash and an immutable HTTPS revision", (t) => {
  for (const invalid of ["revision", "url", "hash", "mode"]) {
    const directory = fixture(t);
    change(directory, "regression.json", (manifest) => {
      if (invalid === "revision") manifest.corpora[0].revision = "main";
      if (invalid === "url") manifest.corpora[0].sourceUrl = manifest.corpora[0].sourceUrl.replace("https:", "http:");
      if (invalid === "hash") manifest.corpora[0].sha256 = "";
      if (invalid === "mode") manifest.publication.documents = "unknown";
    });
    const digest = createHash("sha256").update(readFileSync(resolve(directory, "regression.json"))).digest("hex");
    change(directory, "regression-results.json", (report) => report.manifestSha256 = digest);
    assert.throws(() => readCompatibilityResults(directory), /upstream document links require|unknown document publication/);
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
  const document = readCompatibilityResults(directory).find((corpus) => corpus.id === "holdout").results.find((document) => document.id === "listennotes.com");
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
  const corpus = readCompatibilityResults(directory).find((corpus) => corpus.id === "holdout");
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
