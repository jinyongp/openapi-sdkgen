package typescript

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestServerAddOnAcceptsBinaryInboundBodies(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi": "3.1.1",
  "info": {"title": "Webhook", "version": "1"},
  "paths": {},
  "webhooks": {"orderCreated": {"post": {
    "requestBody": {"content": {"application/pdf": {"schema": {"type": "string", "format": "binary"}}}},
    "responses": {"204": {"description": "Accepted"}}
  }}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	options, err := registry.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (Generator{}).Generate(document, options); err != nil {
		t.Fatalf("server binary media generation = %v", err)
	}
}

func TestGeneratedWebhookRouterDecodesTextAndFormBodies(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1", "info":{"title":"Inbound media","version":"1"}, "paths":{},
  "webhooks": {
    "formReceived": {"post":{"requestBody":{"required":true,"content":{"application/x-www-form-urlencoded":{"schema":{"type":"object","required":["name","count","enabled","tags","meta"],"properties":{"name":{"type":"string"},"count":{"type":"integer"},"enabled":{"type":"boolean"},"tags":{"type":"array","items":{"type":"string"}},"meta":{"type":"object","required":["source"],"properties":{"source":{"type":"string"}}}}},"encoding":{"meta":{"contentType":"application/json"}}}}},"responses":{"204":{"description":"OK"}}}},
    "textReceived": {"post":{"requestBody":{"required":true,"content":{"text/plain":{"schema":{"type":"string","minLength":3}}}},"responses":{"204":{"description":"OK"}}}},
    "xmlReceived": {"post":{"requestBody":{"required":true,"content":{"application/xml":{"schema":{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}}}},"responses":{"204":{"description":"OK"}}}},
    "multipartReceived": {"post":{"requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["name","meta","custom"],"properties":{"name":{"type":"string"},"meta":{"type":"object","required":["source"],"properties":{"source":{"type":"string"}}},"custom":{"type":"object","required":["source"],"properties":{"source":{"type":"string"}}}}},"encoding":{"meta":{"contentType":"application/json"},"custom":{"contentType":"application/vnd.example.part"}}}}},"responses":{"204":{"description":"OK"}}}},
    "binaryReceived": {"post":{"requestBody":{"required":true,"content":{"application/pdf":{"schema":{"type":"string","format":"binary"}}}},"responses":{"204":{"description":"OK"}}}},
    "multiReceived": {"post":{"requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","required":["event_id"],"properties":{"event_id":{"type":"string"}}}},"text/plain":{"schema":{"type":"string"}}}},"responses":{"204":{"description":"OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	options, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	addons, err := options.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := (Generator{}).Generate(document, addons)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	writeTargetArtifacts(t, source, artifacts)
	if err := os.WriteFile(filepath.Join(source, "package.json"), []byte(`{"type":"module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "tsconfig.json"), []byte(serverTSConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	tsc := filepath.Join("..", "..", "..", "test", "typescript", "node_modules", "typescript", "lib", "tsc.js")
	if _, err := os.Stat(tsc); err != nil {
		t.Skipf("TypeScript compiler unavailable for server test: %v", err)
	}
	if output, err := exec.Command("node", tsc, "--project", filepath.Join(source, "tsconfig.json")).CombinedOutput(); err != nil {
		t.Fatalf("compile generated inbound media server: %v\n%s", err, output)
	}
	outputDirectory := filepath.Join(directory, "output")
	if err := os.WriteFile(filepath.Join(outputDirectory, "package.json"), []byte(`{"type":"module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `
import { pathToFileURL } from "node:url";
const { createWebhookRouter } = await import(pathToFileURL(process.argv[1]).href);
const seen = [];
const router = createWebhookRouter({
  formReceived: { POST: async ({ body }) => { if (body.count !== 2 || body.enabled !== true || body.tags.join(",") !== "one,two" || body.meta.source !== "form") throw new Error("form values were not typed"); seen.push(body); return { status: 204 }; } },
  textReceived: { POST: async ({ body }) => { seen.push(body); return { status: 204 }; } },
  xmlReceived: { POST: async ({ body }) => { seen.push(body); return { status: 204 }; } },
  multipartReceived: { POST: async ({ body }) => { if (body.meta.source !== "multipart" || body.custom.source !== "custom") throw new Error("multipart fields were not decoded"); seen.push(body); return { status: 204 }; } },
  binaryReceived: { POST: async ({ body }) => { seen.push({ bytes: body.byteLength }); return { status: 204 }; } },
  multiReceived: { POST: async ({ body }) => { seen.push(body.contentType === "application/json" ? body.value.event_id : body.value); return { status: 204 }; } },
}, { routes: { formReceived: "/form", textReceived: "/text", xmlReceived: "/xml", multipartReceived: "/multipart", binaryReceived: "/binary", multiReceived: "/multi" }, codecs: { "application/vnd.example.part": { decodeParameter: (value) => JSON.parse(value) } } });
if ((await router.fetch(new Request("https://host.test/form", { method: "POST", headers: { "content-type": "application/x-www-form-urlencoded" }, body: "name=widget&count=2&enabled=true&tags=one&tags=two&meta=%7B%22source%22%3A%22form%22%7D" }))).status !== 204) throw new Error("form body rejected");
if ((await router.fetch(new Request("https://host.test/text", { method: "POST", headers: { "content-type": "text/plain" }, body: "hello" }))).status !== 204) throw new Error("text body rejected");
if ((await router.fetch(new Request("https://host.test/text", { method: "POST", headers: { "content-type": "text/plain" }, body: "   " }))).status !== 204) throw new Error("whitespace text body rejected");
if ((await router.fetch(new Request("https://host.test/text", { method: "POST", headers: { "content-type": "text/plain" } }))).status !== 400) throw new Error("empty text body accepted");
if ((await router.fetch(new Request("https://host.test/xml", { method: "POST", headers: { "content-type": "application/xml" }, body: "<item><name>&#65; &#x42;</name></item>" }))).status !== 204) throw new Error("XML body rejected");
if ((await router.fetch(new Request("https://host.test/xml", { method: "POST", headers: { "content-type": "application/xml" }, body: "<item><name>&#0;</name></item>" }))).status !== 400) throw new Error("invalid XML character reference accepted");
const multipart = new FormData(); multipart.set("name", "widget"); multipart.set("meta", new Blob(['{"source":"multipart"}'], { type: "application/json" })); multipart.set("custom", new Blob(['{"source":"custom"}'], { type: "application/vnd.example.part" }));
if ((await router.fetch(new Request("https://host.test/multipart", { method: "POST", body: multipart }))).status !== 204) throw new Error("multipart body rejected");
if ((await router.fetch(new Request("https://host.test/binary", { method: "POST", headers: { "content-type": "application/pdf" }, body: new Uint8Array([1, 2, 3]) }))).status !== 204) throw new Error("binary body rejected");
if ((await router.fetch(new Request("https://host.test/multi", { method: "POST", headers: { "content-type": "application/json" }, body: '{"event_id":"json"}' }))).status !== 204) throw new Error("JSON multi body rejected");
if ((await router.fetch(new Request("https://host.test/multi", { method: "POST", headers: { "content-type": "text/plain" }, body: "text" }))).status !== 204) throw new Error("text multi body rejected");
if ((await router.fetch(new Request("https://host.test/text", { method: "POST", headers: { "content-type": "text/plain" }, body: "no" }))).status !== 400) throw new Error("invalid text body accepted");
if (JSON.stringify(seen) !== JSON.stringify([{ name: "widget", count: 2, enabled: true, tags: ["one", "two"], meta: { source: "form" } }, "hello", "   ", { name: "A B" }, { name: "widget", meta: { source: "multipart" }, custom: { source: "custom" } }, { bytes: 3 }, "json", "text"])) throw new Error("inbound bodies were not decoded");

const limited = createWebhookRouter({
  formReceived: { POST: async () => ({ status: 204 }) },
  textReceived: { POST: async () => ({ status: 204 }) },
  multipartReceived: { POST: async () => ({ status: 204 }) },
  binaryReceived: { POST: async () => ({ status: 204 }) },
  multiReceived: { POST: async () => ({ status: 204 }) },
}, {
  routes: { formReceived: "/form", textReceived: "/text", multipartReceived: "/multipart", binaryReceived: "/binary", multiReceived: "/multi" },
  codecs: { "application/vnd.example.part": { decodeParameter: (value) => JSON.parse(value) } },
  maxBodyBytes: 5,
});
if ((await limited.fetch(new Request("https://host.test/text", { method: "POST", headers: { "content-type": "text/plain" }, body: "hello" }))).status !== 204) throw new Error("body exactly at maxBodyBytes was rejected");
if ((await limited.fetch(new Request("https://host.test/text", { method: "POST", headers: { "content-type": "text/plain" }, body: "helloo" }))).status !== 413) throw new Error("oversized text body was accepted");
if ((await limited.fetch(new Request("https://host.test/binary", { method: "POST", headers: { "content-type": "application/pdf" }, body: new Uint8Array(6) }))).status !== 413) throw new Error("oversized binary body was accepted");
if ((await limited.fetch(new Request("https://host.test/multi", { method: "POST", headers: { "content-type": "application/json" }, body: '{"event_id":"json"}' }))).status !== 413) throw new Error("oversized JSON body was accepted");
if ((await limited.fetch(new Request("https://host.test/form", { method: "POST", headers: { "content-type": "application/x-www-form-urlencoded" }, body: "name=widget&count=2&enabled=true&tags=one&meta=%7B%7D" }))).status !== 413) throw new Error("oversized form body was accepted");
const oversizedMultipart = new FormData(); oversizedMultipart.set("name", "widget"); oversizedMultipart.set("meta", new Blob(['{"source":"multipart"}'], { type: "application/json" })); oversizedMultipart.set("custom", new Blob(['{"source":"custom"}'], { type: "application/vnd.example.part" }));
if ((await limited.fetch(new Request("https://host.test/multipart", { method: "POST", body: oversizedMultipart }))).status !== 413) throw new Error("oversized multipart body was accepted");
const encoder = new TextEncoder();
const chunkedOversized = new ReadableStream({ start(controller) { controller.enqueue(encoder.encode("helloo")); controller.close(); } });
const chunkedRequest = new Request("https://host.test/text", { method: "POST", headers: { "content-type": "text/plain" }, body: chunkedOversized, duplex: "half" });
if (chunkedRequest.headers.has("content-length")) throw new Error("chunked body unexpectedly has Content-Length");
if ((await limited.fetch(chunkedRequest)).status !== 413) throw new Error("oversized body without Content-Length was accepted");
const nonClosingOversized = new ReadableStream({
  start(controller) { controller.enqueue(encoder.encode("helloo")); },
});
const nonClosingRequest = new Request("https://host.test/text", { method: "POST", headers: { "content-type": "text/plain" }, body: nonClosingOversized, duplex: "half" });
if ((await limited.fetch(nonClosingRequest)).status !== 413) throw new Error("oversized non-closing body was not rejected immediately");
if ((await limited.fetch(new Request("https://host.test/text", { method: "POST", headers: { "content-type": "text/plain", "content-length": "1" }, body: "helloo" }))).status !== 413) throw new Error("misleading Content-Length bypassed maxBodyBytes");
const defaultLimited = createWebhookRouter({ binaryReceived: { POST: async () => ({ status: 204 }) } }, { routes: { binaryReceived: "/binary" } });
if ((await defaultLimited.fetch(new Request("https://host.test/binary", { method: "POST", headers: { "content-type": "application/pdf", "content-length": String(8 * 1024 * 1024 + 1) }, body: new Uint8Array([1]) }))).status !== 413) throw new Error("default maxBodyBytes was not enforced");
`
	if output, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(outputDirectory, "server", "webhooks.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute generated inbound media server: %v\n%s", err, output)
	}
}

func TestGeneratedWebhookRouterStreamsSequentialBodies(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.2.0", "info":{"title":"Inbound streams","version":"1"}, "paths":{},
  "webhooks":{"events":{"post":{"requestBody":{"required":true,"content":{"application/x-ndjson":{"itemSchema":{"type":"object","required":["event_id"],"properties":{"event_id":{"type":"string"}}}}}},"responses":{"204":{"description":"OK"}}}},"frames":{"post":{"requestBody":{"required":true,"content":{"multipart/mixed":{"itemSchema":{"type":"object","required":["frame_id"],"properties":{"frame_id":{"type":"string"}}},"itemEncoding":{"contentType":"application/json","headers":{"x-frame":{"required":true,"schema":{"type":"string"}}}}}}},"responses":{"204":{"description":"OK"}}}},"custom":{"post":{"requestBody":{"required":true,"content":{"application/*":{"schema":{"type":"object","required":["event_id"],"properties":{"event_id":{"type":"string"}}}}}},"responses":{"204":{"description":"OK"}}}},"customStream":{"post":{"requestBody":{"required":true,"content":{"application/vnd.example.events":{"itemSchema":{"type":"object","required":["event_id"],"properties":{"event_id":{"type":"string"}}}}}},"responses":{"204":{"description":"OK"}}}},"batch":{"post":{"requestBody":{"required":true,"content":{"application/x-ndjson":{"schema":{"type":"array","items":{"type":"object","required":["event_id"],"properties":{"event_id":{"type":"string"}}}}}}},"responses":{"204":{"description":"OK"}}}},"denied":{"post":{"requestBody":{"required":true,"content":{"application/json":{"schema":false}}},"responses":{"204":{"description":"OK"}}}}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	addons, err := registry.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := (Generator{}).Generate(document, addons)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	writeTargetArtifacts(t, source, artifacts)
	webhookSource, err := os.ReadFile(filepath.Join(source, "server", "webhooks.ts"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(webhookSource), "itemEncoding:") {
		t.Fatal("generated inbound stream plan omitted itemEncoding")
	}
	if !strings.Contains(string(webhookSource), "x-frame") {
		sourceText := string(webhookSource)
		index := strings.Index(sourceText, "itemEncoding:")
		end := index + 400
		if index < 0 {
			index = 0
		}
		if end > len(sourceText) {
			end = len(sourceText)
		}
		t.Fatalf("generated inbound stream plan omitted itemEncoding headers: %q", sourceText[index:end])
	}
	if err := os.WriteFile(filepath.Join(source, "package.json"), []byte(`{"type":"module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "tsconfig.json"), []byte(serverTSConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	tsc := filepath.Join("..", "..", "..", "test", "typescript", "node_modules", "typescript", "lib", "tsc.js")
	if _, err := os.Stat(tsc); err != nil {
		t.Skipf("TypeScript compiler unavailable for server stream test: %v", err)
	}
	if output, err := exec.Command("node", tsc, "--project", filepath.Join(source, "tsconfig.json")).CombinedOutput(); err != nil {
		t.Fatalf("compile generated inbound stream server: %v\n%s", err, output)
	}
	outputDirectory := filepath.Join(directory, "output")
	if err := os.WriteFile(filepath.Join(outputDirectory, "package.json"), []byte(`{"type":"module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `
import { pathToFileURL } from "node:url";
const { createWebhookRouter } = await import(pathToFileURL(process.argv[1]).href);
const seen = [];
let customCodecCalls = 0;
const codecs = {
  "application/vnd.example.event": { async decodeInbound(request) { customCodecCalls++; return JSON.parse(await request.text()); } },
};
const streamCodecs = {
  "application/x-ndjson": { adapter: {
    async *decode(frames) { for await (const frame of frames) yield typeof frame.event_id === "string" ? { ...frame, event_id: "adapted-" + frame.event_id } : frame; },
    async *encode(items) { yield* items; },
  } },
  "multipart/mixed": { adapter: {
    async *decode(frames) { for await (const frame of frames) yield typeof frame.frame_id === "string" ? frame : { frame_id: frame.wrong }; },
    async *encode(items) { yield* items; },
  } },
  "application/vnd.example.events": {
    protocol: {
      async *decode(reader) { const decoder = new TextDecoder(); let pending = ""; while (true) { const chunk = await reader.read(1024); if (chunk === null) break; pending += decoder.decode(chunk, { stream: true }); let newline; while ((newline = pending.indexOf("\n")) >= 0) { const line = pending.slice(0, newline); pending = pending.slice(newline + 1); if (line !== "") yield JSON.parse(line); } } if (pending !== "") yield JSON.parse(pending); },
      encode() { throw new Error("encode not used"); },
    },
    adapter: {
      async *decode(frames) { for await (const frame of frames) yield { ...frame, event_id: "custom-" + frame.event_id }; },
      async *encode(items) { yield* items; },
    },
  },
};
const router = createWebhookRouter({ events: { POST: async ({ body }) => { for await (const item of body) seen.push(item.event_id); return { status: 204 }; } }, frames: { POST: async ({ body }) => { for await (const item of body) seen.push(item.frame_id); return { status: 204 }; } }, custom: { POST: async ({ body }) => { seen.push(body.event_id); return { status: 204 }; } }, customStream: { POST: async ({ body }) => { for await (const item of body) seen.push(item.event_id); return { status: 204 }; } }, batch: { POST: async ({ body }) => { seen.push(body.map((item) => item.event_id).join("|")); return { status: 204 }; } }, denied: { POST: async () => ({ status: 204 }) } }, { routes: { events: "/events", frames: "/frames", custom: "/custom", customStream: "/custom-stream", batch: "/batch", denied: "/denied" }, codecs, streamCodecs, maxStreamFrameBytes: 1024 });
const encoder = new TextEncoder();
const valid = new ReadableStream({ start(controller) { controller.enqueue(encoder.encode('{"event_id":"one"}\n{"ev')); controller.enqueue(encoder.encode('ent_id":"two"}\n')); controller.close(); } });
const validResponse = await router.fetch(new Request("https://host.test/events", { method: "POST", headers: { "content-type": "application/x-ndjson" }, body: valid, duplex: "half" }));
if (validResponse.status !== 204 || seen.join(",") !== "adapted-one,adapted-two") throw new Error("inbound NDJSON stream adapter was not applied");
const invalid = new ReadableStream({ start(controller) { controller.enqueue(encoder.encode('{"wrong":true}\n')); controller.close(); } });
const invalidResponse = await router.fetch(new Request("https://host.test/events", { method: "POST", headers: { "content-type": "application/x-ndjson" }, body: invalid, duplex: "half" }));
if (invalidResponse.status !== 400) throw new Error("invalid inbound stream item was accepted");
const bounded = createWebhookRouter({ events: { POST: async ({ body }) => { for await (const _ of body) { } return { status: 204 }; } }, frames: { POST: async ({ body }) => { for await (const _ of body) { } return { status: 204 }; } } }, { routes: { events: "/events", frames: "/frames" }, maxStreamFrameBytes: 4 });
if ((await bounded.fetch(new Request("https://host.test/events", { method: "POST", headers: { "content-type": "application/x-ndjson" }, body: '{"event_id":"too-long"}' }))).status !== 400) throw new Error("oversized unfinished inbound stream frame was accepted");
const chunkedBody = (chunks) => new ReadableStream({ start(controller) { for (const chunk of chunks) controller.enqueue(encoder.encode(chunk)); controller.close(); } });
const boundedNDJSONBody = '{"event_id":"too-long"}\n';
for (const chunks of [[boundedNDJSONBody], [boundedNDJSONBody.slice(0, 10), boundedNDJSONBody.slice(10)]]) {
  const response = await bounded.fetch(new Request("https://host.test/events", { method: "POST", headers: { "content-type": "application/x-ndjson" }, body: chunkedBody(chunks), duplex: "half" }));
  if (response.status !== 400) throw new Error("oversized completed inbound stream frame was accepted for chunk partition");
}
const boundedMultipartBody = "--frames\r\ncontent-type: application/json\r\n\r\n{\"frame_id\":\"too-long\"}\r\n--frames--\r\n";
for (const chunks of [[boundedMultipartBody], [boundedMultipartBody.slice(0, 50), boundedMultipartBody.slice(50)]]) {
  const response = await bounded.fetch(new Request("https://host.test/frames", { method: "POST", headers: { "content-type": "multipart/mixed; boundary=frames" }, body: chunkedBody(chunks), duplex: "half" }));
  if (response.status !== 400) throw new Error("oversized completed inbound multipart frame was accepted for chunk partition");
}
const missingMultipartHeaderBody = "--frames\r\ncontent-type: application/json\r\n\r\n{\"frame_id\":\"one\"}\r\n--frames--\r\n";
const missingMultipartHeaderResponse = await router.fetch(new Request("https://host.test/frames", { method: "POST", headers: { "content-type": "multipart/mixed; boundary=frames" }, body: missingMultipartHeaderBody }));
if (missingMultipartHeaderResponse.status !== 400) throw new Error("missing required inbound multipart item header was accepted");
const multipartBody = "--frames\r\ncontent-type: application/json\r\nx-frame: first\r\n\r\n{\"frame_id\":\"one\"}\r\n--frames\r\ncontent-type: application/json\r\nx-frame: second\r\n\r\n{\"frame_id\":\"two\"}\r\n--frames--\r\n";
const multipartResponse = await router.fetch(new Request("https://host.test/frames", { method: "POST", headers: { "content-type": "multipart/mixed; boundary=frames" }, body: multipartBody }));
if (multipartResponse.status !== 204 || seen.join(",") !== "adapted-one,adapted-two,one,two") throw new Error("inbound multipart stream was not decoded");
const repairedMultipartBody = "--frames\r\ncontent-type: application/json\r\nx-frame: repaired\r\n\r\n{\"wrong\":\"fixed\"}\r\n--frames--\r\n";
const repairedMultipartResponse = await router.fetch(new Request("https://host.test/frames", { method: "POST", headers: { "content-type": "multipart/mixed; boundary=frames" }, body: repairedMultipartBody }));
if (repairedMultipartResponse.status !== 204 || seen.at(-1) !== "fixed") throw new Error("inbound multipart adapter did not run before item-schema validation");
const customResponse = await router.fetch(new Request("https://host.test/custom", { method: "POST", headers: { "content-type": "application/vnd.example.event" }, body: '{"event_id":"three"}' }));
if (customResponse.status !== 204 || seen.join(",") !== "adapted-one,adapted-two,one,two,fixed,three" || customCodecCalls !== 1) throw new Error("custom inbound body was not decoded");

const boundedComplete = createWebhookRouter({
  custom: { POST: async () => ({ status: 204 }) },
  batch: { POST: async () => ({ status: 204 }) },
}, {
  routes: { custom: "/custom", batch: "/batch" },
  codecs,
  streamCodecs,
  maxBodyBytes: 8,
  maxStreamFrameBytes: 1024,
});
if ((await boundedComplete.fetch(new Request("https://host.test/custom", { method: "POST", headers: { "content-type": "application/vnd.example.event" }, body: '{"event_id":"oversized"}' }))).status !== 413) throw new Error("oversized custom codec body was accepted");
if (customCodecCalls !== 1) throw new Error("custom codec ran before maxBodyBytes rejection");
if ((await boundedComplete.fetch(new Request("https://host.test/batch", { method: "POST", headers: { "content-type": "application/x-ndjson" }, body: '{"event_id":"oversized"}\n' }))).status !== 413) throw new Error("oversized complete sequential body was accepted");

let streamedWithSmallBodyLimit = 0;
const streamBodyLimitIndependent = createWebhookRouter({
  customStream: { POST: async ({ body }) => { for await (const _ of body) streamedWithSmallBodyLimit++; return { status: 204 }; } },
}, {
  routes: { customStream: "/custom-stream" },
  streamCodecs,
  maxBodyBytes: 1,
  maxStreamFrameBytes: 1024,
});
const independentStreamResponse = await streamBodyLimitIndependent.fetch(new Request("https://host.test/custom-stream", { method: "POST", headers: { "content-type": "application/vnd.example.events" }, body: '{"event_id":"streamed"}\n' }));
if (independentStreamResponse.status !== 204 || streamedWithSmallBodyLimit !== 1) throw new Error("maxBodyBytes incorrectly limited a streaming request body");

const customStreamResponse = await router.fetch(new Request("https://host.test/custom-stream", { method: "POST", headers: { "content-type": "application/vnd.example.events" }, body: '{"event_id":"four"}\n{"event_id":"five"}\n' }));
if (customStreamResponse.status !== 204 || seen.join(",") !== "adapted-one,adapted-two,one,two,fixed,three,custom-four,custom-five") throw new Error("custom inbound stream protocol/adapter was not decoded");
const batchResponse = await router.fetch(new Request("https://host.test/batch", { method: "POST", headers: { "content-type": "application/x-ndjson" }, body: '{"event_id":"six"}\n{"event_id":"seven"}\n' }));
if (batchResponse.status !== 204 || seen.at(-1) !== "adapted-six|adapted-seven") throw new Error("complete inbound sequential body was not decoded through the stream adapter");
let abortedBodyCancels = 0;
const hangingRouter = createWebhookRouter({
  customStream: { POST: async ({ body }) => { for await (const _ of body) { } return { status: 204 }; } },
}, {
  routes: { customStream: "/custom-stream" },
  streamCodecs: {
    "application/vnd.example.events": { protocol: {
      async *decode() { await new Promise(() => undefined); yield { event_id: "never" }; },
      encode() { throw new Error("encode not used"); },
    } },
  },
});
const abortController = new AbortController();
const hangingBody = new ReadableStream({ cancel() { abortedBodyCancels++; } });
const abortedRequest = new Request("https://host.test/custom-stream", {
  method: "POST",
  headers: { "content-type": "application/vnd.example.events" },
  body: hangingBody,
  signal: abortController.signal,
  duplex: "half",
});
const abortedPending = hangingRouter.fetch(abortedRequest);
await Promise.resolve();
abortController.abort("stop");
const abortedStatus = await Promise.race([
  abortedPending.then((response) => response.status),
  new Promise((resolve) => setTimeout(() => resolve("timeout"), 250)),
]);
if (abortedStatus === "timeout") throw new Error("aborted inbound custom stream did not settle");
if (abortedBodyCancels !== 1) throw new Error("aborted inbound custom stream body was not cancelled exactly once: " + abortedBodyCancels);
const deniedResponse = await router.fetch(new Request("https://host.test/denied", { method: "POST", headers: { "content-type": "application/json" }, body: "{}" }));
if (deniedResponse.status !== 400) throw new Error("false inbound schema accepted a body");
`
	if output, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(outputDirectory, "server", "webhooks.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute generated inbound stream server: %v\n%s", err, output)
	}
}

func TestGeneratedWebhookRouterUsesDefaultSSEJSONAdapterAndCustomRawFrames(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.2.0",
  "info":{"title":"Inbound SSE","version":"1"},
  "paths":{},
  "webhooks":{"events":{"post":{
    "requestBody":{"required":true,"content":{"text/event-stream":{"itemSchema":{
      "type":"object",
      "required":["value"],
      "additionalProperties":false,
      "properties":{"value":{"type":"string"}}
    }}}},
    "responses":{"204":{"description":"OK"}}
  }}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	addons, err := registry.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := (Generator{}).Generate(document, addons)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	writeTargetArtifacts(t, source, artifacts)
	if err := os.WriteFile(filepath.Join(source, "package.json"), []byte(`{"type":"module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "tsconfig.json"), []byte(serverTSConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	tsc := filepath.Join("..", "..", "..", "test", "typescript", "node_modules", "typescript", "lib", "tsc.js")
	if _, err := os.Stat(tsc); err != nil {
		t.Skipf("TypeScript compiler unavailable for inbound SSE test: %v", err)
	}
	if output, err := exec.Command("node", tsc, "--project", filepath.Join(source, "tsconfig.json")).CombinedOutput(); err != nil {
		t.Fatalf("compile generated inbound SSE server: %v\n%s", err, output)
	}
	outputDirectory := filepath.Join(directory, "output")
	if err := os.WriteFile(filepath.Join(outputDirectory, "package.json"), []byte(`{"type":"module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `
import { pathToFileURL } from "node:url";
const { createWebhookRouter } = await import(pathToFileURL(process.argv[1]).href);
const wire =
  'id: one\ndata: {"value":"first"}\n\n' +
  'data: {"value":"inherited"}\n\n' +
  'id\ndata: {"value":"reset"}\n\n' +
  'data: {"value":"after"}\n\n';

const defaultSeen = [];
const defaultRouter = createWebhookRouter({
  events: { POST: async ({ body }) => {
    for await (const item of body) defaultSeen.push(item.value);
    return { status: 204 };
  } },
}, { routes: { events: "/events" } });
const defaultResponse = await defaultRouter.fetch(new Request("https://host.test/events", {
  method: "POST",
  headers: { "content-type": "text/event-stream" },
  body: wire,
}));
if (defaultResponse.status !== 204) throw new Error("default inbound SSE request failed: " + defaultResponse.status);
if (defaultSeen.join(",") !== "first,inherited,reset,after")
  throw new Error("default inbound SSE JSON adapter mismatch: " + defaultSeen.join(","));

const rawFrames = [];
const customSeen = [];
const adapter = {
  async *decode(frames) {
    for await (const frame of frames) {
      rawFrames.push((frame.id ?? "missing") + ":" + frame.data);
      yield JSON.parse(frame.data);
    }
  },
  async *encode(items) {
    for await (const item of items) yield { data: JSON.stringify(item) };
  },
};
const customRouter = createWebhookRouter({
  events: { POST: async ({ body }) => {
    for await (const item of body) customSeen.push(item.value);
    return { status: 204 };
  } },
}, {
  routes: { events: "/events" },
  streamCodecs: { "text/event-stream": { adapter } },
});
const customResponse = await customRouter.fetch(new Request("https://host.test/events", {
  method: "POST",
  headers: { "content-type": "text/event-stream" },
  body: wire,
}));
if (customResponse.status !== 204) throw new Error("custom inbound SSE request failed: " + customResponse.status);
if (customSeen.join(",") !== "first,inherited,reset,after")
  throw new Error("custom inbound SSE adapter output mismatch: " + customSeen.join(","));
if (rawFrames.join("|") !==
  'one:{"value":"first"}|one:{"value":"inherited"}|:{"value":"reset"}|:{"value":"after"}')
  throw new Error("custom adapter did not receive raw SSE frames: " + rawFrames.join("|"));
`
	if output, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(outputDirectory, "server", "webhooks.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute generated inbound SSE JSON/default adapter server test: %v\n%s", err, output)
	}
}

func TestGeneratedWebhookRouterDecodesCompleteMultipartSequentialBody(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.2.0",
  "info":{"title":"Inbound complete multipart","version":"1"},
  "paths":{},
  "webhooks":{"bundle":{"post":{
    "requestBody":{"required":true,"content":{"multipart/mixed":{
      "schema":{"type":"array","prefixItems":[
        {"type":"object","required":["event_id"],"properties":{"event_id":{"type":"string"}}},
        {"type":"string"}
      ]},
      "prefixEncoding":[
        {"contentType":"application/json","headers":{"X-First":{"required":true,"schema":{"type":"string"}}}},
        {"contentType":"text/plain"}
      ]
    }}},
    "responses":{"204":{"description":"OK"}}
  }}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	addons, err := registry.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := (Generator{}).Generate(document, addons)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	writeTargetArtifacts(t, source, artifacts)
	if err := os.WriteFile(filepath.Join(source, "package.json"), []byte(`{"type":"module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "tsconfig.json"), []byte(serverTSConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	tsc := filepath.Join("..", "..", "..", "test", "typescript", "node_modules", "typescript", "lib", "tsc.js")
	if _, err := os.Stat(tsc); err != nil {
		t.Skipf("TypeScript compiler unavailable for inbound complete multipart test: %v", err)
	}
	if output, err := exec.Command("node", tsc, "--project", filepath.Join(source, "tsconfig.json")).CombinedOutput(); err != nil {
		t.Fatalf("compile generated inbound complete multipart server: %v\n%s", err, output)
	}
	outputDirectory := filepath.Join(directory, "output")
	if err := os.WriteFile(filepath.Join(outputDirectory, "package.json"), []byte(`{"type":"module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `
import { pathToFileURL } from "node:url";
const { createWebhookRouter } = await import(pathToFileURL(process.argv[1]).href);
let seen;
const router = createWebhookRouter({
  bundle: { POST: async ({ body }) => { seen = body; return { status: 204 }; } },
}, { routes: { bundle: "/bundle" } });
const wire =
  "--bundle\r\ncontent-type: application/json\r\nx-first: yes\r\n\r\n{\"event_id\":\"one\"}\r\n" +
  "--bundle\r\ncontent-type: text/plain\r\n\r\nready\r\n" +
  "--bundle--\r\n";
const response = await router.fetch(new Request("https://host.test/bundle", {
  method: "POST",
  headers: { "content-type": "multipart/mixed; boundary=bundle" },
  body: wire,
}));
if (response.status !== 204) throw new Error("complete multipart inbound failed: " + response.status + " " + await response.text());
if (JSON.stringify(seen) !== JSON.stringify([{ event_id: "one" }, "ready"]))
  throw new Error("complete multipart inbound decoded incorrectly: " + JSON.stringify(seen));
`
	if output, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(outputDirectory, "server", "webhooks.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute generated inbound complete multipart server: %v\n%s", err, output)
	}
}
