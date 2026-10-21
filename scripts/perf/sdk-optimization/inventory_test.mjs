import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
const here = path.dirname(fileURLToPath(import.meta.url));
const temp = fs.mkdtempSync(path.join(os.tmpdir(), "sdk-inventory-"));
try {
  const root = path.join(temp, "sdk"), dir = path.join(root, "internal/execution-schemas");
  fs.mkdirSync(dir, { recursive: true });
  // The largest component by bytes has one file; the largest by files has two.
  fs.writeFileSync(path.join(root, "schema.ts"), "export const inputWireSchema = {};\n/*" + "x".repeat(5000) + "*/\n");
  for (const [name, other] of [["a", "b"], ["b", "a"]]) {
    fs.writeFileSync(path.join(dir, name + ".ts"), `import { inputWireSchema as descriptor } from "../../schema.js";\nimport "./${other}.js";\nexport const schemas = Object.fromEntries([["__proto__", descriptor]]);\n`);
  }
  const run = (script, ...args) => spawnSync(process.execPath, [path.join(here, script), ...args], { encoding: "utf8" });
  const output = path.join(temp, "inventory");
  let result = run("inventory.mjs", root, output);
  assert.equal(result.status, 0, result.stderr);
  const summary = JSON.parse(fs.readFileSync(path.join(output, "summary.json"), "utf8"));
  assert.equal(summary.files, 3); assert.equal(summary.maps, 2); assert.equal(summary.mapEntries, 2);
  assert.equal(summary.repeatedEntries, 1); assert.equal(summary.largestSccFiles, 2); assert.equal(summary.unresolvedImports, 0);
  const graph = JSON.parse(fs.readFileSync(path.join(output, "graph.json"), "utf8"));
  assert.equal(graph.files.length, 3);
  assert.equal(graph.dependencies.length, 3);
  assert.deepEqual(graph.files.map((file, index) => [file, graph.sizes[index]]), graph.files.map((file) => [file, fs.statSync(path.join(root, file)).size]));
  result = run("prepare-ts.mjs", root); assert.equal(result.status, 0, result.stderr);
  const config = JSON.parse(fs.readFileSync(path.join(root, "tsconfig.optimization.json"), "utf8"));
  assert.equal(config.files.length, 3); assert.equal(config.compilerOptions.strict, true); assert.equal(config.compilerOptions.noCheck, false);
  assert.equal(JSON.parse(fs.readFileSync(path.join(root, "package.json"), "utf8")).type, "module");
  fs.appendFileSync(path.join(root, "schema.ts"), "// @ts-nocheck\n");
  assert.notEqual(run("prepare-ts.mjs", root).status, 0);
  result = run("inventory.mjs", root, path.join(temp, "unchecked-inventory")); assert.notEqual(result.status, 0);
  console.log("inventory, SCC, full-source config and no-check rejection: pass");
} finally { fs.rmSync(temp, { recursive: true, force: true }); }
