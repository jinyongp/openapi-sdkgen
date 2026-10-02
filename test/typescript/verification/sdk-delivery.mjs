// Generate and verify the real public SDK before measuring its execution dependencies.
import assert from "node:assert/strict";
import { strictCompilerOptions } from "./strict-options.mjs";
import fs from "node:fs";
import path from "node:path";
import { randomUUID, createHash } from "node:crypto";
import { createRequire } from "node:module";
import { fileURLToPath, pathToFileURL } from "node:url";
import { spawnSync } from "node:child_process";
import { parseArgs } from "node:util";
import { brotliCompressSync, gzipSync, constants } from "node:zlib";
import { inspectGenerated, managedDeclarationStats } from "./catalog.mjs";
import os from "node:os";
import { requireVerificationSpace } from "./sdk-delivery-compile.mjs";
import { runMeasured } from "./measured-process.mjs";
import { writeFixedInput } from "./sdk-delivery-input.mjs";
import {
  sdkDeliveryDocument,
  sdkDeliveryWorkloads,
  exerciseGeneratedSDK,
} from "./sdk-delivery-fixture.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const { values } = parseArgs({
  options: {
    generator: { type: "string" },
    sizes: { type: "string", default: "100,1000,10000" },
    "skip-links": { type: "boolean", default: false },
    "source-tree": { type: "string" },
    "resume-run": { type: "string" },
    "document-version": { type: "string", default: "1" },
  },
});
assert(values.generator, "Pass the built generator path explicitly");
const sizes = values.sizes.split(",").map(Number);
assert(
  sizes.length &&
    sizes.every((n) => [100, 1000, 10000].includes(n)) &&
    new Set(sizes).size === sizes.length,
);
const generatorBytes = fs.readFileSync(path.resolve(root, values.generator));
const require = createRequire(new URL("../package.json", import.meta.url));
const tsFile = require.resolve("typescript/package.json");
const tsPackage = JSON.parse(fs.readFileSync(tsFile, "utf8"));
const compiler = path.resolve(path.dirname(tsFile), tsPackage.bin.tsc);
const parser = createRequire(
  path.join(root, "test/typescript/node_modules/.pnpm/node_modules/package.json"),
)("@babel/parser");
const viteRequire = createRequire(fs.realpathSync(require.resolve("vite/package.json")));
const { rolldown } = await import(pathToFileURL(viteRequire.resolve("rolldown")).href);
const hash = (bytes) => createHash("sha256").update(bytes).digest("hex");
const runID = values["resume-run"] ?? randomUUID();
assert.match(runID, /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/);
const directory = path.join(root, ".tmp/sdk-delivery", runID);
if (values["resume-run"]) {
  assert(values["source-tree"], "Interrupted-run recovery requires the verified source tree");
  assert(fs.existsSync(directory), "Interrupted run does not exist");
  assert(
    !fs.existsSync(path.join(directory, "report.json")),
    "Completed or failed reports are immutable; use a new run",
  );
  assert.equal(
    hash(fs.readFileSync(path.join(directory, "generator"))),
    hash(generatorBytes),
    "Interrupted run used another generator",
  );
}
fs.mkdirSync(directory, { recursive: true });
const generator = path.join(directory, "generator");
if (!values["resume-run"]) fs.writeFileSync(generator, generatorBytes, { mode: 0o700 });
const write = (name, data) => {
  fs.mkdirSync(path.dirname(name), { recursive: true });
  fs.writeFileSync(name, data);
};
const run = async (label, command, args, timeout = 240000) => {
  const resourceFile = path.join(directory, `${label}.resources.json`);
  const started = performance.now();
  const result = await runMeasured(command, args, { cwd: root, resourceFile, timeout });
  write(path.join(directory, `${label}.log`), (result.stdout ?? "") + (result.stderr ?? ""));
  assert.equal(
    result.status,
    0,
    `${label} failed (${result.error?.message ?? result.signal ?? result.status}); see ${directory}/${label}.log`,
  );
  return {
    stdout: result.stdout ?? "",
    elapsedMS: performance.now() - started,
    resources: result.resources,
  };
};
const stat = (bytes) => ({
  raw: bytes.length,
  brotli5: brotliCompressSync(bytes, { params: { [constants.BROTLI_PARAM_QUALITY]: 5 } }).length,
  gzip6: gzipSync(bytes, { level: 6 }).length,
});
function bundleInventory(chunks) {
  const total = { raw: 0, brotli5: 0, gzip6: 0 };
  const inventory = chunks.map((chunk) => {
    const bytes = Buffer.from(chunk.code),
      size = stat(bytes);
    for (const key of Object.keys(total)) total[key] += size[key];
    return { path: chunk.fileName, ...size, sha256: hash(bytes) };
  });
  return { ...total, chunks: chunks.length, inventory };
}
function graph(start, sdkRoot) {
  const pending = [...start],
    seen = new Set();
  while (pending.length) {
    const file = path.resolve(pending.pop());
    if (seen.has(file)) continue;
    assert(file.startsWith(sdkRoot + path.sep), `SDK edge escaped tree: ${file}`);
    seen.add(file);
    const ast = parser.parse(fs.readFileSync(file, "utf8"), { sourceType: "module" });
    for (const node of ast.program.body) {
      if (
        ["ImportDeclaration", "ExportNamedDeclaration", "ExportAllDeclaration"].includes(
          node.type,
        ) &&
        node.source?.type === "StringLiteral"
      ) {
        assert(
          node.source.value.startsWith("."),
          `Unexpected external SDK dependency ${node.source.value}`,
        );
        pending.push(path.resolve(path.dirname(file), node.source.value));
      }
    }
  }
  return [...seen].sort();
}
const fileStatistics = new Map();
function inventory(files, sdkRoot) {
  const totals = { raw: 0, brotli5: 0, gzip6: 0 };
  const items = files.map((file) => {
    let item = fileStatistics.get(file);
    if (!item) {
      const bytes = fs.readFileSync(file);
      item = { ...stat(bytes), sha256: hash(bytes) };
      fileStatistics.set(file, item);
    }
    for (const key of Object.keys(totals)) totals[key] += item[key];
    return { path: path.relative(sdkRoot, file), ...item };
  });
  return { ...totals, files: items.length, inventory: items };
}
function lookup(route) {
  const digest = hash("route\0" + route);
  return "selective/lookup/r-" + digest.slice(0, 16) + "/" + digest.slice(16) + ".js";
}
const report = {
  schemaVersion: 2,
  documentVersion: values["document-version"],
  runID,
  status: "running",
  scope:
    "Actual Go-generated SDK, strict source/declarations, native ESM and static bundles. File inventories are not browser HTTP measurements; generation/compile timings are single observations, not latency percentiles. Dense selection is measured for overhead and correctness rather than assumed smaller than the full client.",
  node: process.version,
  typescript: tsPackage.version,
  generatorSHA256: hash(fs.readFileSync(generator)),
  sourceCommit: spawnSync("git", ["rev-parse", "HEAD"], {
    cwd: root,
    encoding: "utf8",
  }).stdout.trim(),
  sourceDirty:
    spawnSync("git", ["status", "--porcelain"], { cwd: root, encoding: "utf8" }).stdout.trim() !==
    "",
  environment: {
    os: os.platform(),
    architecture: os.arch(),
    cpu: os.cpus()[0]?.model,
    totalMemoryBytes: os.totalmem(),
  },
  strictCompilerOptions,
  sizes,
  skipped: values["skip-links"] ? ["lazy-Link invocation"] : [],
  workloads: [],
  generations: [],
  storage: [],
};
write(path.join(directory, "package.json"), '{"type":"module"}\n');
try {
  for (const count of sizes) {
    report.storage.push({
      count,
      ...requireVerificationSpace(
        root,
        count === 10000 ? 2 * 1024 * 1024 * 1024 : 512 * 1024 * 1024,
      ),
    });
    const base = path.join(directory, String(count));
    const input = path.join(base, "input.json");
    const document = sdkDeliveryDocument(count, values["document-version"]);
    assert.equal(Object.keys(document.paths).length, count);
    const inputBytes = JSON.stringify(document);
    const source = values["source-tree"]
        ? fs.realpathSync(path.resolve(root, values["source-tree"]))
        : path.join(base, "source"),
      javascript = path.join(base, "javascript");
    let generated;
    if (values["source-tree"]) {
      assert.equal(sizes.length, 1, "Reusing source requires one explicit size");
      assert(source.startsWith(path.join(root, ".tmp/sdk-delivery") + path.sep));
      const original = path.dirname(source);
      assert.equal(
        hash(fs.readFileSync(path.join(path.dirname(original), "generator"))),
        report.generatorSHA256,
        "Reused source was generated by a different compiler binary",
      );
      writeFixedInput(input, inputBytes, path.join(original, "input.json"));
      generated = { elapsedMS: null };
      report.reusedSource = path.relative(root, source);
    } else {
      writeFixedInput(input, inputBytes);
      generated = await run(`${count}-generate`, generator, [
        "generate",
        "--input",
        input,
        "--output",
        source,
        "--target",
        "typescript",
      ]);
    }
    const owned = inspectGenerated(source);
    assert(
      owned.files["selective/index.ts"],
      "The actual generator does not yet emit the public selective entry",
    );
    const options = {
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
      noEmitOnError: true,
    };
    write(
      path.join(base, "emit.json"),
      JSON.stringify({
        compilerOptions: {
          ...options,
          declaration: true,
          rootDir: source,
          outDir: "javascript",
          declarationDir: "declarations",
        },
        include: [path.join(source, "**/*.ts")],
      }),
    );
    const compiled = await run(
      `${count}-strict-declarations`,
      process.execPath,
      [
        path.join(root, "test/typescript/verification/sdk-delivery-compile.mjs"),
        path.join(base, "emit.json"),
      ],
      1800000,
    );
    const compilation = JSON.parse(compiled.stdout);
    assert.equal(compilation.status, "pass");
    assert.equal(
      compilation.checkedFiles,
      Object.keys(owned.files).filter((file) => file.endsWith(".ts")).length,
      "Generated source coverage is incomplete",
    );
    assert.equal(compilation.checkingPolicy, "strict-roots-and-resolved-imports");
    assert.equal(compilation.checkedInventorySHA256, compilation.rootInventorySHA256);
    assert.equal(compilation.physicalStrictCopy, false);
    assert.equal(
      inspectGenerated(source).treeSha256,
      owned.treeSha256,
      "Strict compilation changed generated sources",
    );
    const witness = `import { routes, createClient, loadOperations } from "./declarations/selective/index.js";
const prepared = await loadOperations([routes["GET /items/item0"], routes["POST /echo"]]);
const api = createClient({operations:prepared});
const result: string = (await api.$operations.getItem0()).id;
void result;
void api.items.item0.get();
void api.$routes["POST /echo"]({body:{id:"a",title:"b",count:1}});
// @ts-expect-error An operation that was not selected is not guaranteed.
api.$operations.getItem1();
// @ts-expect-error An unselected resource node must not exist.
api.items.item1.get();
// @ts-expect-error Body field types must remain exact.
api.$routes["POST /echo"]({body:{id:"a",title:"b",count:"wrong"}});
const maybe = [routes["GET /items/item0"], routes["GET /items/item1"]].filter(()=>Math.random()>0.5);
const dynamic = createClient({operations:await loadOperations(maybe)});
// @ts-expect-error Filter does not guarantee either operation.
dynamic.$operations.getItem0();
void dynamic.$operations.getItem0?.();
import { routes as allRoutes } from "./declarations/selective/all.js";
const every = createClient({operations:await loadOperations(allRoutes)});
const everyID: string = (await every.$operations.getItem0()).id;
void every.items.item0.get();
void everyID;
const possibleEvery = createClient({operations:await loadOperations(Object.values(allRoutes))});
// @ts-expect-error A computed array does not promise nonempty membership.
possibleEvery.$operations.getItem0();
const possibleID: string | undefined = (await possibleEvery.$operations.getItem0?.())?.id;
void possibleID;
`;
    write(path.join(base, "consumer.ts"), witness);
    write(
      path.join(base, "consumer.json"),
      JSON.stringify({ compilerOptions: { ...options, noEmit: true }, files: ["consumer.ts"] }),
    );
    const consumed = await run(`${count}-declaration-consumer`, process.execPath, [
      compiler,
      "--project",
      path.join(base, "consumer.json"),
      "--listFiles",
    ]);
    assert(
      !consumed.stdout.split(/\r?\n/).some((line) => line.startsWith(source + path.sep)),
      "Consumer read source instead of declarations",
    );
    const bootstrap = inventory(
      graph([path.join(javascript, "selective/index.js")], javascript),
      javascript,
    );
    const baseline = inventory(graph([path.join(javascript, "index.js")], javascript), javascript);
    const fullEntry = path.join(base, "full-entry.mjs");
    write(fullEntry, 'export {createClient} from "./javascript/index.js";\n');
    const fullBuild = await rolldown({
      input: fullEntry,
      platform: "browser",
      treeshake: true,
      onwarn: (warning) => {
        throw Error(warning.message);
      },
    });
    const fullOutput = await fullBuild.write({
      dir: path.join(base, "bundle-full"),
      format: "esm",
      minify: true,
      sourcemap: false,
      entryFileNames: "index.js",
    });
    await fullBuild.close();
    const fullBundle = bundleInventory(fullOutput.output.filter((item) => item.type === "chunk"));
    for (const entry of bootstrap.inventory)
      assert(
        !/\/(operations|executions|schemas|resources)\//.test(entry.path),
        "Bootstrap imports operation data",
      );
    if (report.generations.length)
      assert.equal(
        bootstrap.raw,
        report.generations[0].bootstrap.raw,
        "Bootstrap grows with operation count",
      );
    report.generations.push({
      count,
      inputSHA256: hash(fs.readFileSync(input)),
      ownedTreeSHA256: owned.treeSha256,
      generationMS: generated.elapsedMS,
      generationResources: generated.resources ?? null,
      compileMS: compiled.elapsedMS,
      compileResources: compiled.resources,
      source: { files: owned.fileCount, bytes: owned.typescriptBytes },
      declarations: managedDeclarationStats(path.join(base, "declarations"), owned.files),
      compilation,
      declarationConsumerMS: consumed.elapsedMS,
      declarationConsumerResources: consumed.resources,
      bootstrap,
      baseline,
      fullBundle,
      javascript: path.relative(root, javascript),
    });
    for (const workload of sdkDeliveryWorkloads(count)) {
      const entries = [
        path.join(javascript, "selective/index.js"),
        path.join(javascript, "internal/runtime/selected-client.js"),
        ...workload.routes.map((route) => path.join(javascript, lookup(route))),
      ];
      const selected = inventory(graph(entries, javascript), javascript);
      if (["one", "ten", "json-post", "idless"].includes(workload.name)) {
        for (const item of selected.inventory)
          assert(
            !/\/(codecs|http-codecs|http-advanced|http-stream|streaming|wire-xml)\.js$/.test(
              item.path,
            ),
            `Unrelated runtime imported by ${workload.name}: ${item.path}`,
          );
        assert(selected.brotli5 < baseline.brotli5, "JSON selection must reduce native SDK bytes");
      }
      for (const item of selected.inventory)
        assert(
          !/\/(client\/registry|schemas\/wire)\.js$/.test(item.path),
          "Selected graph contains the full registry",
        );
      const worker = path.join(base, `native-${workload.name}.mjs`);
      write(
        worker,
        `import assert from "node:assert/strict";\nimport {createHash} from "node:crypto";\nconst exercise=${exerciseGeneratedSDK.toString()};\nconst routes=${JSON.stringify(workload.routes)};\nconst selected=await exercise(await import("./javascript/selective/index.js"),routes,{selected:true,followLink:${!values["skip-links"]}});\nconst full=await exercise(await import("./javascript/index.js"),routes,{followLink:${!values["skip-links"]}});\nassert.deepEqual(selected.traces,full.traces);\nconsole.log(JSON.stringify({pass:true,requests:selected.traces.length,tracesSHA256:createHash("sha256").update(JSON.stringify(selected.traces)).digest("hex")}));\n`,
      );
      const executed = await run(`${count}-${workload.name}-native`, process.execPath, [worker]);
      const semantic = JSON.parse(executed.stdout);
      const row = {
        count,
        name: workload.name,
        routes: workload.routes,
        bootstrap,
        selected,
        baseline,
        semantic,
      };
      if (
        ["one", "json-post", "json-xml", "json-stream", "mixed"].includes(workload.name) &&
        count === sizes[0]
      ) {
        const providerReferences = workload.routes.map((route) => {
          const source = fs.readFileSync(path.join(javascript, lookup(route)), "utf8");
          const ast = parser.parse(source, { sourceType: "module" });
          const specifier = ast.program.body.find((node) => node.type === "ImportDeclaration")
            .source.value;
          const provider = path.relative(
            javascript,
            path.resolve(path.dirname(path.join(javascript, lookup(route))), specifier),
          );
          return provider.replace(/^internal\/executions\//, "selective/operations/");
        });
        const entry = path.join(base, `static-${workload.name}.mjs`);
        write(
          entry,
          providerReferences
            .map(
              (file, index) =>
                `import {operation as ref${index}} from ${JSON.stringify("./javascript/" + file)};`,
            )
            .join("\n") +
            `\nimport {loadOperations,createClient} from "./javascript/selective/index.js";\nexport {createClient};\nexport function prepare(){return loadOperations([${providerReferences.map((_, index) => `ref${index}`).join(",")}]);}\n`,
        );
        const build = await rolldown({
          input: entry,
          platform: "browser",
          treeshake: true,
          onwarn: (warning) => {
            throw Error(warning.message);
          },
        });
        const bundleDir = path.join(base, `bundle-${workload.name}`);
        const output = await build.write({
          dir: bundleDir,
          format: "esm",
          minify: true,
          sourcemap: false,
          entryFileNames: "index.js",
        });
        await build.close();
        const chunks = output.output.filter((item) => item.type === "chunk");
        row.staticBundle = bundleInventory(chunks);
        row.staticBundle.full = fullBundle;
        assert(
          row.staticBundle.brotli5 < fullBundle.brotli5,
          `Static sparse bundle regression in ${workload.name}`,
        );
        const bundleWorker = path.join(base, `bundle-${workload.name}.mjs`);
        write(
          bundleWorker,
          `const module=await import(${JSON.stringify("./bundle-" + workload.name + "/index.js")});\nconst prepared=await module.prepare();\nconst exercise=${exerciseGeneratedSDK.toString()};\nconst result=await exercise({loadOperations:async()=>prepared,createClient:module.createClient,routes:{}},${JSON.stringify(workload.routes)},{selected:true});\nconsole.log(JSON.stringify({pass:true,requests:result.traces.length}));\n`,
        );
        row.staticBundle.semantic = JSON.parse(
          (await run(`${count}-${workload.name}-bundle`, process.execPath, [bundleWorker])).stdout,
        );
      }
      report.workloads.push(row);
      write(path.join(directory, "report.json"), JSON.stringify(report, null, 2));
      console.log(
        JSON.stringify({
          count,
          workload: workload.name,
          nativeBrotli: selected.brotli5,
          fullBrotli: baseline.brotli5,
          files: selected.files,
          semantic: semantic.pass,
        }),
      );
    }
  }
  report.status = values["skip-links"] ? "partial" : "pass";
} catch (error) {
  report.status = "fail";
  report.error = String(error?.stack ?? error);
  throw error;
} finally {
  write(path.join(directory, "report.json"), JSON.stringify(report, null, 2) + "\n");
  write(
    path.join(root, ".tmp/sdk-delivery/latest.json"),
    JSON.stringify({
      runID,
      status: report.status,
      report: path.relative(root, path.join(directory, "report.json")),
    }) + "\n",
  );
}
