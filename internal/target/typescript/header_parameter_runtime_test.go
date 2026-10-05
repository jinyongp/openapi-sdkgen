package typescript

import (
	"os/exec"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestSchemaHeaderParametersUseBufferedRuntime(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
"openapi":"3.1.1","info":{"title":"Headers","version":"1"},
"paths":{"/headers":{"post":{"operationId":"headers","parameters":[
{"in":"header","name":"X-Required","required":true,"schema":{"type":"string","minLength":2}},
{"in":"header","name":"X-Array","schema":{"type":"array","items":{"type":"integer"}}},
{"in":"header","name":"X-Object","explode":true,"schema":{"type":"object","additionalProperties":{"type":"integer"}}}
],"requestBody":{"content":{"application/json":{"schema":{"type":"object"}}}},
"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"boolean"}}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	options := generator.Options{Clients: map[string]generator.Client{"page": {Selection: &generator.Selection{Operations: []string{"headers"}}}}}
	artifacts, err := (Generator{}).Generate(document, options)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, artifact := range artifacts {
		seen[artifact.Path] = true
	}
	if !seen["internal/runtime/http/request/http-header-schema.ts"] || seen["internal/runtime/http/request/http-services.ts"] || seen["internal/runtime/http/response/http-response-stream-raw.ts"] {
		t.Fatal("schema headers retained general HTTP or sequential response support")
	}
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createClient} from './index.js'; void createClient;`)
	script := `import assert from 'node:assert/strict';import {pathToFileURL} from 'node:url';
const load=file=>import(pathToFileURL(process.argv[1]+'/'+file));
const root=await load('index.js'),named=await load('clients/page/index.js'),selective=await load('selective/index.js');
let calls=0;
const options={baseURL:'https://example.test',fetch:async(url,init)=>{
 assert.equal(String(url),'https://example.test/headers');
 assert.equal(init.headers.get('x-required'),'ok');assert.equal(init.headers.get('x-array'),'1,2');assert.equal(init.headers.get('x-object'),'a=1,b=2');
 assert.equal(init.headers.get('content-type'),'application/json');assert.equal(init.body,'{"value":1}');calls++;return Response.json(true);
}};
const prepared=await selective.loadOperations([selective.operations.headers]);
for(const api of [root.createClient(options),named.createClient(options),selective.createClient({...options,operations:prepared})]){
 assert.equal(await api.$operations.headers({headerParams:{'X-Required':'ok','X-Array':[1,2],'X-Object':{a:1,b:2}},body:{value:1}}),true);
 await assert.rejects(api.$operations.headers({headerParams:{'X-Required':'x'}}));
 await assert.rejects(api.$operations.headers({headerParams:{}}));
 await assert.rejects(api.$operations.headers({headerParams:{'X-Required':'ok','X-Array':[undefined]}}));
 await assert.rejects(api.$operations.headers({headerParams:{'X-Required':'ok'}},{headers:{'X-Required':'bypass'}}));
}assert.equal(calls,3);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
		t.Fatalf("schema header runtime: %v\n%s", err, strings.TrimSpace(string(result)))
	}
}
