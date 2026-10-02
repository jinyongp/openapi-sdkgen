// Execute the actual directive-free SDKs compiled by execution-providers.mjs.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

const output = path.resolve(process.argv[2]);
const cases = JSON.parse(
  fs.readFileSync(new URL("../fixtures/runtime-quality.cases.json", import.meta.url)),
);
const checks = [];
const jsonValue = (value) => JSON.parse(JSON.stringify(value));
for (const version of ["3-0-3", "3-1-1", "3-2-0"]) {
  const { createClient } = await import(
    pathToFileURL(path.join(output, `runtime-quality-${version}/index.js`))
  );
  const requests = [];
  let response;
  const api = createClient({
    baseURL: "https://quality.test",
    fetch: async (url, init) => {
      requests.push({ url: String(url), body: init.body, headers: new Headers(init.headers) });
      return response();
    },
  });
  for (const [name, control] of Object.entries(cases)) {
    if (version === "3-0-3" || (version === "3-1-1" && ["xmlText", "xmlCdata"].includes(name)))
      continue;
    const before = requests.length;
    response = () => new Response(null, { status: 204 });
    const encode = () => api.$operations[name + "Encode"]({ body: control.value });
    if (control.invalid) {
      await assert.rejects(encode);
      assert.equal(requests.length, before, "Invalid input reached the transport");
    } else {
      await encode();
      const body = requests.at(-1).body;
      if (control.media === "application/json") assert.deepEqual(JSON.parse(body), control.value);
      // Namespace declaration placement and equivalent entity spellings can vary.
      // Decode literal XML controls separately from the encoder output below.
      else {
        response = () => new Response(body, { headers: { "Content-Type": control.media } });
        assert.deepEqual(jsonValue(await api.$operations[name + "Decode"]()), control.value);
      }
    }
    response = () =>
      new Response(control.response ?? JSON.stringify(control.value), {
        headers: { "Content-Type": control.media },
      });
    const decode = () => api.$operations[name + "Decode"]();
    if (control.invalid) await assert.rejects(decode);
    else assert.deepEqual(jsonValue(await decode()), control.value, `Literal response: ${name}`);
    checks.push(`${version}:${name}:request-and-response`);
  }
  for (const location of ["query", "header"]) {
    const section = location === "query" ? "query" : "headerParams";
    response = () => new Response(null, { status: 204 });
    for (const required of [true, false]) {
      const call = api.$operations[location + (required ? "Required" : "Optional")];
      await call({ [section]: { filter: null } });
      const request = requests.at(-1);
      assert.equal(
        location === "query"
          ? new URL(request.url).searchParams.get("filter")
          : request.headers.get("filter"),
        "null",
      );
      if (required)
        for (const value of [{}, { filter: undefined }]) {
          const before = requests.length;
          await assert.rejects(() => call({ [section]: value }));
          assert.equal(requests.length, before);
        }
    }
    checks.push(`${version}:${location}:nullable-presence-and-transmission`);
  }
  if (version === "3-0-3")
    for (const [name, media] of Object.entries({
      pdf: "application/pdf",
      zip: "application/zip",
      octet: "application/octet-stream",
      image: "image/png",
    })) {
      const bytes = new Uint8Array([1, 2, 3]);
      response = () => new Response(bytes, { headers: { "Content-Type": media } });
      const stream = await api.$operations[name]();
      assert(stream instanceof ReadableStream);
      assert.deepEqual(new Uint8Array(await new Response(stream).arrayBuffer()), bytes);
      checks.push(`${version}:${name}:binary-response`);
    }
  if (version === "3-2-0") {
    const value = "x".repeat(32768);
    const bytes = new TextEncoder().encode("data: " + JSON.stringify(value) + "\r\n\r\n");
    response = () =>
      new Response(
        new ReadableStream({
          start(controller) {
            for (let offset = 0; offset < bytes.length; offset += 64)
              controller.enqueue(bytes.slice(offset, offset + 64));
            controller.close();
          },
        }),
        { headers: { "Content-Type": "text/event-stream" } },
      );
    const items = [];
    const streamCodec = {
      adapter: {
        async *decode(frames) {
          for await (const frame of frames) yield JSON.parse(frame.data);
        },
      },
    };
    for await (const item of api.$operations.events.stream({ streamCodec })) items.push(item);
    assert.deepEqual(items, [value]);
    checks.push(`${version}:fragmented-sse`);
  }
}
console.log(JSON.stringify({ pass: true, checks }));
