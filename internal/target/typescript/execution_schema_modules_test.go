package typescript

import (
	"encoding/json"
	"fmt"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
)

func TestExecutionSchemaSharingLeavesSmallAndDisjointClosuresAlone(t *testing.T) {
	for _, test := range []struct {
		name   string
		size   int
		common int
	}{
		{"empty", 0, 0}, {"single", 1, 0}, {"disjoint", 80, 0}, {"small-overlap", 80, 63},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := &semanticModulePlan{}
			executions := make(map[string]operationExecutionPlan)
			for _, route := range []string{"a", "b"} {
				var names []string
				for i := 0; i < test.size; i++ {
					prefix := route
					if i < test.common {
						prefix = "common"
					}
					names = append(names, fmt.Sprintf("%s-%03d", prefix, i))
				}
				sort.Strings(names)
				plan.operations = append(plan.operations, operationModulePlan{routeKey: route})
				executions[route] = operationExecutionPlan{inputSchemas: names}
			}
			modules, err := prepareExecutionSchemaModules(plan, executions)
			if err != nil || len(modules) != 0 || len(plan.selective) != 0 {
				t.Fatalf("unprofitable sharing: %#v %v", modules, err)
			}
		})
	}
}

func TestPartialExecutionClosuresPreserveProjectionOrderAndIdentity(t *testing.T) {
	names := []string{"__proto__", "constructor", "2", "10", "é", "𝄞", "\ue000"}
	for i := len(names); i < 64; i++ {
		names = append(names, fmt.Sprintf("Shared%d", i))
	}
	sort.Strings(names)
	schemas := make(map[string]any)
	properties := make(map[string]any)
	for i, name := range names {
		schemas[name] = map[string]any{"type": "string"}
		properties[fmt.Sprintf("p%d", i)] = map[string]any{"$ref": "#/components/schemas/" + name}
	}
	schemas["OutputOnly"] = map[string]any{"type": "string"}
	properties["read"] = map[string]any{"readOnly": true, "$ref": "#/components/schemas/OutputOnly"}
	for _, root := range []string{"RootA", "RootB"} {
		schemas[root] = map[string]any{"type": "object", "properties": properties}
	}
	schemas["Unrelated"] = map[string]any{"type": "boolean"}
	paths := make(map[string]any)
	for _, route := range []string{"a", "again", "b"} {
		root := "RootA"
		if route == "b" {
			root = "RootB"
		}
		content := map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/" + root}}}
		paths["/"+route] = map[string]any{"post": map[string]any{"operationId": route, "requestBody": map[string]any{"content": content}, "responses": map[string]any{"200": map[string]any{"description": "ok", "content": content}}}}
	}
	input, err := json.Marshal(map[string]any{"openapi": "3.1.1", "info": map[string]any{"title": "Partial closures", "version": "1"}, "paths": paths, "components": map[string]any{"schemas": schemas}})
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := prepareSourcePlan(document, false)
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	byPath := make(map[string]executionSchemaModule)
	blocks := 0
	for _, module := range plan.executionSchemas {
		byPath[module.path] = module
		if module.block {
			blocks++
		}
	}
	if blocks != 2 {
		t.Fatalf("shared projection blocks = %d, want 2", blocks)
	}
	for _, module := range plan.executionSchemas {
		if len(module.blocks) == 0 {
			continue
		}
		got := append([]string{}, module.delta...)
		for _, path := range module.blocks {
			block := byPath[path]
			if !block.block || block.projection != module.projection {
				t.Fatalf("projection crossed: %#v", block)
			}
			got = append(got, block.names...)
		}
		sort.Strings(got)
		if !reflect.DeepEqual(got, module.names) {
			t.Fatalf("shared imports widened or lost closure: %v != %v", got, module.names)
		}
	}
	artifacts, err := emitSourcePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	again, err := emitSourcePlan(plan)
	if err != nil || !reflect.DeepEqual(artifacts, again) {
		t.Fatalf("partial emission is not deterministic: %v", err)
	}
	output := compileTypeScriptArtifacts(t, document)
	for _, compiler := range []string{"typescript-5-7", "typescript-5-9", "typescript-6"} {
		t.Run(compiler, func(t *testing.T) {
			tsc := filepath.Join("..", "..", "..", "test", "typescript", "node_modules", compiler, "lib", "tsc.js")
			config := filepath.Join(filepath.Dir(output), "source", "tsconfig.json")
			if result, err := exec.Command("node", tsc, "--project", config, "--noEmit").CombinedOutput(); err != nil {
				t.Fatalf("whole-source partial closure strict check: %v\n%s", err, result)
			}
		})
	}
	a, b, same := plan.executions["POST /a"], plan.executions["POST /b"], plan.executions["POST /again"]
	if a.inputBundle == "" || a.inputBundle != same.inputBundle || a.inputBundle == b.inputBundle || a.inputBundle == a.outputBundle {
		t.Fatalf("closure/projection identity changed: %#v %#v %#v", a, b, same)
	}
	type runtimeCase struct {
		Path string   `json:"path"`
		Keys []string `json:"keys"`
	}
	cases := []runtimeCase{}
	for _, path := range []string{a.inputBundle, a.outputBundle, b.inputBundle, b.outputBundle} {
		cases = append(cases, runtimeCase{Path: strings.TrimSuffix(path, ".ts") + ".js", Keys: byPath[path].names})
	}
	encoded, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	script := `
import { pathToFileURL } from 'node:url';
import { join } from 'node:path';
const root = process.argv[1];
const cases = JSON.parse(process.argv[2]);
const {mergeSchemas} = await import(pathToFileURL(join(root,'internal/execution-schemas/merge.js')));
const descriptor = {type:'string'};
if (Object.keys(mergeSchemas([])).length !== 0) throw new Error('empty merge');
const streams = [[0,3,7,9],[1,4,8],[2,5,6,10],[]].map(indices=>indices.map(index=>[index,'k'+index,descriptor]));
if (JSON.stringify(Object.keys(mergeSchemas(streams)))!==JSON.stringify(Array.from({length:11},(_,index)=>'k'+index))) throw new Error('interleaved heap merge');
for (const blocks of [[[[1,'a',descriptor]],[[1,'a',descriptor]]], [[[2,'b',descriptor],[1,'a',descriptor]]]]) {
  let rejected = false;
  try { mergeSchemas(blocks); } catch { rejected = true; }
  if (!rejected) throw new Error('duplicate/unsorted block accepted');
}
for (const test of cases) {
  const url = pathToFileURL(join(root,test.path));
  const {schemas} = await import(url);
  if ((await import(url)).schemas !== schemas) throw new Error('closure identity');
  const expected = Object.fromEntries(test.keys.map(key=>[key,descriptor]));
  if (Object.getPrototypeOf(schemas)!==Object.prototype || JSON.stringify(Object.keys(schemas))!==JSON.stringify(Object.keys(expected))) throw new Error('prototype/key order');
  if (!Object.hasOwn(schemas,'__proto__') || !Object.hasOwn(schemas,'constructor') || Object.hasOwn(schemas,'Unrelated')) throw new Error('special/subset keys');
  for (const key of test.keys) {
    const property=Object.getOwnPropertyDescriptor(schemas,key);
    if(!property.writable || !property.enumerable || !property.configurable) throw new Error('property descriptor flags');
  }
}
const maps = await Promise.all(cases.map(async test=>(await import(pathToFileURL(join(root,test.path)))).schemas));
if (maps[0]['__proto__'] !== maps[2]['__proto__'] || maps[1]['__proto__'] !== maps[3]['__proto__']) throw new Error('shared descriptor identity');
if (Object.hasOwn(maps[0],'OutputOnly') || !Object.hasOwn(maps[1],'OutputOnly')) throw new Error('projection boundary');
const {createClient} = await import(pathToFileURL(join(root,'index.js')));
const make = ()=>createClient({baseURL:'https://example.test',fetch:async()=>new Response('{"p0":"ok"}',{headers:{'content-type':'application/json'}})});
const one=make(),two=make();
if(one.$operations.a===two.$operations.a) throw new Error('client configuration shared');
await one.$operations.a({body:{p0:'ok'}});
let invalid=false;
try{await one.$operations.a({body:{p0:42}});}catch{invalid=true;}
if(!invalid) throw new Error('schema validation bypassed');
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Clean(output), string(encoded)).CombinedOutput(); err != nil {
		t.Fatalf("partial closure runtime: %v\n%s", err, result)
	}
}

func TestRepeatedExecutionClosuresShareOnlyTheirProjectedSchemas(t *testing.T) {
	input := []byte(`{
  "openapi":"3.1.0","info":{"title":"Shared closures","version":"1"},
  "paths":{
    "/a":{"post":{"operationId":"a","requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Root"}}}},"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Root"}}}}}}},
    "/b":{"post":{"operationId":"b","requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Root"}}}},"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Root"}}}}}}}
  },
  "components":{"schemas":{
    "Root":{"type":"object","properties":{"shared":{"$ref":"#/components/schemas/Shared"},"input":{"writeOnly":true,"$ref":"#/components/schemas/InputOnly"},"output":{"readOnly":true,"$ref":"#/components/schemas/OutputOnly"}}},
    "Shared":{"type":"string"},"InputOnly":{"type":"string"},"OutputOnly":{"type":"string"},"Unrelated":{"type":"boolean"}
  }}
}`)
	document, err := sdkgen.Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := prepareSourcePlan(document, false)
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	if len(plan.executionSchemas) != 2 {
		t.Fatalf("shared projected closures = %d, want 2", len(plan.executionSchemas))
	}
	artifacts, err := emitSourcePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	again, err := emitSourcePlan(plan)
	if err != nil || !reflect.DeepEqual(artifacts, again) {
		t.Fatalf("emission is not stable: %v", err)
	}
	byPath := make(map[string]string)
	for _, artifact := range artifacts {
		byPath[artifact.Path] = string(artifact.Data)
	}
	for _, bundle := range plan.executionSchemas {
		want := []string{"InputOnly", "Root", "Shared"}
		excluded := "OutputOnly"
		if bundle.projection == "output" {
			want, excluded = []string{"OutputOnly", "Root", "Shared"}, "InputOnly"
		}
		if !reflect.DeepEqual(bundle.names, want) || strings.Contains(byPath[bundle.path], excluded) || strings.Contains(byPath[bundle.path], "Unrelated") {
			t.Fatalf("projection dependencies widened: %#v\n%s", bundle, byPath[bundle.path])
		}
		for index, name := range want {
			entry := fmt.Sprintf("[%s, schema%d]", quoteTS(name), index)
			if !strings.Contains(byPath[bundle.path], entry) || !strings.Contains(byPath[bundle.path], bundle.projection+"WireSchema") {
				t.Fatalf("projected schema binding missing: %s", entry)
			}
		}
	}
	for _, module := range plan.modules.operations {
		execution := plan.executions[module.routeKey]
		if execution.inputBundle == "" || execution.outputBundle == "" || execution.inputBundle == execution.outputBundle {
			t.Fatalf("missing independent projection modules: %#v", execution)
		}
		provider := byPath[operationExecutionArtifactPath(module)]
		if !strings.Contains(provider, "bindBase(request, inputSchemas, outputSchemas)") || strings.Contains(provider, "Object.fromEntries") {
			t.Fatalf("provider did not use shared maps: %s", provider)
		}
	}
	imports := regexp.MustCompile(`(?:\bfrom\s+|\bimport\s*\(?\s*)["'](\.[^"']+)["']`)
	for artifact, source := range byPath {
		for _, match := range imports.FindAllStringSubmatch(source, -1) {
			if strings.HasSuffix(match[1], ".js") {
				target := pathpkg.Join(pathpkg.Dir(artifact), strings.TrimSuffix(match[1], ".js")+".ts")
				if _, exists := byPath[target]; !exists {
					t.Fatalf("%s imports missing %s", artifact, target)
				}
			}
		}
	}
}
