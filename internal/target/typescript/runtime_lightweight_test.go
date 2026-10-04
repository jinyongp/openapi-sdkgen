package typescript

import (
	"os/exec"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestLightweightHTTPContractSelection(t *testing.T) {
	for _, test := range []struct {
		name, operation string
		general, query  bool
	}{
		{"bodyless", `{"responses":{"204":{"description":"ok"}}}`, false, false},
		{"schema query", `{"parameters":[{"in":"query","name":"limit","schema":{"type":"integer"}}],"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object"}}}}}}`, false, true},
		{"content query", `{"parameters":[{"in":"query","name":"filter","content":{"application/json":{"schema":{"type":"object"}}}}],"responses":{"204":{"description":"ok"}}}`, true, true},
		{"declared header", `{"responses":{"200":{"description":"ok","headers":{"X-Limit":{"schema":{"type":"integer"}}},"content":{"application/json":{"schema":{"type":"object"}}}}}}`, true, true},
		{"multiple JSON bodies", `{"requestBody":{"content":{"application/json":{"schema":{"type":"object"}},"application/vnd.item+json":{"schema":{"type":"object"}}}},"responses":{"204":{"description":"ok"}}}`, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := runtimeFeatureFixture(t, test.operation, "", "", false)
			paths := make(map[string]bool)
			for _, artifact := range plan.runtimeArtifacts {
				paths[artifact.source] = true
			}
			if paths["http-services.ts"] != test.general || paths["http-basic.ts"] == test.general {
				t.Fatalf("wrong contract-specific request implementation: general=%t paths=%v", test.general, paths)
			}
			if paths["http-query.ts"] != test.query {
				t.Fatalf("wrong query implementation inclusion: want=%t paths=%v", test.query, paths)
			}
			if paths["http-execution.ts"] {
				t.Fatal("generated request composition reached the full compatibility facade")
			}
		})
	}
}

func TestLightweightHTTPQueryRuntime(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1","info":{"title":"Query","version":"1"},
  "paths":{"/items/{id}":{"get":{"operationId":"getItem","parameters":[
    {"in":"path","name":"id","required":true,"schema":{"type":"string","minLength":1}},
    {"in":"query","name":"limit","required":true,"schema":{"type":"integer","minimum":1}},
    {"in":"query","name":"tag","schema":{"type":"array","items":{"type":"string"}}},
    {"in":"query","name":"filter","style":"deepObject","schema":{"type":"object","properties":{"active":{"type":"boolean"}}}},
    {"in":"query","name":"search","allowReserved":true,"schema":{"type":"string"}}
  ],"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}}}}}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := (Generator{}).Generate(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createClient} from './index.js'; void createClient;`)
	script := `
import assert from 'node:assert/strict'; import {pathToFileURL} from 'node:url';
const {createClient,isAPIError}=await import(pathToFileURL(process.argv[1]+'/index.js'));
let calls=0;
const api=createClient({baseURL:'https://query.test',fetch:async(url,init)=>{
 calls++; const parsed=new URL(url);
 assert.equal(parsed.pathname,'/items/a%2Fb');
 assert.equal(parsed.searchParams.get('limit'),'2');
 assert.deepEqual(parsed.searchParams.getAll('tag'),['a b','c/d']);
 assert.equal(parsed.searchParams.get('filter[active]'),'true');
 assert.equal(parsed.searchParams.get('search'),'a/b?c=d');
 assert.equal(new Headers(init.headers).get('x-extra'),'one');
 return Response.json({id:'one'},{headers:{'x-request-id':'trace'}});
}});
const input={path:{id:'a/b'},query:{limit:2,tag:['a b','c/d'],filter:{active:true},search:'a/b?c=d'}};
assert.deepEqual(await api.$operations.getItem(input,{headers:{'X-Extra':'one'}}),{id:'one'});
const raw=await api.$operations.getItem.raw(input,{headers:{'X-Extra':'one'}});
assert.equal(raw.status,200); assert.equal(raw.request.id,'trace');
assert.deepEqual(Object.keys(raw.headers),[]);
for(const invalid of [{path:{id:'a/b'},query:{}},{path:{id:'a/b'},query:{limit:0}},{path:{id:'.'},query:{limit:2}},{path:{id:'a/b'},query:{limit:2,tag:[undefined]}}]){
 await assert.rejects(()=>api.$operations.getItem(invalid),error=>isAPIError(error)&&error.code==='REQUEST_ENCODE_FAILED');
}
assert.equal(calls,2);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
		t.Fatalf("lightweight query contract: %v\n%s", err, result)
	}
}
