import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";
const base = path.resolve(process.argv[2]),
  results = {};
const types = {
  ndjson: "application/x-ndjson",
  sequence: "application/json-seq",
  sse: "text/event-stream",
};
for (const variant of ["full", "selected"]) {
  const load = (fixture) =>
    import(pathToFileURL(path.join(base, variant + "-js", fixture, "index.js")).href);
  const records = [];
  let active;
  const record = (scenario, value) => records.push({ scenario, value });
  async function reject(scenario, action, code) {
    let caught;
    try {
      await action();
    } catch (error) {
      caught = error;
    }
    assert(caught, scenario + " must fail");
    if (code) assert.equal(caught.code, code);
    record(scenario, { code: caught.code, cause: caught.cause?.message ?? null });
  }
  try {
    for (const name of Object.keys(types)) {
      const value = name === "sse" ? { data: "hello" } : { id: "one" };
      const request = await load((active = "request-complete-" + name));
      let calls = 0,
        body;
      const api = request.createClient({
        baseURL: "https://extra.test",
        fetch: async (_url, init) => {
          calls++;
          body = await new Response(init.body).text();
          return new Response(null, { status: 204 });
        },
      });
      await api.$operations.probe({ body: [value] });
      assert(body.includes(name === "sse" ? "hello" : "one"));
      record(active, body);
      await reject(
        active + "-invalid",
        () => api.$operations.probe({ body: [name === "sse" ? { data: 42 } : { id: 42 }] }),
        "REQUEST_ENCODE_FAILED",
      );
      assert.equal(calls, 1);
      const response = await load((active = "response-complete-" + name));
      const result = response.createClient({
        baseURL: "https://extra.test",
        fetch: async () => new Response(body, { headers: { "content-type": types[name] } }),
      });
      const data = await result.$operations.probe();
      assert.deepEqual(JSON.parse(JSON.stringify(data)), [value]);
      record(active, data);
      const raw = await result.$operations.probe.raw();
      assert.equal(await raw.response.text(), body);
      record(active + "-raw-unconsumed", { data: raw.data ?? null, status: raw.status });
      const malformed = response.createClient({
        baseURL: "https://extra.test",
        fetch: async () =>
          new Response(
            name === "sse"
              ? "retry: no\ndata: hello\n\n"
              : (name === "sequence" ? "\u001e" : "") + '{"id":42}\n',
            { headers: { "content-type": types[name] } },
          ),
      });
      if (name !== "sse")
        await reject(
          active + "-invalid",
          () => malformed.$operations.probe(),
          "RESPONSE_DECODE_FAILED",
        );
    }
    for (const kind of [
      "apiKey-header",
      "apiKey-query",
      "apiKey-cookie",
      "http-basic",
      "http-digest",
      "oauth2",
      "openIdConnect",
      "mutualTLS",
      "mixed-security",
    ]) {
      const module = await load((active = "security-" + kind));
      let calls = 0,
        acquired = 0,
        request;
      const credential = kind.startsWith("apiKey-")
        ? { kind: "api-key", value: "secret" }
        : kind === "http-basic"
          ? { kind: "http-basic", username: "user", password: "pass" }
          : kind === "http-digest"
            ? { kind: "http", value: "proof" }
            : kind === "oauth2" || kind === "openIdConnect"
              ? { kind, token: "token" }
              : { kind: "mutual-tls" };
      const credentials =
        kind === "mixed-security"
          ? {
              bearer: { kind: "http-bearer", token: "mixed" },
              key: { kind: "api-key", value: "secret" },
            }
          : { auth: credential };
      const fetch = async (url, init) => {
        calls++;
        request = {
          url: String(url),
          headers: Object.fromEntries(new Headers(init.headers)),
          redirect: init.redirect,
        };
        return Response.json({ id: "one" });
      };
      const options = {
        baseURL: "https://extra.test",
        transport: {
          fetch,
          capabilities: { cookieJar: true, ...(kind === "mutualTLS" ? { mutualTLS: true } : {}) },
        },
        securityProvider: async (context) => {
          acquired++;
          assert.equal(context.origin, "https://extra.test");
          return credentials;
        },
      };
      const api = module.createClient(options);
      const data = await api.$operations.probe();
      assert.equal(data.id, "one");
      assert.equal(calls, 1);
      assert.equal(acquired, kind === "mutualTLS" ? 0 : 1);
      assert.equal(request.redirect, "error");
      if (kind === "apiKey-header") assert.equal(request.headers["x-api-key"], "secret");
      if (kind === "apiKey-query")
        assert.equal(new URL(request.url).searchParams.get("X-API-Key"), "secret");
      if (kind === "apiKey-cookie") assert(request.headers.cookie.includes("X-API-Key=secret"));
      if (kind === "http-basic")
        assert.equal(request.headers.authorization, "Basic " + btoa("user:pass"));
      if (kind === "http-digest") assert.equal(request.headers.authorization, "digest proof");
      if (kind === "oauth2" || kind === "openIdConnect")
        assert.equal(request.headers.authorization, "Bearer token");
      if (kind === "mixed-security") {
        assert.equal(request.headers.authorization, "Bearer mixed");
        assert.equal(new URL(request.url).searchParams.get("key"), "secret");
      }
      record(active, request);
      const bad = module.createClient({
        baseURL: "https://extra.test",
        fetch,
        securityProvider: async () =>
          kind === "mixed-security" ? {} : { auth: { kind: "wrong", value: "wrong" } },
      });
      await reject(
        active + "-invalid-credential",
        () => bad.$operations.probe(),
        "SECURITY_CREDENTIALS_INVALID",
      );
      assert.equal(calls, 1);
      const missing = module.createClient({ baseURL: "https://extra.test", fetch });
      await reject(
        active + "-missing",
        () => missing.$operations.probe(),
        "SECURITY_CREDENTIALS_REQUIRED",
      );
      assert.equal(calls, 1);
      await reject(
        active + "-explicit-choice",
        () => api.$operations.probe({ securityRequirement: "unknown" }),
        "SECURITY_REQUIREMENT_INVALID",
      );
      assert.equal(calls, 1);
    }
    const small = await load((active = "small-json"));
    let auth;
    const fetch = async (_url, init) => {
      auth = new Headers(init.headers).get("authorization");
      return Response.json([{ id: "one", title: "One" }]);
    };
    const supplied = small.createClient({
      baseURL: "https://extra.test",
      fetch,
      securityProvider: async () => ({ bearer: { kind: "http-bearer", token: "provided" } }),
    });
    // The generated scheme name is part of the public provider context.
    const real = small.createClient({
      baseURL: "https://extra.test",
      fetch,
      securityProvider: async (context) =>
        Object.fromEntries(
          context.requirement.schemes.map((scheme) => [
            scheme.name,
            { kind: "http-bearer", token: "provided" },
          ]),
        ),
    });
    await real.$operations.listItems();
    assert.equal(auth, "Bearer provided");
    record("bearer-provider", auth);
    void supplied;
    const wrong = small.createClient({
      baseURL: "https://extra.test",
      fetch,
      securityProvider: async (context) =>
        Object.fromEntries(
          context.requirement.schemes.map((scheme) => [
            scheme.name,
            { kind: "http-basic", username: "user", password: "pass" },
          ]),
        ),
    });
    await reject(
      "bearer-wrong-basic",
      () => wrong.$operations.listItems(),
      "SECURITY_CREDENTIALS_INVALID",
    );
    const customBody = await load((active = "custom-body-jsonish"));
    let body,
      calls = 0;
    const customAPI = customBody.createClient({
      baseURL: "https://extra.test",
      codecs: {
        "application/x-jsonish": { encode: async (value) => "custom:" + JSON.stringify(value) },
      },
      fetch: async (_url, init) => {
        calls++;
        body = init.body;
        return Response.json({ id: "one" });
      },
    });
    assert.equal((await customAPI.$operations.probe({ body: { id: "one" } })).id, "one");
    assert.equal(body, 'custom:{"id":"one"}');
    record("custom-buffered-body", body);
    await reject(
      "custom-buffered-invalid",
      () => customAPI.$operations.probe({ body: { id: 42 } }),
      "REQUEST_ENCODE_FAILED",
    );
    assert.equal(calls, 1);
    const missingCodec = customBody.createClient({
      baseURL: "https://extra.test",
      fetch: async () => {
        throw Error("Should not fetch");
      },
    });
    await reject(
      "custom-buffered-missing-codec",
      () => missingCodec.$operations.probe({ body: { id: "one" } }),
      "REQUEST_ENCODE_FAILED",
    );
    active = "transport-native-headers";
    const server = await import(
      pathToFileURL(path.join(base, variant + "-js", active, "server/webhooks.js")).href
    );
    let context,
      invoked = 0;
    const router = server.createWebhookRouter(
      {
        delivery: {
          POST: async (value) => {
            invoked++;
            context = {
              body: value.body,
              headers: value.params.headerParams,
              operationID: value.operationID,
            };
            return { status: 204, headers: { "x-probe": "yes" } };
          },
        },
      },
      { routes: { delivery: "/hooks/{id}" } },
    );
    const inbound = (
      path = "/hooks/one",
      headers = { Origin: "https://host.test", "Sec-Fetch-Site": "same-origin" },
    ) => new Request("https://inbound.test" + path, { method: "POST", headers });
    const response = await router.fetch(inbound());
    assert.equal(response.status, 204);
    assert.equal(response.headers.get("x-probe"), "yes");
    record("server-bodyless-headers", context);
    const missingHeader = await router.fetch(inbound("/hooks/one", {}));
    assert.equal(missingHeader.status, 400);
    assert.equal(invoked, 1);
    record("server-missing-header", await missingHeader.text());
    const unknownRoute = await router.fetch(inbound("/other"));
    assert.equal(unknownRoute.status, 404);
    assert.equal(invoked, 1);
    record("server-unknown-route", unknownRoute.status);
    const wrongMethod = await router.fetch(
      new Request("https://inbound.test/hooks/one", { headers: { Origin: "https://host.test" } }),
    );
    assert.equal(wrongMethod.status, 404);
    assert.equal(invoked, 1);
    record("server-wrong-method", wrongMethod.status);
    for (const [name, value] of [
      ["204-body", { status: 204, body: "invalid" }],
      ["undeclared-status", { status: 200 }],
      [
        "undeclared-media",
        { status: 200, contentType: "application/xml", body: { value: "unused" } },
      ],
    ]) {
      const invalid = server.createWebhookRouter(
        { delivery: { POST: async () => value } },
        { routes: { delivery: "/hooks/one" } },
      );
      const response = await invalid.fetch(inbound());
      assert.equal(response.status, 500);
      record("server-" + name, await response.text());
    }
  } catch (error) {
    console.error("Native extra failure", variant, active);
    throw error;
  }
  results[variant] = records;
  console.log("Native extra passed", variant, records.length, "scenarios");
}
assert.deepEqual(results.selected, results.full);
fs.writeFileSync(path.join(base, "native-extra-results.json"), JSON.stringify(results, null, 2));
