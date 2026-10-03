import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { registerHooks } from "node:module";
import { fileURLToPath, pathToFileURL } from "node:url";
import { performance } from "node:perf_hooks";

const c = JSON.parse(process.argv[2]);
const loaded = new Set();
registerHooks({
  load(url, context, next) {
    const result = next(url, context);
    if (url.startsWith("file:")) {
      const filename = fileURLToPath(url);
      if (filename.startsWith(c.output + path.sep) && filename.endsWith(".js"))
        loaded.add(path.relative(c.output, filename));
    }
    return result;
  },
});
const traces = [];
const cpuStart = process.cpuUsage();
const start = performance.now();
const module = await import(pathToFileURL(path.join(c.output, c.entry)));
const imported = performance.now();
const atImport = [...loaded].sort();
const settings = {
  baseURL: "https://selection.example.test",
  authorization: "selection-fixture",
  fetch: async (url, init) => {
    traces.push({
      url: String(url),
      method: init.method,
      authorization: new Headers(init.headers).get("authorization"),
      body: init.body ?? null,
    });
    const response = Response.json({ id: "item-1", title: "JSON", count: 2 });
    Object.defineProperty(response, "url", { value: String(url) });
    return response;
  },
};
let prepared;
if (c.lazy) prepared = await module.loadOperations(c.routes.map((route) => module.routes[route]));
const api = c.lazy
  ? module.createClient({ ...settings, operations: prepared })
  : module.createClient(settings);
const ready = performance.now();
const readyCPU = process.cpuUsage(cpuStart);
const atReady = [...loaded].sort();
assert.equal(traces.length, 0);
assert.deepEqual(Object.keys(api.$routes).sort(), [...c.routes].sort());
assert(!Object.hasOwn(api.$operations, "readLinked"));
for (const route of c.routes)
  assert.equal(
    JSON.stringify(await api.$routes[route]()),
    '{"id":"item-1","title":"JSON","count":2}',
  );
const called = performance.now();
const atCalls = [...loaded].sort();
const raw = await api.$operations.source.raw();
assert.equal(
  JSON.stringify(await api.$operations.source.links.detail(raw)),
  '{"id":"item-1","title":"JSON","count":2}',
);
const linked = performance.now();
const endCPU = process.cpuUsage(cpuStart);
assert(!Object.hasOwn(api.$operations, "readLinked"));
assert(
  traces.every(
    (t) => t.method === "GET" && t.authorization === "selection-fixture" && t.body === null,
  ),
);
assert.equal(traces.length, c.routes.length + 2);
console.log(
  JSON.stringify({
    name: c.name,
    importMS: imported - start,
    readyMS: ready - start,
    prepareMS: ready - imported,
    callsMS: called - ready,
    linkMS: linked - called,
    totalMS: linked - start,
    readyCPUms: (readyCPU.user + readyCPU.system) / 1000,
    totalCPUms: (endCPU.user + endCPU.system) / 1000,
    peakRSSKiB: process.resourceUsage().maxRSS,
    atImport,
    atReady,
    atCalls,
    atLink: [...loaded].sort(),
    traces,
    prepareHTTPRequests: 0,
  }),
);
