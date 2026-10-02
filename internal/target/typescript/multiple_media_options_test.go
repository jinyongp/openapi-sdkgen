package typescript

import (
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestMultipleResponseMediaOptionsStrictTypes(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{"openapi":"3.1.1","info":{"title":"Media options","version":"1"},"paths":{"/files/{id}":{"get":{"operationId":"getFile","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"type":"string"}},"text/plain":{"schema":{"type":"string"}}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := (Generator{}).Generate(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	probe := `import { createClient } from "./index.js";
import { createClient as createSelected, loadOperations, operations } from "./selective/index.js";
const api = createClient({baseURL:"https://media.test"});
void api.files("one").get({accept:undefined});
void api.files("one").get.raw({accept:"text/plain"});
void api.$operations.getFile({path:{id:"one"}}, {accept:undefined});
const selected = createSelected({operations:await loadOperations([operations.getFile])});
void selected.files("one").get({accept:"application/json"});
// @ts-expect-error Unsupported media types remain rejected.
void api.files("one").get({accept:"application/xml"});
// @ts-expect-error The required path argument retains its string type.
void api.files(123).get();
`
	compileTypeScriptArtifactSet(t, artifacts, "media-options.ts", probe)
}
