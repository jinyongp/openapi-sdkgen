// Native module-loader proof using the real emitted runtime; public Go lookup output is checked separately.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createHash } from "node:crypto";
import { pathToFileURL } from "node:url";

const javascript = path.resolve(process.argv[2]);
const directory = path.join(path.dirname(javascript), "loader-native");
fs.mkdirSync(path.join(directory, "lookup"), { recursive: true });
const runtime = await import(
  pathToFileURL(path.join(javascript, "lifecycle/internal/runtime/operation-loader.js")).href
);
const generation = "native-loader-fixture";
fs.writeFileSync(
  path.join(directory, "provider.mjs"),
  `globalThis.__sdkgenLoaderProvider=(globalThis.__sdkgenLoaderProvider??0)+1;
export const provider=Object.freeze({abi:1,generation:${JSON.stringify(generation)},route:'GET /a',operationID:'readA',bind(){return ()=>Promise.resolve('a')}});\n`,
);
fs.writeFileSync(
  path.join(directory, "client.mjs"),
  `globalThis.__sdkgenLoaderClient=(globalThis.__sdkgenLoaderClient??0)+1;
export function createSelectedClient(options,providers){return {authorization:options.authorization,providers}}\n`,
);
fs.writeFileSync(path.join(directory, "package.json"), '{"type":"module"}\n');
for (const [kind, key] of [
  ["route", "GET /a"],
  ["operation", "readA"],
]) {
  const hash = createHash("sha256").update(`${kind}\0${key}`).digest("hex");
  const filename = `${kind === "route" ? "r" : "o"}-${hash.slice(0, 16)}/${hash.slice(16)}.js`;
  assert.equal(await runtime.operationLookupFilename(kind, key), filename);
  fs.mkdirSync(path.dirname(path.join(directory, "lookup", filename)), { recursive: true });
  fs.writeFileSync(
    path.join(directory, "lookup", filename),
    `import {provider} from '../../provider.mjs';
export const entry={abi:1,generation:${JSON.stringify(generation)},kind:${JSON.stringify(kind)},key:${JSON.stringify(key)},provider};\n`,
  );
}
const loader = runtime.createOperationLoader({
  generation,
  baseURL: pathToFileURL(directory + path.sep),
  loadClient: () => import(pathToFileURL(path.join(directory, "client.mjs")).href),
});
const ref = loader.operations.readA;
assert.equal(globalThis.__sdkgenLoaderProvider, undefined);
assert.equal(globalThis.__sdkgenLoaderClient, undefined);
const [one, two] = await Promise.all([
  loader.loadOperations([ref, loader.routes["GET /a"]]),
  loader.loadOperations({
    get selected() {
      return [ref];
    },
  }),
]);
const first = loader.createClient({ operations: one, authorization: "first" });
const second = loader.createClient({ operations: two, authorization: "second" });
assert.equal(first.providers.length, 1);
assert.equal(first.providers[0], second.providers[0]);
assert.equal(first.authorization, "first");
assert.equal(second.authorization, "second");
assert.equal(globalThis.__sdkgenLoaderProvider, 1);
assert.equal(globalThis.__sdkgenLoaderClient, 1);
assert.equal(await Promise.resolve(one), one);
await assert.rejects(
  loader.loadOperations(loader.routes["GET /missing"]),
  (error) => error.stage === "MODULE_LOAD" && error.cause?.code === "ERR_MODULE_NOT_FOUND",
);
const foreign = runtime.createOperationLoader({
  generation: "other",
  baseURL: pathToFileURL(directory + path.sep),
  loadClient: async () => {
    throw Error("not needed");
  },
});
await assert.rejects(loader.loadOperations(foreign.routes["GET /a"]), { stage: "IDENTITY" });
console.log(
  JSON.stringify({
    status: "pass",
    checks: [
      "native-exact-lookup",
      "reference-no-evaluation",
      "concurrent-alias-dedup",
      "client-isolation",
      "non-thenable-handle",
      "native-error-cause",
      "foreign-reference",
    ],
  }),
);
