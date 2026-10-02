import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import { test } from "node:test";
import { inspectSource, checkFiles, generatedFiles } from "./declarations.mjs";
import { repositoryRoot, sha256 } from "./catalog.mjs";
import { prepareFixtures } from "./prepare-fixtures.mjs";

const cases = JSON.parse(
  readFileSync(new URL("./declaration-cases.json", import.meta.url), "utf8"),
);
for (const entry of cases) {
  test(`declaration syntax: ${entry.name}`, () => {
    const diagnostics = [];
    inspectSource("fixture.ts", entry.source, (item) => diagnostics.push(item));
    assert.deepEqual(diagnostics.map((item) => item.rule).sort(), entry.rules.toSorted());
    if (entry.name === "unicode-crlf") {
      assert.equal(diagnostics[0].name, "값");
      assert.equal(diagnostics[0].line, 2);
      assert.equal(diagnostics[0].column, 7);
    }
  });
}

function workspace(body) {
  const root = mkdtempSync(resolve(tmpdir(), "sdkgen-declarations-"));
  try {
    body(root);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

function wrapper(args) {
  const result = spawnSync(
    "bash",
    [resolve(repositoryRoot, "scripts/agent/ts-declarations"), ...args],
    {
      cwd: repositoryRoot,
      encoding: "utf8",
      timeout: 30000,
    },
  );
  assert.equal(result.error, undefined);
  return result;
}

function manifest(root, source = "export const value: number = 1;", changes = {}) {
  writeFileSync(resolve(root, "value.ts"), source);
  const value = { version: 1, files: { "value.ts": sha256(source) }, ...changes };
  writeFileSync(resolve(root, ".openapi-sdkgen-manifest.json"), JSON.stringify(value));
}

test("actual wrapper detects annotation removal with either header policy", () => {
  workspace((root) => {
    const valid = "const fn: (value: number) => number = (value: number): number => value;";
    for (const header of ["", "// @ts-nocheck\n"]) {
      manifest(root, header + valid);
      assert.equal(wrapper(["--generated", root]).status, 0);
      for (const [changed, rule] of [
        [valid.replace("const fn: (value: number) => number", "const fn"), "variable-type"],
        [valid.replace("= (value: number)", "= (value)"), "parameter-type"],
        [valid.replace("): number =>", ") =>"), "return-type"],
      ]) {
        manifest(root, header + changed);
        const report = resolve(root, "report.json");
        assert.equal(wrapper(["--generated", root, "--json", report]).status, 1);
        const data = JSON.parse(readFileSync(report, "utf8"));
        assert.equal(data.ok, false);
        assert.equal(data.violations[rule], 1);
        assert.equal(data.files, 1);
      }
    }
  });
});

test("actual wrapper refuses incomplete, malformed and inaccessible inputs", () => {
  workspace((root) => {
    const list = resolve(root, "inputs.json");
    for (const entries of [[], {}, [resolve(root, "missing.ts")], [42]]) {
      writeFileSync(list, JSON.stringify(entries));
      assert.notEqual(wrapper(["--files", list]).status, 0);
    }
    writeFileSync(resolve(root, "broken.ts"), "const value: = 1;");
    assert.equal(wrapper(["--runtime", root]).status, 1);
    writeFileSync(resolve(root, "broken.ts"), Buffer.from([0xff]));
    assert.equal(wrapper(["--runtime", root]).status, 1);
    rmSync(resolve(root, "broken.ts"));
    mkdirSync(resolve(root, "directory.ts"));
    writeFileSync(list, JSON.stringify([resolve(root, "directory.ts")]));
    assert.equal(wrapper(["--files", list]).status, 1);
    rmSync(resolve(root, "directory.ts"), { recursive: true });
    assert.notEqual(wrapper(["--runtime", root]).status, 0);
    assert.notEqual(wrapper(["--unknown", root]).status, 0);
    assert.notEqual(wrapper(["--runtime"]).status, 0);
  });
});

test("actual wrapper writes operational errors to JSON after input options", () => {
  workspace((root) => {
    const report = resolve(root, "report.json");
    assert.equal(wrapper(["--generated", root, "--json", report]).status, 2);
    const data = JSON.parse(readFileSync(report, "utf8"));
    assert.equal(data.ok, false);
    assert.equal(data.errors, 1);
    assert.equal(data.diagnostics[0].rule, "input-error");
    assert.equal(data.diagnostics[0].file, root);
  });
});

test("manifest validates ownership, byte hashes, canonical paths and symlinks", () => {
  workspace((root) => {
    manifest(root);
    assert.equal(generatedFiles(root).length, 1);
    for (const changes of [
      { version: 3 },
      { version: 2 },
      { generation: {} },
      { files: {} },
      { files: [] },
      { files: { "value.ts": "0".repeat(64) } },
      { files: { "./value.ts": sha256("export const value: number = 1;") } },
      { files: { "../outside.ts": "0".repeat(64) } },
      { files: { "missing.ts": "0".repeat(64) } },
    ]) {
      manifest(root, undefined, changes);
      assert.notEqual(wrapper(["--generated", root]).status, 0);
    }
    manifest(root);
    writeFileSync(resolve(root, "unowned.ts"), "export const value: number = 1;");
    assert.notEqual(wrapper(["--generated", root]).status, 0);
    rmSync(resolve(root, "unowned.ts"));
    symlinkSync(resolve(root, "value.ts"), resolve(root, "linked.ts"));
    assert.notEqual(wrapper(["--runtime", root]).status, 0);
  });
});

test("diagnostic bounds do not weaken failure and reports sort deterministically", () => {
  workspace((root) => {
    const file = resolve(root, "value.ts");
    writeFileSync(file, "const fn = value => value;");
    const result = checkFiles([file], 1);
    assert.equal(result.ok, false);
    assert.equal(result.diagnostics.length, 1);
    assert.equal(result.truncated, 2);
    assert.equal(
      Object.values(result.violations).reduce((a, b) => a + b),
      3,
    );
    assert.deepEqual(checkFiles([file]).diagnostics, checkFiles([file]).diagnostics);
    assert.throws(() => checkFiles([file, file]), /duplicate/);
  });
});

test("fixture preparation propagates missing declarations and syntax/input failures", () => {
  workspace((root) => {
    const binary = resolve(root, "generator.mjs");
    for (const [source, invalidManifest, expected] of [
      ["export const value = 1;", false, /variable-type/],
      ["export const value: = 1;", false, /parse-error/],
      ["export const value: number = 1;", true, /hash mismatch/],
    ]) {
      writeFileSync(
        binary,
        `#!/usr/bin/env node\nimport fs from 'node:fs';\nimport crypto from 'node:crypto';\nconst output=process.argv[process.argv.indexOf('--output')+1];\nfs.mkdirSync(output,{recursive:true});\nconst source=${JSON.stringify(source)};\nfs.writeFileSync(output+'/value.ts',source);\nconst digest=${invalidManifest ? "'0'.repeat(64)" : "crypto.createHash('sha256').update(source).digest('hex')"};\nfs.writeFileSync(output+'/.openapi-sdkgen-manifest.json',JSON.stringify({version:1,files:{'value.ts':digest}}));\n`,
        { mode: 0o700 },
      );
      assert.throws(() => prepareFixtures(binary, resolve(root, "generated")), expected);
    }
  });
});
