package typescript

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestOpenAPI32EncodingIgnoresEntriesWithoutInstanceValues(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.2.0",
  "info":{"title":"Encoding applicability","version":"1"},
  "paths":{
    "/named":{"post":{
      "operationId":"namedEncoding",
      "requestBody":{"required":true,"content":{"multipart/form-data":{
        "schema":{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}},
        "encoding":{
          "name":{"contentType":"text/plain"},
          "unused":{"contentType":"application/vnd.missing","headers":{"X-Required":{"required":true,"schema":{"type":"string"}}}}
        }
      }}},
      "responses":{"204":{"description":"OK"}}
    }},
    "/positional":{"post":{
      "operationId":"positionalEncoding",
      "requestBody":{"required":true,"content":{"multipart/mixed":{
        "schema":{"type":"array","items":{"type":"string"}},
        "prefixEncoding":[
          {"contentType":"text/plain"},
          {"contentType":"application/vnd.missing"}
        ]
      }}},
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if metadata := string(document.SourceMetadataJSON); !strings.Contains(metadata, `"unused"`) || !strings.Contains(metadata, `"application/vnd.missing"`) {
		t.Fatalf("source metadata lost ignored encoding entries: %s", metadata)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
const seen = [];
const api = createClient({
  baseURL: "https://api.example.test",
  fetch: async (input, init) => {
    const body = init.body === null || init.body === undefined ? "" : typeof init.body === "string" ? init.body : await new Response(init.body).text();
    seen.push([new URL(String(input)).pathname, body]);
    if (body.includes("X-Required") || body.includes("application/vnd.missing"))
      throw new Error("unused encoding entry affected request: " + body);
    return new Response(null, { status: 204 });
  },
});
await api.$operations.namedEncoding({ body: { name: "one" } });
await api.$operations.positionalEncoding({ body: ["one"] });
if (seen.length !== 2) throw new Error("unexpected request count");
if (!seen[0][1].includes("one") || !seen[1][1].includes("one"))
  throw new Error("active encoding values were not emitted: " + JSON.stringify(seen));
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("execute Encoding applicability runtime test: %v\n%s", err, result)
	}
}
