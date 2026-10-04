import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";
const base = path.resolve(process.argv[2]);
const results = {};
const types = {
  ndjson: "application/x-ndjson",
  sequence: "application/json-seq",
  sse: "text/event-stream",
  multipart: "multipart/mixed",
  custom: "application/x-frames",
};
const item = { id: "one" };
async function* source(value) {
  yield value;
}
const custom = {
  protocol: {
    async *decode(reader) {
      const decoder = new TextDecoder();
      let text = "",
        bytes;
      while ((bytes = await reader.read(1024)) !== null)
        text += decoder.decode(bytes, { stream: true });
      for (const frame of text.split("|")) if (frame) yield JSON.parse(frame);
    },
    encode(frames) {
      const iterator = frames[Symbol.asyncIterator]();
      return new ReadableStream({
        async pull(controller) {
          const next = await iterator.next();
          if (next.done) controller.close();
          else controller.enqueue(new TextEncoder().encode(JSON.stringify(next.value) + "|"));
        },
        async cancel() {
          await iterator.return?.();
        },
      });
    },
  },
};
function wire(name, value = item) {
  if (name === "ndjson") return JSON.stringify(value) + "\n";
  if (name === "sequence") return "\u001e" + JSON.stringify(value) + "\n";
  if (name === "sse") return "data: " + value.data + "\n\n";
  if (name === "custom") return JSON.stringify(value) + "|";
  return (
    "--probe\r\nContent-Type: application/json\r\n\r\n" +
    JSON.stringify(value) +
    "\r\n--probe--\r\n"
  );
}
for (const variant of ["full", "selected"]) {
  const load = (name) =>
    import(pathToFileURL(path.join(base, variant + "-js", name, "index.js")).href);
  const records = [];
  let active = "";
  function record(scenario, value) {
    records.push({ scenario, value });
  }
  async function rejects(scenario, action, code) {
    let caught;
    try {
      await action();
    } catch (error) {
      caught = error;
    }
    assert(caught, scenario + " must fail");
    if (code) assert.equal(caught.code, code, scenario);
    record(scenario, { code: caught.code ?? caught.name, cause: caught.cause?.message ?? null });
  }
  try {
    for (const name of Object.keys(types)) {
      const value = name === "sse" ? { data: "hello" } : item;
      const request = await load((active = "request-" + name));
      let calls = 0,
        actual;
      const api = request.createClient({
        baseURL: "https://feature.test",
        fetch: async (_url, init) => {
          calls++;
          const headers = new Headers(init.headers),
            boundary = headers.get("content-type")?.match(/boundary=([^;]+)/)?.[1];
          let body = await new Response(init.body).text();
          if (boundary) body = body.replaceAll(boundary.replace(/^"|"$/g, ""), "BOUNDARY");
          actual = {
            type: (headers.get("content-type") ?? "").replace(
              /boundary=[^;]+/,
              "boundary=BOUNDARY",
            ),
            body,
            duplex: init.duplex ?? null,
          };
          return new Response(null, { status: 204 });
        },
      });
      await api.$operations.probe(
        { body: source(value) },
        name === "custom" ? { streamCodec: custom } : {},
      );
      assert.equal(calls, 1);
      assert(actual.body.includes(name === "sse" ? "hello" : "one"));
      record(active, actual);
      await api.$operations.probe({ body: source(value) }, { streamCodec: custom });
      assert.equal(actual.body, JSON.stringify(value) + "|");
      record(active + "-protocol-override", actual);
      await rejects(
        active + "-aborted",
        () =>
          api.$operations.probe(
            { body: source(value) },
            { signal: AbortSignal.abort(), streamCodec: custom },
          ),
        "REQUEST_ABORTED",
      );
      const response = await load((active = "response-" + name));
      const type = types[name] + (name === "multipart" ? "; boundary=probe" : "");
      const make = (body, override) =>
        response.createClient({
          baseURL: "https://feature.test",
          fetch: async () => new Response(body, { headers: { "content-type": type } }),
          ...(override ? { streamCodecs: { [types[name]]: custom } } : {}),
        });
      const values = [];
      for await (const entry of make(
        wire(name, value),
        name === "custom",
      ).$operations.probe.stream())
        values.push(entry);
      assert.deepEqual(JSON.parse(JSON.stringify(values)), [value]);
      record(active, values);
      const overridden = [];
      for await (const entry of make(JSON.stringify(value) + "|", true).$operations.probe.stream())
        overridden.push(entry);
      assert.deepEqual(JSON.parse(JSON.stringify(overridden)), [value]);
      record(active + "-protocol-override", overridden);
      await rejects(
        active + "-aborted",
        async () => {
          for await (const _entry of make(wire(name, value)).$operations.probe.stream({
            signal: AbortSignal.abort(),
          })) {
          }
        },
        "REQUEST_ABORTED",
      );
      if (name !== "custom" && name !== "sse")
        await rejects(
          active + "-invalid-item",
          async () => {
            for await (const _entry of make(wire(name, { id: 42 })).$operations.probe.stream()) {
            }
          },
          "RESPONSE_DECODE_FAILED",
        );
      if (name === "ndjson" || name === "sequence" || name === "sse")
        await rejects(
          active + "-frame-limit",
          async () => {
            for await (const _entry of make(wire(name, value)).$operations.probe.stream({
              maxStreamFrameBytes: 1,
            })) {
            }
          },
          "RESPONSE_DECODE_FAILED",
        );
    }
    for (const [fixture, operation, body] of [
      ["only-xmlEcho", "xmlEcho", { id: "one", count: 2 }],
      ["only-formEcho", "formEcho", { id: "one", count: 2 }],
      ["only-multipartEcho", "multipartEcho", { id: "one", count: 2 }],
      ["only-textEcho", "textEcho", "hello"],
    ]) {
      const module = await load((active = fixture));
      let encoded;
      const api = module.createClient({
        baseURL: "https://feature.test",
        fetch: async (_url, init) => {
          encoded =
            init.body instanceof FormData
              ? Object.fromEntries(init.body.entries())
              : String(init.body);
          return fixture === "only-xmlEcho"
            ? new Response("<item><count>2</count><id>one</id></item>", {
                headers: { "content-type": "application/xml" },
              })
            : fixture === "only-textEcho"
              ? new Response(encoded, { headers: { "content-type": "text/plain" } })
              : Response.json(body);
        },
      });
      assert.deepEqual(
        JSON.parse(JSON.stringify(await api.$operations[operation]({ body }))),
        body,
      );
      record(fixture, encoded);
      const raw = await api.$operations[operation].raw({ body });
      assert.deepEqual(JSON.parse(JSON.stringify(raw.data)), body);
      record(fixture + "-raw", raw.status);
      await rejects(
        fixture + "-schema",
        () =>
          api.$operations[operation]({
            body: typeof body === "string" ? 42 : { id: "one", count: -1 },
          }),
        "REQUEST_ENCODE_FAILED",
      );
    }
    const binary = await load((active = "binary"));
    let bytes;
    const binaryAPI = binary.createClient({
      baseURL: "https://feature.test",
      fetch: async (_url, init) => {
        bytes = [...new Uint8Array(await new Response(init.body).arrayBuffer())];
        return new Response(Uint8Array.of(1, 2, 3), {
          headers: { "content-type": "application/octet-stream" },
        });
      },
    });
    const binaryResult = await binaryAPI.$operations.probe({ body: Uint8Array.of(4, 5) });
    assert.deepEqual(bytes, [4, 5]);
    record("binary", {
      sent: bytes,
      received: [...new Uint8Array(await new Response(binaryResult).arrayBuffer())],
    });
    const multipart = await load((active = "request-multipart"));
    let part;
    const multipartAPI = multipart.createClient({
      baseURL: "https://feature.test",
      fetch: async (_url, init) => {
        const headers = new Headers(init.headers),
          boundary = headers.get("content-type").match(/boundary=([^;]+)/)[1];
        part = (await new Response(init.body).text()).replaceAll(boundary, "BOUNDARY");
        return new Response(null, { status: 204 });
      },
    });
    await multipartAPI.$operations.probe(
      { body: source(item) },
      { multipartContentTypes: { 0: "application/xml" } },
    );
    assert(part.toLowerCase().includes("content-type: application/xml"));
    assert(part.includes("one") && part.includes("<"));
    record("multipart-open-XML-part", part);
    const mixed = await load((active = "xml-and-stream"));
    let sent;
    const mixedAPI = mixed.createClient({
      baseURL: "https://feature.test",
      fetch: async (_url, init) => {
        sent = init.body;
        return new Response(wire("ndjson"), { headers: { "content-type": types.ndjson } });
      },
    });
    const mixedItems = [];
    for await (const entry of mixedAPI.$operations.probe.stream({ body: item }))
      mixedItems.push(entry);
    assert.deepEqual(JSON.parse(JSON.stringify(mixedItems)), [item]);
    assert(sent.includes("<id>one</id>"));
    record("xml-and-stream", { sent, mixedItems });
    const dynamic = await load((active = "dynamic"));
    let calls = 0;
    const dynamicAPI = dynamic.createClient({
      baseURL: "https://feature.test",
      fetch: async () => {
        calls++;
        return new Response(null, { status: 204 });
      },
    });
    await dynamicAPI.$operations.probe({ body: { children: [{ children: [] }] } });
    await rejects(
      "dynamic-recursive-invalid",
      () => dynamicAPI.$operations.probe({ body: { children: [{ children: 42 }] } }),
      "REQUEST_ENCODE_FAILED",
    );
    assert.equal(calls, 1);
    record("dynamic-recursive-valid", calls);
    const wildcard = await load((active = "wildcard"));
    const wildcardAPI = wildcard.createClient({
      baseURL: "https://feature.test",
      fetch: async () => new Response("hello", { headers: { "content-type": "text/plain" } }),
      codecs: { "application/x-note": { encode: (value) => String(value) } },
    });
    const wildcardResult = await wildcardAPI.$operations.probe({
      body: { contentType: "application/x-note", value: "hello" },
    });
    assert.equal(wildcardResult, "hello");
    record("wildcard", wildcardResult);
    for (const [fixture, valid, invalid] of [
      ["format-uuid", "550e8400-e29b-41d4-a716-446655440000", "bad-uuid"],
      ["format-ipv4", "127.0.0.1", "999.1.2.3"],
    ]) {
      const module = await load((active = fixture));
      let calls = 0;
      const api = module.createClient({
        baseURL: "https://feature.test",
        fetch: async () => {
          calls++;
          return new Response(null, { status: 204 });
        },
      });
      await api.$operations.probe({ body: valid });
      await rejects(
        fixture + "-invalid",
        () => api.$operations.probe({ body: invalid }),
        "REQUEST_ENCODE_FAILED",
      );
      assert.equal(calls, 1);
      record(fixture + "-valid", calls);
    }
  } catch (error) {
    console.error("Native feature failure", variant, active);
    throw error;
  }
  results[variant] = records;
  console.log("Native features passed", variant, records.length, "scenarios");
}
assert.deepEqual(results.selected, results.full);
fs.writeFileSync(path.join(base, "native-features-results.json"), JSON.stringify(results, null, 2));
