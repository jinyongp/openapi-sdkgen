import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { generatedModuleGraph } from "./runtime-feature-imports.mjs";
const matrix = path.resolve(process.argv[2]);
const catalogPath = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../fixtures/runtime-features/catalog.json",
);
const catalog = JSON.parse(fs.readFileSync(catalogPath, "utf8"));
function graph(root, entry) {
  return new Set(generatedModuleGraph(root, entry).map((file) => path.basename(file)));
}
const rows = [];
const failures = [];
let exclusions = 0;
let controls = 0;
for (const fixture of catalog.fixtures) {
  const base = fixture.group === "server" ? path.join(matrix, "server") : matrix;
  const entry = fixture.group === "server" ? "server/webhooks.js" : "index.js";
  const selected = graph(path.join(base, "selected-js", fixture.name), entry);
  const full = graph(path.join(base, "full-js", fixture.name), entry);
  for (const module of fixture.required ?? []) {
    assert.ok(
      selected.has(module),
      `${fixture.group}/${fixture.name}: required handler missing: ${module}`,
    );
    assert.ok(
      full.has(module),
      `${fixture.group}/${fixture.name}: required control handler missing: ${module}`,
    );
  }
  for (const module of fixture.absent) {
    if (!full.has(module))
      failures.push(`${fixture.group}/${fixture.name}: negative control does not detect ${module}`);
    if (selected.has(module))
      failures.push(`${fixture.group}/${fixture.name}: unneeded value dependency ${module}`);
    exclusions++;
    controls++;
  }
  rows.push({
    name: fixture.name,
    group: fixture.group,
    selectedModules: selected.size,
    fullModules: full.size,
    exclusions: fixture.absent.length,
  });
}
assert.deepEqual(failures, [], failures.join("\n"));
fs.writeFileSync(
  path.join(matrix, "graph-results.json"),
  JSON.stringify({ fixtures: rows, exclusions, controls }, null, 2),
);
console.log(
  `Native ESM graphs passed: ${rows.length} fixtures, ${exclusions} exclusions, ${controls} full-capability controls`,
);
