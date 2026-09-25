import assert from "node:assert/strict";

export const representationBody = JSON.parse(
  '{"label":"root","left":{"value":0},"right":{"value":1},"__proto__":"own","constructor":"own","prototype":"own","foo-bar":"hyphen","foo_bar":"underscore","1":"one","01":"zero-one","":"empty","é":"composed","e\\u0301":"decomposed","a\\u0000b":"nul","opaque":{"property":"schema","schema":"__sdkgen_Input","p":false,"s":0,"__proto__":null},"nullable":null,"next":{"label":"child","left":{"value":2},"right":{"value":3}}}',
);
const collisionRecord = Object.fromEntries([
  ["foo-bar", "modern"],
  ["foo_bar", "legacy"],
  ["__proto__", "prototype"],
  ["constructor", "constructor"],
  ["prototype", "prototype-property"],
  ["toString", "to-string"],
  ['quote"key', "quote"],
  ["back\\slash", "backslash"],
  ["line\nbreak", "control"],
  ["한글", "unicode"],
  ["money", { amount: 1, receipt: "receipt" }],
  ["moneyInput", "literal"],
  ["moneyOutput", 1],
  ["normalizedOne", "one"],
  ["normalizedTwo", 2],
  ["status", "foo-bar"],
]);

export function transportFor(scenario, origin = "https://example.test", token = "trial") {
  const traces = [];
  const contexts = [];
  const fetch = async (input, init) => {
    const request = new Request(input, init);
    const url = new URL(request.url);
    assert.equal(url.origin, origin, "client base URL leaked between instances");
    const body = await request.text();
    traces.push({ method: request.method, url: request.url, headers: [...request.headers], body });
    if (scenario === "lifecycle")
      return url.pathname === "/events"
        ? new Response('{"value":1}\n{"value":2}\n', {
            headers: { "content-type": "application/x-ndjson" },
          })
        : Response.json(JSON.parse(body));
    if (scenario === "github")
      return new Response("Keep it logically awesome.", {
        headers: { "content-type": "text/plain" },
      });
    if (scenario === "stripe") {
      assert.equal(request.headers.get("authorization"), `Bearer ${token}`);
      return Response.json({ object: "balance", available: [], pending: [], livemode: false });
    }
    if (scenario === "oas30") return Response.json({ version: "3.0" });
    if (scenario === "oas31")
      return url.pathname === "/source"
        ? Response.json({ id: "source-1" })
        : new Response(null, { status: 204 });
    if (scenario === "oas32")
      return url.pathname === "/events"
        ? new Response('"first"\n"second"\n', {
            headers: { "content-type": "application/x-ndjson" },
          })
        : Response.json(["sdk", "generator"]);
    if (scenario === "representation") {
      if (url.pathname === "/ping") return new Response(null, { status: 204 });
      if (url.pathname === "/empty") return Response.json({});
      return Response.json(JSON.parse(body));
    }
    if (scenario === "contract") {
      if (url.pathname === "/health") return new Response(null, { status: 204 });
      if (url.pathname === "/tasks")
        return Response.json({
          data: [{ id: "task-1" }],
          meta: { pagination: { nextCursor: null } },
        });
      if (url.pathname.endsWith("/tasks"))
        return Response.json(
          { id: "task-1", projectID: "project-1", ...JSON.parse(body) },
          { status: 201 },
        );
    }
    if (scenario === "collisions") {
      if (url.pathname.startsWith("/pets/modern/")) return Response.json(collisionRecord);
      if (url.pathname === "/streams/modern")
        return new Response(`${JSON.stringify(collisionRecord)}\n`, {
          headers: { "content-type": "application/x-ndjson" },
        });
      return new Response(null, { status: 204 });
    }
    throw new Error(`unexpected mock request: ${scenario} ${request.method} ${request.url}`);
  };
  const securityProvider = (context) => {
    assert.deepEqual(Object.keys(context.operation), ["route", "operationID", "method", "path"]);
    contexts.push({
      operation: context.operation,
      requirement: context.requirement,
      origin: context.origin,
    });
    return Object.fromEntries(
      context.requirement.schemes.map((s) => [
        s.name,
        s.type === "apiKey"
          ? { kind: "api-key", value: token }
          : s.type === "http" && s.scheme === "basic"
            ? { kind: "http-basic", username: token, password: "" }
            : s.type === "http"
              ? { kind: "http-bearer", token }
              : { kind: "oauth2", token },
      ]),
    );
  };
  return { options: { baseURL: origin, fetch, securityProvider }, traces, contexts };
}

export async function exercise(api, scenario) {
  switch (scenario) {
    case "lifecycle": {
      const data = { value: 0, nested: { flag: false } };
      const value = await api.$operations.echoInline({ body: data });
      assert.deepEqual(JSON.parse(JSON.stringify(value)), data);
      const frames = [];
      for await (const item of api.$operations.events.stream()) frames.push(item);
      assert.deepEqual(JSON.parse(JSON.stringify(frames)), [{ value: 1 }, { value: 2 }]);
      await assert.rejects(() => api.$operations.echoInline({ body: { value: -1 } }));
      return { value, frames };
    }
    case "surface":
      return { exercised: "reflection-only" };
    case "github":
      return api.$routes["GET /zen"]();
    case "stripe":
      return api.$routes["GET /v1/balance"]({ securityRequirement: "bearerAuth" });
    case "oas30":
      return api.$routes["GET /"]();
    case "oas31": {
      const raw = await api.$routes["GET /source"].raw();
      await api.$routes["GET /source"].links.follow(raw);
      return raw.data;
    }
    case "oas32": {
      const result = await api.$routes["QUERY /search"]({ query: { term: "sdk" } });
      const items = [];
      for await (const item of api.$routes["GET /events"].stream()) items.push(item);
      return { result, items };
    }
    case "representation": {
      await api.$operations.ping();
      const empty = await api.$operations.empty();
      const result = await api.$operations["echo-record"]({
        path: { id: "record/one" },
        body: representationBody,
      });
      assert.deepEqual(JSON.parse(JSON.stringify(result)), representationBody);
      assert.equal(Object.hasOwn(result, "__proto__"), true);
      await assert.rejects(() =>
        api.$operations["echo-record"]({
          path: { id: "bad" },
          body: { label: "invalid", left: { value: -1 }, right: { value: 0 } },
        }),
      );
      return { empty, result };
    }
    case "contract": {
      await api.$routes["GET /health"]();
      const created = await api.$operations.createTask({
        path: { projectID: "project-1" },
        body: { title: "test", priority: "LOW" },
      });
      const page = await api.$operations.listTasks();
      const items = [];
      for await (const item of api.$operations.listTasks.paginate({ query: { limit: 1 } }))
        items.push(item);
      return { created, page, items };
    }
    case "collisions": {
      const raw = await api.$operations["get-pet"].raw({
        path: { "foo-bar": "pet/one" },
        query: { "foo-bar": "modern", foo_bar: "legacy" },
      });
      for (const name of ["next-step", "next_step", "__proto__", "constructor"])
        await api.$links["get-pet"][name](raw);
      return raw.data;
    }
    default:
      throw new Error(`unknown fixture scenario: ${scenario}`);
  }
}
