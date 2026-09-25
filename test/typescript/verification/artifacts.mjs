import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  writeFileSync,
  existsSync,
  statfsSync,
} from "node:fs";
import { dirname, resolve, relative } from "node:path";
import { parseArgs } from "node:util";
import { pathToFileURL, fileURLToPath } from "node:url";
import { performance } from "node:perf_hooks";
import { gzipSync, brotliCompressSync, constants } from "node:zlib";
import { build } from "vite";
import {
  repositoryRoot,
  containedPath,
  inspectGenerated,
  managedDeclarationStats,
  sha256,
} from "./catalog.mjs";
import { pairedSummary } from "./contracts.mjs";
import { assertWirePropertiesContract } from "./helper-contracts.mjs";

const { values } = parseArgs({
  options: {
    report: { type: "string" },
    fixtures: { type: "string" },
    pairs: { type: "string", default: "3" },
  },
});
assert.ok(values.report, "--report must point to a completed representation-check report");
const reportPath = containedPath(repositoryRoot, values.report);
const input = JSON.parse(readFileSync(reportPath, "utf8"));
assert.equal(input.correctness, "pass", "source comparison must pass before artifact verification");
const pairs = Number(values.pairs);
assert.ok(Number.isInteger(pairs) && pairs > 0 && pairs <= 30);
const chosen = values.fixtures ? new Set(values.fixtures.split(",")) : null;
const fixtures = input.fixtures.filter((f) => !chosen || chosen.has(f.id));
if (chosen)
  for (const id of chosen)
    assert.ok(
      fixtures.some((f) => f.id === id),
      `missing fixture ${id}`,
    );
const disk = statfsSync(dirname(reportPath));
const sourceBytes = fixtures.reduce(
  (sum, f) => sum + f.generated.baseline.typescriptBytes + f.generated.candidate.typescriptBytes,
  0,
);
assert.ok(
  disk.bavail * disk.bsize > Math.max(512 * 1024 * 1024, sourceBytes * 5),
  "insufficient artifact-check storage",
);
const output = mkdtempSync(resolve(dirname(reportPath), "artifacts-"));
const resultPath = resolve(output, "report.json");
const report = {
  version: 1,
  status: "running",
  sourceReport: relative(repositoryRoot, reportPath),
  sourceReportSha256: sha256(readFileSync(reportPath)),
  candidateLabel: input.candidateLabel,
  binarySha256: input.binarySha256,
  node: process.version,
  pairs,
  finalProductionApproval: false,
  fixtures: [],
};
report.harnessSha256 = Object.fromEntries(
  ["artifacts.mjs", "helper-contracts.mjs", "catalog.mjs", "contracts.mjs"].map((name) => [
    name,
    sha256(readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), name))),
  ]),
);
const save = () => writeFileSync(resultPath, JSON.stringify(report, null, 2) + "\n");
const tsc = resolve(repositoryRoot, "test/typescript/node_modules/typescript/lib/tsc.js");
const baseOptions = {
  target: "ES2022",
  module: "NodeNext",
  moduleResolution: "NodeNext",
  lib: ["ES2022", "DOM", "DOM.Iterable"],
  strict: true,
  noUncheckedIndexedAccess: true,
  verbatimModuleSyntax: true,
  isolatedModules: true,
  skipLibCheck: false,
  types: [],
};
const consumer = `import { createClient, type Components, type Operations } from "./index.js";\ntype Equal<L,R>=(<T>()=>T extends L?1:2) extends (<T>()=>T extends R?1:2)?true:false;\ntype Assert<T extends true>=T;\ntype ID=keyof Operations;\ntype Operation=Operations[ID];\ntype Component=Components[keyof Components];\ntype Keys=Assert<Equal<keyof ReturnType<typeof createClient>["$operations"],ID>>;\n// @ts-expect-error An unknown operation must remain rejected.\nconst unknownOperation: ID = "__verification_nonexistent_operation__";\nexport type Probe=[Operation["input"],Operation["output"],Operation["call"],Component["input"],Component["output"],Keys];\nvoid unknownOperation;\n`;
function runTypecheck(args, log) {
  const start = performance.now();
  const p = spawnSync(process.execPath, [tsc, ...args], {
    cwd: repositoryRoot,
    encoding: "utf8",
    timeout: 240000,
    maxBuffer: 8 * 1024 * 1024,
  });
  writeFileSync(log, (p.stdout ?? "") + (p.stderr ?? ""));
  if (p.error) throw p.error;
  assert.notEqual(p.status, null, "TypeScript terminated without an exit status");
  return {
    status: p.status,
    wallMs: performance.now() - start,
    text: (p.stdout ?? "") + (p.stderr ?? ""),
  };
}
function command(args, log) {
  const result = runTypecheck(args, log);
  assert.equal(result.status, 0, "TypeScript failed: " + log + "\n" + result.text);
  return result.wallMs;
}
function diagnosticSignature(result, sourceRoot) {
  const normalized = result.text
    .replaceAll(relative(repositoryRoot, sourceRoot) + "/", "")
    .replaceAll(sourceRoot + "/", "")
    .replace(/\(\d+,\d+\): error /g, ": error ")
    .replace(/__sdkgen_[A-Za-z0-9_]+/g, "__sdkgen_Private")
    .trim();
  if (result.status !== 0)
    assert.ok(
      /error TS\d+/.test(normalized),
      "non-TypeScript failure must not be classified as a baseline diagnostic",
    );
  return normalized;
}
async function bundle(source, fixture, variant) {
  const entry = resolve(output, `${fixture}-${variant}-bundle.ts`);
  writeFileSync(
    entry,
    `export { createClient } from ${JSON.stringify(resolve(source, "index.ts"))};\n`,
  );
  const built = await build({
    root: source,
    configFile: false,
    logLevel: "silent",
    build: {
      target: "es2022",
      minify: "oxc",
      write: false,
      rollupOptions: {
        input: entry,
        preserveEntrySignatures: "strict",
        output: { format: "es", codeSplitting: false },
      },
    },
  });
  const chunks = (Array.isArray(built) ? built : [built])
    .flatMap((r) => r.output)
    .filter((o) => o.type === "chunk");
  assert.equal(chunks.length, 1);
  const code = chunks[0].code;
  writeFileSync(resolve(output, `${fixture}-${variant}.mjs`), code);
  return {
    minifiedBytes: Buffer.byteLength(code),
    gzipBytes: gzipSync(code).length,
    brotliBytes: brotliCompressSync(code, { params: { [constants.BROTLI_PARAM_QUALITY]: 11 } })
      .length,
    modules: Object.keys(chunks[0].modules).length,
    sha256: sha256(code),
  };
}
save();
console.log(`artifact report: ${relative(repositoryRoot, resultPath)}`);
try {
  for (const fixture of fixtures) {
    const row = { id: fixture.id, variants: {}, samples: [] };
    report.fixtures.push(row);
    for (const variant of ["baseline", "candidate"]) {
      const source = resolve(dirname(reportPath), fixture.id, variant, "source");
      const audit = inspectGenerated(source);
      assert.equal(
        audit.treeSha256,
        fixture.generated[variant].treeSha256,
        "generated source changed after comparison",
      );
      const esmRoot = resolve(dirname(reportPath), fixture.id, variant, "esm");
      const esmHash = sha256(
        JSON.stringify(
          Object.keys(audit.files)
            .filter((f) => f.endsWith(".ts"))
            .sort()
            .map((f) => [
              f,
              sha256(readFileSync(containedPath(esmRoot, f.replace(/\.ts$/, ".js")))),
            ]),
        ),
      );
      assert.equal(
        esmHash,
        fixture.generated[variant].esmTreeSha256,
        "transpiled artifact changed after comparison",
      );
      const checkRoot = resolve(output, fixture.id, variant),
        strict = resolve(checkRoot, "strict"),
        consumerRoot = resolve(checkRoot, "consumer"),
        declarations = resolve(checkRoot, "declarations");
      for (const name of Object.keys(audit.files).filter((f) => f.endsWith(".ts"))) {
        const text = readFileSync(containedPath(source, name), "utf8");
        for (const [dir, checked] of [
          [strict, true],
          [consumerRoot, false],
        ]) {
          const destination = containedPath(dir, name);
          mkdirSync(dirname(destination), { recursive: true });
          writeFileSync(destination, checked ? text.replaceAll("// @ts-nocheck\n", "") : text);
        }
      }
      for (const dir of [strict, consumerRoot]) {
        writeFileSync(resolve(dir, "package.json"), '{"type":"module","private":true}\n');
        writeFileSync(resolve(dir, "consumer.ts"), consumer);
        writeFileSync(
          resolve(dir, "tsconfig.json"),
          JSON.stringify({
            compilerOptions: { ...baseOptions, noEmit: true },
            include: dir === strict ? ["**/*.ts"] : ["consumer.ts"],
          }),
        );
      }
      const declarationConfig = resolve(consumerRoot, "declaration.json");
      writeFileSync(
        declarationConfig,
        JSON.stringify({
          compilerOptions: {
            ...baseOptions,
            declaration: true,
            emitDeclarationOnly: true,
            outDir: declarations,
          },
          include: ["**/*.ts"],
        }),
      );
      command(["--project", declarationConfig], resolve(checkRoot, "declaration.log"));
      writeFileSync(resolve(declarations, "package.json"), '{"type":"module","private":true}\n');
      writeFileSync(resolve(checkRoot, "package.json"), '{"type":"module","private":true}\n');
      const downstream = resolve(checkRoot, "downstream.ts");
      writeFileSync(downstream, consumer.replace('"./index.js"', '"./declarations/index.js"'));
      const downstreamConfig = resolve(checkRoot, "downstream.json");
      writeFileSync(
        downstreamConfig,
        JSON.stringify({ compilerOptions: { ...baseOptions, noEmit: true }, files: [downstream] }),
      );
      command(["--project", downstreamConfig], resolve(checkRoot, "downstream.log"));
      const helper = resolve(
        dirname(reportPath),
        fixture.id,
        variant,
        "esm/internal/runtime/wire-properties.js",
      );
      const helperContract = existsSync(helper)
        ? await (async () => {
            const h = await import(pathToFileURL(helper));
            const codecs = await import(pathToFileURL(resolve(dirname(helper), "codecs.js")));
            if (h.wireProperties.length === 2) {
              assert.throws(() => h.wireProperties(["x"], []), /count mismatch/);
              assert.throws(() => h.wireProperties([], [{}]), /count mismatch/);
            }
            const fn =
              h.wireProperties.length === 2
                ? (entries) =>
                    h.wireProperties(
                      entries.map((e) => e[0]),
                      entries.map((e) => e[1]),
                    )
                : h.wireProperties;
            return assertWirePropertiesContract(fn, codecs);
          })()
        : { status: "not-applicable", reason: "baseline has no helper" };
      const strictProbe = runTypecheck(
        ["--project", resolve(strict, "tsconfig.json")],
        resolve(checkRoot, "strict-initial.log"),
      );
      const strictSignature = diagnosticSignature(strictProbe, strict);
      row.variants[variant] = {
        strictStatus: strictProbe.status === 0 ? "pass" : "failed",
        strictDiagnostics: strictProbe.text,
        strictSignature,
        sourceBytes: audit.typescriptBytes,
        sourceTreeSha256: audit.treeSha256,
        strictConfig: resolve(strict, "tsconfig.json"),
        consumerConfig: resolve(consumerRoot, "tsconfig.json"),
        declarations: managedDeclarationStats(declarations, audit.files),
        declarationConsumer: "pass",
        helperContract,
        bundle: await bundle(source, fixture.id, variant),
      };
    }
    assert.equal(
      row.variants.candidate.strictSignature,
      row.variants.baseline.strictSignature,
      "candidate introduced or changed strict diagnostics; inspect raw logs",
    );
    const baselineBlocked = row.variants.baseline.strictStatus !== "pass";
    for (let pair = 0; pair < pairs; pair++) {
      const sample = {};
      for (const variant of pair % 2 ? ["candidate", "baseline"] : ["baseline", "candidate"]) {
        const v = row.variants[variant];
        sample[variant] = {
          strictMs: baselineBlocked
            ? 0
            : command(
                ["--project", v.strictConfig],
                resolve(dirname(v.strictConfig), `check-${pair}.log`),
              ),
          consumerMs: command(
            ["--project", v.consumerConfig],
            resolve(dirname(v.consumerConfig), `check-${pair}.log`),
          ),
        };
      }
      row.samples.push(sample);
    }
    row.typecheck = {
      strict: baselineBlocked
        ? {
            status: "baseline-blocked",
            timing: "not-measured",
            reason:
              "baseline and candidate both fail strict source checking with equivalent recorded diagnostics",
          }
        : pairedSummary(row.samples, "strictMs"),
      consumer: pairedSummary(row.samples, "consumerMs"),
    };
    row.status = baselineBlocked ? "baseline-blocked" : "pass";
    save();
    console.log(`${row.status} ${fixture.id}: strict/source/declaration/helper/bundle checks`);
  }
  report.baselineBlockers = report.fixtures
    .filter((row) => row.status === "baseline-blocked")
    .map((row) => row.id);
  report.status = report.baselineBlockers.length ? "review" : "pass";
  if (report.status === "review") process.exitCode = 2;
} catch (error) {
  report.status = "failed";
  report.error = { message: error.message, stack: error.stack };
  process.exitCode = 1;
} finally {
  report.finishedAt = new Date().toISOString();
  save();
}
console.log(
  JSON.stringify({
    status: report.status,
    report: relative(repositoryRoot, resultPath),
    error: report.error?.message,
  }),
);
