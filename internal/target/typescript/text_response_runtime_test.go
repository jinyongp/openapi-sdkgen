package typescript

import (
	"os/exec"
	"path/filepath"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedRuntimeDecodesReferencedTextCountResponse(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{"openapi":"3.0.4","info":{"title":"Text count","version":"1"},"paths":{"/users/$count":{"get":{"operationId":"getCount","responses":{"2XX":{"$ref":"#/components/responses/Count"}}}},"/label":{"get":{"operationId":"getLabel","responses":{"200":{"description":"Label","content":{"text/plain":{"schema":{"type":"string"}}}}}}}},"components":{"responses":{"Count":{"description":"Count","content":{"text/plain":{"schema":{"$ref":"#/components/schemas/Count"}}}}},"schemas":{"Count":{"type":"integer","format":"int32","minimum":0}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	output := compileSelectedTypeScriptArtifacts(t, document, generator.Options{}, `import {createClient} from './index.js';
const api=createClient({baseURL:'https://example.test'});
export const count:Promise<number>=api.$operations.getCount();
export const label:Promise<string>=api.$operations.getLabel();
export async function rawCount(){const response=await api.$operations.getCount.raw();const value:number=response.data;return value;}
`)
	script := `import assert from "node:assert/strict";
import {pathToFileURL} from "node:url";
const {createClient} = await import(pathToFileURL(process.argv[1]));
let body = "3";
const api = createClient({baseURL:"https://example.test",fetch:async url => new Response(String(url).endsWith("/label") ? "003" : body,{headers:{"content-type":"text/plain; charset=utf-8"}})});
assert.equal(await api.$routes["GET /users/$count"](),3);
assert.equal((await api.$operations.getCount.raw()).data,3);
assert.equal(await api.$operations.getLabel(),"003");
for (const invalid of ["", "3items", "3.5", "NaN", "-1"]) {
 body = invalid;
 await assert.rejects(api.$operations.getCount(),error => error.code === "RESPONSE_DECODE_FAILED");
}
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("text count runtime: %v\n%s", err, result)
	}
}
