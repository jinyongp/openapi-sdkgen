import { createHash } from "node:crypto";
import { copyFileSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { basename, dirname, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

export const corpusNames = ["regression", "holdout", "production32", "modern"];

const sha256 = (bytes) => createHash("sha256").update(bytes).digest("hex");
const documentPath = (entry) => `documents/${encodeURIComponent(entry.id)}/${encodeURIComponent(basename(entry.input))}`;

function upstreamDocumentUrl(manifest, entry) {
  if (!manifest.publication) return null;
  if (manifest.publication.documents !== "upstream") throw new Error("unknown document publication mode");
  const url = new URL(entry.sourceUrl);
  if (!manifest.pinned || !/^[a-f0-9]{64}$/.test(entry.sha256) ||
      !/^[a-f0-9]{40}$/.test(entry.revision) || url.protocol !== "https:" ||
      !url.pathname.split("/").includes(entry.revision)) {
    throw new Error("upstream document links require a pinned HTTPS revision and input hash");
  }
  return url.href;
}

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
    const order = new Map(manifest.corpora.map((entry, index) => [entry.id, index]));
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
      if (!document.operationEmission || (document.documentSuccess && !document.operationEmission.available)) {
        fail("operation emission is unavailable");
      }
      if (!document.operationEmission.available && document.operationEmission.count !== 0) {
        fail("unavailable emission has a measured count");
      }
      const server = document.supportProfiles?.find((profile) => profile.name === "server-addon" && profile.applicable);
      if (server?.success && (server.generation.status !== "pass" ||
          server.typecheck.status !== "pass" || !server.operationEmission?.available)) {
        fail("server success without generation/typecheck/emission evidence");
      }
      if (document.capabilityAdjustedSuccess !== Boolean(document.documentSuccess || server?.success)) {
        fail("adjusted success does not match generation profiles");
      }
      const selectedEmission = document.documentSuccess ? document.operationEmission : server?.success ? server.operationEmission : null;
      const selectedGeneration = document.documentSuccess ? document.generation : server?.success ? server.generation : null;
      const generationDurationMillis = selectedGeneration?.durationMillis ?? null;
      if (generationDurationMillis !== null && (!Number.isFinite(generationDurationMillis) || generationDurationMillis < 0 ||
          report.measurement?.generationScope !== "compile-prepare-write")) {
        fail("generation timing is invalid or has an unknown scope");
      }
      if (selectedEmission && (!Number.isSafeInteger(selectedEmission.count) || selectedEmission.count < 0)) {
        fail("generated operation count is invalid");
      }
      return {
        id: document.id,
        name: entry.displayName ?? document.id,
        version: document.openapiVersion,
        sourceUrl: upstreamDocumentUrl(manifest, entry) ?? (manifest.source?.rawBaseUrl
          ? new URL(entry.input.split("/").map(encodeURIComponent).join("/"), manifest.source.rawBaseUrl).href
          : `/compatibility-results/${documentPath(entry)}`),
        defaultSuccess: document.documentSuccess,
        adjustedSuccess: document.capabilityAdjustedSuccess,
        emitted: document.operationEmission.count,
        generatedOperations: selectedEmission?.count ?? null,
        generationDurationMillis,
        receiving: server?.success ? (document.features ?? []).filter((feature) =>
          feature === "document.webhooks" || feature === "operation.callbacks") : [],
      };
    }).sort((a, b) => order.get(a.id) - order.get(b.id));

    const overall = report.overall;
    const successful = results.filter((document) => document.adjustedSuccess);
    const emission = overall.operationEmission;
    const sum = (field) => documents.reduce((total, document) => total + document.operationEmission[field], 0);
    if (overall.documents !== documents.length ||
        overall.successfulDocuments !== documents.filter((document) => document.documentSuccess).length ||
        overall.capabilityAdjustedDocuments !== documents.filter((document) => document.capabilityAdjustedSuccess).length ||
        emission.availableDocuments !== documents.filter((document) => document.operationEmission.available).length ||
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
      generationDurationMillis: successful.length > 0 && successful.every((document) => document.generationDurationMillis !== null)
        ? successful.reduce((total, document) => total + document.generationDurationMillis, 0) : null,
      measurement: report.measurement ?? null,
      operationOmissions: emission.operationOmissions,
      helperOmissions: emission.helperOmissions,
      reportSha256: sha256(reportBytes),
      manifestSha256: sha256(manifestBytes),
      results,
    };
  });
}

export function publishCompatibilityDocuments(directory, outputDirectory) {
  const testDirectory = resolve(directory, "..");
  for (const id of corpusNames) {
    const manifest = JSON.parse(readFileSync(resolve(directory, `${id}.json`)));
    if (manifest.source?.rawBaseUrl) continue;
    for (const entry of manifest.corpora) {
      if (upstreamDocumentUrl(manifest, entry)) continue;
      const files = [entry, ...(manifest.files ?? []).filter((file) => dirname(file.input) === dirname(entry.input))];
      for (const file of files) {
        const source = resolve(testDirectory, file.input);
        const sourceRelative = relative(testDirectory, source);
        if (sourceRelative.startsWith(`..${sep}`) || sourceRelative === "..") {
          throw new Error(`${id}: document is outside the input directory`);
        }
        const bytes = readFileSync(source);
        if (sha256(bytes) !== file.sha256) throw new Error(`${id}: published document hash mismatch`);
        const destination = resolve(outputDirectory, "documents", entry.id, basename(file.input));
        mkdirSync(dirname(destination), { recursive: true });
        writeFileSync(destination, bytes);
      }
    }
  }
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
  publishCompatibilityDocuments(sourceDirectory, publicDirectory);
  console.log(`Compatibility results: ${corpusNames.length} verified reports; summaries and original JSON prepared.`);
}
