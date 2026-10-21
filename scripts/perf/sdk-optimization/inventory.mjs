// AST inventory adapted from the supplied v2 ts_inventory.cjs. No SDK rewriting.
import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import ts from "../../../test/typescript/node_modules/typescript-5-9/lib/typescript.js";

const [rootArg, outArg] = process.argv.slice(2);
if (!rootArg || !outArg) throw Error("usage: inventory.mjs SDK_ROOT FRESH_OUTPUT");
const root = path.resolve(rootArg), out = path.resolve(outArg);
fs.mkdirSync(out, { recursive: false, mode: 0o700 });
const digest = (s) => crypto.createHash("sha256").update(s).digest("hex");
function* walk(dir) {
  for (const ent of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => Buffer.compare(Buffer.from(a.name), Buffer.from(b.name)))) {
    if (["node_modules", ".git", ".out"].includes(ent.name)) continue;
    const file = path.join(dir, ent.name);
    if (ent.isDirectory()) yield* walk(file);
    else if (ent.isFile() && file.endsWith(".ts")) yield file;
  }
}
const files = [...walk(root)], ids = new Map(files.map((p, i) => [p, i]));
const adj = files.map(() => []), rev = files.map(() => []), sizes = [], unknown = [];
const closures = fs.openSync(path.join(out, "closures.jsonl"), "wx"), inventory = fs.openSync(path.join(out, "files.jsonl"), "wx");
const membership = new Map();
let maps = 0, mapEntries = 0, nocheck = 0, importBytes = 0, statementBytes = 0;
for (let i = 0; i < files.length; i++) {
  const file = files[i], text = fs.readFileSync(file, "utf8"), rel = path.relative(root, file).split(path.sep).join("/");
  sizes[i] = Buffer.byteLength(text);
  if (text.includes("@ts-nocheck")) nocheck++;
  fs.writeSync(inventory, JSON.stringify({ path: rel, bytes: sizes[i], sha256: digest(text) }) + "\n");
  for (const im of ts.preProcessFile(text, true, true).importedFiles) {
    const spec = im.fileName;
    if (!spec.startsWith(".")) { unknown.push([rel, spec]); continue; }
    const target = path.resolve(path.dirname(file), spec);
    const j = [target.replace(/\.js$/, ".ts"), target + ".ts", path.join(target, "index.ts")].map((p) => ids.get(p)).find((n) => n !== undefined);
    if (j === undefined) unknown.push([rel, spec]);
    else { adj[i].push(j); rev[j].push(i); }
  }
  if (!rel.startsWith("internal/execution-schemas/") && !rel.startsWith("internal/executions/")) continue;
  const ast = ts.createSourceFile(file, text, ts.ScriptTarget.Latest, true), imports = new Map();
  for (const n of ast.statements) {
    if (!ts.isImportDeclaration(n) || !ts.isStringLiteral(n.moduleSpecifier) || !n.importClause?.namedBindings || !ts.isNamedImports(n.importClause.namedBindings)) continue;
    for (const el of n.importClause.namedBindings.elements) {
      const exported = (el.propertyName || el.name).text;
      if (!["inputWireSchema", "outputWireSchema"].includes(exported)) continue;
      imports.set(el.name.text, { direction: exported.startsWith("input") ? "input" : "output", module: n.moduleSpecifier.text, bytes: Buffer.byteLength(n.getText(ast)) });
    }
  }
  for (const n of ast.statements) {
    if (!ts.isVariableStatement(n)) continue;
    for (const d of n.declarationList.declarations) {
      if (!ts.isIdentifier(d.name) || !["schemas", "inputSchemas", "outputSchemas"].includes(d.name.text)) continue;
      const call = d.initializer;
      if (!call || !ts.isCallExpression(call) || call.expression.getText(ast) !== "Object.fromEntries") continue;
      if (call.arguments.length !== 1 || !ts.isArrayLiteralExpression(call.arguments[0])) throw Error("unsupported map: " + rel);
      const entries = [], directions = new Set();
      for (const entry of call.arguments[0].elements) {
        if (!ts.isArrayLiteralExpression(entry) || entry.elements.length !== 2 || !ts.isStringLiteral(entry.elements[0]) || !ts.isIdentifier(entry.elements[1])) throw Error("unsupported entry: " + rel);
        const im = imports.get(entry.elements[1].text);
        if (!im) throw Error("unresolved descriptor: " + rel);
        directions.add(im.direction);
        const name = entry.elements[0].text;
        entries.push(name);
        const key = im.direction + ":" + path.resolve(path.dirname(file), im.module);
        if (!membership.has(key)) membership.set(key, []);
        membership.get(key).push(maps);
        importBytes += im.bytes;
      }
      if (directions.size > 1 || new Set(entries).size !== entries.length) throw Error("projection/duplicate mismatch: " + rel);
      const bytes = Buffer.byteLength(n.getText(ast));
      statementBytes += bytes; mapEntries += entries.length;
      fs.writeSync(closures, JSON.stringify({ id: maps++, path: rel, variable: d.name.text, direction: [...directions][0], entries, statementBytes: bytes }) + "\n");
    }
  }
}
fs.closeSync(closures); fs.closeSync(inventory);
// Syntactic import SCCs include value/type/dynamic imports, not checker instantiations.
const seen = new Uint8Array(files.length), order = [];
for (let s = 0; s < files.length; s++) {
  if (seen[s]) continue;
  seen[s] = 1;
  const stack = [[s, 0]];
  while (stack.length) {
    const top = stack[stack.length - 1];
    if (top[1] < adj[top[0]].length) {
      const j = adj[top[0]][top[1]++];
      if (!seen[j]) { seen[j] = 1; stack.push([j, 0]); }
    } else { order.push(top[0]); stack.pop(); }
  }
}
seen.fill(0);
const components = [];
for (let k = order.length - 1; k >= 0; k--) {
  const s = order[k]; if (seen[s]) continue;
  const stack = [s], members = []; let bytes = 0; seen[s] = 1;
  while (stack.length) {
    const v = stack.pop(); members.push(path.relative(root, files[v]).split(path.sep).join("/")); bytes += sizes[v];
    for (const j of rev[v]) if (!seen[j]) { seen[j] = 1; stack.push(j); }
  }
  components.push({ files: members.length, bytes, members });
}
components.sort((a, b) => b.bytes - a.bytes);
const groups = new Map();
for (const [key, maps] of membership) {
  const signature = key.split(":")[0] + ":" + maps.join(",");
  const group = groups.get(signature) || { descriptors: 0, uses: maps.length };
  group.descriptors++; groups.set(signature, group);
}
const summary = { files: files.length, totalBytes: sizes.reduce((a, b) => a + b, 0), nocheckFiles: nocheck,
  maps, mapEntries, statementBytes, descriptorImportBytes: importBytes, distinctProjectedDescriptors: membership.size,
  incidenceGroups: groups.size, repeatedEntries: [...groups.values()].reduce((n, g) => n + g.descriptors * Math.max(0, g.uses - 1), 0),
  largestSccFiles: components.reduce((n, c) => Math.max(n, c.files), 0), largestSccBytes: components[0]?.bytes || 0,
  largestSccByBytes: components[0] ? { files: components[0].files, bytes: components[0].bytes } : null,
  nontrivialSccs: components.filter((c) => c.files > 1).length, unresolvedImports: unknown.length,
  scope: "syntactic inventory; repeatedEntries is an opportunity count, not bytes saved, strict success or a memory prediction" };
fs.writeFileSync(path.join(out, "scc.json"), JSON.stringify({ compiler: ts.version, components, unresolvedImports: unknown }));
fs.writeFileSync(path.join(out, "summary.json"), JSON.stringify(summary, null, 2) + "\n");
fs.writeFileSync(path.join(out, "graph.json"), JSON.stringify({ root, files: files.map((file) => path.relative(root, file).split(path.sep).join("/")), sizes, dependencies: adj }) + "\n");
console.log(JSON.stringify(summary));
if (nocheck || unknown.length) process.exitCode = 1;
