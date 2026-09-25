import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, rmSync } from "node:fs";
import { resolve } from "node:path";
import {
  fixtureRoot,
  repositoryRoot,
  loadCatalog,
  containedPath,
  inputFor,
  inspectGenerated,
} from "./catalog.mjs";

const [binary] = process.argv.slice(2);
assert.ok(binary, "usage: prepare-fixtures.mjs GENERATOR_BINARY");
const generated = resolve(fixtureRoot, "generated");
mkdirSync(generated, { recursive: true });
for (const entry of loadCatalog().local.filter((f) => f.profiles.includes("conformance"))) {
  const input = inputFor(entry);
  const output = containedPath(generated, entry.output);
  // Only catalog-validated children of the existing fixture output root are replaced.
  rmSync(output, { recursive: true, force: true });
  const args = ["generate", "--input", input.path, "--target", "typescript", "--output", output];
  for (const addon of entry.addons) args.push("--with", addon);
  const result = spawnSync(resolve(binary), args, {
    cwd: repositoryRoot,
    encoding: "utf8",
    timeout: 120000,
    maxBuffer: 4 * 1024 * 1024,
  });
  if (result.error) throw result.error;
  if (entry.expectedFailure) {
    assert.notEqual(result.status, 0, `${entry.id}: expected generation failure`);
    for (const message of entry.expectedFailure.contains)
      assert.ok(
        result.stderr.includes(message),
        `${entry.id}: missing ${message}\n${result.stderr}`,
      );
    const golden = readFileSync(containedPath(fixtureRoot, entry.expectedFailure.golden), "utf8");
    assert.equal(
      result.stderr.replaceAll(repositoryRoot, "<ROOT>"),
      golden,
      `${entry.id}: diagnostics changed`,
    );
    assert.equal(existsSync(output), false, `${entry.id}: failed generation published output`);
  } else {
    assert.equal(
      result.status,
      0,
      `${entry.id}: generation failed\n${result.stdout}\n${result.stderr}`,
    );
    inspectGenerated(output);
  }
  console.log(`ok fixture ${entry.id} (${entry.characteristics.join(", ")})`);
}
