// Reproducible request-runtime comparison. No application APIs or network access.
import assert from "node:assert/strict";
import { createHash, randomUUID } from "node:crypto";
import fs from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath, pathToFileURL } from "node:url";
import { brotliCompressSync, gzipSync, constants } from "node:zlib";
import { exerciseRuntimeDelivery } from "./runtime-delivery-fixture.mjs";

const root = fileURLToPath(new URL("../../../", import.meta.url));
const runtimePath = "internal/target/typescript/runtime/internal";
const baselineRef = process.argv[2] ?? "fec52dc85c17969aacdf0edf8de022d0ebd7af80";
const require = createRequire(new URL("../package.json", import.meta.url));
const tsPackageFile = require.resolve("typescript/package.json");
const tsPackage = JSON.parse(fs.readFileSync(tsPackageFile, "utf8"));
const tsCLI = path.resolve(path.dirname(tsPackageFile), tsPackage.bin.tsc);
const parserRequire = createRequire(
  path.join(root, "test/typescript/node_modules/.pnpm/node_modules/package.json"),
);
const parser = parserRequire("@babel/parser");
const viteRequire = createRequire(fs.realpathSync(require.resolve("vite/package.json")));
const { rolldown } = await import(pathToFileURL(viteRequire.resolve("rolldown")).href);
const runID = randomUUID();
const outputRoot = path.join(root, ".tmp/runtime-delivery");
const runDir = path.join(outputRoot, runID);
fs.mkdirSync(runDir, { recursive: true });
const hash = (value) => createHash("sha256").update(value).digest("hex");
const write = (filename, content) => {
  fs.mkdirSync(path.dirname(filename), { recursive: true });
  fs.writeFileSync(filename, content);
};
const git = (...args) => {
  const result = spawnSync("git", args, {
    cwd: root,
    encoding: "utf8",
    maxBuffer: 8 * 1024 * 1024,
  });
  if (result.status !== 0) throw new Error(result.stderr || `git ${args[0]} failed`);
  return result.stdout.trimEnd();
};
const sizes = (source) => ({
  raw: Buffer.byteLength(source),
  brotli5: brotliCompressSync(Buffer.from(source), {
    params: { [constants.BROTLI_PARAM_QUALITY]: 5 },
  }).length,
  gzip6: gzipSync(source, { level: 6 }).length,
});
const schema = {
  types: ["object"],
  properties: {
    todo_id: { property: "todoId", schema: { types: ["string"] } },
    title: { property: "title", schema: { types: ["string"] } },
  },
};
const definitions = {
  get: {
    route: "GET /todos/{id}",
    method: "GET",
    path: "/todos/{id}",
    envelope: "",
    parameters: [
      {
        location: "path",
        name: "id",
        property: "id",
        required: true,
        style: "simple",
        explode: false,
        schema: { types: ["string"] },
      },
    ],
    responses: [{ status: "200", contentType: "application/json", schema, schemaDeclared: true }],
  },
  post: {
    route: "POST /todos",
    method: "POST",
    path: "/todos",
    envelope: "",
    contentType: "application/json",
    requestBodyRequired: true,
    requestBodies: [{ contentType: "application/json", schema, schemaDeclared: true }],
    responses: [{ status: "200", contentType: "application/json", schema, schemaDeclared: true }],
  },
  xml: {
    route: "GET /xml",
    method: "GET",
    path: "/xml",
    envelope: "",
    responses: [
      {
        status: "200",
        contentType: "application/xml",
        schemaDeclared: true,
        schema: {
          types: ["object"],
          xml: { name: "todo" },
          properties: { title: { property: "title", schema: { types: ["string"] } } },
        },
      },
    ],
  },
  stream: {
    route: "GET /events",
    method: "GET",
    path: "/events",
    envelope: "",
    responses: [
      {
        status: "200",
        contentType: "application/x-ndjson",
        schema: { types: ["object"] },
        itemSchema: { types: ["object"] },
        streamFraming: "line-delimited-json",
      },
    ],
  },
};
const workloads = [
  ["get"],
  ["get", "post"],
  ["get", "xml"],
  ["get", "stream"],
  ["get", "xml", "stream"],
];

function compileSources(kind, baseline) {
  const paths =
    kind === "baseline"
      ? git("ls-tree", "-r", "--name-only", baseline, "--", runtimePath)
          .split("\n")
          .filter((name) => name.endsWith(".ts"))
      : fs
          .readdirSync(path.join(root, runtimePath))
          .filter((name) => name.endsWith(".ts"))
          .map((name) => `${runtimePath}/${name}`);
  assert(paths.length > 0, "runtime source inventory must not be empty");
  const hashes = {};
  write(path.join(runDir, kind, "package.json"), '{"type":"module"}\n');
  for (const filename of paths.sort()) {
    const source =
      kind === "baseline"
        ? git("show", `${baseline}:${filename}`) + "\n"
        : fs.readFileSync(path.join(root, filename), "utf8");
    hashes[filename] = hash(source);
    write(path.join(runDir, kind, "source", path.basename(filename)), source);
  }
  const configFile = path.join(runDir, kind, "tsconfig.json");
  write(
    configFile,
    JSON.stringify(
      {
        compilerOptions: {
          target: "ES2022",
          module: "ESNext",
          moduleResolution: "Bundler",
          lib: ["ES2022", "DOM", "DOM.Iterable"],
          types: [],
          strict: true,
          noUncheckedIndexedAccess: true,
          exactOptionalPropertyTypes: true,
          verbatimModuleSyntax: true,
          noEmitOnError: true,
          outDir: "runtime",
          rootDir: "source",
        },
        include: ["source/*.ts"],
      },
      null,
      2,
    ),
  );
  const compiled = spawnSync(process.execPath, [tsCLI, "--project", configFile], {
    cwd: root,
    encoding: "utf8",
    timeout: 120000,
    maxBuffer: 8 * 1024 * 1024,
  });
  write(
    path.join(runDir, kind, "typecheck.log"),
    (compiled.stdout ?? "") + (compiled.stderr ?? ""),
  );
  assert.equal(
    compiled.status,
    0,
    `${kind} strict runtime compile: ${compiled.stdout ?? ""}${compiled.stderr ?? ""}`,
  );
  return hashes;
}
function createEntry(kind, names) {
  const file = path.join(runDir, kind, names.join("-"), "entry.js");
  const compact =
    kind === "candidate" && fs.existsSync(path.join(runDir, kind, "runtime/http-json.js"));
  const hasJSON = names.some((name) => name === "get" || name === "post");
  const hasFull = names.includes("stream");
  const hasXML = names.includes("xml");
  const imports = compact
    ? [
        'import { createRequestContext } from "../runtime/http-core.js";',
        'import { createRequestCore, createHTTPServices } from "../runtime/http-core.js";',
        ...(hasJSON ? ['import { jsonWireCodec } from "../runtime/wire-engine.js";'] : []),
        ...(hasFull
          ? ['import { jsonResponseStreamServices } from "../runtime/http-stream.js";']
          : []),
      ]
    : ['import { createRequest } from "../runtime/http.js";'];
  if (compact && hasXML)
    imports.push(
      'import { xmlWireCodec, bufferedXMLCodecExtensions } from "../runtime/codecs.js";',
    );
  let source = imports.join("\n") + "\nexport function createAPIs(options) {\n";
  if (compact) {
    source += "const context = createRequestContext(options);\n";
    if (hasFull) source += "const full = createRequestCore(context, jsonResponseStreamServices);\n";
    if (hasJSON)
      source +=
        "const json = createRequestCore(context, createHTTPServices(jsonWireCodec, {encodeRequestBody(_contentType, value) { return JSON.stringify(value); }}));\n";
    if (hasXML)
      source +=
        "const xml = createRequestCore(context, createHTTPServices(xmlWireCodec, bufferedXMLCodecExtensions));\n";
  } else source += "const full = createRequest(options);\n";
  source += "return {\n";
  for (const name of names) {
    const request = compact
      ? name === "get" || name === "post"
        ? "json"
        : name === "xml"
          ? "xml"
          : "full"
      : "full";
    source += `${name}: Object.assign((input, options) => ${request}(${JSON.stringify(definitions[name])}, input, options), {raw: (input, options) => ${request}.raw(${JSON.stringify(definitions[name])}, input, options)`;
    if (name === "stream")
      source += `, stream: (input, options) => ${request}.stream(${JSON.stringify(definitions[name])}, input, options)`;
    source += "}),\n";
  }
  source += "};\n}\n";
  write(file, source);
  return file;
}
function graph(entry) {
  const files = new Set();
  const visit = (filename) => {
    filename = path.resolve(filename);
    if (files.has(filename)) return;
    assert(filename.startsWith(runDir + path.sep), "runtime import escaped the comparison tree");
    files.add(filename);
    const text = fs.readFileSync(filename, "utf8");
    const source = parser.parse(text, { sourceType: "module" });
    for (const node of source.program.body) {
      if (
        (node.type === "ImportDeclaration" ||
          node.type === "ExportNamedDeclaration" ||
          node.type === "ExportAllDeclaration") &&
        node.source?.type === "StringLiteral"
      ) {
        const specifier = node.source.value;
        assert(specifier.startsWith("."), `unexpected external import ${specifier}`);
        visit(path.resolve(path.dirname(filename), specifier));
      }
    }
  };
  visit(entry);
  return [...files].sort();
}
async function exercise(entry, names) {
  const traces = [];
  const { createAPIs } = await import(pathToFileURL(entry).href);
  const browserFixture = await exerciseRuntimeDelivery(
    createAPIs,
    names,
    "https://browser-fixture.test/sdk-case",
  );
  assert(browserFixture.checks.includes("two-client-isolation"));
  if (names.includes("post")) {
    for (const corrupt of [
      (init) => ({ ...init, method: "GET" }),
      (init) => ({
        ...init,
        headers: new Headers({
          "content-type": "text/plain",
          authorization: "Bearer first-fixture",
        }),
      }),
      (init) => ({ ...init, body: JSON.stringify({ todo_id: "wrong", title: "JSON" }) }),
    ]) {
      await assert.rejects(
        exerciseRuntimeDelivery(
          (options) =>
            createAPIs({
              ...options,
              fetch: (url, init) =>
                options.fetch(url, init.method === "POST" ? corrupt(init) : init),
            }),
          names,
          "https://browser-fixture.test/sdk-case",
        ),
      );
    }
  }
  const api = createAPIs({
    baseURL: "https://example.test",
    authorization: "Bearer local-fixture",
    fetch: async (url, init) => {
      traces.push({
        url: String(url),
        method: init.method,
        headers: [...new Headers(init.headers)],
        body: init.body,
      });
      if (String(url).endsWith("/events"))
        return new Response('{"n":1}\n{"n":2}\n', {
          headers: { "content-type": "application/x-ndjson" },
        });
      if (String(url).endsWith("/xml"))
        return new Response("<todo><title>XML</title></todo>", {
          headers: { "content-type": "application/xml" },
        });
      return Response.json({ todo_id: "todo-1", title: "JSON" });
    },
  });
  const values = {};
  for (const name of names) {
    if (name === "stream") {
      values[name] = [];
      const result = api.stream.stream();
      assert.equal(typeof result.then, "undefined");
      for await (const item of result) values[name].push(item);
    } else
      values[name] = await api[name](
        name === "get"
          ? { path: { id: "a/b" } }
          : name === "post"
            ? { body: { todoId: "todo-1", title: "JSON" } }
            : undefined,
      );
  }
  if (api.get) {
    assert.equal(values.get.todoId, "todo-1");
    const before = traces.length;
    await assert.rejects(api.get({ path: {} }));
    await assert.rejects(api.get({ path: { id: ".." } }));
    await assert.rejects(
      api.get({ path: { id: "1" } }, { signal: AbortSignal.abort(new Error("cancel")) }),
    );
    assert.equal(traces.length, before);
    const raw = await api.get.raw({ path: { id: "1" } });
    assert.equal(raw.status, 200);
    assert.equal(raw.data.todoId, "todo-1");
  }
  return { traces, values };
}
const report = {
  runID,
  status: "running",
  baselineRef,
  node: process.version,
  typescript: tsPackage.version,
  sourceHashes: {},
  workloads: [],
};
try {
  report.baselineCommit = git("rev-parse", "--verify", `${baselineRef}^{commit}`);
  report.candidateHead = git("rev-parse", "HEAD");
  for (const kind of ["baseline", "candidate"])
    report.sourceHashes[kind] = compileSources(kind, report.baselineCommit);
  for (const names of workloads) {
    let baselineResult;
    for (const kind of ["baseline", "candidate"]) {
      const entry = createEntry(kind, names);
      const nativeFiles = graph(entry);
      if (kind === "candidate" && fs.existsSync(path.join(runDir, kind, "runtime/http-json.js"))) {
        assert(
          !nativeFiles.some((file) => /\/(http-advanced|http-codecs)\.js$/.test(file)),
          "Focused fixtures must not load the full media adapter",
        );
        if (!names.includes("xml"))
          assert(
            !nativeFiles.some((file) => /\/(wire-xml|codecs)\.js$/.test(file)),
            "JSON fixtures must not load XML through a value or empty type import",
          );
        if (!names.includes("stream"))
          assert(
            !nativeFiles.some((file) =>
              /\/(streaming|http-stream|http-json-stream)\.js$/.test(file),
            ),
            "Buffered fixtures must not load stream execution",
          );
      }
      const nativeSize = nativeFiles.reduce(
        (total, filename) => {
          const size = sizes(fs.readFileSync(filename));
          for (const key of Object.keys(total)) total[key] += size[key];
          return total;
        },
        { raw: 0, brotli5: 0, gzip6: 0 },
      );
      const result = await exercise(entry, names);
      if (kind === "baseline") baselineResult = result;
      else assert.deepEqual(result, baselineResult, `${names.join("+")} native differential`);
      const build = await rolldown({
        input: entry,
        platform: "browser",
        treeshake: true,
        onwarn: (warning) => {
          throw new Error(warning.message);
        },
      });
      const dir = path.join(path.dirname(entry), "bundle");
      const output = await build.write({
        dir,
        format: "esm",
        minify: true,
        sourcemap: false,
        entryFileNames: "index.js",
      });
      await build.close();
      const chunks = output.output.filter((item) => item.type === "chunk");
      assert.equal(chunks.length, 1);
      assert.deepEqual(
        await exercise(path.join(dir, "index.js"), names),
        result,
        `${names.join("+")} bundle differential`,
      );
      const row = {
        names,
        kind,
        native: {
          files: nativeFiles.length,
          ...nativeSize,
          inventory: nativeFiles.map((file) => path.relative(runDir, file)),
        },
        bundle: { ...sizes(chunks[0].code), sha256: hash(chunks[0].code) },
        semantic: "pass",
      };
      report.workloads.push(row);
      console.log(
        JSON.stringify({
          names,
          kind,
          native: { files: row.native.files, brotli5: nativeSize.brotli5 },
          bundleBrotli5: row.bundle.brotli5,
          semantic: row.semantic,
        }),
      );
    }
  }
  report.status = "pass";
  report.scope =
    "Runtime native Node ESM and minified static bundle differential; native bytes are per-file compressed inventory, not HTTP transfer or latency. Separate runtime typecheck is required.";
} catch (error) {
  report.status = "fail";
  report.error = { name: error.name, message: error.message, stack: error.stack };
  throw error;
} finally {
  write(path.join(runDir, "report.json"), JSON.stringify(report, null, 2) + "\n");
  write(
    path.join(outputRoot, `${runID}.latest.tmp`),
    JSON.stringify(
      {
        runID,
        status: report.status,
        report: path.relative(root, path.join(runDir, "report.json")),
      },
      null,
      2,
    ) + "\n",
  );
  fs.renameSync(path.join(outputRoot, `${runID}.latest.tmp`), path.join(outputRoot, "latest.json"));
  console.log(
    `Runtime delivery ${report.status}: ${path.relative(root, path.join(runDir, "report.json"))}`,
  );
}
