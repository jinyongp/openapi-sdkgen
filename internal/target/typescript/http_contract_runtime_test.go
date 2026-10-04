package typescript

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestHTTPContractSpecializationRegression(t *testing.T) {
	data, err := os.ReadFile("../../../test/typescript/fixtures/runtime-features/client/small-json.json")
	if err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	paths := input["paths"].(map[string]any)
	for path := range paths {
		if path != "/items/{id}" {
			delete(paths, path)
		}
	}
	data, err = json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(data)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := (Generator{}).Generate(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range artifacts {
		switch filepath.Base(artifact.Path) {
		case "http-path.ts", "http-parameter-sort.ts", "http-server-variables.ts", "http-security-source.ts", "http-security-requirements.ts", "http-response-services.ts", "http-response-stream-raw.ts", "http-request-core.ts":
			t.Fatalf("narrow JSON SDK generated unnecessary implementation %s", artifact.Path)
		}
	}
	// Build the control with the same prepared schemas, operation definition,
	// resources and runtime source, but the complete HTTP/security composition.
	plan, _, _, err := prepareClientSourcePlanWithCoverage(document, generator.Options{}, "")
	if err != nil {
		t.Fatal(err)
	}
	features := append(clientRuntimeFeatures(plan.runtimeFeatures, plan.manifest), runtimeFeature("http.general"), runtimeFeature("security.open"))
	plan.modules.runtimeComposition = prepareRuntimeComposition(features, false)
	control, err := emitClientFactory(document, plan.modules, plan.links, plan.streams)
	if err != nil {
		t.Fatal(err)
	}
	present := make(map[string]bool)
	for _, artifact := range artifacts {
		present[artifact.Path] = true
	}
	full, err := emitRuntimeTemplateArtifacts()
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range full {
		if !present[artifact.Path] {
			artifacts = append(artifacts, artifact)
		}
	}
	artifacts = append(artifacts, Artifact{Path: "internal/client/control.ts", Data: control})
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", "import {createClient} from './index.js'; void createClient;")
	differential, err := filepath.Abs("../../../test/typescript/verification/http-contract-differential.mjs")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := filepath.Abs("../../../test/typescript/verification/runtime-feature-imports.mjs")
	if err != nil {
		t.Fatal(err)
	}
	script := `import assert from 'node:assert/strict';import path from 'node:path';import {pathToFileURL} from 'node:url';
const root=process.argv[1];const {runHTTPContractDifferential}=await import(pathToFileURL(process.argv[2]));
const {generatedModuleGraph}=await import(pathToFileURL(process.argv[3]));
const candidate=await import(pathToFileURL(path.join(root,'index.js'))),control=await import(pathToFileURL(path.join(root,'internal/client/control.js')));
const narrow=new Set([...generatedModuleGraph(root,'internal/client/factory.js')].map(file=>path.basename(file)));
const general=new Set([...generatedModuleGraph(root,'internal/client/control.js')].map(file=>path.basename(file)));
for(const name of ['http-path.js','http-parameter-sort.js','http-server-variables.js','http-security-source.js','http-security-requirements.js','http-response-services.js','http-response-stream-raw.js','http-request-core.js']){assert.equal(narrow.has(name),false,name);assert.equal(general.has(name),true,'control '+name);}
const results=await runHTTPContractDifferential(candidate.createClient,control.createClient);
assert.equal(results.length,29);assert(results.some(x=>x.resultKind==='error'));
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output, differential, graph).CombinedOutput(); err != nil {
		t.Fatalf("generated HTTP contract differential: %v\n%s", err, result)
	}
}

func TestHTTPContractConservativeFacts(t *testing.T) {
	for _, test := range []struct {
		name, schema, style, server, security, version string
		generalPath, variableServer, multiple          bool
	}{
		{name: "simple string", schema: `{"type":"string"}`, style: "simple"},
		{name: "nullable string", schema: `{"type":"string","nullable":true}`, style: "simple", version: "3.0.3"},
		{name: "array", schema: `{"type":"array","items":{"type":"string"}}`, style: "simple", generalPath: true},
		{name: "matrix string", schema: `{"type":"string"}`, style: "matrix", generalPath: true},
		{name: "label string", schema: `{"type":"string"}`, style: "label", generalPath: true},
		{name: "unknown", schema: `{}`, style: "simple", generalPath: true},
		{name: "reference sibling ignored in 3.0", schema: `{"$ref":"#/components/schemas/Values","type":"string"}`, style: "simple", version: "3.0.3", generalPath: true},
		{name: "server variables", schema: `{"type":"string"}`, style: "simple", server: `,"servers":[{"url":"https://{host}/v1","variables":{"host":{"default":"api.test"}}}]`, variableServer: true},
		{name: "multiple requirements", schema: `{"type":"string"}`, style: "simple", security: `,"security":[{},{"Bearer":[]}]`, multiple: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			version := test.version
			if version == "" {
				version = "3.1.1"
			}
			source := `{"openapi":"` + version + `","info":{"title":"HTTP facts","version":"1"},"components":{"schemas":{"Values":{"type":"array","items":{"type":"string"}}},"securitySchemes":{"Bearer":{"type":"http","scheme":"bearer"}}},"paths":{"/items/{id}":{"get":{"operationId":"getItem","parameters":[{"in":"path","name":"id","required":true,"style":"` + test.style + `","schema":` + test.schema + `}],"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"string"}}}}}` + test.server + test.security + `}}}}`
			document, err := sdkgen.Compile([]byte(source))
			if err != nil {
				t.Fatal(err)
			}
			plan, _, _, err := prepareClientSourcePlanWithCoverage(document, generator.Options{}, "")
			if err != nil {
				t.Fatal(err)
			}
			features := plan.runtimeFeatures.operations["GET /items/{id}"]
			for name, want := range map[runtimeFeature]bool{"http.path.general": test.generalPath, "http.server.variables": test.variableServer, "security.multiple-requirements": test.multiple} {
				got := false
				for _, feature := range features {
					if feature == name {
						got = true
					}
				}
				if got != want {
					t.Fatalf("%s=%t, want %t; %v", name, got, want, features)
				}
			}
			if test.multiple {
				for _, dependency := range plan.executions["GET /items/{id}"].composition.imports {
					if strings.Contains(dependency.path, "http-security-bearer") {
						t.Fatal("multiple requirements narrowed to a singleton")
					}
				}
			}
		})
	}
}

func TestHTTPContractMixedNamedAndProviderRegression(t *testing.T) {
	data, err := os.ReadFile("../../../test/typescript/fixtures/runtime-features/client/small-json.json")
	if err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	paths := input["paths"].(map[string]any)
	group := paths["/items"].(map[string]any)["get"].(map[string]any)
	group["operationId"] = "groupItems"
	group["servers"] = []any{map[string]any{"url": "https://{region}.test/v1", "variables": map[string]any{"region": map[string]any{"default": "us", "enum": []any{"us", "eu"}}}}}
	group["security"] = []any{map[string]any{"Bearer": []any{}}, map[string]any{}}
	group["parameters"] = []any{
		map[string]any{"in": "path", "name": "ids", "required": true, "style": "matrix", "schema": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}},
		map[string]any{"in": "query", "name": "order", "x-sort": map[string]any{"format": "field-direction"}, "schema": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": []any{"createdAt:desc", "createdAt:asc"}}}},
	}
	delete(paths, "/items")
	paths["/groups/{ids}"] = map[string]any{"get": group}
	data, err = json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(data)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := (Generator{}).Generate(document, generator.Options{Clients: map[string]generator.Client{
		"small":  {Selection: &generator.Selection{Operations: []string{"getItem"}}},
		"groups": {Selection: &generator.Selection{Operations: []string{"groupItems"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createClient} from './index.js'; import {createClient as small} from './clients/small/index.js'; void [createClient,small];`)
	helper, err := filepath.Abs("../../../test/typescript/verification/runtime-feature-imports.mjs")
	if err != nil {
		t.Fatal(err)
	}
	script := `import assert from 'node:assert/strict';import path from 'node:path';import {pathToFileURL} from 'node:url';
const root=process.argv[1];const {generatedModuleGraph}=await import(pathToFileURL(process.argv[2]));
const graph=entry=>new Set(generatedModuleGraph(root,entry,{allowGenericLoader:true}).map(file=>path.basename(file)));
for(const name of ['http-path.js','http-parameter-sort.js','http-server-variables.js','http-security-source.js','http-security-requirements.js']){
 for(const entry of ['index.js','clients/groups/index.js','internal/executions/groups/by-ids/get.js'])assert.equal(graph(entry).has(name),true,entry+' '+name);
 for(const entry of ['clients/small/index.js','internal/executions/items/by-id/get.js'])assert.equal(graph(entry).has(name),false,entry+' '+name);
}
let calls=0;const fetch=async(url,init)=>{calls++;const parsed=new URL(url);assert.equal(new Headers(init.headers).get('authorization'),'Bearer mixed');
 if(parsed.pathname==='/items/one')return Response.json({id:'one',title:'One'});
 assert.equal(parsed.href,'https://eu.test/v1/groups/;ids=a,b?order=createdAt%3Adesc');return Response.json([{id:'one',title:'One'}]);};
for(const entry of ['index.js','clients/groups/index.js']){const {createClient}=await import(pathToFileURL(path.join(root,entry)));const api=createClient({authorization:'Bearer mixed',server:{variables:{region:'eu'}},fetch});
 await assert.rejects(()=>api.$operations.groupItems({path:{ids:['a','b']},query:{order:[{field:'createdAt',direction:'desc'}]}}),error=>error.code==='SECURITY_REQUIREMENT_REQUIRED');
 assert.deepEqual(await api.$operations.groupItems({path:{ids:['a','b']},query:{order:[{field:'createdAt',direction:'desc'}]}},{securityRequirement:'Bearer'}),[{id:'one',title:'One'}]);
 await assert.rejects(()=>api.$operations.groupItems({path:{ids:['a','b']},query:{order:[{field:'unknown',direction:'desc'}]}},{securityRequirement:'Bearer'}));}
const {createClient}=await import(pathToFileURL(path.join(root,'clients/small/index.js')));const api=createClient({baseURL:'https://small.test',authorization:'Bearer mixed',fetch});assert.deepEqual(await api.$operations.getItem({path:{id:'one'}}),{id:'one',title:'One'});assert.equal(calls,3);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output, helper).CombinedOutput(); err != nil {
		t.Fatalf("mixed named/provider HTTP contracts: %v\n%s", err, result)
	}
}
