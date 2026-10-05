import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";
import { pathToFileURL } from "node:url";
import { gzipSync } from "node:zlib";

// Optional release comparison. Inputs are pinned, local document copies;
// downloading documents and correcting invalid inputs are separate explicit steps.
process.umask(0o077);
assert.equal(process.argv.length, 6, "OUTPUT BASELINE_BINARY CURRENT_BINARY MANIFEST required");
const outputDirectory = path.resolve(process.argv[2]);
const baselineBinary = path.resolve(process.argv[3]);
const currentBinary = path.resolve(process.argv[4]);
const manifest = JSON.parse(fs.readFileSync(process.argv[5], "utf8"));
const require = createRequire(import.meta.url);
const viteRequire = createRequire(fs.realpathSync(require.resolve("vite/package.json")));
const { rolldown } = await import(pathToFileURL(viteRequire.resolve("rolldown")));
const hash = (data) => createHash("sha256").update(data).digest("hex");
const files = (directory) =>
  fs
    .readdirSync(directory, { withFileTypes: true })
    .flatMap((entry) => {
      const file = path.join(directory, entry.name);
      return entry.isDirectory() ? files(file) : [file];
    })
    .sort();
fs.mkdirSync(outputDirectory, { recursive: true, mode: 0o700 });
fs.chmodSync(outputDirectory, 0o700);
if (manifest.private) {
  process.on("uncaughtException", (error) => {
    fs.writeFileSync(
      path.join(outputDirectory, "failure.private.log"),
      error.stack ?? String(error),
    );
    console.error(
      "Private comparison failed; detailed diagnostics remain in the output directory.",
    );
    process.exitCode = 1;
  });
}
const resultsFile = path.join(outputDirectory, "results.json");
const results = fs.existsSync(resultsFile) ? JSON.parse(fs.readFileSync(resultsFile, "utf8")) : [];
const save = () => fs.writeFileSync(resultsFile, JSON.stringify(results, null, 2));
const variants = [
  ["v10", baselineBinary],
  ["current", currentBinary],
];
const provenance = {
  measurementVersion: 1,
  node: process.version,
  bundler: `rolldown ${viteRequire("rolldown/package.json").version}`,
  baselineSHA256: hash(fs.readFileSync(baselineBinary)),
  currentSHA256: hash(fs.readFileSync(currentBinary)),
  gzipLevel: 6,
  platform: "browser",
  format: "esm",
  minify: true,
  manifest,
};
const provenanceFile = path.join(outputDirectory, "provenance.json");
if (fs.existsSync(provenanceFile))
  assert.deepEqual(
    JSON.parse(fs.readFileSync(provenanceFile)),
    provenance,
    "use a fresh output directory after changing inputs or binaries",
  );
else fs.writeFileSync(provenanceFile, JSON.stringify(provenance, null, 2));

function generate(provider, input, routes, kind, count, variant, binary, baselineFull) {
  if (variant === "v10" && kind === "full" && baselineFull) return path.resolve(baselineFull);
  const directory = path.join(outputDirectory, provider, variant, kind + (count ?? ""));
  const sdk = path.join(directory, "sdk");
  const stamp = path.join(directory, "generated.json");
  if (fs.existsSync(stamp)) return sdk;
  assert(!fs.existsSync(sdk), "preserving incomplete output; choose a fresh measurement directory");
  fs.mkdirSync(directory, { recursive: true });
  const config = path.join(directory, "generator.toml");
  fs.writeFileSync(
    config,
    `source = ${JSON.stringify(input)}\ntarget = "typescript"\n` +
      (kind === "root" ? `[selection]\nroutes = ${JSON.stringify(routes.slice(0, count))}\n` : "") +
      (kind === "named"
        ? `[clients.page.selection]\nroutes = ${JSON.stringify(routes.slice(0, count))}\n`
        : ""),
  );
  const run = spawnSync(binary, ["generate", "--config", config, "--output", sdk, "--offline"], {
    encoding: "utf8",
    timeout: 300000,
    maxBuffer: 8 * 1024 * 1024,
  });
  fs.writeFileSync(path.join(directory, "generation.log"), (run.stdout ?? "") + (run.stderr ?? ""));
  assert.equal(
    run.status,
    0,
    `${provider} ${variant} ${kind}${count ?? ""}: ${run.error ?? run.stderr}`,
  );
  fs.writeFileSync(
    stamp,
    JSON.stringify({
      binarySHA256: hash(fs.readFileSync(binary)),
      inputSHA256: hash(fs.readFileSync(input)),
    }),
  );
  return sdk;
}

function staticReferences(sdk, routes) {
  const byRoute = new Map();
  for (const file of files(path.join(sdk, "selective", "operations"))) {
    if (!file.endsWith(".ts")) continue;
    const match = fs.readFileSync(file, "utf8").match(/OperationReference<("(?:\\.|[^"\\])*")>/);
    if (match) byRoute.set(JSON.parse(match[1]), file);
  }
  return routes.map((route) => {
    assert(byRoute.has(route), `missing static reference ${route}`);
    return byRoute.get(route);
  });
}

async function bundle(provider, variant, surface, count, sdk, routes) {
  const key = `${provider}:${variant}:${surface}:${count ?? "all"}`;
  if (results.some((row) => row.key === key)) return;
  const directory = path.join(
    outputDirectory,
    provider,
    variant,
    "bundles",
    surface + (count ?? ""),
  );
  fs.mkdirSync(directory, { recursive: true });
  const entry = path.join(directory, "consumer.ts");
  const selected = routes.slice(0, count ?? routes.length);
  const options =
    "baseURL:'https://example.test',fetch:async()=>{throw new Error('measurement must not send requests')}";
  let source;
  if (surface === "loadOperations") {
    const references = staticReferences(sdk, selected);
    source =
      `import {createClient,loadOperations} from ${JSON.stringify(sdk + "/selective/index.ts")};\n` +
      references
        .map(
          (file, index) => `import {operation as operation${index}} from ${JSON.stringify(file)};`,
        )
        .join("\n") +
      `\nexport const api=createClient({${options},operations:await loadOperations([${references.map((_, index) => `operation${index}`).join(",")}])});`;
  } else {
    const entryPath = surface === "named-client-selection" ? "/clients/page/index.ts" : "/index.ts";
    source = `import {createClient} from ${JSON.stringify(sdk + entryPath)};export const api=createClient({${options}});`;
  }
  fs.writeFileSync(entry, source);
  const warnings = [];
  const build = await rolldown({
    input: entry,
    platform: "browser",
    treeshake: true,
    onLog: (_level, log) => warnings.push(log.message),
  });
  try {
    const { output } = await build.generate({
      format: "esm",
      minify: true,
      entryFileNames: "entry.mjs",
      chunkFileNames: "chunk-[hash].mjs",
    });
    const chunks = output.filter((item) => item.type === "chunk");
    assert.equal(warnings.length, 0, warnings.join("\n"));
    for (const chunk of chunks) fs.writeFileSync(path.join(directory, chunk.fileName), chunk.code);
    const validator = path.join(directory, "validate.mjs");
    fs.writeFileSync(
      validator,
      `try {\nglobalThis.fetch=async()=>{throw Error('unexpected fetch')};const {api}=await import(${JSON.stringify(pathToFileURL(path.join(directory, "entry.mjs")).href)});const routes=Object.keys(api.$routes);for(const route of ${JSON.stringify(selected)})if(!routes.includes(route))throw Error('missing route '+route);if(routes.length!==${selected.length})throw Error('unselected API included');console.log(routes.length);\n} catch(error) { console.error(error.stack); process.exitCode=1; }`,
    );
    const validation = spawnSync(process.execPath, [validator], {
      encoding: "utf8",
      timeout: 60000,
    });
    fs.writeFileSync(
      path.join(directory, "validation.log"),
      (validation.stdout ?? "") + (validation.stderr ?? ""),
    );
    assert.equal(
      validation.status,
      0,
      String(validation.error ?? validation.stderr).slice(0, 2000) + ` signal=${validation.signal}`,
    );
    const sourceFiles = files(sdk);
    const row = {
      key,
      provider,
      variant,
      surface,
      count: count ?? "all",
      jsBytes: chunks.reduce((n, c) => n + Buffer.byteLength(c.code), 0),
      gzipBytes: chunks.reduce((n, c) => n + gzipSync(c.code, { level: 6 }).length, 0),
      chunks: chunks.length,
      entryBytes: Buffer.byteLength(chunks.find((c) => c.isEntry).code),
      sourceBytes: sourceFiles.reduce((n, file) => n + fs.statSync(file).size, 0),
      tsFiles: sourceFiles.filter((file) => file.endsWith(".ts")).length,
      validation: "pass",
      warnings: warnings.length,
    };
    results.push(row);
    save();
    if (!manifest.private) console.log(JSON.stringify(row));
  } finally {
    await build.close();
  }
}

for (const provider of manifest.providers) {
  const input = path.resolve(provider.input);
  const data = fs.readFileSync(input);
  assert.equal(hash(data), provider.sha256, `input changed: ${provider.name}`);
  const document = JSON.parse(data);
  const methods = new Set([
    "get",
    "post",
    "put",
    "delete",
    "patch",
    "head",
    "options",
    "trace",
    "query",
  ]);
  const routes = Object.entries(document.paths)
    .flatMap(([route, item]) =>
      Object.keys(item)
        .filter((method) => methods.has(method))
        .map((method) => method.toUpperCase() + " " + route),
    )
    .filter((route) => !provider.excludedRoutes?.some((item) => item.route === route));
  for (const [variant, binary] of variants) {
    const full = generate(
      provider.name,
      input,
      routes,
      "full",
      null,
      variant,
      binary,
      provider.baselineFull,
    );
    await bundle(provider.name, variant, "full", null, full, routes);
    for (const count of [1, 2]) {
      await bundle(provider.name, variant, "loadOperations", count, full, routes);
      const root = generate(provider.name, input, routes, "root", count, variant, binary);
      await bundle(provider.name, variant, "root-selection", count, root, routes);
      const named = generate(provider.name, input, routes, "named", count, variant, binary);
      await bundle(provider.name, variant, "named-client-selection", count, named, routes);
    }
  }
}

const comparisons = results
  .filter((row) => row.variant === "current")
  .map((row) => {
    const before = results.find(
      (other) =>
        other.provider === row.provider &&
        other.variant === "v10" &&
        other.surface === row.surface &&
        other.count === row.count,
    );
    assert(before, "missing v10 comparison");
    return {
      provider: row.provider,
      surface: row.surface,
      count: row.count,
      before,
      after: row,
      jsRatio: row.jsBytes / before.jsBytes,
      gzipRatio: row.gzipBytes / before.gzipBytes,
      pass: row.jsBytes <= before.jsBytes * 1.05 && row.gzipBytes <= before.gzipBytes * 1.05,
    };
  });
fs.writeFileSync(
  path.join(outputDirectory, "comparisons.json"),
  JSON.stringify(comparisons, null, 2),
);
const failures = comparisons.filter((row) => !row.pass);
console.log(
  JSON.stringify({
    comparisons: comparisons.length,
    failures: failures.map(({ provider, surface, count, jsRatio, gzipRatio }) => ({
      provider: manifest.private ? "private" : provider,
      surface,
      count,
      jsRatio,
      gzipRatio,
    })),
  }),
);
if (failures.length) process.exitCode = 1;
