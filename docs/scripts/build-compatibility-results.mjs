import { createHash } from "node:crypto";
import { copyFileSync, mkdirSync, readFileSync, writeFileSync, rmSync } from "node:fs";
import { basename, dirname, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { gunzipSync } from "node:zlib";
import { corpusNames, readCIMeasurements } from "./ci-measurements.mjs";

export { corpusNames };

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

export function readCompatibilityResults(directory, { reportDirectory = directory, compressed = false } = {}) {
  return corpusNames.map((id) => {
    const manifestBytes = readFileSync(resolve(directory, `${id}.json`));
    const storedReport = readFileSync(resolve(reportDirectory, `${id}-results.json${compressed ? ".gz" : ""}`));
    const reportBytes = compressed ? gunzipSync(storedReport) : storedReport;
    const manifest = JSON.parse(manifestBytes);
    const report = JSON.parse(reportBytes);
    const fail = (message) => { throw new Error(`${id}: ${message}`); };

    if (report.schemaVersion !== 2) fail("an emission-aware report is required");
    if (report.manifestSha256 !== sha256(manifestBytes)) fail("report/manifest hash mismatch");
    const entries = new Map(manifest.corpora.map((entry) => [entry.id, entry]));
    const order = new Map(manifest.corpora.map((entry, index) => [entry.id, index]));
    const documents = report.documents;
    const expected = manifest.corpora.filter(entry => entry.id !== "microsoft-graph-beta" || documents.some(document => document.id === entry.id));
    if (entries.size !== manifest.corpora.length ||
        new Set(documents.map((document) => document.id)).size !== expected.length ||
        documents.length !== expected.length || expected.some(entry => !documents.some(document => document.id === entry.id))) fail("document membership mismatch");

    let results = documents.map((document) => {
      const entry = entries.get(document.id);
      if (!entry || entry.input !== document.input ||
          (entry.sha256 && entry.sha256 !== document.inputSha256) ||
          (entry.gitBlob && entry.gitBlob !== document.gitBlob)) fail("document provenance mismatch");
      if (document.documentSuccess && (document.generation.status !== "pass" ||
          document.typecheck.status !== "pass")) fail("success without generation/typecheck evidence");
      if (!document.operationEmission || (document.generation.status === "pass" && !document.operationEmission.available)) {
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
      if (server?.generation.status === "pass" && !server.operationEmission?.available) {
        fail("server generation without emission evidence");
      }
      if (document.capabilityAdjustedSuccess !== Boolean(document.documentSuccess || server?.success)) {
        fail("adjusted success does not match generation profiles");
      }
      const clientGenerated = document.generation.status === "pass";
      const serverGenerated = server?.generation.status === "pass";
      const selectedEmission = clientGenerated ? document.operationEmission : serverGenerated ? server.operationEmission : null;
      const selectedGeneration = clientGenerated ? document.generation : serverGenerated ? server.generation : null;
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
        clientGenerated,
        serverGenerated: Boolean(serverGenerated),
        emitted: document.operationEmission.count,
        generatedOperations: selectedEmission?.count ?? null,
        generationDurationMillis,
        receiving: serverGenerated ? (document.features ?? []).filter((feature) =>
          feature === "document.webhooks" || feature === "operation.callbacks") : [],
      };
    }).sort((a, b) => order.get(a.id) - order.get(b.id));

    const overall = report.overall;
    const emission = overall.operationEmission;
    const counted = overall.documents === documents.length ? documents
      : documents.filter(document => document.id !== "microsoft-graph-beta");
    const sum = (field) => counted.reduce((total, document) => total + document.operationEmission[field], 0);
    if (overall.documents !== counted.length ||
        overall.successfulDocuments !== counted.filter((document) => document.documentSuccess).length ||
        overall.capabilityAdjustedDocuments !== counted.filter((document) => document.capabilityAdjustedSuccess).length ||
        emission.availableDocuments !== counted.filter((document) => document.operationEmission.available).length ||
        ["count", "operationOmissions", "helperOmissions"].some((field) => emission[field] !== sum(field))) {
      fail("summary does not match document results");
    }

    // Keep the original report and its integrity checks intact. Public
    // verification totals contain the current candidates; Graph generation is
    // published separately with its original measurement provenance.
    results = results.filter(document => document.id !== "microsoft-graph-beta");
    const candidates = documents.filter(document => document.id !== "microsoft-graph-beta");
    const generated = results.filter(document => document.clientGenerated || document.serverGenerated);
    const candidateSum = field => candidates.reduce((total, document) => total + document.operationEmission[field], 0);
    return {
      id,
      documents: candidates.length,
      defaultSuccess: candidates.filter(document => document.documentSuccess).length,
      adjustedSuccess: candidates.filter(document => document.capabilityAdjustedSuccess).length,
      emitted: candidateSum("count"),
      generatedOperations: results.reduce((total, document) => total + (document.generatedOperations ?? 0), 0),
      generationDurationMillis: generated.length > 0 && generated.every((document) => document.generationDurationMillis !== null)
        ? generated.reduce((total, document) => total + document.generationDurationMillis, 0) : null,
      measurement: report.measurement ?? null,
      operationOmissions: candidateSum("operationOmissions"),
      helperOmissions: candidateSum("helperOmissions"),
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

export function readGraphSelection(directory, reportName = "graph-selected-results.json", selectionName = "microsoft-graph-beta") {
  const report = JSON.parse(readFileSync(resolve(directory, reportName)));
  const manifestBytes = readFileSync(resolve(directory, "regression.json"));
  const manifest = JSON.parse(manifestBytes);
  const baseline = JSON.parse(readFileSync(resolve(directory, "graph-full-baseline.json")));
  const full = baseline.document;
  if (baseline.schemaVersion !== 1 || baseline.manifestSha256 !== sha256(manifestBytes) ||
      !/^[a-f0-9]{64}$/.test(baseline.sourceReportSha256) ||
      !Number.isFinite(Date.parse(baseline.measurement?.measuredAt)) ||
      full?.id !== "microsoft-graph-beta" || full.generation?.status !== "pass" ||
      ["artifactCount", "artifactBytes"].some(key => !Number.isSafeInteger(full.generation[key]) || full.generation[key] <= 0) ||
      !Number.isFinite(full.generation.durationMillis) || full.generation.durationMillis <= 0 ||
      full.operationEmission?.available !== true || !Number.isSafeInteger(full.operationEmission.count) || full.operationEmission.count <= 0) {
    throw new Error("Graph selection full-generation baseline mismatch");
  }
  const selected = report.documents?.[0];
  const entry = manifest.corpora.find((item) => item.id === "microsoft-graph-beta");
  const selection = selected?.generationSelection;
  const fixture = readFileSync(resolve(directory, `selections/${selectionName}.toml`));
  const probe = readFileSync(resolve(directory, `selections/${selectionName}.mjs`));
  // This pinned route-only fixture uses a JSON-compatible array of basic strings.
  // Reject other forms instead of accepting an unverified selection policy.
  const policyArray = fixture.toString().match(/^routes\s*=\s*\[([\s\S]*?)^\]/m);
  if (!policyArray) throw new Error("Graph selection route policy is unavailable");
  const policyRoutes = [...new Set(JSON.parse(`[${policyArray[1].replace(/,\s*$/, "")}]`))].sort();
  const expectedAddons = ["graph-metadata-results.json", "graph-count-metadata-results.json"].includes(reportName) ? ["metadata"] : [];
  if (reportName !== "graph-selected-ci-results.json" &&
      JSON.stringify(selected?.generationAddons) !== JSON.stringify(expectedAddons)) {
    throw new Error("Graph selection metadata setting mismatch");
  }
  const historicalVerification = selected?.documentSuccess === true && selected.capabilityAdjustedSuccess === true &&
    selected.typecheck?.status === "pass" && selection?.runtime?.status === "pass";
  const generationOnly = selected?.documentSuccess === false && selected.capabilityAdjustedSuccess === false &&
    selected.typecheck?.status === "not-run" && selection?.runtime?.status === "not-run";
  if (report.schemaVersion !== 2 || report.manifestSha256 !== sha256(manifestBytes) || report.documents.length !== 1 ||
      selected.id !== entry.id || selected.inputSha256 !== entry.sha256 || full.inputSha256 !== selected.inputSha256 ||
      selected.generationScope !== "selected" || selection?.fixtureSha256 !== sha256(fixture) ||
      selection.runtimeProbeSha256 !== sha256(probe) || (!historicalVerification && !generationOnly) ||
      selected.generation.status !== "pass" || selected.operationEmission.available !== true ||
      ["artifactCount", "artifactBytes"].some(key => !Number.isSafeInteger(selected.generation[key]) || selected.generation[key] <= 0) ||
      !Number.isFinite(selected.generation.durationMillis) || selected.generation.durationMillis <= 0 ||
      !/^[a-f0-9]{40}$/.test(report.measurement?.sourceCommit) || report.measurement.sourceDirty !== false ||
      report.overall.documents !== 0 || report.overall.selectedDocuments !== 1 ||
      report.overall.selectedSuccessfulDocuments !== (historicalVerification ? 1 : 0)) {
    throw new Error("Graph selection provenance or verification mismatch");
  }
  const routes = selection.routes, dependencies = selection.dependencyRoutes;
  if (!routes.length || JSON.stringify(routes) !== JSON.stringify([...new Set(routes)].sort()) ||
      JSON.stringify(routes) !== JSON.stringify(policyRoutes) ||
      JSON.stringify(selection.requested) !== JSON.stringify({ routes }) ||
      JSON.stringify(dependencies) !== JSON.stringify([...new Set(dependencies)].sort()) ||
      routes.some((route) => dependencies.includes(route)) || selection.excludedOperations < 0 ||
      selected.operationEmission.count !== routes.length ||
      selected.operationRetention.total !== routes.length + dependencies.length + selection.excludedOperations) {
    throw new Error("Graph selection membership mismatch");
  }
  return { full, selected, measurement: report.measurement, resources: report.resources, sourceUrl: entry.sourceUrl, ciRunUrl: report.ciRunUrl };
}

export function readMetadataComparison(directory) {
  const data = JSON.parse(readFileSync(resolve(directory, "metadata-comparison-results.json")));
  const manifest = JSON.parse(readFileSync(resolve(directory, "regression.json")));
  const graph = readGraphSelection(directory);
  const metadata = readGraphSelection(directory, "graph-metadata-results.json");
  const expected = new Map([["graph-nine", "microsoft-graph-beta"], ["graph-count", "microsoft-graph-beta"], ["github-nine", "github"], ["stripe-nine", "stripe"]]);
  const fail = () => { throw new Error("Metadata comparison provenance or measurement mismatch"); };
  const same = (a, b) => JSON.stringify(a) === JSON.stringify(b);
  if (data.schemaVersion !== 1 || !Array.isArray(data.cases) || data.cases.length !== expected.size ||
      new Set(data.cases.map(item => item.name)).size !== expected.size) fail();
  for (const item of data.cases) {
    const entry = manifest.corpora.find(entry => entry.id === expected.get(item.name));
    const generationOnly = item.document === "microsoft-graph-beta" && item.runtimeCheck === "not-run";
    if (!entry || item.document !== entry.id || item.inputSha256 !== entry.sha256 || item.sourceUrl !== entry.sourceUrl ||
        item.displayName !== entry.displayName || !Array.isArray(item.routes) ||
        item.routes.length !== (item.name === "graph-count" ? 1 : 9) ||
        !same(item.routes, [...new Set(item.routes)].sort()) ||
        (!generationOnly && item.runtimeCheck !== (item.name === "graph-nine" ? "mock-call" : "selection-module-smoke"))) fail();
    for (const [mode, sample] of [["default", item.default], ["metadata", item.metadata]]) {
      if (!sample || !same(sample.generationAddons, mode === "default" ? [] : ["metadata"]) ||
          sample.measurement?.sourceCommit !== graph.measurement.sourceCommit || sample.measurement.sourceDirty !== false ||
          sample.measurement.generationScope !== "compile-prepare-write" ||
          !Number.isFinite(Date.parse(sample.measurement.measuredAt)) ||
          ["goVersion", "os", "architecture", "cpu"].some(key => sample.measurement[key] !== graph.measurement[key]) ||
          (!generationOnly && (!/^\d+\.\d+\.\d+$/.test(sample.measurement.typescriptVersion) ||
            (graph.measurement.typescriptVersion && sample.measurement.typescriptVersion !== graph.measurement.typescriptVersion))) ||
          sample.generation?.status !== "pass" || sample.typecheck?.status !== (generationOnly ? "not-run" : "pass") ||
          sample.selection?.runtime?.status !== (generationOnly ? "not-run" : "pass") ||
          !same(sample.selection.routes, item.routes) || !same(sample.selection.requested, { routes: item.routes }) ||
          sample.operationEmission?.available !== true || sample.operationEmission.count !== item.routes.length ||
          ["artifactBytes", "metadataBytes", "artifactCount", "schemaArtifactCount"].some(key => !Number.isSafeInteger(sample.generation[key]) || sample.generation[key] < 0) ||
          sample.generation.metadataBytes > sample.generation.artifactBytes ||
          [sample.generation.durationMillis, sample.resources?.peakRssBytes, ...(generationOnly ? [] : [sample.typecheck.durationMillis])].some(value => !Number.isFinite(value) || value < 0)) fail();
    }
    const a = item.default, b = item.metadata;
    if (a.measurement.typescriptVersion !== b.measurement.typescriptVersion ||
        !same(a.operationEmission, b.operationEmission) || a.generation.artifactCount !== b.generation.artifactCount ||
        a.generation.schemaArtifactCount !== b.generation.schemaArtifactCount ||
        !same(a.selection.routes, b.selection.routes) || !same(a.selection.dependencyRoutes, b.selection.dependencyRoutes) ||
        a.selection.excludedOperations !== b.selection.excludedOperations ||
        b.generation.artifactBytes - a.generation.artifactBytes !== b.generation.metadataBytes - a.generation.metadataBytes) fail();
    if (item.name === "graph-nine") {
      for (const [sample, report] of [[a, graph], [b, metadata]]) {
        if (!same(sample.generation, report.selected.generation) || !same(sample.selection, report.selected.generationSelection) ||
            !same(sample.measurement, report.measurement) || !same(sample.operationEmission, report.selected.operationEmission)) fail();
      }
    }
  }
  return data;
}

export function readRuntimeQuality(directory) {
  const data = JSON.parse(readFileSync(resolve(directory, "runtime-quality-results.json")));
  const fail = () => { throw new Error("Runtime quality measurement provenance or validation mismatch"); };
  const positive = value => Number.isFinite(value) && value > 0;
  const medianMatches = (samples, median) => samples?.length === 3 &&
    samples.every(positive) && [...samples].sort((left, right) => left - right)[1] === median;
  const options = JSON.parse(readFileSync(new URL("../../internal/tscheck/strict-options.json", import.meta.url))).compilerOptions;
  if (data.schemaVersion !== 1 || data.status !== "pass" || data.sourceDirty !== false ||
      !/^[a-f0-9]{40}$/.test(data.sourceCommit) || !/^[a-f0-9]{40}$/.test(data.baselineCommit) ||
      !Number.isFinite(Date.parse(data.measuredAt)) || data.matrix?.total !== 104 || data.matrix.diagnostics !== 0 ||
      JSON.stringify(data.matrix.versions) !== JSON.stringify(["5.7.3", "5.9.3", "6.0.3", "7.0.2"]) ||
      JSON.stringify(data.matrix.headerPolicies) !== '["included","omitted"]' ||
      JSON.stringify(data.matrix.profiles) !== '["NodeNext","Bundler"]' ||
      data.matrix.casesPerVersionAndPolicy?.NodeNext !== 11 || data.matrix.casesPerVersionAndPolicy?.Bundler !== 2 ||
      Object.entries(options).some(([key, value]) => data.strictCompilerOptions?.[key] !== value) ||
      JSON.stringify(data.delivery?.map(row => row.count)) !== "[100,1000,10000]") fail();
  for (const row of data.delivery) {
    if (row.status !== "pass" || row.checkedFiles !== row.source.files ||
        !/^[a-f0-9]{64}$/.test(row.inputSHA256) ||
        [row.generationMS, row.compileMS, row.source.bytes, row.declarations.bytes,
          row.compileResources?.peakRSSKiB].some(value => !Number.isFinite(value) || value <= 0)) fail();
  }
  const expectedCompilerCases = [100, 1000].flatMap(count => ["baseline", "candidate"].flatMap(kind => ["5.7.3", "7.0.2"].map(version => `${kind}:${count}:${version}`))).sort();
  if (JSON.stringify(data.compilerComparisons?.map(row => `${row.kind}:${row.count}:${row.typescript}`).sort()) !== JSON.stringify(expectedCompilerCases)) fail();
  for (const row of data.compilerComparisons) {
    const input = data.delivery.find(sample => sample.count === row.count);
    if (row.inputSHA256 !== input.inputSHA256 ||
        !medianMatches(row.samples?.map(sample => sample.durationMS), row.medianMS) ||
        !medianMatches(row.samples?.map(sample => sample.peakRSSKiB), row.medianRSSKiB) ||
        row.samples.some(sample => sample.signal !== null ||
          (row.kind === "candidate" ? sample.status !== 0 || sample.errorCount !== 0 : sample.status === 0 || sample.errorCount <= 0))) fail();
  }
  const expectedGenerations = [100, 1000, 10000].flatMap(count => ["baseline", "candidate"].map(kind => `${kind}:${count}`)).sort();
  if (JSON.stringify(data.generationComparisons?.map(row => `${row.kind}:${row.count}`).sort()) !== JSON.stringify(expectedGenerations)) fail();
  for (const row of data.generationComparisons) {
    const input = data.delivery.find(sample => sample.count === row.count);
    if (row.inputSHA256 !== input.inputSHA256 ||
        !medianMatches(row.samples?.map(sample => sample.durationMS), row.medianMS) ||
        !medianMatches(row.samples?.map(sample => sample.peakRSSKiB), row.medianRSSKiB) ||
        row.samples.some(sample => sample.status !== 0 || sample.signal !== null) ||
        (row.kind === "candidate" && (row.files !== input.source.files || row.sourceBytes !== input.source.bytes))) fail();
  }
  for (const kind of ["baseline", "candidate"]) {
    const rows = data.sse?.filter(row => row.kind === kind);
    const expected = [32768, 65536, 131072, 262144].flatMap(bytes => [64, 1024, bytes + 64].map(chunk => `${bytes}:${chunk}`)).sort();
    if (JSON.stringify(rows?.map(row => `${row.dataBytes}:${row.chunkBytes}`).sort()) !== JSON.stringify(expected) ||
        rows.some(row => !medianMatches(row.samplesMS, row.medianMS) || !/^[a-f0-9]{64}$/.test(row.inputSHA256))) fail();
  }
  for (const row of data.sse.filter(row => row.kind === "candidate")) {
    const baseline = data.sse.find(sample => sample.kind === "baseline" && sample.dataBytes === row.dataBytes && sample.chunkBytes === row.chunkBytes);
    if (row.inputSHA256 !== baseline.inputSHA256) fail();
  }
  if (data.native?.pass !== true || data.native.checks?.length !== 35) fail();
  const quality = { ...data };
  delete quality.graph;
  return quality;
}

export function readInspectMeasurements(directory) {
  const data = JSON.parse(readFileSync(resolve(directory, "inspect-results.json")));
  const manifestBytes = readFileSync(resolve(directory, "regression.json"));
  const manifest = JSON.parse(manifestBytes);
  const fail = () => { throw new Error("Inspect measurement provenance or sample mismatch"); };
  const measurement = data.measurement;
  const expected = ["selection-fixture", "github", "stripe", "digitalocean", "microsoft-graph-beta"];
  if (data.schemaVersion !== 1 || !measurement || measurement.sourceDirty !== false ||
      !/^[a-f0-9]{40}$/.test(measurement.sourceCommit) ||
      !Number.isFinite(Date.parse(measurement.measuredAt)) ||
      measurement.manifestSha256 !== sha256(manifestBytes) ||
      measurement.inventoryScope !== "entry paths and mounted Path Item references" ||
      measurement.targetScope !== "full-document-client" || measurement.target !== "typescript" ||
      !Array.isArray(data.cases) || data.cases.length !== expected.length ||
      new Set(data.cases.map(item => item.id)).size !== expected.length ||
      data.cases.some(item => !expected.includes(item.id))) fail();
  for (const item of data.cases) {
    if (!Number.isInteger(item.operationCount) || item.operationCount < 1 ||
        !/^[a-f0-9]{64}$/.test(item.routeIdentitySha256) ||
        item.identityComparison !== "pass" || item.filteredRouteComparison !== "pass") fail();
    if (item.id === "selection-fixture") {
      const fixture = readFileSync(new URL("../../test/fixtures/generation-selection.json", import.meta.url));
      if (item.input !== "test/fixtures/generation-selection.json" || item.inputSha256 !== sha256(fixture) ||
          item.sourceUrl !== null || item.revision !== null) fail();
    } else {
      const entry = manifest.corpora.find(entry => entry.id === item.id);
      if (!entry || item.input !== entry.input || item.inputSha256 !== entry.sha256 ||
          item.sourceUrl !== entry.sourceUrl || item.revision !== entry.revision) fail();
    }
    for (const [name, mode] of [["inventory", item.inventory], ["typescriptAnalysis", item.typescriptAnalysis]]) {
      const repetitions = item.id === "selection-fixture" ? 5 : 3;
      if (!mode || mode.repetitions !== repetitions || !Array.isArray(mode.samples) || mode.samples.length !== repetitions) fail();
      for (const sample of mode.samples) {
        if (sample.exitCode !== 0 ||
            [sample.wallMillis, sample.cpuUserMillis, sample.cpuSystemMillis].some(value => !Number.isFinite(value) || value < 0) ||
            !Number.isInteger(sample.peakRssBytes) || sample.peakRssBytes <= 0 ||
            (name === "inventory" ? !Number.isInteger(sample.documentsRead) || sample.documentsRead < 1 : sample.documentsRead !== null)) fail();
      }
      const walls = mode.samples.map(sample => sample.wallMillis).sort((a, b) => a - b);
      if (mode.wallMedianMillis !== walls[Math.floor(repetitions / 2)] || mode.wallMinMillis !== walls[0] ||
          mode.wallMaxMillis !== walls.at(-1) || mode.peakRssBytes !== Math.max(...mode.samples.map(sample => sample.peakRssBytes))) fail();
    }
  }
  return data;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const docsDirectory = fileURLToPath(new URL("..", import.meta.url));
  const sourceDirectory = resolve(docsDirectory, "../test/compatibility");
  const ciDirectory = resolve(sourceDirectory, "ci");
  const ci = readCIMeasurements(ciDirectory);
  const data = readCompatibilityResults(sourceDirectory, { reportDirectory: ciDirectory, compressed: true });
  for (const corpus of data) {
    corpus.measurement = { ...corpus.measurement, provider: ci.provenance.provider,
      runner: ci.provenance.runner, runUrl: ci.provenance.runUrl };
  }
  const selection = readGraphSelection(sourceDirectory, "graph-selected-ci-results.json");
  if (!/^https:\/\/github\.com\/jinyongp\/openapi-sdkgen\/actions\/runs\/\d+$/.test(selection.ciRunUrl)) {
    throw new Error("Graph selection has no GitHub Actions provenance");
  }
  const graph = { selected: selection.selected, sourceUrl: selection.sourceUrl,
    measurement: selection.measurement, ciRunUrl: selection.ciRunUrl };
  const generatedDirectory = resolve(docsDirectory, ".vitepress/generated");
  const publicDirectory = resolve(docsDirectory, "public/compatibility-results");
  rmSync(generatedDirectory, { recursive: true, force: true });
  rmSync(publicDirectory, { recursive: true, force: true });
  mkdirSync(generatedDirectory, { recursive: true });
  mkdirSync(publicDirectory, { recursive: true });
  writeFileSync(resolve(generatedDirectory, "compatibility-results.json"), `${JSON.stringify(data, null, 2)}\n`);
  writeFileSync(resolve(generatedDirectory, "graph-selection.json"), `${JSON.stringify(graph, null, 2)}\n`);
  writeFileSync(resolve(publicDirectory, "provenance.json"), `${JSON.stringify(ci.provenance, null, 2)}\n`);
  for (const id of corpusNames) {
    copyFileSync(resolve(sourceDirectory, `${id}.json`), resolve(publicDirectory, `${id}.json`));
    writeFileSync(resolve(publicDirectory, `${id}-results.json`), ci.reports[id].bytes);
  }
  const graphReport = JSON.parse(readFileSync(resolve(sourceDirectory, "graph-selected-ci-results.json")));
  delete graphReport.measurement.cpu;
  writeFileSync(resolve(publicDirectory, "graph-selected-ci-results.json"), `${JSON.stringify(graphReport, null, 2)}\n`);
  publishCompatibilityDocuments(sourceDirectory, publicDirectory);
  console.log(`Compatibility results: ${corpusNames.length} verified GitHub Actions reports prepared.`);
}
