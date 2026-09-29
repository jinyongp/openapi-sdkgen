// Deterministic OpenAPI workloads for actual generated SDK delivery validation.
export function sdkDeliveryDocument(count, version = "1") {
  if (!Number.isInteger(count) || count < 16)
    throw new TypeError("Expected at least 16 operations");
  const item = { $ref: "#/components/schemas/Item" };
  const response = (schema = item, media = "application/json") => ({
    200: { description: "Result", content: { [media]: { schema } } },
  });
  const paths = {};
  for (let index = 0; index < count - 6; index++)
    paths[`/items/item${index}`] = {
      get: { operationId: `getItem${index}`, responses: response() },
    };
  paths["/echo"] = {
    post: {
      operationId: "echo",
      requestBody: { required: true, content: { "application/json": { schema: item } } },
      responses: response(),
    },
  };
  paths["/xml"] = {
    get: {
      operationId: "readXML",
      responses: response({ $ref: "#/components/schemas/XMLItem" }, "application/xml"),
    },
  };
  paths["/events"] = {
    get: {
      operationId: "events",
      responses: {
        200: { description: "Frames", content: { "application/x-ndjson": { itemSchema: item } } },
      },
    },
  };
  paths["/source"] = {
    get: {
      operationId: "source",
      responses: {
        200: {
          ...response()["200"],
          links: {
            detail: { operationId: "readLinked", parameters: { itemId: "$response.body#/id" } },
          },
        },
      },
    },
  };
  paths["/linked/{itemId}"] = {
    get: {
      operationId: "readLinked",
      parameters: [{ name: "itemId", in: "path", required: true, schema: { type: "string" } }],
      responses: response(),
    },
  };
  paths["/idless"] = { get: { responses: response() } };
  const fields = {
    id: { type: "string" },
    title: { type: "string" },
    count: { type: "integer", minimum: 0 },
  };
  return {
    openapi: "3.2.0",
    info: { title: "SDK delivery verification", version },
    paths,
    components: {
      schemas: {
        Item: {
          type: "object",
          required: ["id", "title", "count"],
          properties: {
            ...fields,
            next: item,
            attributes: { $ref: "#/components/schemas/Attributes" },
          },
          additionalProperties: false,
        },
        Attributes: {
          type: "object",
          properties: { active: { type: "boolean" } },
          additionalProperties: false,
        },
        XMLItem: {
          type: "object",
          xml: { name: "item" },
          required: ["id", "title", "count"],
          properties: fields,
          additionalProperties: false,
        },
        Unused: { type: "string", enum: ["unused-schema-must-not-be-in-selected-graph"] },
      },
    },
  };
}

export function sdkDeliveryWorkloads(count) {
  return [
    { name: "one", routes: ["GET /items/item0"] },
    { name: "ten", routes: Array.from({ length: 10 }, (_, i) => `GET /items/item${i}`) },
    { name: "json-post", routes: ["GET /items/item0", "POST /echo"] },
    { name: "json-xml", routes: ["GET /items/item0", "GET /xml"] },
    { name: "json-stream", routes: ["GET /items/item0", "GET /events"] },
    { name: "mixed", routes: ["GET /items/item0", "POST /echo", "GET /xml", "GET /events"] },
    { name: "idless", routes: ["GET /idless"] },
    { name: "link", routes: ["GET /source"] },
    {
      name: "dense",
      routes: Object.entries(sdkDeliveryDocument(count).paths).map(
        ([path, item]) => `${Object.keys(item)[0].toUpperCase()} ${path}`,
      ),
    },
  ];
}

// Runs unmodified in Node and browser; API requests are deliberately host-owned fetch calls.
export async function exerciseGeneratedSDK(module, routes, options = {}) {
  const traces = [];
  const item = { id: "item-1", title: "JSON", count: 2 };
  const canonical = (value) =>
    value && typeof value === "object"
      ? Array.isArray(value)
        ? value.map(canonical)
        : Object.fromEntries(
            Object.keys(value)
              .sort()
              .map((key) => [key, canonical(value[key])]),
          )
      : value;
  const equal = (actual, expected, label) => {
    if (JSON.stringify(canonical(actual)) !== JSON.stringify(canonical(expected)))
      throw Error(`${label}: ${JSON.stringify(actual)}`);
  };
  const makeOptions = (name) => ({
    baseURL: `https://${name}.example.test`,
    authorization: `Bearer ${name}-fixture`,
    fetch: async (url, init) => {
      const headers = new Headers(init?.headers);
      const trace = {
        url: String(url),
        method: init?.method,
        authorization: headers.get("authorization"),
        contentType: headers.get("content-type"),
        body: init?.body,
      };
      traces.push(trace);
      if (!trace.url.startsWith(`https://${name}.example.test/`))
        throw Error("Client base URL leaked");
      equal(trace.authorization, `Bearer ${name}-fixture`, "Client credential");
      const path = new URL(trace.url).pathname;
      equal(trace.method, path === "/echo" ? "POST" : "GET", "HTTP method");
      if (path === "/echo") {
        equal(trace.contentType, "application/json", "Body media");
        equal(JSON.parse(trace.body), item, "Encoded body");
      }
      if (path === "/xml")
        return new Response("<item><count>2</count><id>item-1</id><title>XML</title></item>", {
          headers: { "content-type": "application/xml" },
        });
      if (path === "/events")
        return new Response(
          JSON.stringify(item) + "\n" + JSON.stringify({ ...item, id: "item-2" }) + "\n",
          { headers: { "content-type": "application/x-ndjson" } },
        );
      return Response.json(item, { headers: { "x-request-id": "sdk-delivery" } });
    },
  });
  let prepared;
  if (options.selected) {
    prepared = await module.loadOperations(
      options.references ?? routes.map((route) => module.routes[route]),
    );
    equal(traces.length, 0, "Preparation must not send API requests");
  }
  const create = (name) =>
    module.createClient({
      ...makeOptions(name),
      ...(options.selected ? { operations: prepared } : {}),
    });
  const client = create("first");
  if (options.selected)
    equal(Object.keys(client.$routes).sort(), [...routes].sort(), "Selected route membership");
  for (const route of routes) {
    const call = client.$routes[route];
    if (route === "GET /events") {
      const stream = call.stream();
      if (typeof stream.then === "function") throw Error("Stream is not synchronous");
      const values = [];
      for await (const value of stream) values.push(value);
      equal(values, [item, { ...item, id: "item-2" }], "Stream data");
    } else if (route === "POST /echo") equal(await call({ body: item }), item, "POST result");
    else if (route === "GET /linked/{itemId}")
      equal(await call({ path: { itemId: "item-1" } }), item, "Path-bound result");
    else equal(await call(), route === "GET /xml" ? { ...item, title: "XML" } : item, "Result");
  }
  const primary = routes.find(
    (route) =>
      route !== "GET /events" && route !== "POST /echo" && route !== "GET /linked/{itemId}",
  );
  if (primary) {
    const raw = await client.$routes[primary].raw();
    equal(raw.request.id, "sdk-delivery", "Raw request ID");
    const second = create("second");
    await Promise.all([client.$routes[primary](), second.$routes[primary]()]);
    const before = traces.length;
    let failure;
    try {
      await client.$routes[primary]({ signal: AbortSignal.abort() });
    } catch (error) {
      failure = error;
    }
    equal(failure?.code, "REQUEST_ABORTED", "Pre-abort code");
    equal(traces.length, before, "Pre-abort must not reach fetch");
  }
  if (routes.includes("POST /echo")) {
    const before = traces.length;
    let failure;
    try {
      await client.$routes["POST /echo"]({ body: { ...item, count: -1 } });
    } catch (error) {
      failure = error;
    }
    equal(failure?.code, "REQUEST_ENCODE_FAILED", "Invalid body code");
    equal(traces.length, before, "Invalid body must not reach fetch");
  }
  if (routes.includes("GET /source") && options.followLink) {
    const raw = await client.$routes["GET /source"].raw();
    const linked = await client.$routes["GET /source"].links.detail(raw);
    equal(linked, item, "Link result");
    if (options.selected && !routes.includes("GET /linked/{itemId}"))
      equal(
        Object.hasOwn(client.$routes, "GET /linked/{itemId}"),
        false,
        "Helper target is private",
      );
  }
  return { status: "pass", traces, selectedRoutes: routes.length };
}
