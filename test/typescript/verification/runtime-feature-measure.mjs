import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { execFileSync } from "node:child_process";
import { fileURLToPath, pathToFileURL } from "node:url";
import { gzipSync } from "node:zlib";
import { build } from "vite";
import { generatedModuleGraph } from "./runtime-feature-imports.mjs";
const require = createRequire(import.meta.url);
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
const matrix = path.resolve(process.argv[2] ?? path.join(root, ".tmp/runtime-feature-matrix"));
function files(directory) {
  return fs
    .readdirSync(directory, { withFileTypes: true })
    .flatMap((entry) => {
      const filename = path.join(directory, entry.name);
      return entry.isDirectory() ? files(filename) : [filename];
    })
    .sort();
}
function hash(data) {
  return createHash("sha256").update(data).digest("hex");
}
function inventory(directory) {
  const sources = files(directory).filter((file) => file.endsWith(".ts"));
  const runtime = sources.filter((file) => file.includes("/internal/runtime/"));
  const summarize = (selected) => ({
    files: selected.length,
    bytes: selected.reduce((sum, file) => sum + fs.statSync(file).size, 0),
    lines: selected.reduce(
      (sum, file) => sum + fs.readFileSync(file, "utf8").split("\n").length - 1,
      0,
    ),
  });
  return {
    total: summarize(sources),
    runtime: summarize(runtime),
    sourceSHA256: hash(
      sources
        .map((file) => `${path.relative(directory, file)}\0${hash(fs.readFileSync(file))}`)
        .join("\n"),
    ),
  };
}
function packageVersion(name) {
  const resolver = name === "rolldown" ? createRequire(require.resolve("vite")) : require;
  let directory = path.dirname(resolver.resolve(name));
  while (directory !== path.dirname(directory)) {
    const filename = path.join(directory, "package.json");
    if (fs.existsSync(filename)) {
      const pkg = JSON.parse(fs.readFileSync(filename));
      if (pkg.name === name) return pkg.version;
    }
    directory = path.dirname(directory);
  }
  throw new Error(`Cannot identify ${name} version`);
}
const options = "{baseURL:'https://size.test',authorization:'Bearer benchmark'}";
const call = "api.$operations.getItem({path:{id:'one'}})";
const cases = [
  {
    name: "full-capability-control",
    variant: "full",
    entry: `import {createClient} from './index.js';export async function run(){const api=createClient(${options});return ${call}}`,
  },
  {
    name: "default-root",
    variant: "selected",
    entry: `import {createClient} from './index.js';export async function run(){const api=createClient(${options});return ${call}}`,
  },
  {
    name: "static-selective",
    variant: "selected",
    entry: `import {operation} from './selective/operations/items/by-id/get.js';import {createClient,loadOperations} from './selective/index.js';export async function run(){const operations=await loadOperations([operation]);const api=createClient({...${options},operations});return ${call}}`,
  },
  {
    name: "lazy-selective",
    variant: "selected",
    entry: `export async function run(){const [{operation},{createClient,loadOperations}]=await Promise.all([import('./selective/operations/items/by-id/get.js'),import('./selective/index.js')]);const operations=await loadOperations([operation]);const api=createClient({...${options},operations});return ${call}}`,
  },
];
const bundles = [];
for (const item of cases) {
  const sdk = path.join(matrix, item.variant + "-js", "small-json");
  const entryFile = path.join(sdk, "size-entry.mjs");
  fs.writeFileSync(entryFile, item.entry);
  const result = await build({
    configFile: false,
    root: sdk,
    logLevel: "silent",
    build: {
      modulePreload: false,
      target: "es2022",
      minify: "oxc",
      write: false,
      rollupOptions: {
        input: entryFile,
        preserveEntrySignatures: "strict",
        output: { format: "es", entryFileNames: "entry.mjs", chunkFileNames: "chunk-[hash].mjs" },
      },
    },
  });
  assert.ok(!Array.isArray(result) && "output" in result);
  const chunks = result.output.filter((item) => item.type === "chunk");
  const directory = path.join(matrix, "bundles", item.name);
  fs.mkdirSync(directory, { recursive: true });
  for (const chunk of chunks) fs.writeFileSync(path.join(directory, chunk.fileName), chunk.code);
  const entry = chunks.find((chunk) => chunk.isEntry);
  assert.ok(entry);
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    assert.equal(String(url), "https://size.test/items/one");
    assert.equal(new Headers(init?.headers).get("authorization"), "Bearer benchmark");
    return Response.json({ id: "one", title: "One" });
  };
  try {
    const module = await import(pathToFileURL(path.join(directory, entry.fileName)));
    assert.deepEqual(await module.run(), { id: "one", title: "One" });
  } finally {
    globalThis.fetch = originalFetch;
  }
  const outputs = chunks.map((chunk) => ({
    file: chunk.fileName,
    entry: chunk.isEntry,
    bytes: Buffer.byteLength(chunk.code),
    gzipBytes: gzipSync(chunk.code, { level: 6 }).length,
    sha256: hash(chunk.code),
  }));
  bundles.push({
    name: item.name,
    chunks: outputs.length,
    entryBytes: Buffer.byteLength(entry.code),
    entryGzipBytes: gzipSync(entry.code, { level: 6 }).length,
    deployedBytes: outputs.reduce((sum, chunk) => sum + chunk.bytes, 0),
    deployedGzipBytes: outputs.reduce((sum, chunk) => sum + chunk.gzipBytes, 0),
    outputs,
  });
}
const catalog = JSON.parse(
  fs.readFileSync(
    path.join(root, "test/typescript/fixtures/runtime-features/catalog.json"),
    "utf8",
  ),
);
const featureSizes = [];
for (const fixture of catalog.fixtures) {
  const base = fixture.group === "server" ? path.join(matrix, "server") : matrix;
  const entry = fixture.group === "server" ? "server/webhooks.js" : "index.js";
  const variants = {};
  for (const variant of ["selected", "full"]) {
    const sdk = path.join(base, variant + "-js", fixture.name);
    const modules = generatedModuleGraph(sdk, entry);
    const entryFile = path.join(sdk, "feature-size-entry.mjs");
    fs.writeFileSync(
      entryFile,
      fixture.group === "server"
        ? "export {createWebhookRouter} from './server/webhooks.js';"
        : "export {createClient} from './index.js';",
    );
    const built = await build({
      configFile: false,
      root: sdk,
      logLevel: "silent",
      build: {
        modulePreload: false,
        target: "es2022",
        minify: "oxc",
        write: false,
        rollupOptions: {
          input: entryFile,
          preserveEntrySignatures: "strict",
          output: { format: "es" },
        },
      },
    });
    assert.ok(!Array.isArray(built) && "output" in built);
    const chunks = built.output.filter((item) => item.type === "chunk");
    variants[variant] = {
      source: inventory(path.join(base, variant, fixture.name)),
      native: {
        files: modules.length,
        bytes: modules.reduce((sum, file) => sum + fs.statSync(file).size, 0),
        gzipBytes: modules.reduce(
          (sum, file) => sum + gzipSync(fs.readFileSync(file), { level: 6 }).length,
          0,
        ),
      },
      bundle: {
        chunks: chunks.length,
        bytes: chunks.reduce((sum, item) => sum + Buffer.byteLength(item.code), 0),
        gzipBytes: chunks.reduce((sum, item) => sum + gzipSync(item.code, { level: 6 }).length, 0),
      },
    };
  }
  featureSizes.push({ name: fixture.name, group: fixture.group, entry, variants });
}
const input = path.join(root, "test/typescript/fixtures/runtime-features/client/small-json.json");
const generatorFiles = [
  ...files(path.join(root, "internal")),
  ...files(path.join(root, "cmd/openapi-sdkgen")),
].filter(
  (file) =>
    (file.endsWith(".go") && !file.endsWith("_test.go")) ||
    (file.includes("/runtime/") && file.endsWith(".ts")),
);
const result = {
  measuredAt: new Date().toISOString(),
  commit: execFileSync("git", ["rev-parse", "HEAD"], { cwd: root, encoding: "utf8" }).trim(),
  dirty:
    execFileSync("git", ["status", "--porcelain"], { cwd: root, encoding: "utf8" }).trim().length >
    0,
  generatorSourceSHA256: hash(
    generatorFiles
      .map((file) => `${path.relative(root, file)}\0${hash(fs.readFileSync(file))}`)
      .join("\n"),
  ),
  input: path.relative(root, input),
  inputSHA256: hash(fs.readFileSync(input)),
  environment: {
    node: process.version,
    vite: packageVersion("vite"),
    rolldown: packageVersion("rolldown"),
    typescript: packageVersion("typescript"),
    target: "ES2022",
    platform: "browser",
    format: "ESM",
    minify: "oxc",
    modulePreload: false,
    gzipLevel: 6,
    nativeBundleCallsPassed: true,
  },
  source: {
    default: inventory(path.join(matrix, "selected", "small-json")),
    fullCapabilityControl: inventory(path.join(matrix, "full", "small-json")),
  },
  bundles,
  featureSizes,
};
const defaultBundle = bundles.find((item) => item.name === "default-root");
const fullBundle = bundles.find((item) => item.name === "full-capability-control");
assert.ok(defaultBundle.deployedGzipBytes < fullBundle.deployedGzipBytes);
assert.ok(result.source.default.runtime.files < result.source.fullCapabilityControl.runtime.files);
fs.writeFileSync(path.join(matrix, "size-results.json"), JSON.stringify(result, null, 2) + "\n");
console.log(
  JSON.stringify(
    {
      file: path.join(matrix, "size-results.json"),
      source: result.source,
      bundles: bundles.map(({ outputs, ...row }) => row),
    },
    null,
    2,
  ),
);
