package typescript

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func repeatedCompositionFixture(t *testing.T) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(runtimeEmissionFixture), &document); err != nil {
		t.Fatal(err)
	}
	delete(document, "webhooks")
	paths := document["paths"].(map[string]any)
	for _, kind := range []string{"json", "xml"} {
		data, err := json.Marshal(paths["/"+kind])
		if err != nil {
			t.Fatal(err)
		}
		var copy map[string]any
		if err := json.Unmarshal(data, &copy); err != nil {
			t.Fatal(err)
		}
		copy["get"].(map[string]any)["operationId"] = kind + "Copy"
		paths["/"+kind+"-copy"] = copy
	}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestExecutionCompositionSelectionAndFrozenEmission(t *testing.T) {
	document, err := sdkgen.Compile(repeatedCompositionFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	target := Generator{}
	var jsonPath string
	for _, selected := range [][]string{nil, {"json", "jsonCopy"}, {"json"}} {
		options := generator.Options{}
		if selected != nil {
			options.Selection = &generator.Selection{Operations: selected}
		}
		prepared, diagnostics, err := target.Prepare(document, options)
		if err != nil || len(diagnostics) > 0 {
			t.Fatalf("prepare: %v %v", err, diagnostics)
		}
		value, _ := prepared.Value("typescript")
		plan := value.(*sourcePlan)
		want := 2
		if selected != nil {
			want = len(selected) - 1
		}
		if len(plan.executionCompositions) != want {
			t.Fatalf("selection %v: composition groups %d, want %d", selected, len(plan.executionCompositions), want)
		}
		for _, route := range []string{"GET /json", "GET /json-copy"} {
			if execution, exists := plan.executions[route]; exists && want > 0 {
				if jsonPath == "" {
					jsonPath = execution.composition.sharedPath
				}
				if execution.composition.sharedPath != jsonPath {
					t.Fatal("selection or route identity changed the JSON composition path")
				}
			}
		}
		before := fmt.Sprintf("%#v", plan.executionCompositions)
		artifacts, err := target.Emit(prepared)
		if err != nil {
			t.Fatal(err)
		}
		var streamed []Artifact
		if err := target.EmitTo(prepared, generator.ArtifactSinkFunc(func(artifact Artifact) error {
			streamed = append(streamed, artifact)
			return nil
		})); err != nil {
			t.Fatal(err)
		}
		emitted, direct := make(map[string]string), make(map[string]string)
		for _, artifact := range artifacts {
			emitted[artifact.Path] = string(artifact.Data)
		}
		for _, artifact := range streamed {
			direct[artifact.Path] = string(artifact.Data)
		}
		if before != fmt.Sprintf("%#v", plan.executionCompositions) || !reflect.DeepEqual(emitted, direct) || len(artifacts) != len(streamed) {
			t.Fatal("Emit/EmitTo changed prepared composition modules or artifact output")
		}
		for _, artifact := range artifacts {
			if strings.HasPrefix(artifact.Path, "internal/execution-compositions/") && !strings.Contains(string(artifact.Data), "export function createExecutionServices()") {
				t.Fatal("shared module did not export an independent service factory")
			}
		}
	}
}

func TestExecutionCompositionKeepsContractIdentitiesSeparate(t *testing.T) {
	base := []runtimeFeature{"schema.object", "response.media.json"}
	identity := executionCompositionIdentity(base, false)
	if executionCompositionIdentity([]runtimeFeature{base[1], base[0]}, false) != identity {
		t.Fatal("feature order changed a semantic identity")
	}
	if executionCompositionIdentity(base, true) == identity {
		t.Fatal("stream capability was merged into buffered execution")
	}
	for _, fact := range []runtimeFeature{"http.general", "http.path.general", "http.parameter.sort", "http.server.variables", "http.query", "security.http.bearer", "security.multiple-requirements", "request.media.xml", "response.media.multipart", "schema.dynamic", "response.framing.incremental.sse"} {
		if executionCompositionIdentity(append(append([]runtimeFeature(nil), base...), fact), false) == identity {
			t.Fatalf("different contract was merged: %s", fact)
		}
	}
}

func TestExecutionCompositionIndependentInstancesAndProviderContexts(t *testing.T) {
	document, err := sdkgen.Compile(repeatedCompositionFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	plan, _, _, err := prepareClientSourcePlanWithCoverage(document, generator.Options{}, "")
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := emitSourcePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createClient} from './index.js'; void createClient;`)
	paths := make(map[string]string)
	for _, module := range plan.modules.operations {
		paths[module.routeKey] = strings.TrimSuffix(operationExecutionArtifactPath(module), ".ts") + ".js"
	}
	composition := strings.TrimSuffix(plan.executions["GET /json"].composition.sharedPath, ".ts") + ".js"
	graph, err := filepath.Abs("../../../test/typescript/verification/runtime-feature-imports.mjs")
	if err != nil {
		t.Fatal(err)
	}
	script := `import assert from 'node:assert/strict'; import path from 'node:path'; import {pathToFileURL} from 'node:url';
const root=process.argv[1];const load=file=>import(pathToFileURL(path.join(root,file)));
const {createExecutionServices}=await load(process.argv[2]);
const one=createExecutionServices(),two=createExecutionServices();assert.notEqual(one,two);
for(const key of ['encodeRequest','decodeResponse','decodeResponseWireValue'])assert.notEqual(one[key],two[key],key);
const {provider:a}=await load(process.argv[3]);const {provider:b}=await load(process.argv[4]);
assert.equal(a.abi,1);assert.equal(a.generation,b.generation);
const {generatedModuleGraph}=await import(pathToFileURL(process.argv[5]));
const jsonGraph=new Set(generatedModuleGraph(root,process.argv[3]).map(file=>path.basename(file)));
assert.equal(jsonGraph.has('xml-codec.js'),false);
const {createRequestContext}=await load('internal/runtime/http/http-execution-support.js');
let fetched=0;const observed=[];const dto={id:'ok'};
const fetch=async(url,init)=>{fetched++;observed.push([new URL(url).host,new Headers(init.headers).get('authorization')]);return Response.json(dto)};
const first=a.bind(createRequestContext({baseURL:'https://first.test',authorization:'Bearer first',fetch}));
const second=b.bind(createRequestContext({baseURL:'https://second.test',authorization:'Bearer second',fetch}));
assert.deepEqual(await Promise.all([first(),second()]),[dto,dto]);
assert.deepEqual(observed.sort(),[['first.test','Bearer first'],['second.test','Bearer second']]);
await assert.rejects(first({signal:AbortSignal.abort()}),error=>error.code==='REQUEST_ABORTED');
assert.deepEqual(await second(),dto);assert.equal(fetched,3);
dto.id=42;await assert.rejects(first(),error=>error.code==='RESPONSE_DECODE_FAILED');
dto.id='valid-again';assert.deepEqual(await second(),dto);
const {provider:x}=await load(process.argv[6]);const {provider:y}=await load(process.argv[7]);
const xmlOne=x.bind(createRequestContext({baseURL:'https://xml.test',fetch:async()=>new Response('<item><id>one</id></item>',{headers:{'content-type':'application/xml'}})}));
const xmlTwo=y.bind(createRequestContext({baseURL:'https://xml.test',fetch:async()=>new Response('<item><id>two</id></item>',{headers:{'content-type':'application/xml'}})}));
assert.deepEqual(await Promise.all([xmlOne(),xmlTwo()]),[{id:'one'},{id:'two'}]);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output, composition, paths["GET /json"], paths["GET /json-copy"], graph, paths["GET /xml"], paths["GET /xml-copy"]).CombinedOutput(); err != nil {
		t.Fatalf("shared composition isolation: %v\n%s", err, result)
	}
}

func TestExecutionCompositionMutableInputCredentialsCodecsAndConcurrentAbort(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal([]byte(`{"openapi":"3.1.0","info":{"title":"State isolation","version":"1"},"security":[{"Bearer":[]}],"components":{"securitySchemes":{"Bearer":{"type":"http","scheme":"bearer"}}},"paths":{}}`), &document); err != nil {
		t.Fatal(err)
	}
	paths := document["paths"].(map[string]any)
	for _, name := range []string{"a", "b", "c", "d"} {
		media := "application/json"
		if name == "c" || name == "d" {
			media = "application/x-isolated"
		}
		schema := map[string]any{"type": "object", "required": []any{"value"}, "properties": map[string]any{"value": map[string]any{"type": "integer", "minimum": 1}}}
		content := map[string]any{media: map[string]any{"schema": schema}}
		paths["/"+name] = map[string]any{"post": map[string]any{"operationId": name, "requestBody": map[string]any{"required": true, "content": content}, "responses": map[string]any{"200": map[string]any{"description": "ok", "content": content}}}}
	}
	input, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := sdkgen.Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	plan, _, _, err := prepareClientSourcePlanWithCoverage(compiled, generator.Options{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.executionCompositions) != 2 {
		t.Fatal("JSON and custom media contracts did not retain separate shared factories")
	}
	artifacts, err := emitSourcePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createClient} from './index.js'; void createClient;`)
	script := `import assert from 'node:assert/strict';import path from 'node:path';import {pathToFileURL} from 'node:url';
const root=process.argv[1];const load=file=>import(pathToFileURL(path.join(root,file)));
const {createRequestContext}=await load('internal/runtime/http/http-execution-support.js');
const {provider:a}=await load('internal/executions/a/post.js');const {provider:b}=await load('internal/executions/b/post.js');
const acquired=[],requests=[];
const context=(name,fetch,codecs)=>createRequestContext({baseURL:'https://'+name+'.test',codecs,fetch,securityProvider:async()=>{await Promise.resolve();acquired.push(name);return {Bearer:{kind:'http-bearer',token:name}}}});
const fetch=async(url,init)=>{requests.push([new URL(url).host,new Headers(init.headers).get('authorization'),JSON.parse(init.body).value]);return Response.json({value:JSON.parse(init.body).value})};
const first=a.bind(context('one',fetch)),second=b.bind(context('two',fetch));const body={value:1};
assert.deepEqual(await Promise.all([first({body}),second({body})]),[{value:1},{value:1}]);
body.value=0;await assert.rejects(first({body}),error=>error.code==='REQUEST_ENCODE_FAILED');assert.equal(requests.length,2);
body.value=2;assert.deepEqual(await second({body}),{value:2});
assert.deepEqual(requests,[['one.test','Bearer one',1],['two.test','Bearer two',1],['two.test','Bearer two',2]]);
assert(acquired.includes('one')&&acquired.includes('two'));
const {provider:c}=await load('internal/executions/c/post.js');const {provider:d}=await load('internal/executions/d/post.js');
const custom=(name,value)=>context(name,async(url,init)=>{assert.equal(new URL(url).host,name+'.test');assert.equal(new Headers(init.headers).get('authorization'),'Bearer '+name);assert.equal(init.body,name+':2');return new Response(name,{headers:{'content-type':'application/x-isolated'}})}, {'application/x-isolated':{encode:input=>name+':'+input.value,decode:async response=>{assert.equal(await response.text(),name);return {value}}}});
assert.deepEqual(await Promise.all([c.bind(custom('custom-one',11))({body}),d.bind(custom('custom-two',22))({body})]),[{value:11},{value:22}]);
let started;const ready=new Promise(resolve=>started=resolve);const controller=new AbortController();
const pending=a.bind(context('cancel',async(_url,init)=>{started();return new Promise((_resolve,reject)=>init.signal.addEventListener('abort',()=>reject(init.signal.reason),{once:true}))}))({body},{signal:controller.signal});
await ready;const unaffected=second({body});controller.abort();await assert.rejects(pending,error=>error.code==='REQUEST_ABORTED');assert.deepEqual(await unaffected,{value:2});
assert.deepEqual(await first({body}),{value:2});
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
		t.Fatalf("shared assembly state boundaries: %v\n%s", err, result)
	}
}
