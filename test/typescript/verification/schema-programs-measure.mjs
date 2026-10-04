import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { performance } from "node:perf_hooks";
import { pathToFileURL } from "node:url";
import { gzipSync } from "node:zlib";
import { build } from "vite";
import { sdkDeliveryDocument } from "./sdk-delivery-fixture.mjs";
import { runMeasured } from "./measured-process.mjs";
import { strictCompilerOptions } from "./strict-options.mjs";

const output = path.resolve(process.argv[2]);
const baseline = process.argv[3];
const require = createRequire(import.meta.url);
const compiler = path.join(path.dirname(require.resolve("typescript/package.json")), "lib/tsc.js");
const hash = (data) => createHash("sha256").update(data).digest("hex");
const files = (directory) =>
  fs
    .readdirSync(directory, { withFileTypes: true })
    .flatMap((entry) => {
      const file = path.join(directory, entry.name);
      return entry.isDirectory() ? files(file) : [file];
    })
    .sort();
const summarize = (selected) => ({
  files: selected.length,
  bytes: selected.reduce((sum, file) => sum + fs.statSync(file).size, 0),
});
const median = (values) =>
  values.toSorted((left, right) => left - right)[Math.floor(values.length / 2)];
const input = path.join(output, "input.json");
fs.writeFileSync(input, JSON.stringify(sdkDeliveryDocument(1000)));
const results = [];
let sequence = 0;
async function measured(label, command, args) {
  const start = performance.now();
  const result = await runMeasured(command, args, {
    cwd: process.cwd(),
    resourceFile: path.join(output, `${sequence++}-${label}.resources.json`),
    timeout: 300000,
  });
  fs.writeFileSync(path.join(output, `${label}.log`), result.stdout + result.stderr);
  assert.equal(result.status, 0, `${label}: ${result.stdout}\n${result.stderr}`);
  return { milliseconds: performance.now() - start, ...result.resources };
}
const routes = [
  ...Array.from({ length: 8 }, (_, index) => `GET /items/item${index}`),
  "GET /source",
  "GET /idless",
];
const generators = [
  ["current", path.join(output, "current-generator")],
  ...(baseline ? [["before", path.resolve(baseline)]] : []),
];
for (const [variant, generator] of generators) {
  for (const kind of ["full", "selected", "named"]) {
    const directory = path.join(output, `${variant}-${kind}`);
    for (const owned of ["checked", "js", "declarations", "bundle"]) {
      fs.rmSync(path.join(directory, owned), { recursive: true, force: true });
    }
    fs.mkdirSync(directory, { recursive: true });
    const source = path.join(directory, "source");
    const config = path.join(directory, "generator.toml");
    fs.writeFileSync(
      config,
      `source = ${JSON.stringify(input)}\ntarget = "typescript"\n` +
        (kind === "selected" ? `[selection]\nroutes = ${JSON.stringify(routes)}\n` : "") +
        (kind === "named" ? `[clients.page.selection]\nroutes = ${JSON.stringify(routes)}\n` : ""),
    );
    const runs = [];
    for (let trial = 0; trial < 3; trial++) {
      fs.rmSync(source, { recursive: true, force: true });
      runs.push(
        await measured(`${variant}-${kind}-generate-${trial}`, generator, [
          "generate",
          "--config",
          config,
          "--output",
          source,
        ]),
      );
    }
    const sources = files(source).filter((file) => file.endsWith(".ts"));
    const programs = sources.filter((file) => file.includes("/internal/schema-programs/"));
    const checked = path.join(directory, "checked");
    fs.mkdirSync(checked, { recursive: true });
    for (const file of sources) {
      const destination = path.join(checked, path.relative(source, file));
      fs.mkdirSync(path.dirname(destination), { recursive: true });
      fs.writeFileSync(
        destination,
        fs.readFileSync(file, "utf8").replace(/^\/\/ @ts-nocheck\r?\n/gm, ""),
      );
    }
    fs.writeFileSync(path.join(checked, "package.json"), '{"type":"module"}');
    const tsconfig = path.join(directory, "tsconfig.json");
    fs.writeFileSync(
      tsconfig,
      JSON.stringify({
        compilerOptions: {
          ...strictCompilerOptions,
          target: "ES2022",
          module: "NodeNext",
          moduleResolution: "NodeNext",
          lib: ["ES2022", "DOM", "DOM.Iterable"],
          types: [],
          rootDir: checked,
          outDir: path.join(directory, "js"),
        },
        include: [path.join(checked, "**/*.ts")],
      }),
    );
    const sourceCheck = await measured(`${variant}-${kind}-strict`, process.execPath, [
      compiler,
      "--project",
      tsconfig,
    ]);
    fs.writeFileSync(path.join(directory, "js/package.json"), '{"type":"module"}');
    const declarations = await measured(`${variant}-${kind}-declarations`, process.execPath, [
      compiler,
      "--project",
      tsconfig,
      "--declaration",
      "--emitDeclarationOnly",
      "--outDir",
      path.join(directory, "declarations"),
    ]);
    const sdk = path.join(directory, "js");
    const entry = path.join(sdk, "measurement.mjs");
    fs.writeFileSync(
      entry,
      `import {createClient} from ${JSON.stringify(kind === "named" ? "./clients/page/index.js" : "./index.js")};export async function run(){const api=createClient({baseURL:'https://large.test',fetch:async()=>Response.json({id:'one',title:'item',count:1})});return api.$operations.getItem0()}`,
    );
    const bundleStart = performance.now();
    const result = await build({
      configFile: false,
      logLevel: "silent",
      build: {
        target: "ES2022",
        minify: "oxc",
        write: false,
        modulePreload: false,
        rollupOptions: {
          input: entry,
          preserveEntrySignatures: "strict",
          output: { format: "es", entryFileNames: "entry.mjs", chunkFileNames: "chunk-[hash].mjs" },
        },
      },
    });
    assert.ok(!Array.isArray(result) && "output" in result);
    const chunks = result.output.filter((item) => item.type === "chunk");
    const bundled = path.join(directory, "bundle");
    fs.mkdirSync(bundled, { recursive: true });
    for (const chunk of chunks) fs.writeFileSync(path.join(bundled, chunk.fileName), chunk.code);
    const executed = await import(pathToFileURL(path.join(bundled, "entry.mjs")));
    assert.deepEqual(await executed.run(), { id: "one", title: "item", count: 1 });
    const row = {
      variant,
      kind,
      generatorSHA256: hash(fs.readFileSync(generator)),
      source: summarize(sources),
      runtime: summarize(sources.filter((file) => file.includes("/internal/runtime/"))),
      programs: summarize(programs),
      programSourceSHA256: hash(
        programs
          .map((file) => `${path.relative(source, file)}\0${hash(fs.readFileSync(file))}`)
          .join("\n"),
      ),
      generation: {
        milliseconds: median(runs.map((run) => run.milliseconds)),
        peakRSSKiB: median(runs.map((run) => run.peakRSSKiB)),
        runs,
      },
      sourceCheck,
      declarations,
      declarationSource: summarize(
        files(path.join(directory, "declarations")).filter((file) => file.endsWith(".d.ts")),
      ),
      bundle: {
        chunks: chunks.length,
        bytes: chunks.reduce((sum, item) => sum + Buffer.byteLength(item.code), 0),
        gzipBytes: chunks.reduce((sum, item) => sum + gzipSync(item.code, { level: 6 }).length, 0),
        milliseconds: performance.now() - bundleStart,
        realCallPassed: true,
      },
    };
    results.push(row);
    console.error(
      `${variant} ${kind}: ${row.source.files} TS, ${row.programs.files} shared programs, ${row.bundle.gzipBytes} gzip bytes`,
    );
  }
}
const full = results.find((row) => row.variant === "current" && row.kind === "full");
const named = results.find((row) => row.variant === "current" && row.kind === "named");
const selected = results.find((row) => row.variant === "current" && row.kind === "selected");
assert.equal(
  selected.programs.files,
  named.programs.files,
  "named entry duplicated schema programs",
);
assert.equal(
  selected.programSourceSHA256,
  named.programSourceSHA256,
  "named entry changed shared schema programs",
);
fs.writeFileSync(
  path.join(output, "results.json"),
  JSON.stringify(
    {
      measuredAt: new Date().toISOString(),
      node: process.version,
      typescript: require("typescript/package.json").version,
      operations: 1000,
      selectedRoutes: routes,
      inputSHA256: hash(fs.readFileSync(input)),
      settings: {
        target: "ES2022",
        platform: "browser",
        minify: "oxc",
        gzipLevel: 6,
        generationTrials: 3,
        compilerTrials: 1,
        beforeSource: "optional captured worktree; no historical correctness claims",
      },
      results,
    },
    null,
    2,
  ),
);
