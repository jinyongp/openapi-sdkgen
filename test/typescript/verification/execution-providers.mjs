// Check real Go-emitted providers as native ESM and from declarations alone.
// prepare-conformance generates these sources; nothing here synthesizes a provider.
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { inspectGenerated, sha256 } from "./catalog.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../package.json", import.meta.url));
const packageFile = require.resolve("typescript/package.json");
const typescript = JSON.parse(fs.readFileSync(packageFile, "utf8"));
const compiler = path.resolve(path.dirname(packageFile), typescript.bin.tsc);
const runID = randomUUID();
const output = path.join(root, ".tmp/execution-providers", runID);
fs.mkdirSync(output, { recursive: true });
const write = (filename, value) => {
  fs.mkdirSync(path.dirname(filename), { recursive: true });
  fs.writeFileSync(filename, value);
};
const run = (label, arguments_) => {
  const result = spawnSync(process.execPath, arguments_, {
    cwd: root,
    encoding: "utf8",
    timeout: 120000,
    maxBuffer: 8 * 1024 * 1024,
  });
  write(path.join(output, `${label}.log`), `${result.stdout ?? ""}${result.stderr ?? ""}`);
  assert.equal(result.status, 0, `${label}: ${result.stdout ?? ""}${result.stderr ?? ""}`);
  return result.stdout ?? "";
};
const inventories = {};
for (const fixture of ["lifecycle", "bundle-isolation", "baseline-oas31", "execution-media"]) {
  const generated = path.join(root, "test/typescript/fixtures/generated", fixture);
  const inventory = inspectGenerated(generated);
  inventories[fixture] = inventory.treeSha256;
  for (const filename of Object.keys(inventory.files).filter((name) => name.endsWith(".ts"))) {
    write(
      path.join(output, "source", fixture, filename),
      fs.readFileSync(path.join(generated, filename), "utf8").replaceAll("// @ts-nocheck\n", ""),
    );
  }
}
const witness = `
import { provider as json } from "./lifecycle/internal/executions/inline/post.js";
import { provider as streaming } from "./lifecycle/internal/executions/events/get.js";
import { createRequestContext } from "./lifecycle/internal/runtime/http-core.js";
const context = createRequestContext({baseURL:"https://example.test",fetch:async()=>Response.json({value:1})});
const call = json.bind(context);
void call({body:{value:1}});
void call.raw({body:{value:1}});
// @ts-expect-error This must still be the generated numeric body contract.
void call({body:{value:"wrong"}});
// @ts-expect-error Non-streaming operation does not acquire an unimplemented method.
call.stream;
const stream = streaming.bind(context).stream();
void stream[Symbol.asyncIterator];
type Expected = { readonly value: number };
async function consume() { for await (const item of stream) { const expected: Expected = item; void expected; } }
void consume;
import { provider as xml } from "./execution-media/internal/executions/xml/post.js";
import { provider as nested } from "./execution-media/internal/executions/nested/get.js";
import { provider as header } from "./execution-media/internal/executions/header/get.js";
import { provider as idless } from "./execution-media/internal/executions/plain/get.js";
const xmlCall = xml.bind(context);
void xmlCall({body:{id:"one",count:2}});
// @ts-expect-error XML uses the generated object input, not a pre-serialized string.
void xmlCall({body:"<item/>"});
// @ts-expect-error Referenced schema numeric fields retain their type.
void xmlCall({body:{id:"one",count:"wrong"}});
async function consumeMedia() {
  const payload: string = (await nested.bind(context)()).payload;
  const count: number = (await header.bind(context).raw()).headers["X-Item"].count;
  const id: string = (await idless.bind(context)()).id;
  void [payload, count, id];
}
void consumeMedia;
`;
write(path.join(output, "source/type-witness.ts"), witness);
write(path.join(output, "package.json"), '{"type":"module"}\n');
const options = {
  target: "ES2022",
  module: "NodeNext",
  moduleResolution: "NodeNext",
  lib: ["ES2022", "DOM", "DOM.Iterable"],
  types: [],
  strict: true,
  noUncheckedIndexedAccess: true,
  exactOptionalPropertyTypes: true,
  verbatimModuleSyntax: true,
  noEmitOnError: true,
  skipLibCheck: false,
};
write(
  path.join(output, "emit.json"),
  JSON.stringify({
    compilerOptions: {
      ...options,
      declaration: true,
      declarationDir: "declarations",
      outDir: "javascript",
      rootDir: "source",
    },
    include: ["source/**/*.ts"],
  }),
);
run("source-and-declaration-emit", [compiler, "--project", path.join(output, "emit.json")]);
write(path.join(output, "consumer.ts"), witness.replaceAll('from "./', 'from "./declarations/'));
write(
  path.join(output, "consumer.json"),
  JSON.stringify({ compilerOptions: { ...options, noEmit: true }, files: ["consumer.ts"] }),
);
const inputs = run("declarations-only-consumer", [
  compiler,
  "--project",
  path.join(output, "consumer.json"),
  "--listFiles",
]);
assert(
  !inputs
    .split(/\r?\n/)
    .some((filename) => filename.startsWith(path.join(output, "source") + path.sep)),
  "declaration consumer read implementation sources",
);

const runtime = `
import assert from "node:assert/strict";
import {provider as json} from "./javascript/lifecycle/internal/executions/inline/post.js";
import {provider as streaming} from "./javascript/lifecycle/internal/executions/events/get.js";
import {provider as schema} from "./javascript/bundle-isolation/internal/executions/bundle-isolation-sentinel/get.js";
import {createRequestContext} from "./javascript/lifecycle/internal/runtime/http-core.js";
import {createClient} from "./javascript/lifecycle/index.js";
let calls=[];
const options={baseURL:"https://example.test",authorization:"Bearer private",fetch:async(url,init)=>{
 calls.push({url:String(url),method:init.method,authorization:new Headers(init.headers).get("authorization"),body:init.body});
 return Response.json(JSON.parse(init.body),{headers:{"x-request-id":"one"}});
}};
const selected=json.bind(createRequestContext(options));
const full=createClient(options).$operations.echoInline;
const input={body:{value:9,nested:{flag:true}}};
assert.deepEqual(await selected(input),await full(input));
assert.deepEqual(calls[0],calls[1]);
assert.equal((await selected.raw(input)).request.id,"one");
const before=calls.length;
await assert.rejects(selected({body:{value:-1}}),{code:"REQUEST_ENCODE_FAILED"});
await assert.rejects(selected(input,{signal:AbortSignal.abort()}),{code:"REQUEST_ABORTED"});
assert.equal(calls.length,before);
const streamCall=streaming.bind(createRequestContext({baseURL:"https://example.test",fetch:async()=>new Response('{"value":1}\\n{"value":2}\\n',{headers:{"content-type":"application/x-ndjson"}})}));
const items=[]; const stream=streamCall.stream(); assert.equal(typeof stream.then,"undefined");
for await(const item of stream)items.push(item.value);
assert.deepEqual(items,[1,2]);
const shared=schema.bind(createRequestContext({baseURL:"https://example.test",fetch:async()=>Response.json({id:"one",mode:"bundle-enum-sentinel-01"})}));
assert.equal((await shared()).id,"one");
assert.equal(json.profile,"json");assert.equal(streaming.profile,"json-response-stream");
console.log(JSON.stringify({checks:["generated-json-full-parity","raw-metadata","schema-before-fetch","abort-before-fetch","native-sync-stream","referenced-schema-closure","profile-selection"],passed:7}));
`;
write(path.join(output, "native.mjs"), runtime);
const native = JSON.parse(run("native-esm", [path.join(output, "native.mjs")]));
const mediaScript = path.join(root, "test/typescript/verification/execution-media-native.mjs");
const mediaNative = JSON.parse(run("native-media", [mediaScript, path.join(output, "javascript")]));
const parser = createRequire(
  path.join(root, "test/typescript/node_modules/.pnpm/node_modules/package.json"),
)("@babel/parser");
function nativeImports(filename) {
  const seen = new Set();
  const pending = [filename];
  while (pending.length) {
    const current = pending.pop();
    if (seen.has(current)) continue;
    seen.add(current);
    assert(
      current.startsWith(path.join(output, "javascript") + path.sep),
      "provider graph escaped emitted JavaScript",
    );
    const ast = parser.parse(fs.readFileSync(current, "utf8"), { sourceType: "module" });
    for (const node of ast.program.body) {
      if (!node.source) continue;
      assert(node.source.value.startsWith("."), "unexpected external runtime dependency");
      pending.push(path.resolve(path.dirname(current), node.source.value));
    }
  }
  return [...seen].sort().map((name) => path.relative(path.join(output, "javascript"), name));
}
const graphs = {};
for (const relative of [
  "lifecycle/internal/executions/inline/post.js",
  "bundle-isolation/internal/executions/bundle-isolation-sentinel/get.js",
  "execution-media/internal/executions/json/post.js",
  "execution-media/internal/executions/custom/get.js",
  "execution-media/internal/executions/plain/get.js",
]) {
  const modules = nativeImports(path.join(output, "javascript", relative));
  assert(modules.some((name) => name.endsWith("/runtime/http-core.js")));
  assert(modules.some((name) => name.endsWith("/runtime/wire-engine.js")));
  for (const forbidden of [
    "/runtime/http.js",
    "/runtime/http-codecs.js",
    "/runtime/http-advanced.js",
    "/runtime/http-stream.js",
    "/runtime/codecs.js",
    "/runtime/wire-xml.js",
    "/runtime/streaming.js",
    "/schemas/wire.js",
    "/client/registry.js",
  ]) {
    assert(
      !modules.some((name) => name.endsWith(forbidden)),
      `native JSON provider retained ${forbidden}`,
    );
  }
  graphs[relative] = modules;
}
for (const [name, method] of [
  ["xml", "post"],
  ["header", "get"],
  ["parameter", "get"],
  ["embedded", "get"],
  ["nested", "get"],
  ["error", "get"],
  ["form", "post"],
  ["multipart", "post"],
  ["text", "post"],
]) {
  const relative = `execution-media/internal/executions/${name}/${method}.js`;
  const modules = nativeImports(path.join(output, "javascript", relative));
  assert(
    !modules.some(
      (file) => file.endsWith("/schemas/wire.js") || file.endsWith("/client/registry.js"),
    ),
  );
  if (!["form", "multipart", "text"].includes(name)) {
    assert(
      modules.some((file) => file.endsWith("/runtime/codecs.js")),
      `${name} lost XML implementation`,
    );
    assert(
      !modules.some(
        (file) =>
          file.endsWith("/runtime/http-stream.js") || file.endsWith("/runtime/http-advanced.js"),
      ),
      `${name} retained unrelated advanced implementation`,
    );
  } else {
    assert(
      modules.some((file) => file.endsWith("/runtime/http-codecs.js")),
      `${name} lost its general services`,
    );
  }
  graphs[relative] = modules;
}
for (const [relative, modules] of Object.entries(graphs)) {
  assert(
    !modules.some((file) => file.endsWith("/schemas/unused.js")),
    `${relative} retained an unused schema`,
  );
}
// Ensure the dependency check catches a full-runtime edge instead of only accepting the candidate.
const fullGraph = nativeImports(path.join(output, "javascript/lifecycle/internal/runtime/http.js"));
assert(fullGraph.some((name) => name.endsWith("/runtime/http-codecs.js")));
const report = {
  status: "pass",
  runID,
  node: process.version,
  typescript: typescript.version,
  source:
    "Actual Go-emitted artifacts verified against output ownership manifest hashes; verification copies remove only the generated @ts-nocheck directive.",
  checkedImplementationBodies: true,
  scope:
    "Four emitted fixture trees, strict source/declaration emit, d.ts-only downstream, JSON/stream and ten media native Node ESM scenarios. Not browser delivery, public selection, or lazy Link completion.",
  inventories,
  graphs,
  fullRuntimeGraphNegativeControl: true,
  witnessSHA256: sha256(witness),
  nativeSHA256: sha256(runtime),
  native,
  mediaNative,
  mediaNativeSHA256: sha256(fs.readFileSync(mediaScript)),
  consumerDeclarationFiles: inputs
    .split(/\r?\n/)
    .filter((filename) => filename.startsWith(path.join(output, "declarations") + path.sep)).length,
};
write(path.join(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
write(
  path.join(root, ".tmp/execution-providers/latest.json"),
  JSON.stringify({ runID, report: path.relative(root, path.join(output, "report.json")) }) + "\n",
);
console.log(`ok emitted provider native/declaration checks (${runID})`);
