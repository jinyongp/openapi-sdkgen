package typescript

import (
	"bytes"
	"os/exec"
	"testing"

	"openapi-sdkgen/internal/generator"
)

func TestFullSelectionDefaultArtifactsMatchFull(t *testing.T) {
	document := registrySelectionDocument(t)
	full, err := (Generator{}).Generate(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := (Generator{}).Generate(document, generator.Options{Selection: &generator.Selection{
		Operations: []string{"a", "b", "c", "unused"},
		Routes:     []string{"GET /idless/{task-id}"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(full) != len(selected) {
		t.Fatalf("artifact count: full=%d selected=%d", len(full), len(selected))
	}
	for _, artifact := range full {
		if !bytes.Equal(artifact.Data, artifactByPath(t, selected, artifact.Path)) {
			t.Fatalf("equivalent full selection differs at %s", artifact.Path)
		}
	}
}

func TestSelectionRecordsRequireMetadataAddon(t *testing.T) {
	registry, err := generator.NewAddonRegistry(generator.AddonMetadata)
	if err != nil {
		t.Fatal(err)
	}
	for _, metadata := range []bool{false, true} {
		name := "default"
		var addons []string
		if metadata {
			name, addons = "metadata", []string{"metadata"}
		}
		t.Run(name, func(t *testing.T) {
			options, err := registry.Resolve(addons)
			if err != nil {
				t.Fatal(err)
			}
			options.Selection = &generator.Selection{Operations: []string{"a"}}
			options.Clients = map[string]generator.Client{"named": {Selection: &generator.Selection{Operations: []string{"c"}}}}
			probe := `import {openapi} from "./metadata.js";
export const version: "3.2.1" = openapi.version;
`
			if metadata {
				probe += `import {generationSelection,generationClients} from "./metadata.js";
export const routes: readonly ["GET /a"] = generationSelection.routes;
export const dependencies: readonly ["GET /b","GET /c"] = generationSelection.dependencyRoutes;
export const named: readonly ["GET /c"] = generationClients.named.routes;
// @ts-expect-error metadata selection is readonly
generationSelection.routes.push("GET /a");
`
			} else {
				probe += `// @ts-expect-error selection records require metadata addon
import {generationSelection} from "./metadata.js";
// @ts-expect-error client assignments require metadata addon
import {generationClients} from "./metadata.js";
export {generationSelection,generationClients};
`
			}
			output := compileSelectedTypeScriptArtifacts(t, registrySelectionDocument(t), options, probe)
			script := `import assert from 'node:assert/strict';import {pathToFileURL} from 'node:url';
const load=file=>import(pathToFileURL(process.argv[1]+'/'+file));
const metadata=await load('metadata.js');
if(process.argv[2]==='metadata'){
  assert.deepEqual(metadata.generationSelection,{routes:['GET /a'],dependencyRoutes:['GET /b','GET /c']});
  assert.deepEqual(metadata.generationClients,{named:{routes:['GET /c'],dependencyRoutes:[]}});
}else{
  assert.deepEqual(Object.keys(metadata),['openapi']);
}
const selective=await load('selective/index.js');
const seen=[];
const prepared=await selective.loadOperations([selective.operations.a]);
const api=selective.createClient({operations:prepared,baseURL:'https://api.test',fetch:async(url)=>{seen.push(String(url));const r=Response.json({id:'one'});Object.defineProperty(r,'url',{value:String(url)});return r;}});
await api.$links.a.next(await api.$operations.a.raw());
await assert.rejects(selective.loadOperations([selective.routes['GET /b']]),{stage:'INPUT'});
assert.deepEqual(Object.keys(api.$routes),['GET /a']);
assert.deepEqual(seen,['https://api.test/a','https://api.test/b']);
`
			if result, err := exec.Command("node", "--input-type=module", "--eval", script, output, name).CombinedOutput(); err != nil {
				t.Fatalf("runtime: %v\n%s", err, result)
			}
		})
	}
}

func TestSelectionScopeIncludesNamedOnlyProviders(t *testing.T) {
	for _, rootIDs := range [][]string{{"a", "b"}, {"a", "b", "c"}} {
		options := generator.Options{
			Selection: &generator.Selection{Operations: rootIDs},
			Clients:   map[string]generator.Client{"other": {Selection: &generator.Selection{Operations: []string{"unused"}}}},
		}
		output := compileSelectedTypeScriptArtifacts(t, registrySelectionDocument(t), options, "")
		script := `import assert from 'node:assert/strict';import {pathToFileURL} from 'node:url';
const load=file=>import(pathToFileURL(process.argv[1]+'/'+file));
const selective=await load('selective/index.js');
const runtime=await load('internal/runtime/client/operation-loader.js');
const namedOnly=await load('internal/executions/unused/get.js');
for(const ref of [runtime.staticOperationReference(namedOnly.provider),selective.routes['GET /unused']]){
  await assert.rejects(selective.loadOperations([ref]),{stage:'INPUT'});
}
const prepared=await selective.loadOperations([selective.operations.a]);
const seen=[];
const options={baseURL:'https://api.test',fetch:async(url)=>{seen.push(String(url));const response=Response.json({id:'one'});Object.defineProperty(response,'url',{value:String(url)});return response;}};
const api=selective.createClient({...options,operations:prepared});
await api.$links.a.next(await api.$operations.a.raw());
const {createClient}=await load('clients/other/index.js');
assert.deepEqual(Object.keys(createClient(options).$routes),['GET /unused']);
assert.deepEqual(Object.keys(api.$routes),['GET /a']);
assert.deepEqual(seen,['https://api.test/a','https://api.test/b']);
`
		if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
			t.Fatalf("runtime: %v\n%s", err, result)
		}
	}
}
