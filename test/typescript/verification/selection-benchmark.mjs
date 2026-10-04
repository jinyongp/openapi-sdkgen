import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { fileURLToPath, pathToFileURL } from "node:url";
import { gzipSync, brotliCompressSync, constants } from "node:zlib";
import { performance } from "node:perf_hooks";
import { sdkDeliveryDocument } from "./sdk-delivery-fixture.mjs";
import { execFileSync } from "node:child_process";
import os from "node:os";
import { runMeasured } from "./measured-process.mjs";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
const base = path.join(root, ".tmp/selection-benchmark");
if (process.argv.includes("--help")) {
  console.log(
    "Measures 1000 APIs / 10 selections; requires build:dev and installed TypeScript verification dependencies. Writes .tmp/selection-benchmark/report.json. Three generation/bundle trials and six runtime trials.",
  );
  process.exit(0);
}
fs.mkdirSync(base, { recursive: true });
const retained = path.join(base, "configs");
fs.mkdirSync(retained, { recursive: true });
fs.writeFileSync(path.join(retained, "input.json"), JSON.stringify(sdkDeliveryDocument(1000)));

const generator = path.join(base, "generator");
fs.copyFileSync(path.join(root, ".tmp/bin/openapi-sdkgen"), generator);
fs.chmodSync(generator, 0o700);
const compiler = path.join(root, "test/typescript/node_modules/typescript/lib/tsc.js");
const require = createRequire(path.join(root, "test/typescript/package.json"));
const viteRequire = createRequire(fs.realpathSync(require.resolve("vite/package.json")));
const rolldownPath = viteRequire.resolve("rolldown");
const { rolldown } = await import(pathToFileURL(rolldownPath).href);
const routes = [
  ...Array.from({ length: 8 }, (_, i) => `GET /items/item${i}`),
  "GET /source",
  "GET /idless",
];
const hash = (data) => createHash("sha256").update(data).digest("hex");
function files(dir) {
  return fs
    .readdirSync(dir, { withFileTypes: true })
    .flatMap((e) => (e.isDirectory() ? files(path.join(dir, e.name)) : [path.join(dir, e.name)]));
}
function inventory(names, dir) {
  const records = names.map((file) => {
    const data = fs.readFileSync(path.join(dir, file));
    return {
      file,
      bytes: data.length,
      gzipBytes: gzipSync(data, { level: 6 }).length,
      brotliBytes: brotliCompressSync(data, { params: { [constants.BROTLI_PARAM_QUALITY]: 5 } })
        .length,
      sha256: hash(data),
    };
  });
  return {
    files: records.length,
    bytes: records.reduce((n, f) => n + f.bytes, 0),
    gzipBytes: records.reduce((n, f) => n + f.gzipBytes, 0),
    brotliBytes: records.reduce((n, f) => n + f.brotliBytes, 0),
    records,
  };
}
const allRoutes = Object.entries(
  JSON.parse(fs.readFileSync(path.join(retained, "input.json"))).paths,
).map(([route, path]) => `${Object.keys(path)[0].toUpperCase()} ${route}`);
for (const kind of ["full", "all", "selected", "named", "combined"])
  fs.writeFileSync(
    path.join(retained, `${kind}.toml`),
    `source = ${JSON.stringify(path.join(retained, "input.json"))}\ntarget = "typescript"\n` +
      (["all", "selected", "combined"].includes(kind)
        ? `[selection]\nroutes = ${JSON.stringify(kind === "all" ? allRoutes : routes)}\n`
        : "") +
      (["named", "combined"].includes(kind)
        ? `[clients.page.selection]\nroutes = ${JSON.stringify(routes)}\n`
        : ""),
  );
fs.writeFileSync(path.join(base, "progress.log"), "");
function progress(message) {
  fs.appendFileSync(path.join(base, "progress.log"), message + "\n");
}
let seq = 0;
async function run(label, command, args) {
  const result = await runMeasured(command, args, {
    cwd: root,
    resourceFile: path.join(base, `${seq++}-${label}.resource.json`),
    timeout: 300000,
    maxBuffer: 8 * 1024 * 1024,
  });
  assert.equal(result.status, 0, `${label}: ${result.stdout}\n${result.stderr}`);
  return result;
}
const generation = [];
for (const kind of ["full", "all", "selected", "named", "combined"])
  generation.push({ name: kind, runs: [] });
for (let trial = 0; trial < 3; trial++)
  for (const c of [...generation.slice(trial), ...generation.slice(0, trial)]) {
    const source = path.join(base, "generation", c.name, "source");
    fs.rmSync(source, { recursive: true, force: true });
    const start = performance.now();
    const result = await run(`generation-${c.name}-${trial}`, generator, [
      "generate",
      "--config",
      path.join(retained, `${c.name}.toml`),
      "--output",
      source,
    ]);
    c.runs.push({ seconds: (performance.now() - start) / 1000, ...result.resources });
    c.inventory = inventory(
      files(source)
        .map((f) => path.relative(source, f))
        .sort(),
      source,
    );
    progress(`generation ${c.name} ${trial + 1}: ${c.runs.at(-1).seconds.toFixed(3)}s`);
  }
assert.deepEqual(
  generation.find((c) => c.name === "full").inventory,
  generation.find((c) => c.name === "all").inventory,
);
assert.deepEqual(
  generation.find((c) => c.name === "named").inventory,
  generation.find((c) => c.name === "combined").inventory,
);
for (const c of generation)
  c.summary = summary(c.runs, ["seconds", "userSeconds", "systemSeconds", "peakRSSKiB"]);
const trees = {};
for (const kind of ["full", "selected", "named"]) {
  const directory = path.join(base, kind),
    source = path.join(directory, "source"),
    javascript = path.join(directory, "javascript");
  fs.rmSync(directory, { recursive: true, force: true });
  fs.mkdirSync(directory, { recursive: true });
  await run(`generate-${kind}`, generator, [
    "generate",
    "--config",
    path.join(retained, `${kind}.toml`),
    "--output",
    source,
  ]);
  fs.writeFileSync(path.join(source, "package.json"), '{"type":"module"}');
  const config = path.join(directory, "emit.json");
  fs.writeFileSync(
    config,
    JSON.stringify({
      compilerOptions: {
        target: "ES2022",
        module: "NodeNext",
        moduleResolution: "NodeNext",
        lib: ["ES2022", "DOM", "DOM.Iterable"],
        types: [],
        strict: true,
        skipLibCheck: false,
        noEmitOnError: true,
        rootDir: source,
        outDir: javascript,
      },
      include: [path.join(source, "**/*.ts")],
    }),
  );
  await run(`emit-${kind}`, process.execPath, [compiler, "--project", config]);
  fs.writeFileSync(path.join(javascript, "package.json"), '{"type":"module"}');
  trees[kind] = { source, javascript };
  progress(`prepared ${kind}`);
}
const cases = [
  { name: "selection", tree: "selected", kind: "direct", entry: "index.js", lazy: false },
  {
    name: "named-selection",
    tree: "named",
    kind: "direct",
    entry: "clients/page/index.js",
    lazy: false,
  },
  { name: "full-static", tree: "full", kind: "static", lazy: true },
  { name: "selected-static", tree: "selected", kind: "static", lazy: true },
  { name: "full-lookup", tree: "full", kind: "lookup", lazy: true },
  { name: "selected-lookup", tree: "selected", kind: "lookup", lazy: true },
];
for (const c of cases) {
  const tree = trees[c.tree],
    entry = path.join(tree.javascript, `probe-${c.name}.mjs`);
  if (c.kind === "direct") fs.writeFileSync(entry, `export {createClient} from './${c.entry}';\n`);
  else if (c.kind === "lookup")
    fs.writeFileSync(
      entry,
      `export {createClient,loadOperations,routes} from './selective/index.js';\n`,
    );
  else {
    const references = routes.map((route) => {
      const digest = hash("route\0" + route),
        lookup = path.join(
          tree.javascript,
          "selective/lookup",
          `r-${digest.slice(0, 16)}`,
          digest.slice(16) + ".js",
        );
      const specifier = fs
        .readFileSync(lookup, "utf8")
        .match(/import \{ provider \} from "([^"]+)"/)[1];
      return path
        .relative(tree.javascript, path.resolve(path.dirname(lookup), specifier))
        .split(path.sep)
        .join("/")
        .replace("internal/executions/", "selective/operations/");
    });
    fs.writeFileSync(
      entry,
      references.map((ref, i) => `import {operation as ref${i}} from './${ref}';`).join("\n") +
        `\nexport {createClient,loadOperations} from './selective/index.js';\nexport const routes={${routes.map((route, i) => `${JSON.stringify(route)}:ref${i}`).join(",")}};\n`,
    );
  }
  c.input = { app: entry };
  if (c.kind === "lookup")
    for (const f of files(path.join(tree.javascript, "selective/lookup")).filter((f) =>
      f.endsWith(".js"),
    )) {
      const relative = path
        .relative(path.join(tree.javascript, "selective"), f)
        .split(path.sep)
        .join("/")
        .slice(0, -3);
      c.input[relative] = f;
    }
  c.lookupEntryCount = Object.keys(c.input).length - 1;
  c.buildRuns = [];
  c.runtime = [];
}
for (let trial = 0; trial < 3; trial++)
  for (const c of [...cases.slice(trial), ...cases.slice(0, trial)]) {
    const dist = path.join(base, "bundles", c.name);
    fs.rmSync(dist, { recursive: true, force: true });
    const start = performance.now();
    const build = await rolldown({
      input: c.input,
      platform: "browser",
      treeshake: true,
      preserveEntrySignatures: "strict",
      onwarn(warning) {
        throw Error(warning.message);
      },
    });
    const emitted = await build.write({
      dir: dist,
      format: "esm",
      minify: true,
      sourcemap: false,
      entryFileNames: "selective/[name].js",
      chunkFileNames: "selective/shared-[hash].js",
    });
    await build.close();
    c.buildRuns.push({
      ms: performance.now() - start,
      processRSSKiB: process.resourceUsage().maxRSS,
    });
    c.output = dist;
    fs.writeFileSync(path.join(dist, "package.json"), '{"type":"module"}');
    c.deployed = inventory(
      emitted.output
        .filter((o) => o.type === "chunk")
        .map((o) => o.fileName)
        .sort(),
      dist,
    );
    progress(`build ${c.name} ${trial + 1}: ${c.deployed.files} JS, ${c.deployed.bytes} B`);
  }
let expected;
for (let trial = 0; trial < 6; trial++)
  for (const c of [...cases.slice(trial), ...cases.slice(0, trial)]) {
    const spec = {
      name: c.name,
      output: c.output,
      entry: "selective/app.js",
      routes,
      lazy: c.lazy,
    };
    const result = await run(`runtime-${c.name}-${trial}`, process.execPath, [
      path.join(root, "test/typescript/verification/selection-benchmark-runtime.mjs"),
      JSON.stringify(spec),
    ]);
    const runtime = JSON.parse(result.stdout);
    expected ??= runtime.traces;
    assert.deepEqual(runtime.traces, expected);
    const deployedSet = new Set(c.deployed.records.map((r) => r.file));
    for (const f of runtime.atLink)
      assert(deployedSet.has(f), `undeployed runtime dependency: ${f}`);
    c.runtime.push({
      ...runtime,
      loaded: Object.fromEntries(
        [
          ["import", runtime.atImport],
          ["ready", runtime.atReady],
          ["link", runtime.atLink],
        ].map(([stage, names]) => [stage, inventory(names, c.output)]),
      ),
    });
    progress(`run ${c.name} ${trial + 1}: ready ${runtime.readyMS.toFixed(2)} ms`);
  }
function summary(runs, keys) {
  return Object.fromEntries(
    keys.map((key) => {
      const v = runs.map((r) => r[key]).sort((a, b) => a - b);
      return [key, { median: v[Math.floor(v.length / 2)], min: v[0], max: v.at(-1) }];
    }),
  );
}
for (const c of cases) {
  c.summary = summary(c.runtime, [
    "importMS",
    "readyMS",
    "prepareMS",
    "linkMS",
    "totalMS",
    "peakRSSKiB",
  ]);
  c.buildSummary = summary(c.buildRuns, ["ms"]);
  for (const stage of ["import", "ready", "link"])
    assert(
      c.runtime.every(
        (r) => JSON.stringify(r.loaded[stage]) === JSON.stringify(c.runtime[0].loaded[stage]),
      ),
    );
}
const report = {
  schemaVersion: 1,
  measuredAt: new Date().toISOString(),
  commit: execFileSync("git", ["rev-parse", "HEAD"], { cwd: root, encoding: "utf8" }).trim(),
  environment: {
    cpu: os.cpus()[0]?.model,
    os: os.type(),
    release: os.release(),
    architecture: os.arch(),
    totalMemoryBytes: os.totalmem(),
    go: execFileSync("go", ["version"], { encoding: "utf8" }).trim(),
  },
  generation,
  node: process.version,
  typescript: JSON.parse(fs.readFileSync(path.join(compiler, "../../package.json"))).version,
  rolldown: JSON.parse(fs.readFileSync(viteRequire.resolve("rolldown/package.json"))).version,
  generatorSHA256: hash(fs.readFileSync(generator)),
  inputSHA256: hash(fs.readFileSync(path.join(retained, "input.json"))),
  routes,
  conditions:
    "Current generator, same 1000-operation source and 10 direct routes plus one private Link target. Rolldown minified browser-platform split ESM, executed in Node 24 with mocked fetch, not browser/network latency. Static references import exactly the same 10 providers. Dynamic lookup deployment includes all generated operation and route lookup entry modules as minified bundler inputs, preserving hashed paths; no unbundled raw JS copied. Initial/import, ready, Link-followed loaded files and all deployable chunks measured separately. gzip level 6 and Brotli quality 5 summed per response/file. Three rotated bundle builds, six rotated fresh runtime processes per case, warm filesystem, serial workloads. Build process high-water RSS not used for per-build comparisons.",
  cases,
};
fs.writeFileSync(path.join(base, "report.json"), JSON.stringify(report, null, 2));
console.error("Benchmark results: .tmp/selection-benchmark/report.json");
console.error(
  JSON.stringify(
    cases.map((c) => ({
      name: c.name,
      ready: c.runtime[0].loaded.ready.bytes,
      link: c.runtime[0].loaded.link.bytes,
      total: c.deployed.bytes,
      readyMS: c.summary.readyMS.median,
    })),
  ),
);
