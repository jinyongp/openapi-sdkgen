package typescript

import (
	"os/exec"
	"path/filepath"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestWireContractXMLBranchRegression(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1", "info":{"title":"XML branches","version":"1"},
  "components":{"schemas":{
    "Count":{"type":"integer","minimum":2},
    "Payload":{"type":"object","xml":{"name":"payload"},"oneOf":[
      {"required":["kind","wire_count"],"properties":{
        "kind":{"const":"count"},"wire_count":{"$ref":"#/components/schemas/Count","xml":{"name":"value"}}
      }},
      {"required":["kind","wire_label"],"properties":{
        "kind":{"const":"label"},"wire_label":{"type":"string","xml":{"name":"value"}}
      }}
    ]}
  }},
  "paths":{"/payload":{"post":{"operationId":"savePayload",
    "requestBody":{"required":true,"content":{"application/xml":{"schema":{"$ref":"#/components/schemas/Payload"}}}},
    "responses":{"200":{"description":"OK","content":{"application/xml":{"schema":{"$ref":"#/components/schemas/Payload"}}}}}
  }}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
let payload = '<payload><kind>label</kind><value>hello</value></payload>';
let requests = 0;
const api = createClient({ baseURL: "https://api.example.test", fetch: async (_input, init) => {
  requests++;
  if (init.body !== '<payload><kind>count</kind><value>2</value></payload>') throw new Error("XML request branch lost: " + init.body);
  return new Response(payload, { headers: { "content-type": "application/xml" } });
} });
const value = await api.$operations.savePayload({ body: { kind: "count", wire_count: 2 } });
if (value.kind !== "label" || value.wire_label !== "hello") throw new Error("XML response branch lost: " + JSON.stringify(value));
try { await api.$operations.savePayload({ body: { kind: "count", wire_count: 1 } }); throw new Error("invalid request accepted"); }
catch (error) { if (error.message === "invalid request accepted") throw error; }
if (requests !== 1) throw new Error("invalid request reached fetch");
payload = '<payload><kind>count</kind><value>1</value></payload>';
try { await api.$operations.savePayload({ body: { kind: "count", wire_count: 2 } }); throw new Error("invalid branch accepted"); }
catch (error) { if (error.message === "invalid branch accepted") throw error; }
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute generated XML branches: %v\n%s", err, result)
	}
}
