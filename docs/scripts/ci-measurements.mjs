import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { gunzipSync, gzipSync } from "node:zlib";

export const corpusNames = ["regression", "holdout", "production32", "modern"];
export const graphNames = ["graph-full", "graph-selected"];
const sha256 = bytes => createHash("sha256").update(bytes).digest("hex");
const privateFields = new Set(["cpu", "hostname", "release", "totalMemoryBytes"]);

export function publicMeasurement(value) {
  if (Array.isArray(value)) return value.map(publicMeasurement);
  if (value && typeof value === "object") {
    return Object.fromEntries(Object.entries(value)
      .filter(([key]) => !privateFields.has(key))
      .map(([key, item]) => [key, publicMeasurement(item)]));
  }
  return value;
}

function validateProvenance(provenance) {
  if (provenance.schemaVersion !== 1 || provenance.provider !== "github-actions" ||
      provenance.verification !== "passed" || !["ubuntu-latest", "ubuntu-24.04"].includes(provenance.runner) ||
      !/^https:\/\/github\.com\/jinyongp\/openapi-sdkgen\/actions\/runs\/\d+$/.test(provenance.runUrl) ||
      !/^[a-f0-9]{40}$/.test(provenance.sourceCommit)) {
    throw new Error("GitHub Actions measurement provenance is invalid");
  }
}

function validSource(report, sourceCommit) {
  return [report.measurement, ...Object.values(report.documentMeasurements ?? {})].every(measurement =>
    measurement?.sourceDirty === false && measurement.sourceCommit === sourceCommit &&
    Number.isFinite(Date.parse(measurement.measuredAt)));
}

export function readCIMeasurements(directory) {
  const provenance = JSON.parse(readFileSync(resolve(directory, "provenance.json")));
  validateProvenance(provenance);
  const names = [...corpusNames, ...graphNames.filter(id => provenance.reports?.[id])];
  const reports = Object.fromEntries(names.map(id => {
    const bytes = gunzipSync(readFileSync(resolve(directory, `${id}-results.json.gz`)));
    const report = JSON.parse(bytes);
    if (sha256(bytes) !== provenance.reports?.[id] ||
        !validSource(report, provenance.sourceCommit) ||
        JSON.stringify(report) !== JSON.stringify(publicMeasurement(report))) {
      throw new Error(`${id}: CI measurement integrity or privacy check failed`);
    }
    return [id, { bytes, report }];
  }));
  return { provenance, reports };
}

export function exportCIMeasurements({ input, output, runUrl, sourceCommit, runner }) {
  const provenance = {
    schemaVersion: 1, provider: "github-actions", verification: "passed",
    runUrl, sourceCommit, runner, reports: {},
  };
  validateProvenance(provenance);
  const reports = [...corpusNames, ...graphNames].map(id => {
    const report = publicMeasurement(JSON.parse(readFileSync(resolve(input, `${id}-results.json`))));
    if (!validSource(report, sourceCommit)) {
      throw new Error(`${id}: benchmark commit does not match the Actions run`);
    }
    const bytes = Buffer.from(`${JSON.stringify(report, null, 2)}\n`);
    provenance.reports[id] = sha256(bytes);
    return [id, bytes];
  });
  mkdirSync(output, { recursive: true });
  for (const [id, bytes] of reports) writeFileSync(resolve(output, `${id}-results.json.gz`), gzipSync(bytes));
  writeFileSync(resolve(output, "provenance.json"), `${JSON.stringify(provenance, null, 2)}\n`);
  readCIMeasurements(output);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const [command, ...args] = process.argv.slice(2);
  if (command !== "export" || args.length !== 10) throw new Error("export requires input, output, run URL, commit, and runner");
  const options = Object.fromEntries(Array.from({ length: 5 }, (_, index) =>
    [args[index * 2].replace(/^--/, "").replace(/-([a-z])/g, (_, letter) => letter.toUpperCase()), args[index * 2 + 1]]));
  exportCIMeasurements(options);
  console.log("ok GitHub Actions measurements exported without machine identifiers");
}
