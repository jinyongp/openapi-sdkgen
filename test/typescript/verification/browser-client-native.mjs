// Runs the public generated API with native ESM, including the actual Go-emitted lookup tree.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { createHash } from "node:crypto";

const root = path.resolve(process.argv[2]);
const load = (fixture, file) => import(pathToFileURL(path.join(root, fixture, file)).href);
const lifecycle = await load("lifecycle", "browser/index.js");
const contract = await load("client", "browser/index.js");
const fullContract = await load("client", "index.js");
const exact = await load("selection-public", "browser/index.js");
const exactNames = await load("selection-public", "browser/all.js");
const exactFull = await load("selection-public", "index.js");
const checks = [];

let getterReads = 0;
const feature = {
  get calls() {
    getterReads++;
    return [lifecycle.operations.echoInline];
  },
};
const [prepared, concurrent] = await Promise.all([
  lifecycle.loadOperations([feature, feature, lifecycle.routes["POST /inline"]]),
  lifecycle.loadOperations([lifecycle.operations.echoInline]),
]);
assert.equal(getterReads, 1);
assert.equal(await Promise.resolve(prepared), prepared);
const calls = [];
const fetch = async (url, init) => {
  calls.push({
    url: String(url),
    method: init.method,
    authorization: new Headers(init.headers).get("authorization"),
    body: JSON.parse(init.body),
  });
  return Response.json(JSON.parse(init.body));
};
const first = lifecycle.createClient({
  operations: prepared,
  baseURL: "https://first.test",
  authorization: "Bearer first",
  fetch,
});
const second = lifecycle.createClient({
  operations: concurrent,
  baseURL: "https://second.test",
  authorization: "Bearer second",
  fetch,
});
assert.equal(calls.length, 0);
assert.deepEqual(Object.keys(first.$routes), ["POST /inline"]);
assert.equal(first.inline.post, first.$operations.echoInline);
assert.equal(first.$routes["POST /inline"], first.$operations.echoInline);
assert.notEqual(first.inline.post, second.inline.post);
assert.equal(Object.hasOwn(first, "events"), false);
assert.equal((await first.inline.post({ body: { value: 1 } })).value, 1);
assert.equal((await second.inline.post.raw({ body: { value: 2 } })).data.value, 2);
assert.deepEqual(
  calls.map((row) => row.authorization),
  ["Bearer first", "Bearer second"],
);
assert.deepEqual(
  calls.map((row) => row.url),
  ["https://first.test/inline", "https://second.test/inline"],
);
assert(calls.every((row) => row.method === "POST"));
await assert.rejects(first.inline.post({ body: { value: -1 } }), { code: "REQUEST_ENCODE_FAILED" });
await assert.rejects(first.inline.post({ body: { value: 1 } }, { signal: AbortSignal.abort() }), {
  code: "REQUEST_ABORTED",
});
assert.equal(calls.length, 2);
checks.push("public-native-lookup-alias-dedup-getter-and-client-isolation");

const staticReference = (await load("lifecycle", "browser/operations/inline/post.js")).operation;
const staticClient = lifecycle.createClient({
  operations: await lifecycle.loadOperations([staticReference]),
  baseURL: "https://static.test",
  fetch,
});
assert.equal((await staticClient.inline.post({ body: { value: 3 } })).value, 3);
checks.push("static-and-dynamic-reference-compatibility");

const streaming = lifecycle.createClient({
  operations: await lifecycle.loadOperations([lifecycle.routes["GET /events"]]),
  baseURL: "https://stream.test",
  fetch: async () =>
    new Response('{"value":1}\n{"value":2}\n', {
      headers: { "content-type": "application/x-ndjson" },
    }),
});
const stream = streaming.events.get.stream();
assert.equal(typeof stream[Symbol.asyncIterator], "function");
assert.equal(typeof stream.then, "undefined");
const items = [];
for await (const item of stream) items.push(item.value);
assert.deepEqual(items, [1, 2]);
assert.equal(Object.hasOwn(streaming, "inline"), false);
checks.push("public-sync-stream-handle");

const captured = [];
const options = {
  baseURL: "https://nested.test/api",
  fetch: async (url, init) => {
    captured.push({ url: String(url), method: init.method });
    return Response.json({ data: { id: "widget/2", name: "nested" } });
  },
};
const nested = contract.createClient({
  ...options,
  operations: await contract.loadOperations([contract.operations.getCustomerWidget]),
});
const full = fullContract.createClient(options);
assert.deepEqual(
  await nested.customers("customer/1").widgets("widget/2").get(),
  await full.customers("customer/1").widgets("widget/2").get(),
);
assert.deepEqual(captured[0], captured[1]);
assert.equal(captured[0].url, "https://nested.test/api/customers/customer%2F1/widgets/widget%2F2");
checks.push("native-nested-resource-full-client-parity");

const keyList = Object.keys(exactNames.operations);
assert(
  keyList.includes("then") && keyList.includes("__proto__") && keyList.includes("constructor"),
);
const selected = exact.createClient({
  operations: await exact.loadOperations([
    Object.values(exactNames.operations),
    exactNames.routes["GET /idless"],
  ]),
  baseURL: "https://exact.test",
  fetch: async () => new Response(null, { status: 204 }),
});
for (const key of keyList) await selected.$operations[key]();
await selected.$routes["GET /idless"]();
assert.equal(Object.getPrototypeOf(selected.$operations), null);
assert.equal(Object.hasOwn(selected.$operations, "idless"), false);
const fullExact = exactFull.createClient({
  baseURL: "https://exact.test",
  fetch: async () => new Response(null, { status: 204 }),
});
assert.deepEqual(Object.keys(selected).sort(), Object.keys(fullExact).sort());
for (const [kind, keys] of [
  ["operation", keyList],
  ["route", Object.keys(exactNames.routes)],
]) {
  for (const key of keys) {
    const hash = createHash("sha256").update(`${kind}\0${key}`).digest("hex");
    const relative = `browser/lookup/${kind === "route" ? "r" : "o"}-${hash.slice(0, 16)}/${hash.slice(16)}.js`;
    assert(
      fs.existsSync(path.join(root, "selection-public", relative)),
      `Go did not emit ${relative}`,
    );
    const { entry } = await load("selection-public", relative);
    assert.equal(entry.key, key);
    assert.equal(entry.kind, kind);
  }
}
checks.push("names-only-enumeration-exact-ids-collisions-and-idless-routes");

const empty = lifecycle.createClient({
  operations: await lifecycle.loadOperations([]),
  fetch: async () => {
    throw Error("not called");
  },
});
assert.deepEqual(Object.keys(empty.$routes), []);
await assert.rejects(lifecycle.loadOperations(contract.routes["GET /health"]), {
  stage: "IDENTITY",
});
await assert.rejects(
  lifecycle.loadOperations(lifecycle.routes["GET /missing"]),
  (error) => error.stage === "MODULE_LOAD" && error.cause?.code === "ERR_MODULE_NOT_FOUND",
);
checks.push("empty-and-foreign-generation-and-real-module-failure");
// Evaluation counters are appended only to an independent copy of generated JS.
// The strict source/declaration output and graph/size inputs remain untouched.
const linkRoot = path.join(path.dirname(root), "native-link-probe");
fs.cpSync(path.join(root, "selection-links"), linkRoot, { recursive: true });
globalThis.__sdkgenLinkedEvaluations = {};
for (const [label, relative] of [
  ["item", "internal/executions/items/by-id/get.js"],
  ["xml", "internal/executions/xml/get.js"],
]) {
  fs.appendFileSync(
    path.join(linkRoot, relative),
    `\nglobalThis.__sdkgenLinkedEvaluations[${JSON.stringify(label)}]=(globalThis.__sdkgenLinkedEvaluations[${JSON.stringify(label)}]??0)+1;\n`,
  );
}
const linked = await import(pathToFileURL(path.join(linkRoot, "browser/index.js")).href);
const linkedPrepared = await linked.loadOperations([linked.operations.getSource]);
assert.deepEqual(globalThis.__sdkgenLinkedEvaluations, {});
const linkedRequests = [];
const linkedFetch = async (url, init) => {
  linkedRequests.push({
    url: String(url),
    authorization: new Headers(init.headers).get("authorization"),
  });
  return String(url).endsWith("/xml")
    ? new Response("<item><id>xml</id></item>", { headers: { "content-type": "application/xml" } })
    : Response.json({ id: "a/b" });
};
const linkedFirst = linked.createClient({
  operations: linkedPrepared,
  baseURL: "https://first-link.test",
  authorization: "Bearer first",
  fetch: linkedFetch,
});
const linkedSecond = linked.createClient({
  operations: linkedPrepared,
  baseURL: "https://second-link.test",
  authorization: "Bearer second",
  fetch: linkedFetch,
});
assert.equal(linkedRequests.length, 0);
const [firstRaw, secondRaw] = await Promise.all([
  linkedFirst.source.get.raw(),
  linkedSecond.source.get.raw(),
]);
assert.deepEqual(globalThis.__sdkgenLinkedEvaluations, {});
const values = await Promise.all([
  linkedFirst.source.get.links.follow(firstRaw),
  linkedSecond.source.get.links.follow(secondRaw),
]);
assert.deepEqual(
  values.map((value) => value.id),
  ["a/b", "a/b"],
);
assert.equal(globalThis.__sdkgenLinkedEvaluations.item, 1);
assert.equal(globalThis.__sdkgenLinkedEvaluations.xml, undefined);
assert.deepEqual(
  linkedRequests.slice(2).sort((a, b) => a.url.localeCompare(b.url)),
  [
    { url: "https://first-link.test/items/a%2Fb", authorization: "Bearer first" },
    { url: "https://second-link.test/items/a%2Fb", authorization: "Bearer second" },
  ],
);
assert.deepEqual(Object.keys(linkedFirst.$routes), ["GET /source"]);
assert.deepEqual(Object.keys(linkedFirst.$operations), ["getSource"]);
assert.equal(Object.hasOwn(linkedFirst, "items"), false);
checks.push("lazy-native-target-evaluation-once-and-client-private-binding");
assert.equal((await linkedFirst.source.get.links.xml(firstRaw)).id, "xml");
assert.equal(globalThis.__sdkgenLinkedEvaluations.xml, 1);
assert.equal(Object.hasOwn(linkedFirst.$routes, "GET /xml"), false);
checks.push("uninvoked-helper-target-absent-and-idless-xml-follow");
const both = linked.createClient({
  operations: await linked.loadOperations([linked.operations.getSource, linked.operations.getItem]),
  baseURL: "https://cycle.test",
  fetch: linkedFetch,
});
const itemRaw = await both.$operations.getItem.raw({ path: { id: "a/b" } });
assert.equal((await both.$operations.getItem.links.back(itemRaw)).id, "a/b");
assert.equal(globalThis.__sdkgenLinkedEvaluations.item, 1);
checks.push("cyclic-link-without-preparation-recursion");
const unavailable = path.join(linkRoot, "internal/executions/unavailable/get.js");
fs.renameSync(unavailable, unavailable + ".withheld");
let failureRequests = 0;
const failure = linked.createClient({
  operations: await linked.loadOperations([linked.operations.failureSource]),
  baseURL: "https://failure.test",
  fetch: async () => {
    failureRequests++;
    return new Response(null, { status: 204 });
  },
});
const failureRaw = await failure.$operations.failureSource.raw();
await assert.rejects(
  failure.$operations.failureSource.links.follow(failureRaw),
  (error) => error.stage === "MODULE_LOAD" && error.cause?.code === "ERR_MODULE_NOT_FOUND",
);
assert.equal(failureRequests, 1);
checks.push("actual-missing-helper-module-cause-before-target-request");
console.log(
  JSON.stringify({
    status: "pass",
    scope:
      "Actual Go-emitted public entries, lookup modules and selected clients in native Node ESM; independent generated-copy evaluation probes for lazy Links; injected API fetch, not browser HTTP delivery.",
    checks,
    passed: checks.length,
  }),
);
