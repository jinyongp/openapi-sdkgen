// Compile the actual guide snippets against a freshly generated public SDK.
import assert from "node:assert/strict";
import { strictCompilerOptions } from "./strict-options.mjs";
import fs from "node:fs";
import path from "node:path";
import { randomUUID } from "node:crypto";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { spawnSync } from "node:child_process";
import { inspectGenerated, sha256 } from "./catalog.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const generated = path.join(root, "test/typescript/fixtures/generated/client");
const inventory = inspectGenerated(generated);
const english = fs.readFileSync(
  path.join(root, "docs/typescript/guide/selective-client.md"),
  "utf8",
);
const korean = fs.readFileSync(
  path.join(root, "docs/ko/typescript/guide/selective-client.md"),
  "utf8",
);
const englishReference = fs.readFileSync(
  path.join(root, "docs/typescript/reference/client-api.md"),
  "utf8",
);
const koreanReference = fs.readFileSync(
  path.join(root, "docs/ko/typescript/reference/client-api.md"),
  "utf8",
);
const blocks = (text) => [...text.matchAll(/```ts\n([\s\S]*?)```/g)].map((match) => match[1]);
function example(text, anchor, index = 0) {
  const heading = new RegExp(`^## [^\\n]*\\{#${anchor}\\}[^\\n]*\\n`, "m").exec(text);
  assert(heading, `Missing documentation section: #${anchor}`);
  const remaining = text.slice(heading.index + heading[0].length);
  const next = remaining.search(/^## /m);
  const snippet = blocks(next === -1 ? remaining : remaining.slice(0, next))[index];
  assert(snippet, `Missing TypeScript example ${index + 1} in #${anchor}`);
  return snippet;
}
function examplesFrom(guide, reference) {
  return [
    example(guide, "prepare"),
    example(guide, "features"),
    example(guide, "features", 1),
    example(reference, "computed-selection"),
    example(reference, "selection-enumeration"),
    example(guide, "bundlers"),
  ];
}
const examples = examplesFrom(english, englishReference);
assert.deepEqual(
  examplesFrom(korean, koreanReference),
  examples,
  "Localized guides and references must demonstrate the same API",
);
const directory = path.join(root, ".tmp/selection-docs", randomUUID());
fs.mkdirSync(directory, { recursive: true });
const write = (name, content) => fs.writeFileSync(path.join(directory, name), content);
const actual = (text) => text.replaceAll("./generated/api", generated);
const common = `import {createClient,loadOperations,operations,routes} from ${JSON.stringify(generated + "/selective/index.js")};\n`;
write("package.json", '{"type":"module"}\n');
write("main.ts", actual(examples[0]));
write("tasks.operations.ts", actual(examples[1]));
write("features.ts", actual(examples[2]));
write("dynamic.ts", common + actual(examples[3]));
write("enumeration.ts", actual(examples[4]));
write("static.operations.ts", actual(examples[5]));
write(
  "static-client.ts",
  actual(examples[2]).replace("./tasks.operations.js", "./static.operations.js"),
);
write(
  "tsconfig.json",
  JSON.stringify({
    compilerOptions: {
      ...strictCompilerOptions,
      target: "ES2022",
      module: "NodeNext",
      moduleResolution: "NodeNext",
      lib: ["ES2022", "DOM", "DOM.Iterable"],
      types: [],
      strict: true,
      noUncheckedIndexedAccess: true,
      exactOptionalPropertyTypes: true,
      verbatimModuleSyntax: true,
      skipLibCheck: false,
      noEmit: true,
    },
    include: ["*.ts"],
  }),
);
const require = createRequire(new URL("../package.json", import.meta.url));
const packageFile = require.resolve("typescript/package.json");
const typescript = JSON.parse(fs.readFileSync(packageFile, "utf8"));
const compiler = path.resolve(path.dirname(packageFile), typescript.bin.tsc);
const result = spawnSync(
  process.execPath,
  [compiler, "--project", path.join(directory, "tsconfig.json")],
  { cwd: root, encoding: "utf8", timeout: 120000, maxBuffer: 4 * 1024 * 1024 },
);
write("typecheck.log", (result.stdout ?? "") + (result.stderr ?? ""));
const report = {
  status: result.status === 0 ? "pass" : "fail",
  snippets: examples.length,
  compileContexts: examples.length + 1,
  generatedTreeSHA256: inventory.treeSha256,
  englishSHA256: sha256(english),
  koreanSHA256: sha256(korean),
  englishReferenceSHA256: sha256(englishReference),
  koreanReferenceSHA256: sha256(koreanReference),
  typescript: typescript.version,
  exit: result.status,
  scope:
    "Exact guide and selective-reference snippets typecheck against the real generated client; no remote API is contacted.",
};
write("report.json", JSON.stringify(report, null, 2) + "\n");
assert.equal(
  result.status,
  0,
  `Guide examples failed: ${directory}/typecheck.log\n${result.stdout ?? ""}${result.stderr ?? ""}`,
);
console.log(
  JSON.stringify({ ...report, report: path.relative(root, path.join(directory, "report.json")) }),
);
