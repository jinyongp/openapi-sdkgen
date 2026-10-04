// Execute the product-emitted collector, then bundle the same module consumer.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { pathToFileURL } from "node:url";

const javascript = path.resolve(process.argv[2]);
const directory = path.join(path.dirname(javascript), "selection-native");
fs.mkdirSync(directory, { recursive: true });
const entry = path.join(directory, "entry.mjs");
const source = `
import { collectSelectionReferences } from "../javascript/lifecycle/internal/runtime/client/selection.js";
import defaults, { a, b } from "./feature.mjs";
import * as feature from "./feature.mjs";
export function run() {
  const refs = new WeakMap([[a,a], [b,b]]);
  const collect = value => collectSelectionReferences(value, object => refs.get(object));
  const array = collect([defaults]).map(value => value.id);
  const module = collect(feature).map(value => value.id);
  let reads = 0;
  const group = { get selected() { reads++; return [a,b]; } };
  const shared = collect([group,group]).map(value => value.id);
  let graph = a, visits = 0;
  for(let i=0;i<60;i++) graph = [graph,graph];
  const dag = collectSelectionReferences(graph, value => {visits++;return refs.get(value);}).map(value=>value.id);
  let thenReads = 0;
  const namedThen = collect({get then(){thenReads++;return a;}}).map(value=>value.id);
  const error = new Error("application getter");
  let sameError = false;
  try { collect({get value(){throw error;}}); } catch(cause) {sameError = cause === error;}
  const descriptors = Object.getOwnPropertyNames(feature).map(key => ({key,getter:typeof Object.getOwnPropertyDescriptor(feature,key).get === "function"}));
  return {array,module,shared,reads,dag,visits,namedThen,thenReads,sameError,descriptors};
}
`;
fs.writeFileSync(
  path.join(directory, "feature.mjs"),
  'export const a=Object.freeze({id:"A"}); export const b=Object.freeze({id:"B"}); export default [a,b];\n',
);
fs.writeFileSync(entry, source);
const native = (await import(pathToFileURL(entry).href)).run();
function verify(result) {
  assert.deepEqual(result.array, ["A", "B"]);
  assert.deepEqual(result.module, result.array);
  assert.deepEqual(result.shared, result.array);
  assert.equal(result.reads, 1);
  assert.deepEqual(result.dag, ["A"]);
  assert.equal(result.visits, 61);
  assert.deepEqual(result.namedThen, ["A"]);
  assert.equal(result.thenReads, 1);
  assert.equal(result.sameError, true);
}
verify(native);
assert(native.descriptors.every((value) => !value.getter));
const require = createRequire(new URL("../package.json", import.meta.url));
const viteRequire = createRequire(fs.realpathSync(require.resolve("vite/package.json")));
const { rolldown } = await import(pathToFileURL(viteRequire.resolve("rolldown")).href);
const build = await rolldown({
  input: entry,
  platform: "browser",
  treeshake: true,
  onwarn(warning) {
    throw new Error(warning.message);
  },
});
const bundle = path.join(directory, "bundle.mjs");
try {
  await build.write({ file: bundle, format: "esm", minify: false });
} finally {
  await build.close();
}
const bundled = (await import(pathToFileURL(bundle).href)).run();
verify(bundled);
assert(
  bundled.descriptors.some((value) => value.getter),
  "the bundled namespace getter boundary was not exercised",
);
assert.deepEqual({ ...native, descriptors: undefined }, { ...bundled, descriptors: undefined });
console.log(
  JSON.stringify({
    status: "pass",
    scope:
      "Product-emitted collector with a test-owned reference decoder; not the public loader, SDK reference ABI, browser network or selected client.",
    native,
    bundled,
  }),
);
