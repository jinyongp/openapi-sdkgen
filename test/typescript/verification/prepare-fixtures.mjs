import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { existsSync, mkdirSync, readFileSync, rmSync } from "node:fs";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { generatedFiles, checkFiles } from "./declarations.mjs";
import { fixtureRoot, repositoryRoot, loadCatalog, containedPath, inputFor } from "./catalog.mjs";

export function prepareFixtures(binary, generated = resolve(fixtureRoot, "generated")) {
  assert.ok(binary, "usage: prepare-fixtures.mjs GENERATOR_BINARY");
  mkdirSync(generated, { recursive: true });
  let files = 0;
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
      const report = checkFiles(generatedFiles(output));
      assert.ok(
        report.ok,
        `${entry.id}: declaration check failed\n${report.diagnostics
          .slice(0, 10)
          .map((item) => `${item.file}:${item.line}:${item.column} [${item.rule}] ${item.name}`)
          .join("\n")}`,
      );
      files += report.files;
    }
    console.log(`ok fixture ${entry.id} (${entry.characteristics.join(", ")})`);
  }
  console.log(`ok catalog declarations: ${files} manifest-owned TypeScript files`);
  return files;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url))
  prepareFixtures(process.argv[2]);
