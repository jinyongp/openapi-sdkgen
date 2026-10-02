package typescript

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

const generationSelectionFixture = `{"openapi":"3.2.1","info":{"title":"Selected","version":"1"},"paths":{
"/a":{"get":{"operationId":"a","responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Item"}}},"links":{"next":{"operationId":"b"}}}}}},
"/b":{"get":{"operationId":"b","responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Item"}}},"links":{"back":{"operationId":"a"},"next":{"operationId":"c"}}}}}},
"/c":{"get":{"operationId":"c","responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Item"}}}}}}},
"/unused":{"get":{"operationId":"unused","x-envelope":false,"responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Unused"}}}}}}},
"/hidden":{"get":{"operationId":"hidden","x-sdk-visibility":"hidden","responses":{"204":{"description":"OK"}}}},
"/idless/{task-id}":{"get":{"parameters":[{"name":"task-id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"204":{"description":"OK"}}}}
},"components":{"schemas":{"Item":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}},"Unused":{"type":"integer"}}}}`

func selectedFixtureDocument(t *testing.T) *ir.Document {
	t.Helper()
	document, err := sdkgen.Compile([]byte(generationSelectionFixture))
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func TestGenerationSelectionLimitsPublicSurfaceAndKeepsLinkClosure(t *testing.T) {
	document := selectedFixtureDocument(t)
	before, _ := json.Marshal(document)
	options := generator.Options{Selection: &generator.Selection{Operations: []string{"a"}}}
	plan, diagnostics, err := (Generator{}).Prepare(document, options)
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("prepare: %v, %#v", err, diagnostics)
	}
	value, _ := plan.Value("typescript")
	prepared := value.(*sourcePlan)
	if len(prepared.manifest.Operations) != 3 || len(prepared.selection.dependencies) != 2 {
		t.Fatalf("closure = %#v", prepared.manifest)
	}
	artifacts, err := (Generator{}).Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	sources := make(map[string]string)
	for _, artifact := range artifacts {
		sources[artifact.Path] = string(artifact.Data)
	}
	for _, path := range []string{"internal/client/types.ts", "internal/routes/index.ts", "selective/types.ts", "selective/all.ts"} {
		for _, excluded := range []string{`"GET /b"`, `"GET /c"`, `"GET /unused"`, `readonly "b":`, `readonly "c":`} {
			if strings.Contains(sources[path], excluded) {
				t.Fatalf("%s exposes %s:\n%s", path, excluded, sources[path])
			}
		}
	}
	for path := range sources {
		if strings.Contains(path, "unused") || strings.Contains(path, "Unused") {
			t.Fatalf("excluded artifact %s", path)
		}
	}
	after, _ := json.Marshal(document)
	if !bytes.Equal(before, after) {
		t.Fatal("selection preparation mutated compiler IR")
	}
	permuted, err := (Generator{}).Generate(document, generator.Options{Selection: &generator.Selection{Operations: []string{"a", "a"}, Routes: []string{"GET /a"}}})
	if err != nil || !reflect.DeepEqual(artifacts, permuted) {
		t.Fatalf("equivalent selector output differs: %v", err)
	}
}

func TestGenerationSelectionRejectsInvalidAndHiddenEntries(t *testing.T) {
	for _, selection := range []*generator.Selection{{}, {Operations: []string{"typo"}}, {Operations: []string{"hidden"}}, {Operations: []string{"a", "typo"}}, {Routes: []string{"get /a"}}, {Routes: []string{"GET /idless/:task-id"}}} {
		plan, values, err := (Generator{}).Prepare(selectedFixtureDocument(t), generator.Options{Selection: selection})
		if err != nil || !diagnostic.HasErrors(values) {
			t.Fatalf("selection %#v: %v, %#v", selection, err, values)
		}
		if artifacts, err := (Generator{}).Emit(plan); err == nil || len(artifacts) != 0 {
			t.Fatalf("invalid selection emitted %d artifacts, %v", len(artifacts), err)
		}
	}
	if _, err := (Generator{}).Generate(selectedFixtureDocument(t), generator.Options{Selection: &generator.Selection{Routes: []string{"GET /idless/{task-id}"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestGenerationSelectionRuntimeAndStrictTypes(t *testing.T) {
	options := generator.Options{Selection: &generator.Selection{Operations: []string{"a"}}}
	probe := `import {createClient} from "./index.js";
import {operations, routes} from "./selective/index.js";
declare const api: ReturnType<typeof createClient>;
api.$operations.a; api.$routes["GET /a"]; api.$links.a.next;
// @ts-expect-error dependency-only operation
api.$operations.b;
// @ts-expect-error dependency-only route
api.$routes["GET /b"];
// @ts-expect-error dependency-only resource
api.b;
// @ts-expect-error dependency-only Link source
api.$links.b;
// @ts-expect-error excluded operation reference
operations.b;
// @ts-expect-error excluded route reference
routes["GET /b"];
`
	output := compileSelectedTypeScriptArtifacts(t, selectedFixtureDocument(t), options, probe)
	script := `import {pathToFileURL} from 'node:url';
const root=await import(pathToFileURL(process.argv[1]+"/index.js"));
const selective=await import(pathToFileURL(process.argv[1]+"/selective/index.js"));
const all=await import(pathToFileURL(process.argv[1]+"/selective/all.js"));
const rejectPrivate=async()=>{for(const reference of [selective.routes['GET /b'],selective.operations.b]){let rejected=false;try{await selective.loadOperations([reference]);}catch{rejected=true;}if(!rejected)throw Error('private dependency prepared publicly');}};
await rejectPrivate();
const seen=[];
const fetch=async(url,init)=>{seen.push([String(url),new Headers(init.headers).get('Authorization')]);const r=new Response('{"id":"one"}',{headers:{'content-type':'application/json'}});Object.defineProperty(r,'url',{value:String(url)});return r;};
const api=root.createClient({baseURL:'https://root.test',fetch});
if(Object.keys(api.$operations).join(',')!=='a'||Object.keys(api.$routes).join(',')!=='GET /a'||Object.keys(api.$links).join(',')!=='a'||'b' in api)throw Error('dependency exposed by root');
await api.$links.a.next(await api.$operations.a.raw());
const prepared=await selective.loadOperations([selective.operations.a]);
const first=selective.createClient({baseURL:'https://first.test',authorization:'first',fetch,operations:prepared});
const second=selective.createClient({baseURL:'https://second.test',authorization:'second',fetch,operations:prepared});
await first.$links.a.next(await first.$operations.a.raw());
await second.$links.a.next(await second.$operations.a.raw());
await rejectPrivate();
if(Object.keys(all.operations).join(',')!=='a'||Object.keys(first.$operations).join(',')!=='a'||Object.keys(first.$routes).join(',')!=='GET /a')throw Error('dependency exposed by selective');
if(seen.length!==6||!seen[1][0].endsWith('/b')||seen[3][1]!=='first'||seen[5][1]!=='second')throw Error('Link binding/request isolation '+JSON.stringify(seen));
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
		t.Fatalf("runtime: %v\n%s", err, result)
	}
}

func TestGenerationSelectionRequiresPublicEntryAfterOmission(t *testing.T) {
	input := strings.Replace(generationSelectionFixture, `"operationId":"a","responses"`, `"operationId":"a","requestBody":{"content":{"application/json":{"schema":{"type":"string"}}}},"responses"`, 1)
	document, err := sdkgen.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Selection: &generator.Selection{Operations: []string{"a"}}})
	if err != nil {
		t.Fatal(err)
	}
	var empty bool
	for _, value := range diagnostics {
		if value.Code == "SDKGEN-E512" {
			empty = true
		}
	}
	if !empty {
		t.Fatalf("omitted public entry diagnostics = %#v", diagnostics)
	}
	if _, err := (Generator{}).Generate(document, generator.Options{Selection: &generator.Selection{Operations: []string{"a", "b"}}}); err != nil {
		t.Fatalf("direct supported B: %v", err)
	}
}

func compileSelectedTypeScriptArtifacts(t *testing.T, document *ir.Document, options generator.Options, probe string) string {
	t.Helper()
	artifacts, err := (Generator{}).Generate(document, options)
	if err != nil {
		t.Fatal(err)
	}
	for index := range artifacts {
		artifacts[index].Data = bytes.ReplaceAll(artifacts[index].Data, []byte("// @ts-nocheck\n"), nil)
	}
	return compileTypeScriptArtifactSet(t, artifacts, "selection.probe.ts", probe)
}

func TestGenerationSelectionKeepsLoadedExternalLinkClosure(t *testing.T) {
	input, err := filepath.Abs("../../../test/fixtures/support-gaps/links/root.json")
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.CompileFile(input)
	if err != nil {
		t.Fatal(err)
	}
	output := compileSelectedTypeScriptArtifacts(t, document, generator.Options{Selection: &generator.Selection{Operations: []string{"getSource"}}}, "")
	script := `import {pathToFileURL} from 'node:url'; const {createClient}=await import(pathToFileURL(process.argv[1]+"/index.js"));
const seen=[]; const api=createClient({baseURL:'https://source.test',fetch:async(url)=>{seen.push(String(url));const r=new Response('{"id":"one /two"}',{headers:{'content-type':'application/json'}});Object.defineProperty(r,'url',{value:String(url)});return r;}});
await api.$links.getSource.item(await api.$operations.getSource.raw());
if(Object.keys(api.$operations).join(',')!=='getSource'||seen[1]!=='https://target.example.test/v1/items/one%20%2Ftwo')throw Error('external selected Link '+JSON.stringify(seen));`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, result)
	}
}

func TestGenerationSelectionServerIncludesDirectCallbacksAndWebhooks(t *testing.T) {
	input := `{"openapi":"3.2.1","info":{"title":"Selected inbound","version":"1"},"paths":{
"/a":{"get":{"operationId":"a","callbacks":{"chosen":{"$ref":"#/components/callbacks/Used"}},"responses":{"204":{"description":"OK","links":{"next":{"operationId":"b"}}}}}},
"/b":{"get":{"operationId":"b","callbacks":{"dependencyCallback":{"{$request.query.url}":{"post":{"responses":{"204":{"description":"OK"}}}}}},"responses":{"204":{"description":"OK"}}}}
},"webhooks":{"event":{"post":{"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Event"}}}},"responses":{"204":{"description":"OK"}}}}},
"components":{"callbacks":{
"Used":{"{$request.query.url}":{"post":{"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Callback"}}}},"responses":{"204":{"description":"OK"}}}}},
"Unused":{"{$request.query.url}":{"post":{"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Unused"}}}},"responses":{"204":{"description":"OK"}}}}}
},"schemas":{"Event":{"type":"string"},"Callback":{"type":"string"},"Unused":{"type":"string"}}}}`
	document, err := sdkgen.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := generator.NewAddonRegistry(generator.AddonServer)
	options, _ := registry.Resolve([]string{"server"})
	options.Selection = &generator.Selection{Operations: []string{"a"}}
	plan, diagnostics, err := (Generator{}).Prepare(document, options)
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("prepare: %v, %#v", err, diagnostics)
	}
	value, _ := plan.Value("typescript")
	prepared := value.(*sourcePlan)
	if len(prepared.webhooks) != 1 || len(prepared.callbacks) != 2 {
		t.Fatalf("inbound: %#v, %#v", prepared.webhooks, prepared.callbacks)
	}
	for _, callback := range prepared.callbacks {
		if callback.sourceOperationID == "b" || callback.componentName == "Unused" {
			t.Fatalf("excluded callback = %#v", callback)
		}
	}
	for _, schema := range prepared.modules.schemas {
		if schema.name == "Unused" {
			t.Fatal("unused callback schema emitted")
		}
	}
	compileSelectedTypeScriptArtifacts(t, document, options, "")
	options.Clients = map[string]generator.Client{"second": {Selection: &generator.Selection{Operations: []string{"b"}}}}
	plan, diagnostics, err = (Generator{}).Prepare(document, options)
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("named inbound preparation: %v, %#v", err, diagnostics)
	}
	value, _ = plan.Value("typescript")
	prepared = value.(*sourcePlan)
	if len(prepared.callbacks) != 3 || len(prepared.webhooks) != 1 {
		t.Fatalf("named direct callback union = %#v", prepared.callbacks)
	}
	compileSelectedTypeScriptArtifacts(t, document, options, "")
}
