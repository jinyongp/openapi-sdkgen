import { createHash } from "node:crypto";
import { copyFileSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

export const corpusNames = ["holdout", "production32", "modern"];

const sha256 = (bytes) => createHash("sha256").update(bytes).digest("hex");

export function readCompatibilityResults(directory) {
  return corpusNames.map((id) => {
    const manifestBytes = readFileSync(resolve(directory, `${id}.json`));
    const reportBytes = readFileSync(resolve(directory, `${id}-results.json`));
    const manifest = JSON.parse(manifestBytes);
    const report = JSON.parse(reportBytes);
    const fail = (message) => { throw new Error(`${id}: ${message}`); };

    if (report.schemaVersion !== 2) fail("an emission-aware report is required");
    if (report.manifestSha256 !== sha256(manifestBytes)) fail("report/manifest hash mismatch");
    const entries = new Map(manifest.corpora.map((entry) => [entry.id, entry]));
    const documents = report.documents;
    if (entries.size !== manifest.corpora.length ||
        new Set(documents.map((document) => document.id)).size !== entries.size ||
        documents.length !== entries.size) fail("document membership mismatch");

    const results = documents.map((document) => {
      const entry = entries.get(document.id);
      if (!entry || entry.input !== document.input ||
          (entry.sha256 && entry.sha256 !== document.inputSha256) ||
          (entry.gitBlob && entry.gitBlob !== document.gitBlob)) fail("document provenance mismatch");
      if (document.documentSuccess && (document.generation.status !== "pass" ||
          document.typecheck.status !== "pass")) fail("success without generation/typecheck evidence");
      if (!document.operationEmission.available) fail("operation emission is unavailable");
      const server = document.supportProfiles?.find((profile) => profile.name === "server-addon" && profile.applicable);
      if (server?.success && (server.generation.status !== "pass" ||
          server.typecheck.status !== "pass" || !server.operationEmission?.available)) {
        fail("server success without generation/typecheck/emission evidence");
      }
      if (document.capabilityAdjustedSuccess !== Boolean(document.documentSuccess || server?.success)) {
        fail("adjusted success does not match generation profiles");
      }
      const selectedEmission = document.documentSuccess ? document.operationEmission : server?.success ? server.operationEmission : null;
      if (selectedEmission && (!Number.isSafeInteger(selectedEmission.count) || selectedEmission.count < 0)) {
        fail("generated operation count is invalid");
      }
      return {
        id: document.id,
        version: document.openapiVersion,
        defaultSuccess: document.documentSuccess,
        adjustedSuccess: document.capabilityAdjustedSuccess,
        emitted: document.operationEmission.count,
        generatedOperations: selectedEmission?.count ?? null,
        receiving: server?.success ? (document.features ?? []).filter((feature) =>
          feature === "document.webhooks" || feature === "operation.callbacks") : [],
      };
    });

    const overall = report.overall;
    const emission = overall.operationEmission;
    const sum = (field) => documents.reduce((total, document) => total + document.operationEmission[field], 0);
    if (overall.documents !== documents.length ||
        overall.successfulDocuments !== documents.filter((document) => document.documentSuccess).length ||
        overall.capabilityAdjustedDocuments !== documents.filter((document) => document.capabilityAdjustedSuccess).length ||
        emission.availableDocuments !== documents.length ||
        ["count", "operationOmissions", "helperOmissions"].some((field) => emission[field] !== sum(field))) {
      fail("summary does not match document results");
    }

    return {
      id,
      documents: overall.documents,
      defaultSuccess: overall.successfulDocuments,
      adjustedSuccess: overall.capabilityAdjustedDocuments,
      emitted: emission.count,
      generatedOperations: results.reduce((total, document) => total + (document.generatedOperations ?? 0), 0),
      operationOmissions: emission.operationOmissions,
      helperOmissions: emission.helperOmissions,
      reportSha256: sha256(reportBytes),
      manifestSha256: sha256(manifestBytes),
      results,
    };
  });
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const docsDirectory = fileURLToPath(new URL("..", import.meta.url));
  const sourceDirectory = resolve(docsDirectory, "../test/compatibility");
  const data = readCompatibilityResults(sourceDirectory);
  const generatedDirectory = resolve(docsDirectory, ".vitepress/generated");
  const publicDirectory = resolve(docsDirectory, "public/compatibility-results");
  mkdirSync(generatedDirectory, { recursive: true });
  mkdirSync(publicDirectory, { recursive: true });
  writeFileSync(resolve(generatedDirectory, "compatibility-results.json"), `${JSON.stringify(data, null, 2)}\n`);
  for (const id of corpusNames) {
    for (const name of [`${id}.json`, `${id}-results.json`]) {
      copyFileSync(resolve(sourceDirectory, name), resolve(publicDirectory, name));
    }
  }
  console.log("Compatibility results: 3 verified reports; summaries and original JSON prepared.");
}
