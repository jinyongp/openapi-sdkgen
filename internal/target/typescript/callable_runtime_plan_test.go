package typescript

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestCallableInputContractSelectionRegression(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"input binding","version":"1"},"paths":{
"/plain":{"get":{"operationId":"plain","responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}}}}},
"/required/{id}":{"get":{"operationId":"required","parameters":[{"in":"path","name":"id","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}}}}},
"/optional":{"get":{"operationId":"optional","parameters":[{"in":"query","name":"id","schema":{"type":"string"}}],"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}}}}}
}}`))
	if err != nil {
		t.Fatal(err)
	}
	policies := map[string]string{"plain": "operation-bind-none.ts", "required": "operation-bind-input.ts", "optional": "operation-bind-optional.ts"}
	for _, name := range []string{"plain", "required", "optional"} {
		artifacts, err := (Generator{}).Generate(document, generator.Options{Selection: &generator.Selection{Operations: []string{name}}})
		if err != nil {
			t.Fatal(err)
		}
		found := make(map[string]bool)
		for _, artifact := range artifacts {
			found[filepath.Base(artifact.Path)] = true
			if strings.HasPrefix(artifact.Path, "internal/operations/") && !strings.Contains(string(artifact.Data), strings.TrimSuffix(policies[name], ".ts")+".js") {
				t.Fatalf("%s did not reference its prepared binder", artifact.Path)
			}
		}
		for other, policy := range policies {
			if found[policy] != (other == name) {
				t.Fatalf("selection %s included %s=%t", name, policy, found[policy])
			}
		}
		if found["callables.ts"] || found["stream-binding.ts"] {
			t.Fatal("buffered selection generated a generic or streaming binding facade")
		}
	}
	options := generator.Options{Selection: &generator.Selection{Operations: []string{"plain"}}, Clients: map[string]generator.Client{
		"plain":    {Selection: &generator.Selection{Operations: []string{"plain"}}},
		"required": {Selection: &generator.Selection{Operations: []string{"required"}}},
		"optional": {Selection: &generator.Selection{Operations: []string{"optional"}}},
	}}
	artifacts, err := (Generator{}).Generate(document, options)
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createClient} from './index.js'; void createClient;`)
	graph, err := filepath.Abs("../../../test/typescript/verification/runtime-feature-imports.mjs")
	if err != nil {
		t.Fatal(err)
	}
	script := `import assert from 'node:assert/strict';import path from 'node:path';import {pathToFileURL} from 'node:url';
const root=process.argv[1];const {generatedModuleGraph}=await import(pathToFileURL(process.argv[2]));
for(const name of ['plain','required','optional']){
 const entry='clients/'+name+'/index.js';const graph=new Set(generatedModuleGraph(root,entry,{allowGenericLoader:true}).map(file=>path.basename(file)));
 const policy={plain:'none',required:'input',optional:'optional'}[name];
 for(const candidate of ['none','input','optional'])assert.equal(graph.has('operation-bind-'+candidate+'.js'),candidate===policy,name+' '+candidate);
 assert.equal(graph.has('callables.js'),false);assert.equal(graph.has('stream-binding.js'),false);
 const {createClient}=await import(pathToFileURL(path.join(root,entry)));let requests=[];let invalid=false;
 const api=createClient({baseURL:'https://binding.test',fetch:async(url,init)=>{requests.push({url:new URL(url),headers:new Headers(init.headers)});return Response.json({id:invalid?42:'one'});}});
 const call=api.$operations[name];const options={headers:{'x-binding':'options'}};
 assert.equal(call.name,'');assert.equal(call.raw.name,'');assert.equal(call.length,name==='required'?1:0);assert.equal(call.raw.length,call.length);
 if(name==='plain'){assert.deepEqual(await call(options),{id:'one'});assert.equal(requests.at(-1).headers.get('x-binding'),'options');assert.equal((await call.raw(options)).response.status,200);}
 if(name==='required'){assert.deepEqual(await call({path:{id:'one'}},options),{id:'one'});assert.equal(requests.at(-1).url.pathname,'/required/one');assert.equal(requests.at(-1).headers.get('x-binding'),'options');assert.deepEqual(await api.required('one').get(options),{id:'one'});}
 if(name==='optional'){
  for(const args of [[],[options],[{query:{id:'two'}}],[undefined,options],[{query:{id:'two'}},options]]){
   assert.deepEqual(await call(...args),{id:'one'});const request=requests.at(-1);assert.equal(request.url.searchParams.get('id'),args[0]?.query?.id??null);assert.equal(request.headers.get('x-binding'),(args[0]===options||args[1]===options)?'options':null);
   assert.deepEqual((await call.raw(...args)).data,{id:'one'});
  }
 }
 invalid=true;await assert.rejects(()=>name==='required'?call({path:{id:'one'}}):call(),error=>error.code==='RESPONSE_DECODE_FAILED');
}
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output, graph).CombinedOutput(); err != nil {
		t.Fatalf("callable input contract regression: %v\n%s", err, result)
	}
}
