import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";
const base = path.join(path.resolve(process.argv[2]), "server"),
  results = {};
const types = {
  json: "application/json",
  xml: "application/xml",
  dynamic: "application/json",
  mixed: "application/json",
  form: "application/x-www-form-urlencoded",
  multipart: "multipart/form-data",
  text: "text/plain",
  binary: "application/octet-stream",
  custom: "application/x-probe",
  ndjson: "application/x-ndjson",
  sequence: "application/json-seq",
  sse: "text/event-stream",
  "stream-multipart": "multipart/mixed; boundary=probe",
  "stream-custom": "application/x-frames",
};
const value = { id: "one", count: 2 };
const streamCodec = {
  protocol: {
    async *decode(reader) {
      const decoder = new TextDecoder();
      let text = "",
        chunk;
      while ((chunk = await reader.read(1024)) !== null)
        text += decoder.decode(chunk, { stream: true });
      for (const frame of text.split("|")) if (frame) yield JSON.parse(frame);
    },
  },
};
function framed(kind, item) {
  if (kind === "ndjson") return JSON.stringify(item) + "\n";
  if (kind === "sequence") return "\u001e" + JSON.stringify(item) + "\n";
  if (kind === "sse") return "data: " + item.data + "\n\n";
  if (kind === "stream-custom") return JSON.stringify(item) + "|";
  return (
    "--probe\r\nContent-Type: application/json\r\n\r\n" + JSON.stringify(item) + "\r\n--probe--\r\n"
  );
}
function request(name, bad = false, xmlPart = false) {
  const key = name.replace(/^complete-/, "").replace(/^inbound-only-/, ""),
    type = types[key],
    expected = key === "sse" ? { data: "hello" } : value;
  let body,
    headers = { "content-type": type };
  if (["ndjson", "sequence", "sse", "stream-multipart", "stream-custom"].includes(key))
    body =
      key === "sse" && bad
        ? "event: unexpected\ndata: hello\n\n"
        : framed(key, bad ? { id: 42 } : expected);
  else if (key === "json" || key === "custom" || key === "mixed")
    body = JSON.stringify(bad ? { id: 42 } : value);
  else if (key === "dynamic")
    body = JSON.stringify(bad ? { children: "bad" } : { children: [{ children: [] }] });
  else if (key === "xml") body = `<item><id>one</id><count>${bad ? "bad" : "2"}</count></item>`;
  else if (key === "form") body = "id=one&count=" + (bad ? "bad" : "2");
  else if (key === "multipart") {
    body = new FormData();
    body.set("id", "one");
    body.set(
      "count",
      xmlPart ? new Blob(["<count>2</count>"], { type: "application/xml" }) : bad ? "bad" : "2",
    );
    headers = {};
  } else if (key === "binary") body = new Uint8Array([1, 2, 3]);
  else body = "hello";
  return new Request("https://inbound.test/delivery", { method: "POST", headers, body });
}
const report = JSON.parse(fs.readFileSync(path.join(base, "report.json"), "utf8"));
for (const variant of ["full", "selected"]) {
  const records = [];
  let active;
  try {
    for (const fixture of report.fixtures) {
      const name = (active = fixture.name),
        key = name.replace(/^complete-/, "").replace(/^inbound-only-/, ""),
        streamed =
          ["ndjson", "sequence", "sse", "stream-multipart", "stream-custom"].includes(key) &&
          !name.startsWith("complete-");
      const { createWebhookRouter } = await import(
        pathToFileURL(path.join(base, variant + "-js", name, "server/webhooks.js")).href
      );
      let seen,
        calls = 0;
      const options = {
        routes: { delivery: "/delivery" },
        codecs: { "application/x-probe": { decodeInbound: async (request) => request.json() } },
        streamCodecs: key === "stream-custom" ? { [types[key]]: streamCodec } : {},
      };
      const handlers = {
        delivery: {
          POST: async (context) => {
            calls++;
            const body = key === "mixed" ? context.body.value : context.body;
            if (streamed || body?.[Symbol.asyncIterator]) {
              seen = [];
              for await (const item of body) seen.push(item);
            } else if (key === "binary") seen = [...new Uint8Array(body)];
            else seen = body;
            return {
              status: 200,
              ...(name === "xml" ? { contentType: "application/xml" } : {}),
              body: { accepted: Array.isArray(seen) && key !== "binary" ? seen.length : 1 },
            };
          },
        },
      };
      const router = createWebhookRouter(handlers, options);
      const response = await router.fetch(request(name));
      assert.equal(response.status, 200, name + " positive case");
      const expected =
        key === "binary"
          ? [1, 2, 3]
          : key === "text"
            ? "hello"
            : key === "sse"
              ? { data: "hello" }
              : key === "dynamic"
                ? { children: [{ children: [] }] }
                : value;
      assert.deepEqual(
        JSON.parse(JSON.stringify(seen)),
        streamed || name.startsWith("complete-") ? [expected] : expected,
      );
      const encoded = await response.text();
      assert(encoded.includes(name === "xml" ? "<accepted>1</accepted>" : '"accepted":1'));
      records.push({ scenario: name + "-valid", seen, encoded });
      const before = calls;
      const unsupported = await router.fetch(
        new Request("https://inbound.test/delivery", {
          method: "POST",
          headers: { "content-type": "application/x-unknown" },
          body: "bad",
        }),
      );
      assert.equal(unsupported.status, 415);
      assert.equal(calls, before);
      records.push({
        scenario: name + "-unsupported",
        status: unsupported.status,
        text: await unsupported.text(),
      });
      const absent = await router.fetch(
        new Request("https://inbound.test/delivery", { method: "POST" }),
      );
      assert(absent.status >= 400);
      assert.equal(calls, before);
      records.push({
        scenario: name + "-required",
        status: absent.status,
        text: await absent.text(),
      });
      if (!["text", "binary"].includes(key)) {
        const invalid = await router.fetch(request(name, true));
        assert(invalid.status >= 400);
        records.push({
          scenario: name + "-invalid",
          status: invalid.status,
          text: await invalid.text(),
        });
      }
      if (name === "multipart") {
        const xml = await router.fetch(request(name, false, true));
        assert.equal(xml.status, 200);
        assert.equal(seen.count, 2);
        records.push({ scenario: "inbound-multipart-xml-part", seen });
      }
      if (key === "mixed")
        for (const format of ["xml", "ndjson"]) {
          const response = await router.fetch(request(format));
          assert.equal(response.status, 200);
          assert.deepEqual(JSON.parse(JSON.stringify(seen)), format === "ndjson" ? [value] : value);
          records.push({ scenario: name + "-" + format, seen });
        }
      if (streamed || name.startsWith("complete-")) {
        const tiny = createWebhookRouter(handlers, { ...options, maxStreamFrameBytes: 2 });
        const limited = await tiny.fetch(request(name));
        assert(limited.status >= 400);
        records.push({
          scenario: name + "-frame-limit",
          status: limited.status,
          text: await limited.text(),
        });
        const overridden = createWebhookRouter(handlers, {
          ...options,
          streamCodecs: { [types[key].split(";", 1)[0]]: streamCodec },
        });
        const input = new Request("https://inbound.test/delivery", {
          method: "POST",
          headers: { "content-type": types[key] },
          body: JSON.stringify(key === "sse" ? { data: "hello" } : value) + "|",
        });
        assert.equal((await overridden.fetch(input)).status, 200);
        records.push({ scenario: name + "-protocol-override", seen });
      }
    }
  } catch (error) {
    console.error("Server native failure", variant, active);
    throw error;
  }
  results[variant] = records;
  console.log("Native server passed", variant, records.length, "scenarios");
}
assert.deepEqual(results.selected, results.full);
fs.writeFileSync(path.join(base, "native-results.json"), JSON.stringify(results, null, 2));
