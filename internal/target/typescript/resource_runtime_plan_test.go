package typescript

import (
	"os/exec"
	"path/filepath"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestResourceBindingContractSelectionRegression(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"resource binding","version":"1"},"paths":{
"/none/{id}":{"get":{"operationId":"none","parameters":[{"in":"path","name":"id","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}},"400":{"description":"bad","content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}}}}},
"/required/{id}":{"get":{"operationId":"required","parameters":[{"in":"path","name":"id","required":true,"schema":{"type":"string"}},{"in":"query","name":"q","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}}}}},
"/optional/{id}":{"get":{"operationId":"optional","parameters":[{"in":"path","name":"id","required":true,"schema":{"type":"string"}},{"in":"query","name":"q","schema":{"type":"string"}}],"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}}}}}
}}`))
	if err != nil {
		t.Fatal(err)
	}
	options := generator.Options{Clients: map[string]generator.Client{
		"none":     {Selection: &generator.Selection{Operations: []string{"none"}}},
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
for(const name of ['none','required','optional']){
 const entry='index.js';const graph=new Set(generatedModuleGraph(root,'internal/resources/'+name+'/by-id.js',{allowGenericLoader:true}).map(file=>path.basename(file)));
 const policy={none:'none',required:'input',optional:'optional'}[name];
 for(const candidate of ['none','input','optional'])assert.equal(graph.has('resource-bind-'+candidate+'.js'),candidate===policy,name+' '+candidate);
 for(const excluded of ['resource-binding.js','resource-stream-binding.js','resource-helper-binding.js'])assert.equal(graph.has(excluded),false,name+' '+excluded);
 assert.equal(graph.has('callable-arguments.js'),name==='optional',name+' optional input helper');
 const {createClient,isOperationHTTPError}=await import(pathToFileURL(path.join(root,entry)));let requests=[];let status=200;
 const clientOptions={baseURL:'https://binding.test',fetch:async(url,init)=>{requests.push({url:new URL(url),headers:new Headers(init.headers)});return Response.json({id:'one'},{status});}};
 const api=createClient(clientOptions);
 const call=api[name]('bound/id').get;const options={headers:{'x-binding':'options'}};
 assert.equal(call.name,'');assert.equal(call.raw.name,'');assert.equal(call.length,name==='required'?1:0);assert.equal(call.raw.length,call.length);
 for(const helper of ['links','paginate','stream'])assert.equal(Object.hasOwn(call,helper),false,name+' '+helper);
 const cases=name==='none'?[[options]]:name==='required'?[[{query:{q:'two'}},options]]:[[],[options],[{query:{q:'two'}}],[undefined,options],[{query:{q:'two'}},options]];
 for(const args of cases){
  assert.deepEqual(await call(...args),{id:'one'});const request=requests.at(-1);assert.equal(request.url.pathname,'/'+name+'/bound%2Fid');
  assert.equal(request.url.searchParams.get('q'),args[0]?.query?.q??null);assert.equal(request.headers.get('x-binding'),(args[0]===options||args[1]===options)?'options':null);
  const raw=await call.raw(...args);assert.deepEqual(raw.data,{id:'one'});assert.equal(raw.status,200);assert.equal(raw.response.status,200);
 }
 const named=await import(pathToFileURL(path.join(root,'clients/'+name+'/index.js')));const generic=named.createClient(clientOptions)[name]('bound/id').get;
 const args=name==='required'?[{query:{q:'two'}},options]:[options];assert.deepEqual(await generic(...args),{id:'one'});assert.deepEqual((await generic.raw(...args)).data,{id:'one'});
 assert.equal(generic.name,call.name);assert.equal(generic.length,call.length);assert.equal(generic.raw.name,call.raw.name);assert.equal(generic.raw.length,call.raw.length);
 if(name==='required')await assert.rejects(()=>call(),error=>error.code==='REQUEST_ENCODE_FAILED');
 if(name==='none'){
  status=400;let failure;try{await call()}catch(error){failure=error}assert(failure);assert.equal(isOperationHTTPError(failure,call),true);assert.equal(isOperationHTTPError(failure,call.raw),true);
  status=200;
  const {createGeneratedSelectedClient,createSelectedClient}=await import(pathToFileURL(path.join(root,'internal/runtime/client/selected-client.js')));
  const {provider}=await import(pathToFileURL(path.join(root,'internal/executions/none/by-id/get.js')));
  const legacy={...provider,resources:provider.resources.map(({bindResource,...placement})=>placement)};
  const resolve=()=>Promise.reject(new Error('unexpected private lookup'));
  assert.throws(()=>createGeneratedSelectedClient(clientOptions,[legacy],resolve).none('missing'),error=>error.stage==='BINDING');
  assert.deepEqual(await createSelectedClient(clientOptions,[legacy],resolve).none('legacy').get(),{id:'one'});
 }
}
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output, graph).CombinedOutput(); err != nil {
		t.Fatalf("resource binding regression: %v\n%s", err, result)
	}
}
