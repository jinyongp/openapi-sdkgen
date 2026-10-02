import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";
import { fileURLToPath, pathToFileURL } from "node:url";
import { randomUUID, createHash } from "node:crypto";
import { parseArgs } from "node:util";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const { values } = parseArgs({
  options: {
    generator: { type: "string" },
    benchmark: { type: "string" },
    "graph-source": { type: "string" },
  },
});
assert(values.generator, "Pass a built generator");
const generator = path.resolve(root, values.generator);
const directory = path.join(root, ".tmp/named-clients-verification", randomUUID());
const sdk = path.join(directory, "sdk");
const require = createRequire(path.join(root, "test/typescript/package.json"));
const { build } = await import(pathToFileURL(require.resolve("vite")).href);
const tsPackagePath = require.resolve("typescript/package.json");
const tsPackage = JSON.parse(fs.readFileSync(tsPackagePath, "utf8"));
const tsc = path.resolve(path.dirname(tsPackagePath), tsPackage.bin.tsc);
const write = (name, text) => {
  fs.mkdirSync(path.dirname(name), { recursive: true });
  fs.writeFileSync(name, text);
};
const reference = (name) => ({ $ref: "#/components/schemas/" + name });
const stringMarker = (marker) => ({ type: "string", enum: [marker] });
const object = (properties) => ({
  type: "object",
  required: Object.keys(properties),
  properties,
  additionalProperties: false,
});

{
  const schemas = {
    Shared: object({ marker: stringMarker("SHARED_ALL_CLIENTS") }),
    SharedAB: object({ marker: stringMarker("SHARED_A_B_ONLY") }),
    AOnly: object({
      marker: stringMarker("A_ONLY_SCHEMA"),
      shared: reference("Shared"),
      pair: reference("SharedAB"),
    }),
    BOnly: object({
      marker: stringMarker("B_ONLY_SCHEMA"),
      shared: reference("Shared"),
      pair: reference("SharedAB"),
    }),
    COnly: object({
      marker: stringMarker("C_ONLY_SCHEMA"),
      shared: reference("Shared"),
      child: reference("CChild"),
      directed: reference("Directed"),
    }),
    CChild: object({ marker: stringMarker("C_CHILD_SCHEMA") }),
    Directed: object({
      secret: { ...reference("InputOnly"), writeOnly: true },
      result: { ...reference("OutputOnly"), readOnly: true },
    }),
    InputOnly: object({ marker: stringMarker("AB_PROJECTED_INPUT") }),
    OutputOnly: object({ marker: stringMarker("C_PROJECTED_OUTPUT") }),
  };
  const paths = {};
  for (const [name, schema] of [
    ["a", "AOnly"],
    ["b", "BOnly"],
    ["c", "COnly"],
    ["common", "Shared"],
  ]) {
    paths["/" + name] = {
      get: {
        operationId: name + "Get",
        responses: {
          200: {
            description: "OK",
            content: { "application/json": { schema: reference(schema) } },
          },
        },
      },
    };
  }
  paths["/input"] = {
    post: {
      operationId: "inputGet",
      requestBody: {
        required: true,
        content: { "application/json": { schema: reference("Directed") } },
      },
      responses: { 204: { description: "OK" } },
    },
  };
  const document = {
    openapi: "3.2.1",
    info: { title: "Named clients delivery", version: "1" },
    paths,
    components: { schemas },
  };
  write(path.join(directory, "input.json"), JSON.stringify(document, null, 2));
  const linked = structuredClone(document);
  linked.paths["/c"].get.responses["200"].links = { followB: { operationId: "bGet" } };
  write(path.join(directory, "input-linked.json"), JSON.stringify(linked, null, 2));
}

write(path.join(directory, "package.json"), '{"type":"module"}\n');
const clients = { a: ["a", "common", "input"], b: ["b"], c: ["c", "common"] };
const generation = {};
for (const suffix of ["", "-linked"]) {
  const config = path.join(directory, "sdk" + suffix + ".toml");
  const groups = Object.entries(clients)
    .map(
      ([name, operations]) =>
        `[clients.${name}.selection]\noperations = ${JSON.stringify(operations.map((value) => value + "Get"))}`,
    )
    .join("\n");
  write(
    config,
    `source = "./input${suffix}.json"\ntarget = "typescript"\noutput = "./sdk${suffix}"\n${groups}\n`,
  );
  const started = performance.now();
  const result = spawnSync(generator, ["generate", "--config", config], {
    cwd: root,
    encoding: "utf8",
    timeout: 120000,
  });
  write(
    path.join(directory, "generation" + suffix + ".log"),
    (result.stdout ?? "") + (result.stderr ?? ""),
  );
  assert.equal(result.status, 0, "Named TOML generation failed; see generation log");
  generation[suffix || "plain"] = {
    wallMS: performance.now() - started,
    inputSHA256: createHash("sha256")
      .update(fs.readFileSync(path.join(directory, "input" + suffix + ".json")))
      .digest("hex"),
  };
}
write(
  path.join(sdk, "clients/c/consumer.ts"),
  `
import { createClient, type Components, type ComponentOutput } from './index.js';
const api = createClient({ baseURL: 'https://c.test' });
void api.c.get();
void api.common.get();
// @ts-expect-error A resource belongs to another client.
void api.a.get();
// @ts-expect-error B operation belongs to another client.
void api.$operations.bGet();
// @ts-expect-error B route belongs to another client.
void api.$routes['GET /b']();
// @ts-expect-error Other clients' component models are unavailable.
type AOnly = Components['AOnly'];
// @ts-expect-error The output projection excludes the input-only field.
type Secret = ComponentOutput<'Directed'>['secret'];
`,
);
const sourceFiles = [];
function walk(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const file = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(file);
    else if (file.endsWith(".ts")) sourceFiles.push(file);
  }
}
walk(sdk);
for (const file of sourceFiles)
  write(file, fs.readFileSync(file, "utf8").replace(/^\/\/ @ts-nocheck\r?\n/gm, ""));
const javascript = path.join(directory, "javascript");
write(
  path.join(directory, "tsconfig.json"),
  JSON.stringify(
    {
      compilerOptions: {
        target: "ES2022",
        module: "NodeNext",
        moduleResolution: "NodeNext",
        lib: ["ES2022", "DOM", "DOM.Iterable"],
        types: [],
        strict: true,
        noUncheckedIndexedAccess: true,
        verbatimModuleSyntax: true,
        skipLibCheck: false,
        rootDir: sdk,
        outDir: javascript,
      },
      include: ["sdk/**/*.ts"],
    },
    null,
    2,
  ),
);
const compile = spawnSync(
  process.execPath,
  [tsc, "--project", path.join(directory, "tsconfig.json")],
  { encoding: "utf8", timeout: 60000 },
);
write(path.join(directory, "typecheck.log"), (compile.stdout ?? "") + (compile.stderr ?? ""));
assert.equal(compile.status, 0, "Strict source/consumer typecheck failed; see typecheck.log");

// Instrument only native JS copies; bundle tests read the uninstrumented TS source.
for (const file of sourceFiles) {
  const relative = path.relative(sdk, file).replace(/\.ts$/, ".js");
  const compiled = path.join(javascript, relative);
  if (!fs.existsSync(compiled)) continue;
  fs.appendFileSync(
    compiled,
    `\nglobalThis.__clientsProbeEvaluations.add(${JSON.stringify(relative)});\n`,
  );
}
globalThis.__clientsProbeEvaluations = new Set();
const moduleC = await import(pathToFileURL(path.join(javascript, "clients/c/index.js")).href);
const cImports = [...globalThis.__clientsProbeEvaluations].sort();
assert(!cImports.some((file) => /internal\/(operations|executions)\/(a|b)\//.test(file)));
assert(!cImports.some((file) => /internal\/schemas\/(aonly|bonly|shared-ab)\.js$/.test(file)));
assert(
  !cImports.some((file) =>
    /schema-projections\/(input|output)\/(aonly|bonly|shared-ab)\.js$/.test(file),
  ),
);
assert(cImports.includes("internal/schema-projections/output/cchild.js"));
assert(cImports.includes("internal/schema-projections/output/shared.js"));
assert(!cImports.some((file) => file.includes("schema-projections/input/")));
assert(!cImports.some((file) => file.includes("inputonly.js")));

const shared = { marker: "SHARED_ALL_CLIENTS" };
const responses = {
  "/a": { marker: "A_ONLY_SCHEMA", shared, pair: { marker: "SHARED_A_B_ONLY" } },
  "/b": { marker: "B_ONLY_SCHEMA", shared, pair: { marker: "SHARED_A_B_ONLY" } },
  "/c": {
    marker: "C_ONLY_SCHEMA",
    shared,
    child: { marker: "C_CHILD_SCHEMA" },
    directed: { result: { marker: "C_PROJECTED_OUTPUT" } },
  },
  "/common": shared,
};
const requests = [];
const options = (name) => ({
  baseURL: "https://" + name + ".test",
  authorization: "Bearer " + name,
  fetch: async (url, init) => {
    const parsed = new URL(String(url));
    requests.push({
      url: String(url),
      authorization: new Headers(init.headers).get("authorization"),
    });
    return Response.json(responses[parsed.pathname]);
  },
});
const c = moduleC.createClient(options("c"));
assert.deepEqual(Object.keys(c.$routes).sort(), ["GET /c", "GET /common"]);
assert.equal(Object.hasOwn(c, "a"), false);
assert.equal(Object.hasOwn(c, "b"), false);
assert.deepEqual(JSON.parse(JSON.stringify(await c.c.get())), responses["/c"]);
const invalid = moduleC.createClient({
  ...options("invalid"),
  fetch: async () => Response.json({ ...responses["/c"], child: { marker: 123 } }),
});
await assert.rejects(invalid.c.get(), (error) => error.code === "RESPONSE_DECODE_FAILED");
const moduleA = await import(pathToFileURL(path.join(javascript, "clients/a/index.js")).href);
const a = moduleA.createClient(options("a"));
assert.notEqual(a.common.get, c.common.get);
await a.common.get();
await c.common.get();
assert.deepEqual(
  requests.map((item) => item.authorization),
  ["Bearer c", "Bearer a", "Bearer c"],
);
assert.equal(requests[1].url, "https://a.test/common");
assert.equal(requests[2].url, "https://c.test/common");

async function bundle(label, entrypoints) {
  const result = await build({
    root: directory,
    configFile: false,
    logLevel: "silent",
    build: {
      target: "es2022",
      write: false,
      minify: false,
      rollupOptions: {
        input: entrypoints,
        preserveEntrySignatures: "strict",
        output: {
          format: "es",
          entryFileNames: "[name].mjs",
          chunkFileNames: "chunks/[name]-[hash].mjs",
        },
      },
    },
  });
  assert(!Array.isArray(result) && "output" in result);
  const chunks = result.output.filter((item) => item.type === "chunk");
  for (const chunk of chunks)
    write(path.join(directory, "bundles", label, chunk.fileName), chunk.code);
  return chunks;
}
function staticGraph(chunks, entry) {
  const byName = new Map(chunks.map((chunk) => [chunk.fileName, chunk]));
  const seen = new Set(),
    pending = [entry];
  while (pending.length) {
    const name = pending.pop();
    if (seen.has(name)) continue;
    seen.add(name);
    const chunk = byName.get(name);
    assert(chunk, "Missing chunk " + name);
    pending.push(...chunk.imports);
  }
  return [...seen].sort();
}
const entryC = path.join(sdk, "clients/c/index.ts");
const alone = await bundle("c-only", { c: entryC });
const entries = Object.fromEntries(
  Object.keys(clients).map((name) => [name, path.join(sdk, "clients", name, "index.ts")]),
);
const together = await bundle("all-clients", entries);
const graphC = staticGraph(together, "c.mjs");
const codeC = alone.map((chunk) => chunk.code).join("\n");
const loadedC = together
  .filter((chunk) => graphC.includes(chunk.fileName))
  .map((chunk) => chunk.code)
  .join("\n");
const forbiddenMarkers = [
  "A_ONLY_SCHEMA",
  "B_ONLY_SCHEMA",
  "SHARED_A_B_ONLY",
  "AB_PROJECTED_INPUT",
];
assert(
  forbiddenMarkers.every((marker) => !codeC.includes(marker)),
  "C-only bundle includes another client schema",
);
assert(
  ["C_ONLY_SCHEMA", "C_CHILD_SCHEMA", "SHARED_ALL_CLIENTS", "C_PROJECTED_OUTPUT"].every((marker) =>
    codeC.includes(marker),
  ),
  "C bundle lost a required schema",
);
const full = await bundle("full-client-control", { full: path.join(sdk, "index.ts") });
const fullCode = full.map((chunk) => chunk.code).join("\n");
assert(
  forbiddenMarkers.every((marker) => fullCode.includes(marker)),
  "Negative control did not retain full-client schemas",
);
const sharedOwners = together.filter((chunk) =>
  Object.keys(chunk.modules).some((file) =>
    file.endsWith("/internal/schema-projections/output/shared.ts"),
  ),
);
const commonOwners = together.filter((chunk) =>
  Object.keys(chunk.modules).some((file) => file.endsWith("/internal/executions/common/get.ts")),
);
assert.equal(sharedOwners.length, 1, "Common schema emitted into more than one chunk");
assert.equal(commonOwners.length, 1, "Overlapping operation emitted into more than one chunk");
assert(
  forbiddenMarkers.every((marker) => !loadedC.includes(marker)),
  "C multi-entry dependency graph includes another client schema",
);
const runtimeOwners = new Map();
for (const chunk of together)
  for (const file of Object.keys(chunk.modules)) {
    if (!file.includes("/internal/runtime/")) continue;
    assert(!runtimeOwners.has(file), "Runtime module duplicated: " + file);
    runtimeOwners.set(file, chunk.fileName);
  }
write(
  path.join(directory, "app.ts"),
  `
export const loadA = () => import('./sdk/clients/a/index.js');
export const loadB = () => import('./sdk/clients/b/index.js');
export const loadC = () => import('./sdk/clients/c/index.js');
`,
);
const pages = await bundle("lazy-pages", { app: path.join(directory, "app.ts") });
const pageC = pages.find((chunk) => chunk.facadeModuleId === entryC);
assert(pageC, "Missing C page dynamic chunk");
const pageCGraph = staticGraph(pages, pageC.fileName);
const pageCCode = pages
  .filter((chunk) => pageCGraph.includes(chunk.fileName))
  .map((chunk) => chunk.code)
  .join("\n");
assert(
  forbiddenMarkers.every((marker) => !pageCCode.includes(marker)),
  "Lazy C page graph includes another client schema",
);
const bundleC = await import(pathToFileURL(path.join(directory, "bundles/c-only/c.mjs")).href);
assert.deepEqual(
  JSON.parse(JSON.stringify(await bundleC.createClient(options("bundled-c")).c.get())),
  responses["/c"],
);

const linkedSdk = path.join(directory, "sdk-linked");
const linkedSource = [];
function walkLinked(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const file = path.join(dir, entry.name);
    if (entry.isDirectory()) walkLinked(file);
    else if (file.endsWith(".ts")) linkedSource.push(file);
  }
}
walkLinked(linkedSdk);
for (const file of linkedSource)
  write(file, fs.readFileSync(file, "utf8").replace(/^\/\/ @ts-nocheck\r?\n/gm, ""));
const linkedJS = path.join(directory, "javascript-linked");
const linkedConfig = JSON.parse(fs.readFileSync(path.join(directory, "tsconfig.json"), "utf8"));
linkedConfig.compilerOptions.rootDir = linkedSdk;
linkedConfig.compilerOptions.outDir = linkedJS;
linkedConfig.include = ["sdk-linked/**/*.ts"];
write(path.join(directory, "tsconfig-linked.json"), JSON.stringify(linkedConfig, null, 2));
const linkedCompile = spawnSync(
  process.execPath,
  [tsc, "--project", path.join(directory, "tsconfig-linked.json")],
  { encoding: "utf8", timeout: 60000 },
);
write(
  path.join(directory, "typecheck-linked.log"),
  (linkedCompile.stdout ?? "") + (linkedCompile.stderr ?? ""),
);
assert.equal(linkedCompile.status, 0, "Linked strict typecheck failed; see typecheck-linked.log");
for (const file of linkedSource) {
  const relative = path.relative(linkedSdk, file).replace(/\.ts$/, ".js");
  fs.appendFileSync(
    path.join(linkedJS, relative),
    `\nglobalThis.__clientsProbeLinkEvaluations.add(${JSON.stringify(relative)});\n`,
  );
}
globalThis.__clientsProbeLinkEvaluations = new Set();
const linkedModule = await import(pathToFileURL(path.join(linkedJS, "clients/c/index.js")).href);
const linkedClient = linkedModule.createClient(options("linked-c"));
const linkedResponse = await linkedClient.c.get.raw();
assert.equal(
  globalThis.__clientsProbeLinkEvaluations.has("internal/schema-projections/output/bonly.js"),
  false,
);
assert.deepEqual(Object.keys(linkedClient.$routes).sort(), ["GET /c", "GET /common"]);
const linkedResult = await linkedClient.c.get.links.followB(linkedResponse);
assert.deepEqual(JSON.parse(JSON.stringify(linkedResult)), responses["/b"]);
assert.equal(
  globalThis.__clientsProbeLinkEvaluations.has("internal/schema-projections/output/bonly.js"),
  true,
);
assert.equal(
  globalThis.__clientsProbeLinkEvaluations.has("internal/schema-projections/output/aonly.js"),
  false,
);
assert.equal(Object.hasOwn(linkedClient, "b"), false);
assert.deepEqual(Object.keys(linkedClient.$routes).sort(), ["GET /c", "GET /common"]);
assert.equal(requests.at(-1).url, "https://linked-c.test/b");
assert.equal(requests.at(-1).authorization, "Bearer linked-c");
const linkedChunks = await bundle("linked-c", { c: path.join(linkedSdk, "clients/c/index.ts") });
const linkedInitial = staticGraph(linkedChunks, "c.mjs");
const linkedInitialCode = linkedChunks
  .filter((chunk) => linkedInitial.includes(chunk.fileName))
  .map((chunk) => chunk.code)
  .join("\n");
assert(
  forbiddenMarkers.every((marker) => !linkedInitialCode.includes(marker)),
  "Link target schema entered initial C bundle",
);
assert(
  linkedChunks
    .filter((chunk) => !linkedInitial.includes(chunk.fileName))
    .some((chunk) => chunk.code.includes("B_ONLY_SCHEMA")),
  "Link target was not emitted separately",
);
const report = {
  version: 1,
  sourceCommit: spawnSync("git", ["rev-parse", "HEAD"], {
    cwd: root,
    encoding: "utf8",
  }).stdout.trim(),
  measuredAt: new Date().toISOString(),
  node: process.version,
  typescript: tsPackage.version,
  scope:
    "Actual TOML-configured named entries emitted by the built generator, strict source/consumer checks and native/default Vite production delivery.",
  generation,
  strictTypecheck: "pass",
  resourceRouteOperationVisibility: "pass",
  transitiveResponseDecoding: "pass",
  invalidTransitiveResponseRejected: "pass",
  clientCredentialIsolation: "pass",
  nativeCImports: cImports,
  cOnlyBundle: {
    privateSchemaExclusion: "pass",
    requiredSchemaInclusion: "pass",
    fullClientNegativeControl: "pass",
    chunks: alone.map((chunk) => ({ path: chunk.fileName, bytes: Buffer.byteLength(chunk.code) })),
  },
  multiEntryBundle: {
    staticCChunks: graphC,
    otherClientSchemaMarkers: forbiddenMarkers.filter((marker) => loadedC.includes(marker)),
    commonSchemaChunk: sharedOwners[0].fileName,
    commonOperationChunk: commonOwners[0].fileName,
    commonModuleDeduplication: "pass",
    runtimeModuleDeduplication: "pass",
    runtimeModules: runtimeOwners.size,
  },
  lazyPages: { cChunks: pageCGraph, otherClientSchemaExclusion: "pass" },
  links: {
    targetLoadedOnHelperInvocation: "pass",
    targetPublicExposureExcluded: "pass",
    targetUsesSourceClientConfiguration: "pass",
    bundledTargetInitiallyDeferred: "pass",
    initialCChunks: linkedInitial,
  },
};
if (values["graph-source"]) report.graph = await measureGraph();
write(path.join(directory, "report.json"), JSON.stringify(report, null, 2) + "\n");
console.log(
  JSON.stringify({
    typecheck: report.strictTypecheck,
    nativeCPrivateSchemaExclusion: "pass",
    cOnlyBundlePrivateSchemaExclusion: "pass",
    sharedModuleDeduplication: "pass",
    multiEntryCForeignSchemaMarkers: report.multiEntryBundle.otherClientSchemaMarkers,
    report: path.join(directory, "report.json"),
  }),
);

async function measureGraph() {
  assert(values.benchmark, "Pass the built phase measurement driver for Graph");
  const source = path.resolve(root, values["graph-source"]);
  const manifest = fs.readFileSync(
    path.join(root, "test/compatibility/selections/microsoft-graph-beta.toml"),
    "utf8",
  );
  const expectedHash = manifest.match(/^input_sha256 = "([a-f0-9]{64})"$/m)[1];
  const actualHash = createHash("sha256").update(fs.readFileSync(source)).digest("hex");
  assert.equal(actualHash, expectedHash, "Graph source differs from the pinned corpus");
  const routes = [...manifest.matchAll(/^\s+"([A-Z]+ [^"]+)",$/gm)].map((match) => match[1]);
  assert.equal(routes.length, 9);
  const cases = {};
  for (const named of [false, true]) {
    const label = named ? "graph-clients" : "graph-selection";
    const output = path.join(directory, label);
    const config = path.join(directory, label + ".toml");
    const groups = named
      ? Object.entries({
          users: routes.slice(0, 5),
          groups: routes.slice(5, 7),
          drives: routes.slice(7),
        })
          .map(
            ([name, selected]) =>
              `[clients.${name}.selection]\nroutes = ${JSON.stringify(selected)}`,
          )
          .join("\n")
      : "";
    write(
      config,
      `source = ${JSON.stringify(source)}\ntarget = "typescript"\noutput = ${JSON.stringify(output)}\n[selection]\nroutes = ${JSON.stringify(routes)}\n${groups}\n`,
    );
    const metrics = path.join(directory, label + "-resources.json");
    const started = performance.now();
    const result = spawnSync(
      "/usr/bin/time",
      [
        "-f",
        '{"userSeconds":%U,"systemSeconds":%S,"peakRSSKiB":%M}',
        "-o",
        metrics,
        path.resolve(root, values.benchmark),
        "--config",
        config,
      ],
      { cwd: root, encoding: "utf8", timeout: 600000, maxBuffer: 1024 * 1024 },
    );
    write(path.join(directory, label + ".log"), (result.stdout ?? "") + (result.stderr ?? ""));
    assert.equal(result.status, 0, label + " generation failed; see log");
    const generationWallMS = performance.now() - started;
    const measured = JSON.parse(result.stdout);
    assert.equal(measured.compileCalls, 1);
    assert.equal(measured.prepareCalls, 1);
    assert.equal(measured.moduleAnalysisPasses, 1);
    assert.equal(measured.operationModules, 9);
    const publicEntries = named ? ["users", "groups", "drives"] : ["root"];
    const inventories = {};
    for (const name of publicEntries) {
      const entry = named
        ? path.join(output, "clients", name, "index.ts")
        : path.join(output, "index.ts");
      const bundleStarted = performance.now();
      const chunks = await bundle(label + "-" + name, { entry });
      const initial = staticGraph(chunks, "entry.mjs");
      const loaded = chunks.filter((chunk) => initial.includes(chunk.fileName));
      inventories[name] = {
        bundleWallMS: performance.now() - bundleStarted,
        initialBytes: loaded.reduce((sum, chunk) => sum + Buffer.byteLength(chunk.code), 0),
        initialChunks: initial,
        totalChunks: chunks.length,
        modules: [...new Set(loaded.flatMap((chunk) => Object.keys(chunk.modules)))]
          .map((file) => path.relative(output, file))
          .sort(),
      };
    }
    cases[named ? "clients" : "selection"] = {
      ...measured,
      ...JSON.parse(fs.readFileSync(metrics, "utf8")),
      wallMS: generationWallMS,
      entries: inventories,
    };
  }
  return {
    inputSHA256: actualHash,
    inputBytes: fs.statSync(source).size,
    routes,
    scope:
      "Same pinned input and 9-route root selection; shared compiler/target publication API driver, with and without three named clients. Phase/resource timings cover generation; Vite dependency inventories are recorded separately.",
    cases,
  };
}
