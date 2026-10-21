// Internal strict-check ownership plan. Never rewrites generated SDK sources.
import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import ts from "../../../test/typescript/node_modules/typescript-5-9/lib/typescript.js";

const [inventoryArg, outputArg, limitArg = "67108864", countArg = "25000"] = process.argv.slice(2);
if (!inventoryArg || !outputArg) throw Error("usage: project-plan.mjs INVENTORY FRESH_OUTPUT [MAX_SOURCE_BYTES MAX_FILES]");
const inventory = path.resolve(inventoryArg), output = path.resolve(outputArg);
const limit = Number(limitArg), countLimit = Number(countArg);
if (!Number.isSafeInteger(limit) || limit < 1 || !Number.isSafeInteger(countLimit) || countLimit < 1) throw Error("invalid project limit");
const sha = (text) => crypto.createHash("sha256").update(text).digest("hex");
const load = (name) => JSON.parse(fs.readFileSync(path.join(inventory, name), "utf8"));
const graph = load("graph.json"), scc = load("scc.json"), summary = load("summary.json");
const entries = fs.readFileSync(path.join(inventory, "files.jsonl"), "utf8").trim().split("\n").map(JSON.parse);
const n = graph.files.length;
if (!n || graph.sizes.length !== n || graph.dependencies.length !== n || entries.length !== n || summary.files !== n || summary.nocheckFiles || summary.unresolvedImports || scc.unresolvedImports.length) throw Error("incomplete inventory");
const ids = new Map(graph.files.map((file, id) => [file, id]));
if (ids.size !== n) throw Error("duplicate inventory source");
if (graph.sizes.reduce((sum, size) => sum + size, 0) !== summary.totalBytes) throw Error("inventory byte total mismatch");
const root = path.resolve(graph.root);
function* sources(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const file = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (entry.name === "node_modules") throw Error("unexpected node_modules in SDK");
      yield* sources(file);
    } else if (entry.isFile() && entry.name.endsWith(".ts")) yield path.relative(root, file).split(path.sep).join("/");
  }
}
const actual = [...sources(root)];
if (actual.length !== n || actual.some((file) => !ids.has(file))) throw Error("source inventory is incomplete");
for (let id = 0; id < n; id++) {
  const file = graph.files[id], entry = entries[id];
  if (path.isAbsolute(file) || file.split("/").includes("..") || file.endsWith(".d.ts") || entry.path !== file || entry.bytes !== graph.sizes[id]) throw Error("invalid source ownership entry or unsupported input declaration");
  const text = fs.readFileSync(path.join(root, file), "utf8");
  if (Buffer.byteLength(text) !== entry.bytes || sha(text) !== entry.sha256 || text.includes("@ts-nocheck")) throw Error("changed/unchecked source: " + file);
  const source = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, false, ts.ScriptKind.TS);
  if (!ts.isExternalModule(source) || source.referencedFiles.length || source.typeReferenceDirectives.length || source.libReferenceDirectives.length || source.hasNoDefaultLib) throw Error("global/reference source requires a separate ownership design: " + file);
  function visit(node) {
    if (ts.isModuleDeclaration(node) && (ts.isStringLiteral(node.name) || (node.flags & ts.NodeFlags.GlobalAugmentation))) throw Error("augmentation requires a separate ownership design: " + file);
    ts.forEachChild(node, visit);
  }
  visit(source);
}
const componentOf = new Int32Array(n).fill(-1);
const components = scc.components.map((component, index) => {
  const members = component.members.map((file) => {
    const id = ids.get(file);
    if (id === undefined || componentOf[id] !== -1) throw Error("invalid SCC membership: " + file);
    componentOf[id] = index;
    return id;
  }).sort((a, b) => a - b);
  const bytes = members.reduce((sum, id) => sum + graph.sizes[id], 0);
  if (members.length !== component.files || bytes !== component.bytes || members.length > countLimit || bytes > limit) throw Error("SCC exceeds project limit or has invalid metadata: " + graph.files[members[0]]);
  return { members, bytes, dependencies: new Set(), importers: [], first: members[0] };
});
if (componentOf.some((id) => id < 0)) throw Error("unowned source");
for (let id = 0; id < n; id++) {
  for (const dependency of graph.dependencies[id]) {
    if (!Number.isSafeInteger(dependency) || dependency < 0 || dependency >= n) throw Error("invalid dependency");
    const from = componentOf[id], to = componentOf[dependency];
    if (from !== to) components[from].dependencies.add(to);
  }
}
const remaining = components.map((component) => component.dependencies.size), ready = [];
// Min heap retains deterministic source order without repeated O(n log n) sorts.
const before = (a, b) => components[a].first < components[b].first;
function push(value) {
  let at = ready.length; ready.push(value);
  while (at) { const parent = (at - 1) >>> 1; if (!before(value, ready[parent])) break; ready[at] = ready[parent]; at = parent; }
  ready[at] = value;
}
function pop() {
  const value = ready[0], last = ready.pop();
  if (ready.length) {
    let at = 0;
    while (at * 2 + 1 < ready.length) {
      let child = at * 2 + 1;
      if (child + 1 < ready.length && before(ready[child + 1], ready[child])) child++;
      if (!before(ready[child], last)) break;
      ready[at] = ready[child]; at = child;
    }
    ready[at] = last;
  }
  return value;
}
components.forEach((component, from) => {
  for (const to of component.dependencies) components[to].importers.push(from);
  if (!remaining[from]) push(from);
});
const groups = [], owner = new Int32Array(n).fill(-1);
let visited = 0;
while (ready.length) {
  const id = pop(), component = components[id]; visited++;
  let group = groups[groups.length - 1];
  if (!group || group.bytes + component.bytes > limit || group.files.length + component.members.length > countLimit) {
    group = { id: groups.length, bytes: 0, files: [], dependencies: [] }; groups.push(group);
  }
  group.bytes += component.bytes;
  for (const file of component.members) { owner[file] = group.id; group.files.push(graph.files[file]); }
  for (const importer of component.importers) if (--remaining[importer] === 0) push(importer);
}
if (visited !== components.length) throw Error("invalid cyclic condensation graph");
const dependencies = groups.map(() => new Set());
for (let id = 0; id < n; id++) {
  for (const dependency of graph.dependencies[id]) {
    const from = owner[id], to = owner[dependency];
    if (to > from) throw Error("forward dependency in ownership plan");
    if (to !== from) dependencies[from].add(to);
  }
}
groups.forEach((group) => { group.files.sort(); group.dependencies = [...dependencies[group.id]].sort((a, b) => a - b); });
fs.mkdirSync(output, { mode: 0o700 });
const ownership = entries.map((entry, id) => JSON.stringify({ ...entry, owner: owner[id] })).join("\n") + "\n";
fs.writeFileSync(path.join(output, "ownership.jsonl"), ownership, { flag: "wx" });
const plan = { version: 1, root, inventory, compiler: ts.version, files: n, totalBytes: summary.totalBytes, sourceLimitBytes: limit, fileLimit: countLimit, ownershipSha256: sha(ownership), groups };
fs.writeFileSync(path.join(output, "plan.json"), JSON.stringify(plan) + "\n", { flag: "wx" });
console.log(JSON.stringify({ files: n, totalBytes: plan.totalBytes, groups: groups.length, largestGroupBytes: Math.max(...groups.map((g) => g.bytes)), scope: "complete SCC-preserving ownership; strict checks have not run" }));
