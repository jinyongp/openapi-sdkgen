package typescript

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestWireContractMultipartEncodingRegression(t *testing.T) {
	const named = `{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","properties":{
      "payload":{},"xml":{"$ref":"#/components/schemas/XML"},"custom":{"type":"object"},
      "file":{"contentMediaType":"application/octet-stream"},"tags":{"type":"array","items":{"type":"string"}}
    }},"encoding":{
      "payload":{"contentType":"application/json; charset=utf-8"},
      "xml":{"contentType":"application/xml; charset=utf-8"},
      "custom":{"contentType":"application/x-custom; version=1"},
      "tags":{"contentType":"application/json","explode":true}
    }}}}`
	optional := strings.Replace(named, `"payload":{"contentType":"application/json; charset=utf-8"}`, `"payload":{"contentType":"application/json; charset=utf-8","headers":{"X-Check":{"schema":{"type":"integer"}}}}`, 1)
	required := strings.Replace(optional, `"X-Check":{"schema"`, `"X-Check":{"required":true,"schema"`, 1)
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.2.0", "info":{"title":"Multipart bytes","version":"1"},
  "components":{"schemas":{"XML":{"type":"object","xml":{"name":"payload"},"properties":{"count":{"type":"integer"}}}}},
  "paths":{
    "/plain":{"post":{"operationId":"plain","requestBody":` + named + `,"responses":{"204":{"description":"ok"}}}},
    "/optional":{"post":{"operationId":"optional","requestBody":` + optional + `,"responses":{"204":{"description":"ok"}}}},
    "/required":{"post":{"operationId":"required","requestBody":` + required + `,"responses":{"204":{"description":"ok"}}}},
    "/ordered":{"post":{"operationId":"ordered","requestBody":{"required":true,"content":{"multipart/mixed":{
      "schema":{"type":"array","prefixItems":[{}, {"$ref":"#/components/schemas/XML"}, {"type":"object"}]},
      "prefixEncoding":[{"contentType":"application/json; charset=utf-8"},{"contentType":"application/xml"},{"contentType":"application/x-custom; version=1"}]
    }}},"responses":{"204":{"description":"ok"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
let calls = 0, codecCalls = 0, blobCodec = false, expected;
const partText = async value => typeof value === "string" ? value : value.text();
const api = createClient({ baseURL: "https://api.example.test", codecs: {
  "application/x-custom": { encode: async (_value, context) => {
    codecCalls++; if (context.contentType !== "application/x-custom; version=1") throw new Error("codec parameters lost");
    return blobCodec ? new Blob(["id=1"]) : new URLSearchParams({ id: "1" });
  } }
}, fetch: async (input, init) => {
  calls++;
  const request = new Request(input, init);
  if (new URL(request.url).pathname === "/ordered") {
    const text = await request.text();
    for (const value of [JSON.stringify(expected), "<payload><count>2</count></payload>", "id=1"])
      if (!text.includes("\r\n\r\n" + value + "\r\n")) throw new Error("positional bytes changed: " + text);
  } else {
    const form = await request.formData();
    const json = await partText(form.get("payload"));
    if (json !== JSON.stringify(expected) || JSON.stringify(JSON.parse(json)) !== JSON.stringify(expected)) throw new Error("JSON payload changed on " + new URL(request.url).pathname + ": " + json);
    if (await partText(form.get("xml")) !== "<payload><count>2</count></payload>") throw new Error("XML payload changed");
    if (await partText(form.get("custom")) !== "id=1") throw new Error("custom payload changed");
    if (form.get("file").name !== "raw.bin" || (await form.get("file").arrayBuffer()).byteLength !== 4) throw new Error("File contract changed");
    if ((await Promise.all(form.getAll("tags").map(partText))).join(",") !== 'a,b') throw new Error("exploded form strings changed");
  }
  return new Response(null, { status: 204 });
} });
for (expected of ["hello", 3, true, null, { id: 1 }, [1, 2]]) {
  const body = { payload: expected, xml: { count: 2 }, custom: { id: 1 }, file: new File([new Uint8Array([0,1,127,255])], "raw.bin"), tags: ["a","b"] };
  await api.$operations.plain({ body });
  await api.$operations.plain({ body }, { multipartHeaders: { payload: {} } });
  await api.$operations.optional({ body });
  await api.$operations.optional({ body }, { multipartHeaders: { payload: { "X-Check": "2" } } });
  await api.$operations.ordered({ body: [expected, { count: 2 }, { id: 1 }] });
}
if (calls !== 30 || codecCalls !== 30) throw new Error("codec was skipped or duplicated");
expected = "blob"; blobCodec = true;
await api.$operations.plain({ body: { payload: expected, xml: { count: 2 }, custom: {}, file: new File(["data"], "raw.bin"), tags: ["a","b"] } });
const before = calls;
try { await api.$operations.required({ body: { payload: "bad" } }); throw new Error("required header accepted"); }
catch (error) { if (error.message === "required header accepted") throw error; }
if (calls !== before) throw new Error("required header reached fetch");
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute generated multipart byte contracts: %v\n%s", err, result)
	}
}
