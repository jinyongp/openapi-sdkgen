package typescript

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestMediaRootIgnoresContradictoryContentMediaTypeWithoutWeakeningNestedSchemas(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Media context","version":"1"},
  "components":{"schemas":{
    "EncodedPayload":{
      "type":"string",
      "contentMediaType":"application/json",
      "contentSchema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}
    }
  }},
  "paths":{
    "/inline":{"post":{
      "operationId":"sendInline",
      "requestBody":{"required":true,"content":{"text/plain":{"schema":{
        "type":"string",
        "contentMediaType":"application/json",
        "contentSchema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}
      }}}},
      "responses":{"204":{"description":"OK"}}
    }},
    "/referenced":{"get":{
      "operationId":"getReferenced",
      "responses":{"200":{"description":"OK","content":{"text/plain":{"schema":{"$ref":"#/components/schemas/EncodedPayload"}}}}}
    }},
    "/nested":{"post":{
      "operationId":"sendNested",
      "requestBody":{"required":true,"content":{"application/json":{"schema":{
        "type":"object",
        "required":["payload"],
        "properties":{"payload":{"$ref":"#/components/schemas/EncodedPayload"}}
      }}}},
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if metadata := string(document.SourceMetadataJSON); !strings.Contains(metadata, `"contentMediaType":"application/json"`) {
		t.Fatalf("source metadata lost contextual contentMediaType: %s", metadata)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
const seen = [];
const api = createClient({
  baseURL: "https://api.example.test",
  fetch: async (input, init) => {
    const path = new URL(String(input)).pathname;
    seen.push(path);
    if (path === "/inline") {
      if (String(init.body) !== "not-json") throw new Error("text request changed");
      return new Response(null, { status: 204 });
    }
    if (path === "/referenced")
      return new Response("not-json", { status: 200, headers: { "content-type": "text/plain" } });
    throw new Error("unexpected fetch: " + path);
  },
});
await api.$operations.sendInline({ body: "not-json" });
if (await api.$operations.getReferenced() !== "not-json")
  throw new Error("referenced text response changed");
const before = seen.length;
let rejected = false;
try { await api.$operations.sendNested({ body: { payload: "not-json" } }); } catch { rejected = true; }
if (!rejected || seen.length !== before)
  throw new Error("nested contentMediaType/contentSchema semantics were weakened by media-root compatibility");
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute media-root compatibility runtime test: %v\n%s", err, result)
	}
}

func TestLegacyXMLWireSemanticsRemainStableAcrossOpenAPI30And31(t *testing.T) {
	for _, version := range []string{"3.0.3", "3.1.1"} {
		t.Run(version, func(t *testing.T) {
			document, err := sdkgen.Compile([]byte(`{
  "openapi":"` + version + `",
  "info":{"title":"Legacy XML","version":"1"},
  "paths":{"/payload":{"post":{
    "operationId":"savePayload",
    "requestBody":{"required":true,"content":{"application/xml":{"schema":{
      "type":"object","xml":{"name":"payload"},"required":["id","name"],
      "properties":{
        "id":{"type":"integer","xml":{"name":"id","attribute":true}},
        "name":{"type":"string","xml":{"name":"name"}}
      }
    }}}},
    "responses":{"200":{"description":"OK","content":{"application/xml":{"schema":{
      "type":"object","xml":{"name":"payload"},
      "properties":{
        "id":{"type":"integer","xml":{"name":"id","attribute":true}},
        "name":{"type":"string","xml":{"name":"name"}}
      }
    }}}}}
  }}}
}`))
			if err != nil {
				t.Fatal(err)
			}
			output := compileTypeScriptArtifacts(t, document)
			script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
const api = createClient({ baseURL: "https://api.example.test", fetch: async (_input, init) => {
  if (String(init.body) !== '<payload id="7"><name>Milo</name></payload>')
    throw new Error("legacy XML request mismatch: " + init.body);
  return new Response('<payload id="8"><name>Rex</name></payload>', { status: 200, headers: { "content-type": "application/xml" } });
} });
const value = await api.$operations.savePayload({ body: { id: 7, name: "Milo" } });
if (value.id !== 8 || value.name !== "Rex") throw new Error("legacy XML response mismatch");
`
			if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
				t.Fatalf("execute %s legacy XML runtime test: %v\n%s", version, err, result)
			}
		})
	}
}
