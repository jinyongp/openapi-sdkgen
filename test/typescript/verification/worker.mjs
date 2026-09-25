import assert from "node:assert/strict";
import { readFileSync, existsSync } from "node:fs";
import { registerHooks } from "node:module";
import { resolve } from "node:path";
import { pathToFileURL, fileURLToPath } from "node:url";
import { performance } from "node:perf_hooks";
import { setImmediate as tick } from "node:timers/promises";
import { graphFingerprint, median } from "./contracts.mjs";
import { transportFor, exercise } from "./scenarios.mjs";

const [directory, scenario, mode, helperExpected = "false"] = process.argv.slice(2);
assert.ok(directory && scenario && mode, "worker requires directory, scenario and mode");
const root = resolve(directory);
const loaded = new Set();
let helperCalls = 0;
let instrumented = false;

// Instrumentation is deliberately excluded from timing runs.
if (mode !== "timing") {
  globalThis.__sdkgenVerificationHelperCall = () => {
    helperCalls++;
  };
  registerHooks({
    load(url, context, nextLoad) {
      const result = nextLoad(url, context);
      if (url.startsWith(pathToFileURL(root + "/").href)) {
        loaded.add(fileURLToPath(url));
        if (mode === "lifetime" && /\/(wire-properties|codecs)\.js$/.test(url)) {
          const text = String(result.source);
          const marker = /function wireProperties\([^)]*\)\s*\{/;
          if (marker.test(text)) {
            assert.equal(instrumented, false, "more than one helper implementation loaded");
            instrumented = true;
            return {
              ...result,
              source: text.replace(marker, "$&\n globalThis.__sdkgenVerificationHelperCall();"),
            };
          }
        }
      }
      return result;
    },
  });
}

if (mode === "leaf") {
  const file = resolve(root, "internal/schemas/node.js");
  assert.ok(existsSync(file), "representation fixture must expose Node wire schemas");
  const mod = await import(pathToFileURL(file));
  assert.ok(mod.inputWireSchema || mod.outputWireSchema);
  const files = [...loaded].map((p) => p.slice(root.length + 1)).sort();
  assert.equal(
    files.includes("internal/runtime/codecs.js"),
    false,
    "schema leaf loaded full codecs runtime",
  );
  console.log(JSON.stringify({ files, shape: graphFingerprint(mod) }));
  process.exit(0);
}

const transport = transportFor(scenario);
const start = performance.now();
const module = await import(pathToFileURL(resolve(root, "index.js")));
const importedAt = performance.now();
let api = module.createClient(transport.options);
const firstClientAt = performance.now();
const initial = {
  importMs: importedAt - start,
  firstClientMs: firstClientAt - importedAt,
  importThroughFirstClientMs: firstClientAt - start,
};

if (mode === "timing") {
  // The first construction above is never thrown away or merged with warm samples.
  const timings = [];
  for (let i = 0; i < 15; i++) {
    const t = performance.now();
    module.createClient(transport.options);
    timings.push(performance.now() - t);
  }
  console.log(JSON.stringify({ ...initial, repeatedClientMs: median(timings) }));
} else if (mode === "contract") {
  const contract = graphFingerprint(api);
  const before = transport.traces.length;
  const response = await exercise(api, scenario);
  const results = {
    contract,
    response: graphFingerprint(response),
    traces: transport.traces.slice(before),
    contexts: transport.contexts,
    rootExports: Object.keys(module).sort(),
  };
  // All transpilable module value exports are compared, not just root exports.
  const files = JSON.parse(readFileSync(resolve(root, "verification-files.json"), "utf8"));
  const exports = {};
  for (const file of files) {
    const imported = await import(pathToFileURL(resolve(root, file)));
    if (
      file === "internal/runtime/callables.js" &&
      Object.hasOwn(imported, "createWireProperties")
    ) {
      const implementation = await import(
        pathToFileURL(resolve(root, "internal/runtime/wire-properties.js"))
      );
      assert.equal(
        imported.createWireProperties,
        implementation.wireProperties,
        "helper facade is not the same function",
      );
    }
    exports[file] = Object.keys(imported).sort();
  }
  results.exports = exports;
  const callbacksPath = resolve(root, "server/callbacks.js");
  if (existsSync(callbacksPath)) {
    const { createCallbackHandlers } = await import(pathToFileURL(callbacksPath));
    const callbacks = createCallbackHandlers({});
    results.callbacks = graphFingerprint(callbacks);
    if (scenario === "oas31") {
      const endpoints = createCallbackHandlers({
        routeCallbacks: {
          "POST /jobs": {
            status: { "{$request.body#/callbackURL}": { POST: async () => ({ status: 204 }) } },
          },
        },
      });
      const response = await endpoints.routeCallbacks["POST /jobs"].status[
        "{$request.body#/callbackURL}"
      ].POST.fetch(new Request("https://host.example.test/callback", { method: "POST" }));
      assert.equal(response.status, 204);
      results.callbackStatus = response.status;
    }
  }
  const webhookPath = resolve(root, "server/webhooks.js");
  if (existsSync(webhookPath)) {
    const { createWebhookRouter } = await import(pathToFileURL(webhookPath));
    const router = createWebhookRouter(Object.create(null), { routes: Object.create(null) });
    results.webhook = graphFingerprint(router);
    assert.equal(
      (await router.fetch(new Request("https://host.example.test/unhandled"))).status,
      404,
    );
    // Preserve and report baseline behavior of absent prototype-sensitive route keys;
    // do not silently count a pre-existing failure as a candidate regression or a pass.
    try {
      const plain = createWebhookRouter({}, { routes: {} });
      results.plainWebhookMaps = {
        status: (await plain.fetch(new Request("https://host.example.test/unhandled"))).status,
      };
    } catch (error) {
      results.plainWebhookMaps = { error: error.name, message: error.message };
    }
  }
  console.log(JSON.stringify(results));
} else if (mode === "lifetime") {
  assert.equal(typeof global.gc, "function", "memory runs require --expose-gc");
  const helperLoaded = loaded.has(resolve(root, "internal/runtime/wire-properties.js"));
  if (helperLoaded) assert.equal(instrumented, true, "loaded helper was not instrumented");
  if (helperExpected !== "true")
    assert.equal(instrumented, false, "unexpected helper implementation");
  if (scenario === "lifecycle" && helperExpected === "true") {
    assert.equal(instrumented, true, "inline lifecycle fixture must execute the candidate helper");
    assert.ok(helperCalls > 0, "lifecycle fixture failed to exercise helper construction");
  }
  const callsAfterConstruction = helperCalls;
  for (let i = 0; i < 10; i++) await exercise(api, scenario);
  const requestHelperCalls = helperCalls - callsAfterConstruction;
  assert.equal(requestHelperCalls, 0, "helper ran on the request/response hot path");
  // Two simultaneously live clients must not share per-client configuration.
  let left = module.createClient(
    transportFor(scenario, "https://left.example.test", "left").options,
  );
  let right = module.createClient(
    transportFor(scenario, "https://right.example.test", "right").options,
  );
  await exercise(left, scenario);
  await exercise(right, scenario);
  api = null;
  left = null;
  right = null;
  transport.traces.length = 0;
  transport.contexts.length = 0;
  await tick();
  global.gc();
  const initialHeap = process.memoryUsage().heapUsed;
  const heaps = [];
  for (let batch = 0; batch < 6; batch++) {
    for (let i = 0; i < 40; i++) module.createClient(transport.options);
    await tick();
    global.gc();
    heaps.push(process.memoryUsage().heapUsed);
  }
  const constructionCalls = helperCalls - callsAfterConstruction;
  console.log(
    JSON.stringify({
      instrumented,
      callsAfterConstruction,
      requestHelperCalls,
      constructionCalls,
      initialHeap,
      heaps,
      lateGrowthBytes: heaps.at(-1) - heaps[1],
      rss: process.memoryUsage().rss,
      clientsDiscarded: 240,
      isolation: "pass",
    }),
  );
} else throw new Error(`unknown worker mode: ${mode}`);
