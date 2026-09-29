import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";
import { randomUUID } from "node:crypto";
import { fileURLToPath } from "node:url";
import { test } from "node:test";
import { compileGenerated } from "./sdk-delivery-compile.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../package.json", import.meta.url));
const packageFile = require.resolve("typescript/package.json");
const compiler = path.resolve(
  path.dirname(packageFile),
  JSON.parse(fs.readFileSync(packageFile, "utf8")).bin.tsc,
);
const options = {
  target: "ES2022",
  module: "NodeNext",
  moduleResolution: "NodeNext",
  types: [],
  strict: true,
  exactOptionalPropertyTypes: true,
  noUncheckedIndexedAccess: true,
  skipLibCheck: false,
  noEmitOnError: true,
  declaration: true,
  rootDir: "source",
  outDir: "javascript",
  declarationDir: "declarations",
};
function fixture(t, source) {
  const directory = path.join(root, ".tmp/sdk-delivery-compiler-tests", randomUUID());
  fs.mkdirSync(path.join(directory, "source"), { recursive: true });
  fs.writeFileSync(path.join(directory, "package.json"), '{"type":"module"}\n');
  fs.writeFileSync(path.join(directory, "source/index.ts"), source);
  fs.writeFileSync(
    path.join(directory, "emit.json"),
    JSON.stringify({ compilerOptions: options, include: ["source/**/*.ts"] }),
  );
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }));
  return directory;
}

test("the native strict check agrees with a physical copy and restores exact input", (t) => {
  const original =
    '// @ts-nocheck\nimport { value } from "./value.js";\nexport const result: number = value;\n';
  const directory = fixture(t, original);
  fs.writeFileSync(
    path.join(directory, "source/value.ts"),
    "// @ts-nocheck\nexport const value = 7;\n",
  );
  const result = compileGenerated(path.join(directory, "emit.json"), 1);
  assert.equal(result.status, "pass");
  assert.equal(result.batches.length, 1);
  assert.equal(result.checkedRoots, 1);
  assert.equal(result.checkedFiles, 2);
  assert.equal(result.checkedInventorySHA256, result.rootInventorySHA256);
  assert.deepEqual(
    JSON.parse(fs.readFileSync(path.join(directory, "emit.json.checked-files.json"))),
    ["index.ts", "value.ts"],
  );
  assert.equal(result.strippedFiles, 2);
  assert.equal(fs.readFileSync(path.join(directory, "source/index.ts"), "utf8"), original);
  assert.equal(fs.existsSync(path.join(directory, "strict")), false);
  fs.mkdirSync(path.join(directory, "strict"));
  for (const name of ["index.ts", "value.ts"])
    fs.writeFileSync(
      path.join(directory, "strict", name),
      fs
        .readFileSync(path.join(directory, "source", name), "utf8")
        .replaceAll("// @ts-nocheck\n", ""),
    );
  fs.writeFileSync(
    path.join(directory, "reference.json"),
    JSON.stringify({
      compilerOptions: {
        ...options,
        rootDir: "strict",
        outDir: "reference-js",
        declarationDir: "reference-dts",
      },
      include: ["strict/**/*.ts"],
    }),
  );
  const expected = spawnSync(
    process.execPath,
    [compiler, "-p", path.join(directory, "reference.json")],
    { encoding: "utf8", timeout: 60000 },
  );
  assert.equal(expected.status, 0, expected.stdout + expected.stderr);
  for (const name of ["index", "value"]) {
    assert.equal(
      fs.readFileSync(path.join(directory, "javascript", name + ".js"), "utf8"),
      fs.readFileSync(path.join(directory, "reference-js", name + ".js"), "utf8"),
    );
    assert.equal(
      fs.readFileSync(path.join(directory, "declarations", name + ".d.ts"), "utf8"),
      fs.readFileSync(path.join(directory, "reference-dts", name + ".d.ts"), "utf8"),
    );
  }
});

test("a generated no-check directive cannot hide a broken implementation", (t) => {
  const original = '// @ts-nocheck\nexport const broken: number = "not a number";\n';
  const directory = fixture(t, original);
  const result = compileGenerated(path.join(directory, "emit.json"));
  assert.equal(result.status, "fail");
  assert.match(result.diagnostics, /TS2322/);
  assert.equal(fs.existsSync(path.join(directory, "javascript")), false);
  assert.equal(fs.readFileSync(path.join(directory, "source/index.ts"), "utf8"), original);
});

test("declared optional properties retain exact assignment checks", (t) => {
  const directory = fixture(
    t,
    "// @ts-nocheck\nexport const broken: { field?: number } = { field: undefined };\n",
  );
  const result = compileGenerated(path.join(directory, "emit.json"));
  assert.equal(result.status, "fail");
  assert.match(result.diagnostics, /TS2375/);
});

test("a type error in a later root batch cannot be skipped", (t) => {
  const directory = fixture(t, "// @ts-nocheck\nexport const valid = 1;\n");
  const invalid = '// @ts-nocheck\nexport const broken: number = "wrong";\n';
  fs.writeFileSync(path.join(directory, "source/zzz.ts"), invalid);
  const result = compileGenerated(path.join(directory, "emit.json"), 1);
  assert.equal(result.status, "fail");
  assert.equal(result.batches.length, 2);
  assert.equal(result.checkedRoots, 1);
  assert.match(result.diagnostics, /TS2322/);
  assert.equal(fs.readFileSync(path.join(directory, "source/zzz.ts"), "utf8"), invalid);
});

test("an error in a type-only imported implementation fails before it can be counted as checked", (t) => {
  const directory = fixture(
    t,
    '// @ts-nocheck\nimport type { Contract } from "./value.js";\nexport const result: Contract = { value: 1 };\n',
  );
  const dependency =
    '// @ts-nocheck\nexport interface Contract { value: number }\nexport const broken: number = "wrong";\n';
  fs.writeFileSync(path.join(directory, "source/value.ts"), dependency);
  const result = compileGenerated(path.join(directory, "emit.json"), 1);
  assert.equal(result.status, "fail");
  assert.equal(result.batches.length, 1);
  assert.equal(result.checkedFiles, 0);
  assert.match(result.diagnostics, /TS2322/);
  assert.equal(fs.readFileSync(path.join(directory, "source/value.ts"), "utf8"), dependency);
});

test("sources absent from every import graph remain explicit checked roots", (t) => {
  const directory = fixture(t, '// @ts-nocheck\nexport { value } from "./value.js";\n');
  fs.writeFileSync(
    path.join(directory, "source/value.ts"),
    "// @ts-nocheck\nexport const value = 1;\n",
  );
  fs.writeFileSync(
    path.join(directory, "source/unreferenced.ts"),
    "// @ts-nocheck\nexport const other = 2;\n",
  );
  const result = compileGenerated(path.join(directory, "emit.json"), 1);
  assert.equal(result.status, "pass");
  assert.equal(result.checkedFiles, 3);
  assert.equal(result.checkedRoots, 2);
  assert.equal(result.batches.length, 2);
  assert.equal(result.checkedInventorySHA256, result.rootInventorySHA256);
  assert(fs.existsSync(path.join(directory, "declarations/unreferenced.d.ts")));
});

test("noCheck cannot bypass verification even when strict is enabled", (t) => {
  const original = '// @ts-nocheck\nexport const broken: number = "wrong";\n';
  const directory = fixture(t, original);
  const config = path.join(directory, "emit.json");
  const settings = JSON.parse(fs.readFileSync(config, "utf8"));
  settings.compilerOptions.noCheck = true;
  fs.writeFileSync(config, JSON.stringify(settings));
  assert.throws(() => compileGenerated(config), /Unchecked compilation/);
  assert.equal(fs.readFileSync(path.join(directory, "source/index.ts"), "utf8"), original);
});

test("recovery rejects another compiler's lock before changing any source", async (t) => {
  const { createHash } = await import("node:crypto");
  const { restoreGeneratedSources } = await import("./sdk-delivery-compile.mjs");
  const original = "// @ts-nocheck\nexport const value = 1;\n";
  const stripped = original.replaceAll("// @ts-nocheck\n", "");
  const directory = fixture(t, stripped);
  const sourceRoot = path.join(directory, "source");
  const filename = path.join(sourceRoot, "index.ts");
  const journalFile = path.join(directory, "interrupted.source-journal.json");
  const lock = path.join(sourceRoot, ".sdk-delivery-strict-lock");
  const otherOwner = path.join(directory, "active.source-journal.json");
  const digest = (text) => createHash("sha256").update(text).digest("hex");
  const journal = JSON.stringify({
    id: randomUUID(),
    sourceRoot,
    entries: [
      { name: "index.ts", offsets: [0], original: digest(original), stripped: digest(stripped) },
    ],
  });
  fs.writeFileSync(journalFile, journal);
  fs.writeFileSync(lock, otherOwner);
  assert.throws(() => restoreGeneratedSources(journalFile), /Another compiler owns/);
  assert.equal(fs.readFileSync(filename, "utf8"), stripped);
  assert.equal(fs.readFileSync(lock, "utf8"), otherOwner);
  assert.equal(fs.readFileSync(journalFile, "utf8"), journal);
});

for (const changed of [false, true])
  test(`interrupted-source recovery ${changed ? "preserves unexpected edits" : "restores exact bytes and releases its lock"}`, async (t) => {
    const { createHash } = await import("node:crypto");
    const { restoreGeneratedSources } = await import("./sdk-delivery-compile.mjs");
    const original = "// @ts-nocheck\n// λ\n// @ts-nocheck\nexport const value = 1;\n";
    const stripped = original.replaceAll("// @ts-nocheck\n", "");
    const directory = fixture(t, original);
    const sourceRoot = path.join(directory, "source");
    const filename = path.join(sourceRoot, "index.ts");
    const journalFile = path.join(directory, "emit.json.source-journal.json");
    const lock = path.join(sourceRoot, ".sdk-delivery-strict-lock");
    const digest = (text) => createHash("sha256").update(text).digest("hex");
    const replacement = changed ? stripped + "// unexpected edit\n" : stripped;
    fs.writeFileSync(filename, replacement);
    fs.writeFileSync(lock, journalFile);
    fs.writeFileSync(
      journalFile,
      JSON.stringify({
        id: randomUUID(),
        sourceRoot,
        entries: [
          {
            name: "index.ts",
            offsets: [0, 5],
            original: digest(original),
            stripped: digest(stripped),
          },
        ],
      }),
    );
    if (changed) {
      assert.throws(
        () => restoreGeneratedSources(journalFile),
        /Generated source changed during verification/,
      );
      assert.equal(fs.readFileSync(filename, "utf8"), replacement);
      assert(fs.existsSync(journalFile));
      assert(fs.existsSync(lock));
    } else {
      restoreGeneratedSources(journalFile);
      assert.equal(fs.readFileSync(filename, "utf8"), original);
      assert.equal(fs.existsSync(journalFile), false);
      assert.equal(fs.existsSync(lock), false);
    }
  });
