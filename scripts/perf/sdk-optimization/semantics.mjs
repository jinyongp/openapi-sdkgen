// Independent baseline observations for a future ordered closure merge.
import assert from "node:assert/strict";
const names = ["", "10", "2", "__proto__", "constructor", "z", "\ue000", "\u{10000}"];
const rows = names.map((name, ordinal) => ({ name, ordinal, value: { ordinal } }));
const baseline = Object.fromEntries(rows.map((row) => [row.name, row.value]));
const blocks = [rows.filter((row) => row.ordinal % 2 === 0), rows.filter((row) => row.ordinal % 2 !== 0)];
const merged = blocks.flat().sort((a, b) => a.ordinal - b.ordinal);
const candidate = Object.fromEntries(merged.map((row) => [row.name, row.value]));
assert.deepEqual(Reflect.ownKeys(candidate), Reflect.ownKeys(baseline));
assert.equal(Object.getPrototypeOf(candidate), Object.getPrototypeOf(baseline));
for (const name of names) {
  assert.deepEqual(Object.getOwnPropertyDescriptor(candidate, name), Object.getOwnPropertyDescriptor(baseline, name));
  assert.equal(candidate[name], baseline[name]);
}
assert.equal(Object.getPrototypeOf(Object.assign({}, baseline)), baseline["__proto__"]);
assert.notDeepEqual(["\ue000", "\u{10000}"].sort(), ["\ue000", "\u{10000}"].sort((a, b) => Buffer.compare(Buffer.from(a), Buffer.from(b))));
console.log("ordered own keys, descriptors, descriptor identity, prototype and UTF-8/UTF-16 counterexample: pass");
