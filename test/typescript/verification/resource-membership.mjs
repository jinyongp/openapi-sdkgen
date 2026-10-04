// Exercise the actual generated member-selection type without loading unrelated SDK leaves.
import assert from "node:assert/strict";
import { strictCompilerOptions } from "./strict-options.mjs";
import fs from "node:fs";
import path from "node:path";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { spawnSync } from "node:child_process";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const require = createRequire(new URL("../package.json", import.meta.url));
const packageFile = require.resolve("typescript/package.json");
const metadata = JSON.parse(fs.readFileSync(packageFile, "utf8"));
const compiler = path.resolve(path.dirname(packageFile), metadata.bin.tsc);
const parser = createRequire(
  path.join(root, "test/typescript/node_modules/.pnpm/node_modules/package.json"),
)("@babel/parser");
const hash = (value) => createHash("sha256").update(value).digest("hex");
const routeUnion = (mask) =>
  [0, 1, 2]
    .filter((bit) => mask & (1 << bit))
    .map((bit) => `"r${bit}"`)
    .join(" | ") || "never";

function finiteSetWitness(helper) {
  const members = { a: 1, b: 2, c: 4, ab: 3, bc: 6, all: 7, empty: 0 };
  const cases = [];
  for (let possible = 0; possible < 8; possible++) {
    for (let guaranteed = 0; guaranteed < 8; guaranteed++) {
      if ((guaranteed & possible) !== guaranteed) continue;
      // The oracle is ordinary set membership, not a second conditional-type implementation.
      const fields = Object.entries(members)
        .filter(([, routes]) => (routes & possible) !== 0)
        .map(
          ([name, routes]) =>
            `readonly ${name}${(routes & guaranteed) === 0 ? "?" : ""}: Values["${name}"];`,
        );
      cases.push(
        `type Law${cases.length} = Assert<Equal<Normalize<SelectedMembers<Values, Routes, ${routeUnion(guaranteed)}, ${routeUnion(possible)}>>, {${fields.join(" ")}}>>;`,
      );
    }
  }
  return {
    cases: cases.length,
    source: `type RouteKey = "r0" | "r1" | "r2";
interface Routes { a: "r0"; b: "r1"; c: "r2"; ab: "r0" | "r1"; bc: "r1" | "r2"; all: RouteKey; empty: never; }
interface Values { a: () => 0; b: () => 1; c: () => 2; ab: {readonly x: "a"}; bc: readonly [1, 2]; all: (x: 1) => 2; empty: never; }
type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false;
type Assert<T extends true> = T;
type Normalize<T> = { [K in keyof T]: T[K] };
${helper}
${cases.join("\n")}
`,
  };
}

function wideWitness(helper, count) {
  const routes = Array.from({ length: count }, (_, index) => `"r${index}"`).join(" | ");
  const values = Array.from(
    { length: count },
    (_, index) => `readonly k${index}: () => ${index};`,
  ).join("\n");
  const members = Array.from(
    { length: count },
    (_, index) => `readonly k${index}: "r${index}";`,
  ).join("\n");
  return `type RouteKey = ${routes};
interface Values { ${values} }
interface Routes { ${members} }
${helper}
declare const possible: SelectedMembers<Values, Routes, never, RouteKey>;
const maybe: ${count - 1} | undefined = possible.k${count - 1}?.();
// @ts-expect-error A possible member cannot be called without narrowing.
possible.k${count - 1}();
declare const required: SelectedMembers<Values, Routes, RouteKey, RouteKey>;
const sure: ${count - 1} = required.k${count - 1}();
// @ts-expect-error Selected member properties remain readonly.
required.k${count - 1} = () => ${count - 1};
declare const sparse: SelectedMembers<Values, Routes, "r${count - 1}", "r${count - 1}">;
const selected: ${count - 1} = sparse.k${count - 1}();
// @ts-expect-error An unselected route cannot appear on the resource.
sparse.k0();
void maybe; void sure; void selected;
`;
}

export function verifyResourceMembership(generatedTypes, outputDirectory) {
  const sourceFile = fs.realpathSync(generatedTypes);
  assert(sourceFile.startsWith(root), "Use generated types within this repository");
  const output = path.resolve(outputDirectory);
  assert(output.startsWith(path.join(root, ".tmp") + path.sep));
  assert(!fs.existsSync(output), "Use a fresh resource-membership evidence directory");
  fs.mkdirSync(output, { recursive: true });
  const source = fs.readFileSync(sourceFile, "utf8");
  const ast = parser.parse(source, { sourceType: "module", plugins: ["typescript"] });
  const declaration = ast.program.body.find(
    (node) => node.type === "TSTypeAliasDeclaration" && node.id.name === "SelectedMembers",
  );
  assert(declaration, "Actual generated SelectedMembers declaration is missing");
  const helper = source.slice(declaration.start, declaration.end);
  const report = {
    status: "running",
    scope:
      "Actual Go-emitted member-selection type, finite-set oracle and 10000-member type-instantiation regression. This is not the complete 10000-operation SDK or a browser measurement.",
    typescript: metadata.version,
    generatedTypes: path.relative(root, sourceFile),
    generatedTypesSHA256: hash(source),
    helperSHA256: hash(helper),
    checks: [],
  };
  const compile = (name, text) => {
    fs.writeFileSync(path.join(output, name + ".ts"), text);
    const config = path.join(output, name + ".json");
    fs.writeFileSync(
      config,
      JSON.stringify({
        compilerOptions: {
          ...strictCompilerOptions,
          strict: true,
          noEmit: true,
          skipLibCheck: false,
          exactOptionalPropertyTypes: true,
          noUncheckedIndexedAccess: true,
          types: [],
          lib: ["es2022"],
        },
        files: [name + ".ts"],
      }),
    );
    const result = spawnSync(
      process.execPath,
      [compiler, "--project", config, "--singleThreaded"],
      {
        encoding: "utf8",
        timeout: 30000,
        killSignal: "SIGKILL",
        maxBuffer: 1024 * 1024,
      },
    );
    const diagnostics = (result.stdout ?? "") + (result.stderr ?? "");
    fs.writeFileSync(path.join(output, name + ".log"), diagnostics);
    report.checks.push({
      name,
      exitCode: result.status,
      signal: result.signal,
      sourceSHA256: hash(text),
    });
    return { ...result, diagnostics };
  };
  try {
    const laws = finiteSetWitness(helper);
    report.finiteSetCases = laws.cases;
    const finite = compile("finite-set-laws", laws.source);
    assert.equal(finite.status, 0, finite.diagnostics);
    const wide = compile("wide-10000", wideWitness(helper, 10000));
    assert.equal(wide.status, 0, wide.diagnostics);
    const oldHelper = helper.replaceAll(
      /Extract<Routes\[Key\], ([GP])>/g,
      "Extract<$1, Routes[Key]>",
    );
    assert.notEqual(
      oldHelper,
      helper,
      "The negative control must restore the old global-first distribution",
    );
    const negative = compile("global-first-negative", wideWitness(oldHelper, 10000));
    assert.notEqual(negative.status, 0);
    assert.match(
      negative.diagnostics,
      /TS2589/,
      "Expected the original type-instantiation failure, not an unrelated compiler failure",
    );
    // Check the actual emitted guarantee algorithm, not a substitute test implementation.
    const selectionSource = fs.readFileSync(
      path.resolve(path.dirname(sourceFile), "../internal/runtime/client/selection-types.ts"),
      "utf8",
    );
    report.selectionTypesSHA256 = hash(selectionSource);
    fs.writeFileSync(
      path.join(output, "selection-runtime.ts"),
      selectionSource.replaceAll("// @ts-nocheck\n", ""),
    );
    const count = 10000;
    const routes = Array.from({ length: count }, (_, index) => `"r${index}"`).join(" | ");
    const fields = Array.from(
      { length: count },
      (_, index) => `readonly k${index}: OperationReference<"r${index}">;`,
    ).join("\n");
    const dense = compile(
      "dense-guarantees-10000",
      `
import type { OperationReference, GuaranteedSelection, PossibleSelection } from "./selection-runtime.js";
type Routes = ${routes};
interface Group { ${fields} }
type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false;
type Assert<T extends true> = T;
export type Required = Assert<Equal<GuaranteedSelection<Group, Routes>, Routes>>;
export type Optional = Assert<Equal<GuaranteedSelection<Partial<Group>, Routes>, never>>;
export type Array = Assert<Equal<GuaranteedSelection<Group[keyof Group][], Routes>, never>>;
export type Candidates = Assert<Equal<PossibleSelection<Partial<Group>, Routes>, Routes>>;
`,
    );
    assert.equal(dense.status, 0, dense.diagnostics);
    report.status = "pass";
    return report;
  } catch (error) {
    report.status = "fail";
    report.error = String(error?.stack ?? error);
    throw error;
  } finally {
    fs.writeFileSync(path.join(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(path.resolve(process.argv[1])).href) {
  assert(
    process.argv[2] && process.argv[3],
    "Pass generated selective/types.ts and a fresh evidence directory",
  );
  console.log(JSON.stringify(verifyResourceMembership(process.argv[2], process.argv[3])));
}
