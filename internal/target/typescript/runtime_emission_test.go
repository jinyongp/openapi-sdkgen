package typescript

import (
	"bytes"
	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const runtimeEmissionFixture = `{"openapi":"3.2.0","info":{"title":"Emission","version":"1"},"paths":{"/json":{"get":{"operationId":"json","responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","properties":{"id":{"type":"string"}}}}}}}}},"/xml":{"get":{"operationId":"xml","responses":{"200":{"description":"ok","content":{"application/xml":{"schema":{"type":"object","xml":{"name":"item"},"properties":{"id":{"type":"string"}}}}}}}}}},"webhooks":{"notify":{"post":{"operationId":"notify","requestBody":{"content":{"application/json":{"schema":{"type":"object"}}}},"responses":{"204":{"description":"ok"}}}}}}`

func TestRuntimeFeatureNamedCompositionRegression(t *testing.T) {
	document, err := sdkgen.Compile([]byte(runtimeEmissionFixture))
	if err != nil {
		t.Fatal(err)
	}
	delete(document.Raw, "webhooks")
	artifacts, err := (Generator{}).Generate(document, generator.Options{Clients: map[string]generator.Client{
		"json": {Selection: &generator.Selection{Operations: []string{"json"}}},
		"xml":  {Selection: &generator.Selection{Operations: []string{"xml"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", `import {createClient} from './clients/json/index.js'; void createClient;`)
	helper, err := filepath.Abs("../../../test/typescript/verification/runtime-feature-imports.mjs")
	if err != nil {
		t.Fatal(err)
	}
	script := `import assert from 'node:assert/strict'; import path from 'node:path'; import {pathToFileURL} from 'node:url';
const {generatedModuleGraph}=await import(pathToFileURL(process.argv[2]));
const graph=entry=>new Set(generatedModuleGraph(process.argv[1],entry,{allowGenericLoader:true}).map(file=>path.basename(file)));
assert.equal(graph('clients/json/index.js').has('xml-codec.js'),false);
assert.equal(graph('clients/xml/index.js').has('xml-codec.js'),true);
assert.equal(graph('index.js').has('xml-codec.js'),true);
for(const name of ['json','xml']){const {createClient}=await import(pathToFileURL(path.join(process.argv[1],'clients',name,'index.js')));const api=createClient({baseURL:'https://named.test',fetch:async url=>{assert.equal(new URL(url).pathname,'/'+name);return name==='xml'?new Response('<item><id>one</id></item>',{headers:{'content-type':'application/xml'}}):Response.json({id:'one'});}});assert.deepEqual(Object.keys(api.$operations),[name]);assert.deepEqual(await api.$operations[name](),{id:'one'});}`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output, helper).CombinedOutput(); err != nil {
		t.Fatalf("named runtime composition: %v\n%s", err, result)
	}
}

func TestRuntimeFeatureEmissionRegression(t *testing.T) {
	document, err := sdkgen.Compile([]byte(runtimeEmissionFixture))
	if err != nil {
		t.Fatal(err)
	}
	clientDocument := *document
	clientDocument.Raw = make(map[string]any, len(document.Raw))
	for key, value := range document.Raw {
		if key != "webhooks" {
			clientDocument.Raw[key] = value
		}
	}
	target := Generator{}
	for _, selected := range []bool{false, true} {
		options := generator.Options{}
		if selected {
			options.Selection = &generator.Selection{Operations: []string{"json"}}
		}
		plan, diagnostics, err := target.Prepare(&clientDocument, options)
		if err != nil || len(diagnostics) > 0 {
			t.Fatalf("prepare: %v %v", err, diagnostics)
		}
		value, _ := plan.Value("typescript")
		prepared := value.(*sourcePlan)
		before := append([]runtimeTemplateArtifact(nil), prepared.runtimeArtifacts...)
		artifacts, err := target.Emit(plan)
		if err != nil {
			t.Fatal(err)
		}
		emitted := make(map[string][]byte)
		for _, artifact := range artifacts {
			emitted[artifact.Path] = artifact.Data
		}
		var streamed []Artifact
		err = target.EmitTo(plan, generator.ArtifactSinkFunc(func(artifact Artifact) error { streamed = append(streamed, artifact); return nil }))
		if err != nil {
			t.Fatal(err)
		}
		if len(streamed) != len(artifacts) {
			t.Fatal("Emit/EmitTo artifact count differs")
		}
		for _, artifact := range streamed {
			if !bytes.Equal(emitted[artifact.Path], artifact.Data) {
				t.Fatalf("Emit/EmitTo changed %s", artifact.Path)
			}
		}
		if !reflect.DeepEqual(before, prepared.runtimeArtifacts) {
			t.Fatal("emission changed the prepared runtime closure")
		}
		_, xml := emitted["internal/runtime/xml-codec.ts"]
		if xml == selected {
			t.Fatalf("XML artifact inclusion selected=%t xml=%t", selected, xml)
		}
		for _, name := range []string{"security-basic", "wire-format-date", "http-multipart-request", "stream-sse", "links", "pagination"} {
			if emitted["internal/runtime/"+name+".ts"] != nil {
				t.Fatalf("unused runtime artifact %s", name)
			}
		}
		if strings.Contains(string(emitted["internal/client/factory.ts"]), "http-codecs") {
			t.Fatal("default client imported the full compatibility composition")
		}
	}
	client, err := target.Generate(&clientDocument, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	options, err := registry.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	withServer, err := target.Generate(document, options)
	if err != nil {
		t.Fatal(err)
	}
	server := make(map[string][]byte)
	for _, artifact := range withServer {
		server[artifact.Path] = artifact.Data
	}
	for _, artifact := range client {
		if !bytes.Equal(artifact.Data, server[artifact.Path]) {
			t.Fatalf("server add-on changed client identity/artifact %s", artifact.Path)
		}
	}
	if strings.Contains(string(server["server/webhook-runtime.ts"]), "xml-codec") || !strings.Contains(string(server["server/webhooks.ts"]), "./webhook-runtime.js") {
		t.Fatal("JSON webhook did not use its narrow composition")
	}
	compileTypeScriptArtifactSet(t, withServer, "consumer.ts", `import {createWebhookRouter} from './server/webhooks.js'; import {decodeInboundBody} from './server/runtime.js';void createWebhookRouter;void decodeInboundBody;`)
}
