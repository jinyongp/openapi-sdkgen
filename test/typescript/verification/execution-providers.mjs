// Check real Go-emitted providers as native ESM and from declarations alone.
// prepare-conformance generates these sources; nothing here synthesizes a provider.
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { inspectGenerated, sha256, loadCatalog } from "./catalog.mjs";
import { strictCompilerOptions, assertCheckedSources } from "./strict-options.mjs";
import { verifyResourceMembership } from "./resource-membership.mjs";

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
const checkedFiles = [];
for (const fixture of new Set([
  "lifecycle",
  "bundle-isolation",
  "baseline-oas31",
  "execution-media",
  "client",
  "selection-public",
  "selection-links",
  "discriminator-dependencies",
  ...loadCatalog()
    .local.filter((entry) => entry.profiles.includes("conformance") && !entry.expectedFailure)
    .map((entry) => entry.output),
])) {
  const generated = path.join(root, "test/typescript/fixtures/generated", fixture);
  const inventory = inspectGenerated(generated);
  inventories[fixture] = inventory.treeSha256;
  for (const filename of Object.keys(inventory.files).filter((name) => name.endsWith(".ts"))) {
    write(
      path.join(output, "source", fixture, filename),
      fs.readFileSync(path.join(generated, filename), "utf8").replaceAll("// @ts-nocheck\n", ""),
    );
    checkedFiles.push(path.join(output, "source", fixture, filename));
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
import type { ResourceMethod as LocalResource, Input as LocalInput, Output as LocalOutput, RawResponse as LocalRaw } from "./lifecycle/internal/operations/inline/post.js";
import type { OperationPublicType } from "./lifecycle/internal/runtime/contract-types.js";
import type { ResourceCall as PublicResource, RouteInput, RouteOutput, RouteRawResponse, OperationInput } from "./lifecycle/internal/routes/helpers.js";
type Same<A, B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false;
type AssertSame<Value extends true> = Value;
type LocalContractIdentity = [
  AssertSame<Same<LocalResource, PublicResource<"POST /inline">>>,
  AssertSame<Same<OperationPublicType<LocalInput>, RouteInput<"POST /inline">>>,
  AssertSame<Same<OperationPublicType<LocalOutput>, RouteOutput<"POST /inline">>>,
  AssertSame<Same<OperationPublicType<LocalRaw>, RouteRawResponse<"POST /inline">>>,
  AssertSame<Same<OperationInput<LocalResource>, OperationInput<PublicResource<"POST /inline">>>>
];
const localContractIdentity: LocalContractIdentity = [true, true, true, true, true];
void localContractIdentity;
import type { Input as CatInput, Output as CatOutput } from "./discriminator-dependencies/internal/schemas/cat.js";
const inputCat: CatInput = { kind: "cat", lives: 9, secret: "private" };
const outputCat: CatOutput = { kind: "cat", lives: 9, label: "public" };
// @ts-expect-error Read-only fields remain absent from the mapped input projection.
const wrongCatInput: CatInput = { kind: "cat", lives: 9, label: "public" };
// @ts-expect-error Write-only fields remain absent from the mapped output projection.
const wrongCatOutput: CatOutput = { kind: "cat", lives: 9, secret: "private" };
// @ts-expect-error The discriminator-only target retains its declared field types.
const wrongCatLives: CatInput = { kind: "cat", lives: "nine" };
void [inputCat, outputCat, wrongCatInput, wrongCatOutput, wrongCatLives];
`;
write(path.join(output, "source/type-witness.ts"), witness);
const selectionTypeSource = fs.readFileSync(
  path.join(root, "test/typescript/tests/selection-types.ts"),
  "utf8",
);
const selectionWitness = selectionTypeSource.replaceAll("../fixtures/generated/", "./");
write(path.join(output, "source/selection-witness.ts"), selectionWitness);
const selectiveTypeSource = fs.readFileSync(
  path.join(root, "test/typescript/tests/selective-types.ts"),
  "utf8",
);
const selectiveWitness = selectiveTypeSource.replaceAll("../fixtures/generated/", "./");
write(path.join(output, "source/selective-witness.ts"), selectiveWitness);
write(
  path.join(output, "selective-consumer.ts"),
  selectiveWitness.replaceAll('from "./', 'from "./declarations/'),
);
write(path.join(output, "package.json"), '{"type":"module"}\n');
const options = {
  ...strictCompilerOptions,
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
assertCheckedSources(checkedFiles);
run("source-and-declaration-emit", [compiler, "--project", path.join(output, "emit.json")]);
write(path.join(output, "consumer.ts"), witness.replaceAll('from "./', 'from "./declarations/'));
write(
  path.join(output, "selection-consumer.ts"),
  selectionWitness.replaceAll('from "./', 'from "./declarations/'),
);
write(
  path.join(output, "consumer.json"),
  JSON.stringify({
    compilerOptions: { ...options, noEmit: true },
    files: ["consumer.ts", "selection-consumer.ts", "selective-consumer.ts"],
  }),
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

const isolatedTypeGraphs = {};
const assertIsolatedTypeGraph = (files, leaf) => {
  assert(
    files.some((file) => file.endsWith(leaf)),
    "The isolated operation was not checked",
  );
  assert(
    !files.some((file) => file.includes("/internal/routes/")),
    "Operation type dependency loads the whole route registry",
  );
  assert(
    files
      .filter((file) => file.includes("/internal/operations/"))
      .every((file) => file.endsWith(leaf)),
    "Operation type dependency loads an unrelated operation",
  );
};
for (const surface of ["source", "declarations"]) {
  const extension = surface === "source" ? ".ts" : ".d.ts";
  const leaf = "lifecycle/internal/operations/inline/post" + extension;
  const config = path.join(output, `${surface}-isolated-operation.json`);
  write(
    config,
    JSON.stringify({
      compilerOptions: { ...options, noEmit: true },
      files: [path.join(surface, leaf)],
    }),
  );
  const files = run(`${surface}-isolated-operation`, [compiler, "--project", config, "--listFiles"])
    .split(/\r?\n/)
    .filter((file) => file.startsWith(path.join(output, surface) + path.sep));
  assertIsolatedTypeGraph(files, leaf);
  isolatedTypeGraphs[surface] = files.map((file) =>
    path.relative(path.join(output, surface), file),
  );
}
const fullTypesConfig = path.join(output, "full-route-types-control.json");
write(
  fullTypesConfig,
  JSON.stringify({
    compilerOptions: { ...options, noEmit: true },
    files: ["source/lifecycle/internal/routes/helpers.ts"],
  }),
);
const fullTypeFiles = run("full-route-types-control", [
  compiler,
  "--project",
  fullTypesConfig,
  "--listFiles",
])
  .split(/\r?\n/)
  .filter((file) => file.startsWith(path.join(output, "source") + path.sep));
assert.throws(
  () => assertIsolatedTypeGraph(fullTypeFiles, "lifecycle/internal/operations/inline/post.ts"),
  /whole route registry/,
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
const discriminatorRuntime = `
import assert from "node:assert/strict";
import {createClient as createFull} from "./javascript/discriminator-dependencies/index.js";
import {createClient, loadOperations, operations} from "./javascript/discriminator-dependencies/selective/index.js";
const trace = [];
const configuration = {baseURL:"https://example.test", fetch:async (url, init)=>{
  trace.push({url:String(url), method:init.method, body:init.body});
  return Response.json(JSON.parse(init.body));
}};
const prepared = await loadOperations([operations.echoPet]);
assert.equal(trace.length, 0);
const selected = createClient({...configuration, operations:prepared}).$operations.echoPet;
const full = createFull(configuration).$operations.echoPet;
for (const kind of ["cat", "unknown"]) {
  const input = {body:{kind, lives:9}};
  assert.deepEqual(await selected(input), await full(input));
  assert.deepEqual(trace.at(-1), trace.at(-2));
  assert.equal((await selected.raw(input)).status, 200);
}
const before = trace.length;
for (const call of [selected, full]) {
  await assert.rejects(call({body:{kind:42}}), {code:"REQUEST_ENCODE_FAILED"});
}
assert.equal(trace.length, before);
const badResponse = {...configuration, fetch:async()=>Response.json({kind:42})};
for (const call of [createClient({...badResponse, operations:prepared}).$operations.echoPet, createFull(badResponse).$operations.echoPet]) {
  await assert.rejects(call({body:{kind:"cat"}}), {code:"RESPONSE_DECODE_FAILED"});
}
console.log(JSON.stringify({status:"pass", checks:["mapping-cycle-preparation","selected-full-traces","default-mapping-preparation","raw-response","invalid-request-before-fetch","invalid-response"]}));
`;
write(path.join(output, "discriminator-native.mjs"), discriminatorRuntime);
const discriminatorNative = JSON.parse(
  run("native-discriminator-dependencies", [path.join(output, "discriminator-native.mjs")]),
);
const mediaScript = path.join(root, "test/typescript/verification/execution-media-native.mjs");
const mediaNative = JSON.parse(run("native-media", [mediaScript, path.join(output, "javascript")]));
const qualityScript = path.join(root, "test/typescript/verification/runtime-quality-native.mjs");
const runtimeQuality = JSON.parse(
  run("native-runtime-quality", [qualityScript, path.join(output, "javascript")]),
);
const selectionScript = path.join(root, "test/typescript/verification/selection-native.mjs");
const selectionNative = JSON.parse(
  run("native-selection", [selectionScript, path.join(output, "javascript")]),
);
const loaderScript = path.join(root, "test/typescript/verification/operation-loader-native.mjs");
const loaderNative = JSON.parse(
  run("native-loader", [loaderScript, path.join(output, "javascript")]),
);
const selectiveClientScript = path.join(
  root,
  "test/typescript/verification/selective-client-native.mjs",
);
const selectiveClientNative = JSON.parse(
  run("native-public-client", [selectiveClientScript, path.join(output, "javascript")]),
);
const selectiveLinksScript = path.join(
  root,
  "test/typescript/verification/selective-links-native.mjs",
);
const selectiveLinksNative = JSON.parse(
  run("native-and-bundled-links", [selectiveLinksScript, path.join(output, "javascript")]),
);
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
  "discriminator-dependencies/internal/executions/echo/post.js",
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
    "/runtime/selection-types.js",
    "/runtime/selection.js",
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
const resourceMembership = verifyResourceMembership(
  path.join(output, "source/lifecycle/selective/types.ts"),
  path.join(output, "resource-membership"),
);
const report = {
  status: "pass",
  resourceMembership,
  runID,
  node: process.version,
  typescript: typescript.version,
  source:
    "Actual Go-emitted artifacts verified against output ownership manifest hashes; verification copies remove only the generated @ts-nocheck directive.",
  checkedImplementationBodies: true,
  scope:
    "Actual Go-emitted fixtures: strict implementation/declaration checks, d.ts-only public client consumption, native media/selection and native/static-bundled lazy-Link execution. Browser HTTP delivery and large-SDK scale are separate validations.",
  inventories,
  graphs,
  fullRuntimeGraphNegativeControl: true,
  isolatedTypeGraphs,
  wholeRouteTypeGraphNegativeControl: true,
  witnessSHA256: sha256(witness),
  selectionWitnessSHA256: sha256(selectionTypeSource),
  selectionTypeAssertions: (selectionTypeSource.match(/Assert<Equal</g) ?? []).length,
  selectionNegativeAssertions: (selectionTypeSource.match(/@ts-expect-error/g) ?? []).length,
  nativeSHA256: sha256(runtime),
  native,
  mediaNative,
  runtimeQuality,
  runtimeQualitySHA256: sha256(fs.readFileSync(qualityScript)),
  discriminatorNative,
  discriminatorWitnessSHA256: sha256(discriminatorRuntime),
  selectionNative,
  loaderNative,
  selectiveClientNative,
  selectiveLinksNative,
  selectiveLinksSHA256: sha256(fs.readFileSync(selectiveLinksScript)),
  selectiveClientWitnessSHA256: sha256(fs.readFileSync(selectiveClientScript)),
  selectiveTypeWitnessSHA256: sha256(selectiveTypeSource),
  loaderNativeSHA256: sha256(fs.readFileSync(loaderScript)),
  selectionNativeSHA256: sha256(fs.readFileSync(selectionScript)),
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
