import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  writeFileSync,
  statSync,
  symlinkSync,
} from "node:fs";
import { resolve, relative } from "node:path";
import { parseArgs } from "node:util";
import { loadCatalog, repositoryRoot, inputFor, inspectGenerated, sha256 } from "./catalog.mjs";

const { values } = parseArgs({
  options: {
    baseline: { type: "string" },
    candidate: { type: "string" },
    fixtures: { type: "string", default: "representation,lifecycle,collisions" },
  },
});
assert.ok(values.baseline && values.candidate, "--baseline and --candidate binaries are required");
const binaries = {
  baseline: resolve(repositoryRoot, values.baseline),
  candidate: resolve(repositoryRoot, values.candidate),
};
const hashes = Object.fromEntries(
  Object.entries(binaries).map(([k, p]) => [k, sha256(readFileSync(p))]),
);
assert.notEqual(hashes.baseline, hashes.candidate, "migration requires distinct binaries");
const catalog = loadCatalog(),
  ids = new Set(values.fixtures.split(","));
const fixtures = catalog.local.filter((f) => ids.has(f.id) && !f.expectedFailure);
assert.equal(fixtures.length, ids.size, "unknown migration fixture");
const invalid = inputFor(catalog.local.find((f) => f.id === "diagnostics"));
const parent = resolve(repositoryRoot, ".tmp/representation-migration");
mkdirSync(parent, { recursive: true });
const output = mkdtempSync(resolve(parent, "run-"));
const reportPath = resolve(output, "report.json");
const report = {
  version: 1,
  status: "running",
  binarySha256: hashes,
  finalProductionApproval: false,
  fixtures: [],
};
const save = () => writeFileSync(reportPath, JSON.stringify(report, null, 2) + "\n");
function generate(which, entry, destination, incremental = false, inputOverride) {
  const args = [
    "generate",
    "--input",
    inputOverride ?? inputFor(entry).path,
    "--target",
    "typescript",
    "--output",
    destination,
  ];
  for (const addon of entry.addons) args.push("--with", addon);
  if (incremental) args.push("--incremental");
  const result = spawnSync(binaries[which], args, {
    cwd: repositoryRoot,
    encoding: "utf8",
    timeout: 180000,
    maxBuffer: 4 * 1024 * 1024,
  });
  if (result.error) throw result.error;
  return result;
}
function mustGenerate(...args) {
  const r = generate(...args);
  assert.equal(r.status, 0, r.stderr);
}
function snapshot(dir) {
  const audit = inspectGenerated(dir);
  const names = [...Object.keys(audit.files), ".openapi-sdkgen-manifest.json"].sort();
  return Object.fromEntries(
    names.map((name) => [
      name,
      {
        sha256: sha256(readFileSync(resolve(dir, name))),
        mtime: String(statSync(resolve(dir, name), { bigint: true }).mtimeNs),
      },
    ]),
  );
}
save();
if (process.env.SCRIPT_VERBOSE === "1")
  console.error(`migration report: ${relative(repositoryRoot, reportPath)}`);
try {
  for (const entry of fixtures) {
    const row = { id: entry.id, input: inputFor(entry), checks: [] };
    report.fixtures.push(row);
    const old = resolve(output, entry.id, "upgrade"),
      fresh = resolve(output, entry.id, "fresh");
    mustGenerate("baseline", entry, old);
    const before = inspectGenerated(old);
    const oldManifest = JSON.parse(
      readFileSync(resolve(old, ".openapi-sdkgen-manifest.json"), "utf8"),
    );
    mustGenerate("candidate", entry, old, true);
    const after = inspectGenerated(old);
    const newManifest = JSON.parse(
      readFileSync(resolve(old, ".openapi-sdkgen-manifest.json"), "utf8"),
    );
    assert.notDeepEqual(
      oldManifest.generation,
      newManifest.generation,
      "candidate incorrectly reused the baseline identity",
    );
    assert.notDeepEqual(
      before.files,
      after.files,
      "migration did not change any expected artifacts",
    );
    row.checks.push("distinct generator identity invalidates old output reuse");
    mustGenerate("candidate", entry, fresh);
    assert.deepEqual(after.files, inspectGenerated(fresh).files);
    row.checks.push("fresh and incremental managed outputs are byte-identical");
    const stable = snapshot(old);
    mustGenerate("candidate", entry, old, true);
    assert.deepEqual(snapshot(old), stable);
    row.checks.push("same-version no-op preserves content and nanosecond mtimes");
    const invalidResult = generate("candidate", entry, old, true, invalid.path);
    assert.notEqual(invalidResult.status, 0);
    assert.deepEqual(snapshot(old), stable);
    row.checks.push("diagnostic failure preserves the complete prior managed output");
    writeFileSync(resolve(output, `${entry.id}-invalid.log`), invalidResult.stderr);
    const editedPath = resolve(old, "index.ts"),
      sentinel = readFileSync(editedPath, "utf8") + "\n// simulated user edit: retain this\n";
    writeFileSync(editedPath, sentinel);
    const refusal = generate("candidate", entry, old, true);
    assert.notEqual(refusal.status, 0);
    assert.equal(readFileSync(editedPath, "utf8"), sentinel);
    row.checks.push("edited managed source is refused and preserved");
    writeFileSync(resolve(output, `${entry.id}-edit-refusal.log`), refusal.stderr);
    for (const kind of ["file", "directory", "symlink"]) {
      const dir = resolve(output, entry.id, `new-helper-${kind}`);
      mustGenerate("baseline", entry, dir);
      const untouched = snapshot(dir),
        target = resolve(dir, "internal/runtime/wire-properties.ts");
      assert.equal(
        Object.hasOwn(inspectGenerated(dir).files, "internal/runtime/wire-properties.ts"),
        false,
      );
      const userFile = resolve(output, `${entry.id}-${kind}-user.txt`);
      if (kind === "file") writeFileSync(target, "unmanaged user content\n");
      else if (kind === "directory") mkdirSync(target);
      else {
        writeFileSync(userFile, "external user content\n");
        symlinkSync(userFile, target);
      }
      const result = generate("candidate", entry, dir, true);
      assert.notEqual(result.status, 0, `new helper overwrote an unmanaged ${kind}`);
      assert.deepEqual(snapshot(dir), untouched);
      if (kind === "file") assert.equal(readFileSync(target, "utf8"), "unmanaged user content\n");
      if (kind === "symlink")
        assert.equal(readFileSync(userFile, "utf8"), "external user content\n");
      writeFileSync(resolve(output, `${entry.id}-${kind}-refusal.log`), result.stderr);
      row.checks.push(`new helper ${kind} conflict is refused without partial publication`);
    }
    row.status = "pass";
    save();
    if (process.env.SCRIPT_VERBOSE === "1")
      console.error(`ok ${entry.id}: ${row.checks.length} migration/publication checks`);
  }
  for (const [which, binary] of Object.entries(binaries))
    assert.equal(sha256(readFileSync(binary)), hashes[which]);
  report.status = "pass";
} catch (error) {
  report.status = "failed";
  report.error = { message: error.message, stack: error.stack };
  process.exitCode = 1;
} finally {
  save();
}
console.log(
  JSON.stringify({
    status: report.status,
    report: relative(repositoryRoot, reportPath),
    error: report.error?.message,
  }),
);
