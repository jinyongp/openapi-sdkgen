import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

const base = path.resolve(process.argv[2]);
const results = {};
for (const variant of ["full", "selected"]) {
  const load = (fixture, entry = "index.js") =>
    import(pathToFileURL(path.join(base, variant + "-js", fixture, entry)).href);
  const { createClient, isAPIError } = await load("small-json");
  const records = [];
  const requests = [];
  const fetch = async (url, init) => {
    requests.push({
      url: String(url),
      method: init.method,
      authorization: new Headers(init.headers).get("authorization"),
      body: init.body,
    });
    const headers = { "x-request-id": "trace-1" };
    if (String(url).endsWith("/bad"))
      return Response.json({ error: "Rejected" }, { status: 400, headers });
    if (init.method === "POST") return Response.json(JSON.parse(init.body), { headers });
    return Response.json(
      String(url).endsWith("/items") ? [{ id: "one", title: "One" }] : { id: "one", title: "One" },
      { headers },
    );
  };
  const api = createClient({ baseURL: "https://api.test", authorization: "Bearer first", fetch });
  records.push({
    scenario: "public-surface",
    routes: Object.keys(api.$routes),
    operations: Object.keys(api.$operations),
    resources: Object.keys(api).filter((key) => !key.startsWith("$")),
  });
  assert.equal(requests.length, 0);
  records.push({ scenario: "json-list", data: await api.$operations.listItems() });
  const raw = await api.$operations.listItems.raw();
  records.push({
    scenario: "raw",
    status: raw.status,
    data: raw.data,
    contentType: raw.contentType,
    request: raw.request,
  });
  records.push({
    scenario: "json-body",
    data: await api.$operations.createItem({ body: { id: "new", title: "New" } }),
  });
  records.push({
    scenario: "escaped-path",
    data: await api.$operations.getItem({ path: { id: "a/b" } }),
  });
  assert(requests.at(-1).url.endsWith("/items/a%2Fb"));
  records.push({ scenario: "resource-call", data: await api.items("one").get() });
  const second = createClient({
    baseURL: "https://other.test",
    authorization: "Bearer second",
    fetch,
  });
  await second.$operations.listItems();
  assert.equal(requests.at(-1).authorization, "Bearer second");
  assert.equal(requests.at(-1).url, "https://other.test/items");
  records.push({
    scenario: "instance-isolation",
    requests: requests.map((value) => ({ ...value, body: value.body ?? null })),
  });
  async function errorCase(scenario, action, expectedCode) {
    let error;
    try {
      await action();
    } catch (caught) {
      error = caught;
    }
    assert(error && isAPIError(error), "Missing normalized APIError for " + scenario);
    if (expectedCode !== undefined) assert.equal(error.code, expectedCode);
    records.push({
      scenario,
      code: error.code,
      status: error.status ?? null,
      data: error.data ?? null,
    });
  }
  await errorCase("http-error", () => api.$operations.getItem({ path: { id: "bad" } }));
  const beforeInvalid = requests.length;
  await errorCase(
    "invalid-body",
    () => api.$operations.createItem({ body: { id: "" } }),
    "REQUEST_ENCODE_FAILED",
  );
  await errorCase(
    "pre-abort",
    () => api.$operations.listItems({ signal: AbortSignal.abort() }),
    "REQUEST_ABORTED",
  );
  assert.equal(requests.length, beforeInvalid);
  const missing = createClient({ baseURL: "https://api.test", fetch });
  await errorCase(
    "missing-credentials",
    () => missing.$operations.listItems(),
    "SECURITY_CREDENTIALS_REQUIRED",
  );
  assert.equal(requests.length, beforeInvalid);
  const malformed = createClient({
    baseURL: "https://api.test",
    authorization: "Bearer first",
    fetch: async () => new Response("{bad", { headers: { "content-type": "application/json" } }),
  });
  await errorCase(
    "invalid-response",
    () => malformed.$operations.listItems(),
    "RESPONSE_DECODE_FAILED",
  );
  const network = createClient({
    baseURL: "https://api.test",
    authorization: "Bearer first",
    fetch: async () => {
      throw new TypeError("Offline");
    },
  });
  await errorCase("network-error", () => network.$operations.listItems(), "NETWORK_ERROR");
  const timeout = createClient({
    baseURL: "https://api.test",
    authorization: "Bearer first",
    timeoutMS: 10,
    fetch: async (_url, init) =>
      new Promise((_resolve, reject) =>
        init.signal.addEventListener("abort", () => reject(init.signal.reason), { once: true }),
      ),
  });
  await errorCase("timeout", () => timeout.$operations.listItems(), "REQUEST_TIMEOUT");
  const xml = await load("only-xmlEcho");
  let sentXML;
  const xmlClient = xml.createClient({
    baseURL: "https://api.test",
    fetch: async (_url, init) => {
      sentXML = init.body;
      return new Response("<item><count>2</count><id>one</id></item>", {
        headers: { "content-type": "application/xml" },
      });
    },
  });
  const xmlData = await xmlClient.$operations.xmlEcho({ body: { id: "one", count: 2 } });
  assert.equal(xmlData.id, "one");
  assert.equal(xmlData.count, 2);
  records.push({ scenario: "buffered-xml", data: xmlData, body: sentXML });
  const lifecycle = await load("lifecycle");
  const streaming = lifecycle.createClient({
    baseURL: "https://api.test",
    fetch: async () =>
      new Response('{"value":1}\n{"value":2}\n', {
        headers: { "content-type": "application/x-ndjson" },
      }),
  });
  const stream = streaming.$operations.events.stream();
  assert.equal("then" in stream, false);
  const values = [];
  for await (const value of stream) values.push(value);
  assert.deepEqual(
    values.map((value) => value.value),
    [1, 2],
  );
  records.push({ scenario: "json-stream", values });
  const selective = await load("selection-links", "selective/index.js");
  const prepared = await selective.loadOperations([selective.operations.getSource]);
  const linkedRequests = [];
  const selected = selective.createClient({
    operations: prepared,
    baseURL: "https://api.test",
    fetch: async (url) => {
      linkedRequests.push(String(url));
      const response = String(url).endsWith("/xml")
        ? new Response("<item><id>one</id></item>", {
            headers: { "content-type": "application/xml" },
          })
        : Response.json({ id: "one" });
      Object.defineProperty(response, "url", { value: String(url) });
      return response;
    },
  });
  const source = await selected.$operations.getSource.raw();
  const linked = await selected.$operations.getSource.links.follow(source);
  const linkedXML = await selected.$operations.getSource.links.xml(source);
  assert.equal(linked.id, "one");
  assert.equal(linkedXML.id, "one");
  assert.deepEqual(Object.keys(selected.$routes), ["GET /source"]);
  records.push({ scenario: "native-lazy-links", linked, linkedXML, requests: linkedRequests });
  results[variant] = records;
  console.log("Native ESM passed", variant, records.length, "scenarios");
}
assert.deepEqual(
  results.selected,
  results.full,
  "Selected and full-capability compositions differ",
);
fs.writeFileSync(path.join(base, "native-basic-results.json"), JSON.stringify(results, null, 2));
