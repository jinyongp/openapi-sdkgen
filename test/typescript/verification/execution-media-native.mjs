// Exercises real emitted media providers in Node's native ESM loader.
// The supplied directory was compiled by execution-providers.mjs, without @ts-nocheck.
import assert from "node:assert/strict";
import path from "node:path";
import { pathToFileURL } from "node:url";

const root = path.resolve(process.argv[2], "execution-media");
const load = (relative) => import(pathToFileURL(path.join(root, relative)).href);
const { createRequestContext } = await load("internal/runtime/http/http-execution-support.js");
const { createClient } = await load("index.js");
const providers = {};
for (const [name, method] of [
  ["json", "post"],
  ["xml", "post"],
  ["form", "post"],
  ["multipart", "post"],
  ["text", "post"],
  ["header", "get"],
  ["parameter", "get"],
  ["embedded", "get"],
  ["nested", "get"],
  ["error", "get"],
  ["custom", "get"],
  ["plain", "get"],
]) {
  providers[name] = (await load(`internal/executions/${name}/${method}.js`)).provider;
}
const item = { id: "one", count: 2 };
const decodedItem = { ...item };
function assertDecoded(actual, fields) {
  assert.equal(Object.getPrototypeOf(actual), Object.prototype);
  assert.deepEqual(actual, fields);
}
const xml = "<item><count>2</count><id>one</id></item>";
const invalidXML = "<item><count>-1</count><id>one</id></item>";
const checks = [];
function setup(fetch, extra = {}) {
  const options = { baseURL: "https://execution.test", fetch, ...extra };
  return { context: createRequestContext(options), full: createClient(options) };
}
{
  const calls = [];
  const { context, full } = setup(
    async (url, init) => {
      calls.push({
        url: String(url),
        body: init.body,
        authorization: new Headers(init.headers).get("authorization"),
      });
      return Response.json(item);
    },
    { authorization: "Bearer scoped" },
  );
  assert.equal(providers.json.profile, "json");
  assertDecoded(await providers.json.bind(context)({ body: item }), item);
  assertDecoded(await full.$operations.jsonEcho({ body: item }), item);
  assert.deepEqual(calls[0], calls[1]);
  assert.deepEqual(JSON.parse(calls[0].body), item);
  assert.equal(calls[0].authorization, "Bearer scoped");
  const before = calls.length;
  await assert.rejects(providers.json.bind(context)({ body: { id: "one", count: -1 } }), {
    code: "REQUEST_ENCODE_FAILED",
  });
  await assert.rejects(
    providers.json.bind(context)({ body: item }, { signal: AbortSignal.abort() }),
    { code: "REQUEST_ABORTED" },
  );
  assert.equal(calls.length, before);
  checks.push("json-metadata-profile-body-auth-and-input-failures");
}
{
  const bodies = [];
  const { context, full } = setup(async (_url, init) => {
    bodies.push(init.body);
    return new Response(xml, { headers: { "content-type": "application/xml" } });
  });
  assert.equal(providers.xml.profile, "buffered-xml");
  assertDecoded(await providers.xml.bind(context)({ body: item }), item);
  assertDecoded(await full.$operations.xmlEcho({ body: item }), item);
  assert.deepEqual(bodies, [xml, xml]);
  checks.push("xml-request-response-parity");
}
{
  const urls = [];
  const { context, full } = setup(async (url) => {
    urls.push(String(url));
    return Response.json(item, { headers: { "X-Item": xml } });
  });
  assertDecoded(await providers.parameter.bind(context)({ query: { item } }), item);
  assertDecoded(await full.$operations.xmlParameter({ query: { item } }), item);
  assert.equal(urls[0], urls[1]);
  assert.equal(new URL(urls[0]).searchParams.get("item"), xml);
  assertDecoded((await providers.header.bind(context).raw()).headers["X-Item"], item);
  assertDecoded((await full.$operations.xmlHeader.raw()).headers["X-Item"], item);
  checks.push("xml-parameter-and-raw-header-parity");
}
{
  const bodies = [];
  const { context, full } = setup(async (_url, init) => {
    bodies.push(
      init.body instanceof FormData ? Object.fromEntries(init.body.entries()) : String(init.body),
    );
    return Response.json(item);
  });
  for (const [name, id] of [
    ["form", "formEcho"],
    ["multipart", "multipartEcho"],
  ]) {
    assert.equal(providers[name].profile, "general");
    assertDecoded(await providers[name].bind(context)({ body: item }), item);
    assertDecoded(await full.$operations[id]({ body: item }), item);
  }
  assert.equal(bodies[0], bodies[1]);
  assert.deepEqual(Object.fromEntries(new URLSearchParams(bodies[0])), { id: "one", count: "2" });
  assert.deepEqual(bodies[2], { id: "one", count: "2" });
  assert.deepEqual(bodies[2], bodies[3]);
  checks.push("form-and-multipart-general-parity");
}
{
  const { context, full } = setup(
    async (_url, init) => new Response(init.body, { headers: { "content-type": "text/plain" } }),
  );
  assert.equal(await providers.text.bind(context)({ body: "hello" }), "hello");
  assert.equal(await full.$operations.textEcho({ body: "hello" }), "hello");
  checks.push("text-body-parity");
}
{
  const { context, full } = setup(async () => Response.json(invalidXML));
  assert.equal(await providers.embedded.bind(context)(), invalidXML);
  assert.equal(await full.$operations.embeddedXML(), invalidXML);
  checks.push("media-root-annotation-parity");
}
{
  let payload = xml;
  const { context, full } = setup(async () => Response.json({ payload }));
  assert.equal(providers.nested.profile, "buffered-xml");
  assertDecoded(await providers.nested.bind(context)(), { payload });
  assertDecoded(await full.$operations.nestedXML(), { payload });
  payload = invalidXML;
  await assert.rejects(providers.nested.bind(context)(), { code: "RESPONSE_DECODE_FAILED" });
  await assert.rejects(full.$operations.nestedXML(), { code: "RESPONSE_DECODE_FAILED" });
  checks.push("nested-content-xml-validation-parity");
}
{
  const { context, full } = setup(
    async () => new Response(xml, { status: 400, headers: { "content-type": "application/xml" } }),
  );
  assert.equal(providers.error.profile, "buffered-xml");
  await assert.rejects(providers.error.bind(context)(), { status: 400, data: decodedItem });
  await assert.rejects(full.$operations.xmlError(), { status: 400, data: decodedItem });
  checks.push("xml-error-data-parity");
}
{
  const urls = [];
  let decoded = 0;
  const { context, full } = setup(
    async (url) => {
      urls.push(String(url));
      return new Response("record", { headers: { "content-type": "application/x-record" } });
    },
    {
      codecs: {
        "application/x-number": { encodeParameter: async (value) => `n:${value}` },
        "application/x-record": {
          decode: async (response) => {
            assert.equal(await response.text(), "record");
            decoded++;
            return item;
          },
        },
      },
    },
  );
  assert.equal(providers.custom.profile, "json");
  assertDecoded(await providers.custom.bind(context)({ query: { number: 7 } }), item);
  assertDecoded(await full.$operations.customCodec({ query: { number: 7 } }), item);
  assert.equal(urls[0], urls[1]);
  assert.equal(new URL(urls[0]).searchParams.get("number"), "n:7");
  assert.equal(decoded, 2);
  checks.push("custom-parameter-response-callback-parity");
}
{
  const { context, full } = setup(async () => Response.json(item));
  assert.equal(providers.plain.route, "GET /plain");
  assert.equal(Object.hasOwn(providers.plain, "operationID"), false);
  assertDecoded(await providers.plain.bind(context)(), await full.$routes["GET /plain"]());
  checks.push("idless-route-parity");
}
console.log(JSON.stringify({ passed: checks.length, checks }));
