import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { containedPath, repositoryRoot } from "./catalog.mjs";
import { generatedFiles, checkFiles } from "./declarations.mjs";

const [binary] = process.argv.slice(2);
assert.ok(binary && process.argv.length === 3, "usage: prepare-declarations.mjs BINARY");
const catalog = JSON.parse(
  readFileSync(new URL("./declaration-options.json", import.meta.url), "utf8"),
);
const root = resolve(repositoryRoot, ".tmp/declaration-fixtures");
mkdirSync(root, { recursive: true });
const files = [];
const ids = new Set();
for (const entry of catalog) {
  assert.match(entry.id, /^[a-z0-9][a-z0-9-]*$/);
  assert.ok(!ids.has(entry.id), `duplicate declaration fixture: ${entry.id}`);
  ids.add(entry.id);
  for (const policy of ["false", "true"]) {
    const output = containedPath(root, `${entry.id}-${policy}`);
    rmSync(output, { recursive: true, force: true });
    const args = [
      "generate",
      "--input",
      containedPath(repositoryRoot, entry.input),
      "--target",
      "typescript",
      "--output",
      output,
      `--typecheck=${policy}`,
      ...entry.args,
    ];
    if (entry.config) args.push("--config", containedPath(repositoryRoot, entry.config));
    const result = spawnSync(resolve(binary), args, {
      cwd: repositoryRoot,
      encoding: "utf8",
      timeout: 120000,
      maxBuffer: 4 * 1024 * 1024,
    });
    assert.equal(result.error, undefined);
    assert.equal(result.status, 0, `${entry.id}: generation failed\n${result.stderr}`);
    files.push(...generatedFiles(output));
  }
}
const inventory = resolve(root, "inputs.json");
writeFileSync(inventory, JSON.stringify(files, null, 2) + "\n");
const result = checkFiles(files);
const report = resolve(root, "report.json");
writeFileSync(report, JSON.stringify(result, null, 2) + "\n");
assert.ok(result.ok, `declaration violations in option fixtures; inspect ${report}`);
console.log(
  `ok declaration options: ${catalog.length * 2} outputs, ${files.length} owned TypeScript files`,
);
