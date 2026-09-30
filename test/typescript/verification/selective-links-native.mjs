// Verify real generated Link modules in native ESM and a self-contained static bundle.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { pathToFileURL } from "node:url";

const javascript = path.resolve(process.argv[2]);
const directory = path.join(path.dirname(javascript), "selective-links-native");
fs.mkdirSync(directory, { recursive: true });
const sdk = "../javascript/selection-links";
const nativeEntry = path.join(directory, "native.mjs");
const staticEntry = path.join(directory, "static.mjs");
fs.writeFileSync(
  nativeEntry,
  `
import * as api from ${JSON.stringify(sdk + "/selective/index.js")};
export async function prepare(options) { return api.createClient({...options,operations:await api.loadOperations([api.operations.getSource])}); }
export async function ready(options) { return api.createClient({...options,operations:await api.loadOperations([api.operations.getSource,api.operations.getItem])}); }
export async function failure(options) { return api.createClient({...options,operations:await api.loadOperations([api.operations.failureSource])}); }
`,
);
fs.writeFileSync(
  staticEntry,
  `
import * as api from ${JSON.stringify(sdk + "/selective/index.js")};
import {operation as source} from ${JSON.stringify(sdk + "/selective/operations/source/get.js")};
export async function prepare(options) { return api.createClient({...options,operations:await api.loadOperations([source])}); }
export async function ready(options) { const {operation:item}=await import(${JSON.stringify(sdk + "/selective/operations/items/by-id/get.js")}); return api.createClient({...options,operations:await api.loadOperations([source,item])}); }
export async function failure(options) { const {operation:ref}=await import(${JSON.stringify(sdk + "/selective/operations/failure-source/get.js")}); return api.createClient({...options,operations:await api.loadOperations([ref])}); }
`,
);

async function withoutFiles(files, run) {
  const moved = [];
  try {
    for (const filename of [...new Set(files)]) {
      const temporary = filename + ".withheld";
      assert(!fs.existsSync(temporary));
      fs.renameSync(filename, temporary);
      moved.push([filename, temporary]);
    }
    return await run();
  } finally {
    for (const [filename, temporary] of moved.reverse()) fs.renameSync(temporary, filename);
  }
}

async function exercise(entry, privateFiles, unavailableFiles) {
  const requests = [];
  const fetch = async (url, init) => {
    requests.push({
      url: String(url),
      method: init.method,
      authorization: new Headers(init.headers).get("authorization"),
    });
    if (String(url).endsWith("/failure-source") || String(url).endsWith("/unavailable"))
      return new Response(null, { status: 204 });
    if (String(url).endsWith("/xml"))
      return new Response("<item><id>xml</id></item>", {
        headers: { "content-type": "application/xml" },
      });
    return Response.json({ id: "a/b" });
  };
  let module, first, second, one, two;
  await withoutFiles(privateFiles, async () => {
    module = await import(pathToFileURL(entry).href);
    [first, second] = await Promise.all([
      module.prepare({ baseURL: "https://first.test", authorization: "Bearer first", fetch }),
      module.prepare({ baseURL: "https://second.test", authorization: "Bearer second", fetch }),
    ]);
    assert.equal(requests.length, 0, "preparation sent an API request");
    [one, two] = await Promise.all([first.source.get.raw(), second.source.get.raw()]);
    assert.deepEqual(Object.keys(first.$routes), ["GET /source"]);
    assert.deepEqual(Object.keys(first.$links), ["getSource"]);
    assert.equal(first.$links.getSource, first.source.get.links);
  });
  const linked = await Promise.all([
    first.$links.getSource.follow(one),
    second.source.get.links.follow(two),
  ]);
  assert.deepEqual(
    linked.map((value) => value.id),
    ["a/b", "a/b"],
  );
  assert.deepEqual(
    requests.slice(2, 4).sort((a, b) => a.url.localeCompare(b.url)),
    [
      { url: "https://first.test/items/a%2Fb", method: "GET", authorization: "Bearer first" },
      { url: "https://second.test/items/a%2Fb", method: "GET", authorization: "Bearer second" },
    ],
  );
  assert.equal((await first.source.get.links.xml(one)).id, "xml");
  assert.equal(Object.hasOwn(first.$routes, "GET /xml"), false);
  assert.equal(Object.hasOwn(first.$operations, "getItem"), false);
  assert.equal(Object.hasOwn(first.$links, "getItem"), false);
  const ready = await module.ready({ baseURL: "https://ready.test", fetch });
  const target = await ready.items("a/b").get.raw();
  assert.equal((await ready.items("a/b").get.links.back(target)).id, "a/b");
  assert.equal(ready.$links.getItem, ready.$operations.getItem.links);
  const before = requests.length;
  await assert.rejects(
    first.source.get.links.follow(one, { options: { signal: AbortSignal.abort() } }),
    { code: "REQUEST_ABORTED" },
  );
  assert.equal(requests.length, before);
  await withoutFiles(unavailableFiles, async () => {
    const failed = await module.failure({ baseURL: "https://failure.test", fetch });
    const raw = await failed.failureSource.get.raw();
    assert.equal(raw.status, 204);
    assert.equal(raw.contentType, undefined);
    const count = requests.length;
    await assert.rejects(
      failed.$links.failureSource.follow(raw),
      (error) => error.stage === "MODULE_LOAD" && error.cause instanceof Error,
    );
    assert.equal(requests.length, count, "a missing code module must not send the target request");
  });
  return {
    status: "pass",
    privateFilesWithheld: privateFiles.length,
    missingTargetFiles: unavailableFiles.length,
    checks: [
      "prepare-with-private-files-absent",
      "two-client-authentication",
      "exact-root-resource-link-aliases",
      "private-target-not-public",
      "idless-xml-target",
      "ready-cyclic-target",
      "pre-abort-no-request",
      "bodyless-raw-response",
      "original-module-load-cause",
    ],
  };
}

const generated = path.join(javascript, "selection-links");
const native = await exercise(
  nativeEntry,
  [
    path.join(generated, "internal/executions/items/by-id/get.js"),
    path.join(generated, "internal/executions/xml/get.js"),
    path.join(generated, "internal/runtime/codecs.js"),
  ],
  [path.join(generated, "internal/executions/unavailable/get.js")],
);

const require = createRequire(new URL("../package.json", import.meta.url));
const viteRequire = createRequire(fs.realpathSync(require.resolve("vite/package.json")));
const { rolldown } = await import(pathToFileURL(viteRequire.resolve("rolldown")).href);
const build = await rolldown({
  input: staticEntry,
  platform: "browser",
  treeshake: true,
  onwarn(warning) {
    throw Error(warning.message);
  },
});
const bundleDirectory = path.join(directory, "bundle");
let result;
try {
  result = await build.write({
    dir: bundleDirectory,
    format: "esm",
    entryFileNames: "entry.mjs",
    chunkFileNames: "chunks/[name]-[hash].mjs",
    minify: true,
  });
} finally {
  await build.close();
}
const chunks = result.output.filter((value) => value.type === "chunk");
const targetChunks = (endings) =>
  chunks
    .filter((chunk) =>
      Object.keys(chunk.modules).some((filename) =>
        endings.some((ending) => filename.endsWith(ending)),
      ),
    )
    .map((chunk) => path.join(bundleDirectory, chunk.fileName));
const privateChunks = targetChunks([
  "/internal/executions/items/by-id/get.js",
  "/internal/executions/xml/get.js",
  "/internal/runtime/codecs.js",
]);
const unavailableChunks = targetChunks(["/internal/executions/unavailable/get.js"]);
assert(
  privateChunks.length > 0 && unavailableChunks.length > 0,
  "target implementations were not found in bundled output",
);
assert(
  !privateChunks.includes(path.join(bundleDirectory, "entry.mjs")),
  "Link targets were pulled into the eager entry chunk",
);
const bundled = await exercise(
  path.join(bundleDirectory, "entry.mjs"),
  privateChunks,
  unavailableChunks,
);
assert(
  !fs.existsSync(path.join(bundleDirectory, "lookup")),
  "the static bundle unexpectedly needed a copied lookup tree",
);
console.log(
  JSON.stringify({
    status: "pass",
    scope:
      "Actual generated native and static-bundled Link execution. Withholding physical target files proves preparation does not load them. API fetch is injected; this is not browser HTTP transfer measurement.",
    native,
    bundled,
    chunks: chunks.map((chunk) => ({
      file: chunk.fileName,
      imports: chunk.imports,
      dynamicImports: chunk.dynamicImports,
    })),
  }),
);
