import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { resolve, relative, sep } from "node:path";
import { performance } from "node:perf_hooks";

const [corpusArgument, outputArgument] = process.argv.slice(2);
assert(corpusArgument && outputArgument, "corpus directory and output are required");
const root = process.cwd();
const corpus = resolve(corpusArgument);
const output = resolve(outputArgument);
const binary = resolve(".tmp/bin/openapi-sdkgen");
const directory = resolve(".tmp/inspect-benchmark");
mkdirSync(directory, { recursive: true });
const sha = (bytes) => createHash("sha256").update(bytes).digest("hex");
const git = (...args) => execFileSync("git", args, { encoding: "utf8" }).trim();
assert.equal(git("status", "--porcelain"), "", "measure a clean committed source");
const sourceCommit = git("rev-parse", "HEAD");
const manifestBytes = readFileSync("test/compatibility/regression.json");
const manifest = JSON.parse(manifestBytes);
const wanted = ["github", "stripe", "digitalocean", "microsoft-graph-beta"];
function corpusPath(input) {
  const path = resolve(corpus, input);
  const local = relative(corpus, path);
  assert(local !== ".." && !local.startsWith(`..${sep}`), "contained corpus input");
  return path;
}
for (const file of manifest.files ?? []) {
  assert.equal(sha(readFileSync(corpusPath(file.input))), file.sha256, file.input);
}
const cases = wanted.map((id) => {
  const document = manifest.corpora.find((entry) => entry.id === id);
  assert(document, id);
  assert.equal(sha(readFileSync(corpusPath(document.input))), document.sha256, id);
  return { ...document, path: corpusPath(document.input), repetitions: 3 };
});
const fixture = "test/fixtures/generation-selection.json";
cases.unshift({ id: "selection-fixture", input: fixture, path: resolve(fixture), sha256: sha(readFileSync(fixture)), repetitions: 5 });

const results = {
  schemaVersion: 1,
  measurement: {
    sourceCommit, sourceDirty: false, measuredAt: new Date().toISOString(),
    manifestSha256: sha(manifestBytes), platform: process.platform, arch: process.arch,
    environment: "local cached documents; sequential process measurements",
    inventoryScope: "entry paths and mounted Path Item references",
    targetScope: "full-document-client", target: "typescript",
    processTimeoutMillis: 300000,
  },
  cases: [],
};
const routeIdentity = (report) => report.operations.map(({ route, operationId }) => [route, operationId]).sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0);
const resources = (text, label) => {
  const line = text.split("\n").find((entry) => entry.trimStart().startsWith(`${label}:`));
  assert(line, `missing ${label}`);
  return Number(line.slice(line.indexOf(":") + 1).trim());
};
function run(document, mode, iteration, filters = []) {
  const resourcePath = resolve(directory, `${document.id}-${mode}-${iteration}.resources.txt`);
  const args = ["inspect", "--input", document.path, "--format", "json", ...(mode === "target" ? ["--target", "typescript"] : []), ...filters];
  const started = performance.now();
  const json = execFileSync("/usr/bin/time", ["-v", "-o", resourcePath, binary, ...args], {
    encoding: "utf8", timeout: results.measurement.processTimeoutMillis,
    maxBuffer: 128 * 1024 * 1024, stdio: ["ignore", "pipe", "pipe"],
    env: { ...process.env, LC_ALL: "C" },
  });
  const wallMillis = performance.now() - started;
  const report = JSON.parse(json);
  assert.equal(report.schemaVersion, 1);
  assert.equal(report.matched, report.operations.length);
  assert.equal(report.target ?? "", mode === "target" ? "typescript" : "");
  const text = readFileSync(resourcePath, "utf8");
  return {
    report,
    sample: {
      wallMillis, cpuUserMillis: resources(text, "User time (seconds)") * 1000,
      cpuSystemMillis: resources(text, "System time (seconds)") * 1000,
      peakRssBytes: resources(text, "Maximum resident set size (kbytes)") * 1024,
      documentsRead: report.documentsRead ?? null,
      exitCode: resources(text, "Exit status"),
    },
  };
}
function summarize(samples) {
  const walls = samples.map(({ wallMillis }) => wallMillis).sort((a, b) => a - b);
  return { repetitions: samples.length, wallMedianMillis: walls[Math.floor(walls.length / 2)], wallMinMillis: walls[0], wallMaxMillis: walls.at(-1), peakRssBytes: Math.max(...samples.map(({ peakRssBytes }) => peakRssBytes)), samples };
}
for (const document of cases) {
  let identity;
  const inventorySamples = [], targetSamples = [];
  for (const mode of ["inventory", "target"]) {
    for (let iteration = 0; iteration < document.repetitions; iteration++) {
      const { report, sample } = run(document, mode, iteration);
      assert.equal(report.matched, report.total);
      const routes = routeIdentity(report);
      if (!identity) identity = routes;
      assert.deepEqual(routes, identity, `${document.id} ${mode} API identity differs`);
      assert.equal(sample.exitCode, 0);
      (mode === "inventory" ? inventorySamples : targetSamples).push(sample);
      console.log(`ok ${document.id} ${mode} ${iteration + 1}/${document.repetitions}: ${report.total} operations, ${Math.round(sample.wallMillis)}ms`);
    }
  }
  const chosen = identity[0][0];
  const filtered = run(document, "inventory", "filter", ["--route", chosen]);
  assert.equal(filtered.report.matched, 1);
  assert.deepEqual(routeIdentity(filtered.report), identity.slice(0, 1));
  results.cases.push({
    id: document.id, input: document.input, inputSha256: document.sha256,
    sourceUrl: document.sourceUrl ?? null, revision: document.revision ?? null,
    operationCount: identity.length, routeIdentitySha256: sha(JSON.stringify(identity)),
    identityComparison: "pass", filteredRouteComparison: "pass",
    inventory: summarize(inventorySamples), typescriptAnalysis: summarize(targetSamples),
  });
  writeFileSync(resolve(directory, "progress.json"), JSON.stringify(results, null, 2) + "\n");
}
assert.equal(git("rev-parse", "HEAD"), sourceCommit);
assert.equal(git("status", "--porcelain"), "");
writeFileSync(output, JSON.stringify(results, null, 2) + "\n");
console.log(`ok inspect corpus: ${results.cases.length} API identity comparisons; inventory and TypeScript resource measurements saved`);
