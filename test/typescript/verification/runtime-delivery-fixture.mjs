// Self-contained so the same function runs in the browser page and harness tests.
// SDK assets use real HTTP; API requests terminate at the injected Fetch boundary.
export async function exerciseRuntimeDelivery(createAPIs, names, baseURL) {
  const requests = [];
  const checks = [];
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
      throw new Error(`${label}: ${JSON.stringify(actual)}`);
  };
  const expected = { todoId: "todo-1", title: "JSON" };
  function client(authorization, suffix = "") {
    const urlBase = baseURL + suffix;
    return createAPIs({
      baseURL: urlBase,
      authorization,
      fetch: async (url, init) => {
        const headers = new Headers(init?.headers);
        const request = {
          url: String(url),
          method: init?.method,
          authorization: headers.get("authorization"),
          contentType: headers.get("content-type"),
          body: init?.body,
        };
        requests.push(request);
        equal(request.authorization, authorization, "Client authorization");
        if (!request.url.startsWith(urlBase + "/")) throw new Error("Client base URL escaped");
        const route = request.url.slice(urlBase.length);
        equal(request.method, route === "/todos" ? "POST" : "GET", "HTTP method");
        if (route === "/todos") {
          equal(request.contentType, "application/json", "POST content type");
          equal(JSON.parse(request.body), { todo_id: "todo-1", title: "JSON" }, "POST wire body");
        } else if (route !== "/xml" && route !== "/events") {
          equal(route, "/todos/a%2Fb", "Encoded path");
        }
        if (route === "/events")
          return new Response('{"n":1}\n{"n":2}\n', {
            headers: { "content-type": "application/x-ndjson" },
          });
        if (route === "/xml")
          return new Response("<todo><title>XML</title></todo>", {
            headers: { "content-type": "application/xml" },
          });
        return Response.json(
          { todo_id: "todo-1", title: "JSON" },
          { headers: { "x-request-id": "browser-fixture" } },
        );
      },
    });
  }
  const api = client("Bearer first-fixture");
  equal(requests.length, 0, "No request during client creation");
  for (const name of names) {
    if (name === "stream") {
      const stream = api.stream.stream();
      if (typeof stream.then === "function") throw new Error("Stream became thenable");
      const items = [];
      for await (const item of stream) items.push(item);
      equal(items, [{ n: 1 }, { n: 2 }], "Stream response");
    } else if (name === "xml") equal(await api.xml(), { title: "XML" }, "XML response");
    else
      equal(
        await api[name](name === "get" ? { path: { id: "a/b" } } : { body: expected }),
        expected,
        "Mapped response",
      );
    checks.push(`request-${name}`);
  }
  async function rejectsBeforeFetch(invoke, code) {
    const before = requests.length;
    let failure;
    try {
      await invoke();
    } catch (error) {
      failure = error;
    }
    equal(failure?.code, code, "Request failure");
    equal(requests.length, before, "Rejected request must not reach fetch");
  }
  await rejectsBeforeFetch(
    () => api.get({ path: { id: "a/b" } }, { signal: AbortSignal.abort() }),
    "REQUEST_ABORTED",
  );
  checks.push("get-abort-before-fetch");
  if (names.includes("post")) {
    await rejectsBeforeFetch(() => api.post({ body: { title: 7 } }), "REQUEST_ENCODE_FAILED");
    await rejectsBeforeFetch(
      () => api.post({ body: expected }, { signal: AbortSignal.abort() }),
      "REQUEST_ABORTED",
    );
    checks.push("post-invalid-before-fetch", "post-abort-before-fetch");
  }
  const raw = await api.get.raw({ path: { id: "a/b" } });
  equal(raw.data, expected, "Raw response");
  equal(raw.request.id, "browser-fixture", "Raw request ID");
  const second = client("Bearer second-fixture", "/second");
  await Promise.all([api.get({ path: { id: "a/b" } }), second.get({ path: { id: "a/b" } })]);
  checks.push("raw-metadata", "two-client-isolation");
  return { apiTransport: "injected-fetch", passed: checks.length, checks, requests };
}
