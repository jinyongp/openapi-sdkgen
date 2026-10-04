import assert from "node:assert/strict";
const scenarios = [
  { name: "ordinary" },
  { name: "raw", raw: true },
  { name: "declared-400", response: () => Response.json({ error: "bad" }, { status: 400 }) },
  {
    name: "undeclared-500",
    response: () =>
      Response.json(
        { error: { code: "SERVER", message: "failed", details: { x: 1 } } },
        { status: 500 },
      ),
  },
  { name: "invalid-output", response: () => Response.json({ title: "missing" }) },
  {
    name: "malformed-json",
    response: () => new Response("{bad", { headers: { "Content-Type": "application/json" } }),
  },
  {
    name: "plain-mutable-prototype-keys",
    response: () =>
      Response.json(
        JSON.parse('{"id":"one","title":"One","__proto__":{"x":1},"constructor":"own","extra":2}'),
      ),
  },
  { name: "unicode-path", input: { path: { id: "한 글/%" } } },
  { name: "dot-segment", input: { path: { id: ".." } } },
  { name: "invalid-path", input: { path: { id: 5 } } },
  { name: "missing-path", input: { path: {} } },
  { name: "reserved-header", requestOptions: { headers: { authorization: "Bearer bad" } } },
  { name: "wrong-authorization", client: { authorization: "Basic abc" } },
  { name: "missing-credentials", client: { authorization: undefined } },
  { name: "single-requirement-explicit", requestOptions: { securityRequirement: "Bearer" } },
  {
    name: "provider-sync",
    client: {
      authorization: undefined,
      securityProvider: () => ({ Bearer: { kind: "http-bearer", token: "provided" } }),
    },
  },
  {
    name: "provider-async",
    client: {
      authorization: undefined,
      securityProvider: async () => ({ Bearer: { kind: "http-bearer", token: "provided" } }),
    },
  },
  { name: "provider-missing", client: { authorization: undefined, securityProvider: () => ({}) } },
  {
    name: "provider-extra",
    client: {
      authorization: undefined,
      securityProvider: () => ({
        Bearer: { kind: "http-bearer", token: "provided" },
        Other: { kind: "http-bearer", token: "bad" },
      }),
    },
  },
  {
    name: "provider-invalid",
    client: {
      authorization: undefined,
      securityProvider: () => ({ Bearer: { kind: "http-bearer", token: "" } }),
    },
  },
  { name: "relative-server", client: { baseURL: undefined, origin: "https://relative.test" } },
  {
    name: "unknown-server",
    client: { baseURL: undefined, origin: "https://relative.test", server: { id: "missing" } },
  },
  {
    name: "undeclared-xml",
    response: () => new Response("<item/>", { headers: { "Content-Type": "application/xml" } }),
  },
  {
    name: "undeclared-text",
    response: () => new Response("42", { headers: { "Content-Type": "text/plain" } }),
  },
  {
    name: "undeclared-custom-codec",
    client: { codecs: { "application/x-custom": { decode: async () => ({ custom: 1 }) } } },
    response: () => new Response("custom", { headers: { "Content-Type": "application/x-custom" } }),
  },
  { name: "bodyless-204", response: () => new Response(null, { status: 204 }) },
  { name: "invalid-timeout", requestOptions: { timeoutMS: 0 } },
  { name: "already-aborted", aborted: true },
  { name: "timeout", requestOptions: { timeoutMS: 3 }, pendingFetch: true },
];
function errorView(error) {
  if (error === undefined) return undefined;
  return {
    name: error?.name,
    message: error?.message,
    code: error?.code,
    status: error?.status,
    data: error?.data,
    details: error?.details,
    cause: error?.cause ? errorView(error.cause) : undefined,
  };
}
async function observe(createClient, scenario) {
  const calls = [];
  const original = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    calls.push({
      url,
      method: init.method,
      headers: Object.fromEntries(new Headers(init.headers)),
      redirect: init.redirect,
    });
    if (scenario.pendingFetch) return await new Promise(() => {});
    return scenario.response
      ? scenario.response()
      : Response.json({ id: "one", title: "One" }, { headers: { "x-request-id": "id-1" } });
  };
  try {
    const client = createClient({
      baseURL: "https://size.test",
      authorization: "Bearer benchmark",
      ...scenario.client,
    });
    const input = scenario.input ?? { path: { id: "one" } };
    const options = { ...scenario.requestOptions };
    if (scenario.aborted) {
      const controller = new AbortController();
      controller.abort("cancelled");
      options.signal = controller.signal;
    }
    const value = scenario.raw
      ? await client.$operations.getItem.raw(input, options)
      : await client.$operations.getItem(input, options);
    const data = scenario.raw ? value.data : value;
    if (data && typeof data === "object") {
      assert.equal(Object.getPrototypeOf(data), Object.prototype);
      const descriptor = Object.getOwnPropertyDescriptor(data, "id");
      if (descriptor) assert.equal(descriptor.writable, true);
    }
    return {
      calls,
      result: scenario.raw
        ? {
            status: value.status,
            contentType: value.contentType,
            data: value.data,
            headers: value.headers,
            request: value.request,
          }
        : value,
    };
  } catch (error) {
    return { calls, error: errorView(error) };
  } finally {
    globalThis.fetch = original;
  }
}
export async function runHTTPContractDifferential(candidate, control) {
  const comparisons = [];
  for (const scenario of scenarios) {
    const expected = await observe(control, scenario);
    const actual = await observe(candidate, scenario);
    assert.deepEqual(actual, expected, scenario.name);
    comparisons.push({
      name: scenario.name,
      matched: true,
      resultKind: actual.error ? "error" : "success",
      errorCode: actual.error?.code,
    });
  }
  assert(comparisons.some((x) => x.resultKind === "error"));
  return comparisons;
}
