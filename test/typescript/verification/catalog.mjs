import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync, realpathSync, lstatSync } from "node:fs";
import { dirname, resolve, relative, isAbsolute, sep } from "node:path";
import { fileURLToPath } from "node:url";

export const fixtureRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../fixtures");
export const repositoryRoot = resolve(fixtureRoot, "../../..");
const segment = /^[a-z0-9][a-z0-9-]*$/;
const scenarios = new Set([
  "contract",
  "collisions",
  "surface",
  "oas30",
  "oas31",
  "oas32",
  "representation",
  "lifecycle",
  "github",
  "stripe",
]);

export function sha256(value) {
  return createHash("sha256").update(value).digest("hex");
}

/** Validate lexical and existing filesystem ancestry before any fixture IO. */
export function containedPath(root, name) {
  assert.equal(typeof name, "string");
  assert.ok(name.length > 0 && !isAbsolute(name) && !name.includes("\\") && !name.includes("\0"));
  const target = resolve(root, name);
  const rel = relative(root, target);
  assert.ok(rel && rel !== ".." && !rel.startsWith(`..${sep}`), `path escapes root: ${name}`);
  let cursor = target;
  for (;;) {
    const info = lstatSync(cursor, { throwIfNoEntry: false });
    if (info) assert.equal(info.isSymbolicLink(), false, `symlink fixture path: ${cursor}`);
    if (cursor === resolve(root)) break;
    cursor = dirname(cursor);
  }
  return target;
}

export function validateCatalog(catalog) {
  assert.equal(catalog.version, 1, "unsupported fixture catalog version");
  assert.ok(Array.isArray(catalog.local) && Array.isArray(catalog.external));
  const ids = new Set();
  const outputs = new Set();
  for (const [kind, entries] of [
    ["local", catalog.local],
    ["external", catalog.external],
  ]) {
    for (const entry of entries) {
      assert.match(entry.id, segment);
      assert.ok(!ids.has(entry.id), `duplicate fixture id: ${entry.id}`);
      ids.add(entry.id);
      assert.ok(
        entry.profiles?.length && entry.characteristics?.length,
        `fixture metadata: ${entry.id}`,
      );
      assert.ok(
        entry.profiles.every((p) =>
          ["conformance", "preimplementation", "large", "lifetime"].includes(p),
        ),
      );
      if (kind === "local") {
        assert.match(entry.input, /^[a-z0-9][a-z0-9-]*\.openapi\.json$/);
        assert.match(entry.output, segment);
        assert.ok(!outputs.has(entry.output), `duplicate output: ${entry.output}`);
        outputs.add(entry.output);
        assert.ok(Array.isArray(entry.addons) && entry.addons.every((x) => x === "server"));
        assert.equal(new Set(entry.addons).size, entry.addons.length);
        if (entry.expectedFailure) {
          assert.equal(entry.scenario, undefined);
          assert.match(entry.expectedFailure.golden, /^[a-z0-9-]+\.report\.txt$/);
          assert.ok(entry.expectedFailure.contains.length > 0);
        } else assert.ok(scenarios.has(entry.scenario), `unknown scenario: ${entry.scenario}`);
      } else {
        assert.match(entry.sha256, /^[0-9a-f]{64}$/);
        assert.ok(scenarios.has(entry.scenario));
        assert.ok(Number.isSafeInteger(entry.operations) && Number.isSafeInteger(entry.schemas));
      }
    }
  }
  return catalog;
}

export function loadCatalog() {
  return validateCatalog(JSON.parse(readFileSync(resolve(fixtureRoot, "catalog.json"), "utf8")));
}

export function inputFor(entry, externalInputs = {}) {
  const path = entry.input ? containedPath(fixtureRoot, entry.input) : externalInputs[entry.id];
  assert.ok(path, `provide an explicit local input for external corpus ${entry.id}`);
  const input = realpathSync(path);
  const bytes = readFileSync(input);
  const digest = sha256(bytes);
  if (entry.sha256) assert.equal(digest, entry.sha256, `pinned input mismatch: ${entry.id}`);
  const document = JSON.parse(bytes.toString("utf8"));
  const verbs = new Set([
    "get",
    "put",
    "post",
    "delete",
    "options",
    "head",
    "patch",
    "trace",
    "query",
  ]);
  const operations = Object.values(document.paths ?? {}).reduce(
    (n, item) => n + Object.keys(item).filter((key) => verbs.has(key)).length,
    0,
  );
  const schemas = Object.keys(document.components?.schemas ?? {}).length;
  if (entry.operations !== undefined) assert.equal(operations, entry.operations);
  if (entry.schemas !== undefined) assert.equal(schemas, entry.schemas);
  return { path: input, sha256: digest, bytes: bytes.length, operations, schemas };
}

/** Manifest-owned bytes only; stale or edited generated output is not benchmark input. */
export function inspectGenerated(root) {
  const manifest = JSON.parse(readFileSync(resolve(root, ".openapi-sdkgen-manifest.json"), "utf8"));
  assert.ok([1, 2].includes(manifest.version), "unsupported output manifest version");
  assert.equal(
    manifest.version === 2,
    manifest.generation != null,
    "manifest generation identity/version mismatch",
  );
  let typescriptBytes = 0;
  for (const [name, digest] of Object.entries(manifest.files)) {
    const data = readFileSync(containedPath(root, name));
    assert.equal(sha256(data), digest, `generated artifact hash mismatch: ${name}`);
    if (name.endsWith(".ts")) typescriptBytes += data.length;
  }
  return {
    files: manifest.files,
    fileCount: Object.keys(manifest.files).length,
    typescriptBytes,
    treeSha256: sha256(JSON.stringify(manifest.files)),
  };
}

/** Count only declarations corresponding to generator-owned TypeScript sources. */
export function managedDeclarationStats(root, sourceFiles) {
  const names = Object.keys(sourceFiles)
    .filter((name) => name.endsWith(".ts"))
    .map((name) => name.slice(0, -3) + ".d.ts");
  assert.equal(new Set(names).size, names.length, "declaration target collision");
  return {
    files: names.length,
    bytes: names.reduce((sum, name) => sum + readFileSync(containedPath(root, name)).length, 0),
  };
}
