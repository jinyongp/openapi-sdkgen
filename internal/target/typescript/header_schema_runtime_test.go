package typescript

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestWireContractResponseHeaderRegression(t *testing.T) {
	tests := []struct {
		name, schema, valid, invalid, expected string
		explode                                bool
	}{
		{"allOf integer", `{"allOf":[{"$ref":"#/components/schemas/Count"},{"minimum":2}]}`, "2", "1", `2`, false},
		{"union string", `{"oneOf":[{"type":"integer"},{"type":"string","pattern":"^[a-z]+$"}]}`, "hello", "1.5", `"hello"`, false},
		{"union numeric", `{"anyOf":[{"type":"integer"},{"type":"string"}]}`, "2", "", `2`, false},
		{"ref allOf array", `{"allOf":[{"$ref":"#/components/schemas/Counts"},{"minItems":2}]}`, "2,3", "2,no", `[2,3]`, false},
		{"joined object", `{"allOf":[{"$ref":"#/components/schemas/Metadata"},{"properties":{"enabled":{"type":"boolean"}}}],"required":["enabled"]}`, "count,2,enabled,true", "count,2,enabled,invalid", `{"count":2,"enabled":true}`, false},
		{"direct pattern additional", `{"type":"object","properties":{"count":{"type":"integer"}},"patternProperties":{"^count$":{"minimum":2},"^flag":{"type":"boolean"}},"additionalProperties":{"type":"number"}}`, "count=2,flagged=true,extra=1.5", "count=1,flagged=true", `{"count":2,"flagged":true,"extra":1.5}`, true},
		{"correlated branches", `{"type":"object","oneOf":[{"properties":{"x":{"type":"integer"},"y":{"const":"2"}},"required":["x","y"]},{"properties":{"x":{"const":"2"},"y":{"type":"integer"}},"required":["x","y"]}]}`, "x=2,y=2", "x=oops,y=oops", `{"x":2,"y":"2"}`, true},
		{"conditional", `{"type":"object","properties":{"flag":{"type":"boolean"}},"if":{"properties":{"flag":{"const":true}},"required":["flag"]},"then":{"properties":{"count":{"type":"integer"}},"required":["count"]},"else":{"properties":{"label":{"type":"string"}},"required":["label"]}}`, "flag=true,count=2", "flag=true,count=no", `{"flag":true,"count":2}`, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// The numeric/string union accepts every text; its negative control is
			// the required-header absence rather than an impossible invalid string.
			invalid := test.invalid
			if test.name == "union numeric" {
				invalid = "<missing>"
			}
			header := fmt.Sprintf(`{"required":true,"explode":%t,"schema":%s}`, test.explode, test.schema)
			document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.2.0","info":{"title":"Header schema","version":"1"},
  "components":{"schemas":{"Count":{"type":"integer"},"Counts":{"type":"array","items":{"$ref":"#/components/schemas/Count"}},"Metadata":{"type":"object","required":["count"],"properties":{"count":{"$ref":"#/components/schemas/Count"}}}}},
  "paths":{
    "/headers":{"get":{"operationId":"headers","responses":{"200":{"description":"ok","headers":{"X-Value":` + header + `},"content":{"application/json":{"schema":{"type":"object"}}}}}}},
    "/upload":{"post":{"operationId":"upload","requestBody":{"content":{"multipart/form-data":{"schema":{"type":"object","properties":{"payload":{"type":"string"}}},"encoding":{"payload":{"contentType":"text/plain","headers":{"X-Value":` + header + `}}}}}},"responses":{"204":{"description":"ok"}}}},
    "/parts":{"get":{"operationId":"parts","responses":{"200":{"description":"ok","content":{"multipart/mixed":{"schema":{"type":"array","items":{"type":"string"}},"itemEncoding":{"contentType":"text/plain","headers":{"X-Value":` + header + `}}}}}}}}
  }
}`))
			if err != nil {
				t.Fatal(err)
			}
			output := compileTypeScriptArtifacts(t, document)
			script := fmt.Sprintf(`
import { pathToFileURL } from "node:url";
import { deepStrictEqual } from "node:assert";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
const valid = %q, invalid = %q, expected = %s;
let header = valid, requests = 0;
const api = createClient({ baseURL: "https://api.example.test", fetch: async input => {
  requests++;
  const path = new URL(input).pathname;
  if (path === "/upload") return new Response(null, { status: 204 });
  const supplied = header === "<missing>" ? {} : { "X-Value": header };
  if (path === "/headers") return Response.json({}, { headers: supplied });
  return new Response("--part\r\nContent-Type: text/plain\r\n" + (header === "<missing>" ? "" : "X-Value: " + header + "\r\n") + "\r\nbody\r\n--part--\r\n", { headers: { "content-type": "multipart/mixed; boundary=part" } });
} });
const raw = await api.$operations.headers.raw();
deepStrictEqual(JSON.parse(JSON.stringify(Object.values(raw.headers)[0])), expected);
deepStrictEqual(await api.$operations.parts(), ["body"]);
await api.$operations.upload({ body: { payload: "body" } }, { multipartHeaders: { payload: { "X-Value": valid } } });
for (header of [invalid, "<missing>"]) {
  for (const operation of [api.$operations.headers.raw, api.$operations.parts]) {
    try { await operation(); throw new Error("invalid response header accepted"); }
    catch (error) { if (error.code !== "RESPONSE_DECODE_FAILED") throw error; }
  }
  const before = requests;
  try { await api.$operations.upload({ body: { payload: "body" } }, { multipartHeaders: { payload: header === "<missing>" ? {} : { "X-Value": header } } }); throw new Error("invalid request header accepted"); }
  catch (error) { if (error.code !== "REQUEST_ENCODE_FAILED") throw error; }
  if (requests !== before) throw new Error("invalid request header reached fetch");
}
`, test.valid, invalid, test.expected)
			if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
				t.Fatalf("execute generated header contract: %v\n%s", err, result)
			}
		})
	}
}
