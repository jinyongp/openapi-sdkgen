package typescript

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestExternalResponseLinkDoesNotFetchUnloadedTarget(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer server.Close()
	document, err := sdkgen.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Link IO","version":"1"},"paths":{"/source":{"get":{"responses":{"200":{"description":"OK","links":{"remote":{"operationRef":"` + server.URL + `/unloaded.json#/paths/~1target/get"}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 0 || len(diagnostics) != 1 || diagnostics[0].Code != "SDKGEN-W509" {
		t.Fatalf("unloaded target IO=%d diagnostics=%#v", calls.Load(), diagnostics)
	}
}

func TestExternalResponseLinkClosureRuntime(t *testing.T) {
	input, err := filepath.Abs("../../../test/fixtures/support-gaps/links/root.json")
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.CompileFile(input)
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
const seen=[];
const api=createClient({baseURL:"https://source.example.test",fetch:async(url)=>{
 seen.push(String(url));
 const response=new Response(JSON.stringify({id:"one /two"}),{headers:{"content-type":"application/json"}});
 Object.defineProperty(response,"url",{value:String(url)});return response;
}});
const source=await api.$operations.getSource.raw();
const target=await api.$links.getSource.item(source);
if(target.id!=="one /two"||typeof api.$links.getItem.back!=="function")throw new Error("external Link target or cycle missing");
if(seen.join(',')!=="https://source.example.test/source,https://target.example.test/v1/items/one%20%2Ftwo")throw new Error("external Link source/server/path mapping: "+seen);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, result)
	}
}

func TestExternalResponseLinkLockedRemoteClosure(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "root.json")
	remoteURL := "https://links.example.test/defs/target.json"
	remote, err := os.ReadFile("../../../test/fixtures/support-gaps/links/target.json")
	if err != nil {
		t.Fatal(err)
	}
	remote = []byte(strings.ReplaceAll(string(remote), "https://target.example.test/v1", "./v2"))
	digest := sha256.Sum256(remote)
	encoded := hex.EncodeToString(digest[:])
	if err := os.Mkdir(filepath.Join(directory, ".openapi-sdkgen-cache"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, ".openapi-sdkgen-cache", encoded), remote, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input+".openapi-sdkgen.lock", []byte(`{"version":1,"references":{"`+remoteURL+`":"`+encoded+`"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	root := `{"openapi":"3.2.0","info":{"title":"Remote Link closure","version":"1"},"paths":{
 "/source":{"get":{"operationId":"getSource","responses":{"200":{"description":"OK","links":{"item":{"operationRef":"` + remoteURL + `#/paths/~1items~1{id}/get","parameters":{"id":"one"}}}}}}},
 "/items/{id}":{"$ref":"` + remoteURL + `#/paths/~1items~1{id}"}}}`
	if err := os.WriteFile(input, []byte(root), 0600); err != nil {
		t.Fatal(err)
	}
	options := sdkgen.CompileOptions{RemoteRefAllowlist: []string{"https://links.example.test"}, Offline: true}
	document, err := sdkgen.CompileFileWithOptions(input, options)
	if err != nil {
		t.Fatal(err)
	}
	probe := `import { createClient } from "./index.js";
declare const api: ReturnType<typeof createClient>;
api.$links.getSource.item;
// @ts-expect-error unmounted source document stays outside the closure
api.$links.getItem.back;
`
	output := compileTypeScriptArtifactsWithProbe(t, document, "external-remote.probe.ts", probe)
	script := `
import { pathToFileURL } from "node:url";
const {createClient}=await import(pathToFileURL(process.argv[1]).href);
const seen=[];
const api=createClient({baseURL:'https://source.example.test',fetch:async(url)=>{
 seen.push(String(url));const response=new Response('{"id":"one"}',{headers:{'content-type':'application/json'}});Object.defineProperty(response,'url',{value:String(url)});return response;
}});
await api.$links.getSource.item(await api.$operations.getSource.raw());
if(seen.join(',')!=='https://source.example.test/source,https://links.example.test/defs/v2/items/one')throw new Error('external relative server resolved against source response: '+seen);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, result)
	}
	if err := os.Remove(filepath.Join(directory, ".openapi-sdkgen-cache", encoded)); err != nil {
		t.Fatal(err)
	}
	if _, err := sdkgen.CompileFileWithOptions(input, options); err == nil {
		t.Fatal("offline missing cache accepted")
	}
	options.RemoteRefAllowlist = nil
	if _, err := sdkgen.CompileFileWithOptions(input, options); err == nil {
		t.Fatal("unallowlisted remote closure accepted")
	}
}

func TestExternalResponseLinkEntryFragmentUsesPhysicalMount(t *testing.T) {
	directory := t.TempDir()
	target, err := os.ReadFile("../../../test/fixtures/support-gaps/links/target.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "target.json"), target, 0600); err != nil {
		t.Fatal(err)
	}
	root := `{"openapi":"3.2.0","info":{"title":"Physical mount Link","version":"1"},"paths":{
 "/source":{"get":{"operationId":"getSource","responses":{"200":{"description":"OK","links":{"item":{"operationRef":"#/paths/~1items~1{id}/get","parameters":{"id":"one"}}}}}}},
 "/items/{id}":{"$ref":"target.json#/paths/~1items~1{id}"}}}`
	input := filepath.Join(directory, "root.json")
	if err := os.WriteFile(input, []byte(root), 0600); err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.CompileFile(input)
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifactsWithProbe(t, document, "physical-mount.probe.ts", `import {createClient} from './index.js';declare const api:ReturnType<typeof createClient>;api.$links.getSource.item;`)
	script := `
import {pathToFileURL} from 'node:url';const {createClient}=await import(pathToFileURL(process.argv[1]).href);
const seen=[];const api=createClient({baseURL:'https://source.example.test',fetch:async(url)=>{seen.push(String(url));const response=new Response('{"id":"one"}',{headers:{'content-type':'application/json'}});Object.defineProperty(response,'url',{value:String(url)});return response;}});
await api.$links.getSource.item(await api.$operations.getSource.raw());
if(seen.join(',')!=='https://source.example.test/source,https://target.example.test/v1/items/one')throw new Error('physical mount server/path changed: '+seen);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, result)
	}
}

func TestExternalResponseLinkImportedFragmentUsesSourceServer(t *testing.T) {
	directory := t.TempDir()
	sourceData, err := os.ReadFile("../../../test/fixtures/support-gaps/links/source.json")
	if err != nil {
		t.Fatal(err)
	}
	targetData, err := os.ReadFile("../../../test/fixtures/support-gaps/links/target.json")
	if err != nil {
		t.Fatal(err)
	}
	var source, target map[string]any
	if err := json.Unmarshal(sourceData, &source); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(targetData, &target); err != nil {
		t.Fatal(err)
	}
	source["servers"] = []any{map[string]any{"url": "https://imported.example.test/v3"}}
	paths := source["paths"].(map[string]any)
	paths["/items/{id}"] = target["paths"].(map[string]any)["/items/{id}"]
	link := paths["/source"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)["links"].(map[string]any)["item"].(map[string]any)
	link["operationRef"] = "#/paths/~1items~1{id}/get"
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "source.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	root := `{"openapi":"3.2.0","info":{"title":"Imported fragment Link","version":"1"},"paths":{"/source":{"$ref":"source.json#/paths/~1source"},"/items/{id}":{"$ref":"source.json#/paths/~1items~1{id}"}}}`
	input := filepath.Join(directory, "root.json")
	if err := os.WriteFile(input, []byte(root), 0600); err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.CompileFile(input)
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `
import {pathToFileURL} from 'node:url';const {createClient}=await import(pathToFileURL(process.argv[1]).href);
const seen=[];const api=createClient({baseURL:'https://source.example.test',fetch:async(url)=>{seen.push(String(url));const response=new Response('{"id":"one"}',{headers:{'content-type':'application/json'}});Object.defineProperty(response,'url',{value:String(url)});return response;}});
await api.$links.getSource.item(await api.$operations.getSource.raw());
if(seen.join(',')!=='https://source.example.test/source,https://imported.example.test/v3/items/one')throw new Error('imported fragment server changed: '+seen);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, result)
	}
}

func TestExternalResponseLinkChangedMountPathIsIsolated(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"source.json", "target.json"} {
		data, err := os.ReadFile(filepath.Join("../../../test/fixtures/support-gaps/links", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	root := `{"openapi":"3.2.0","info":{"title":"Relocated mount","version":"1"},"paths":{"/source":{"$ref":"source.json#/paths/~1source"},"/proxy/{id}":{"$ref":"target.json#/paths/~1items~1{id}"}}}`
	input := filepath.Join(directory, "root.json")
	if err := os.WriteFile(input, []byte(root), 0600); err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.CompileFile(input)
	if err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, value := range diagnostics {
		if value.Code == "SDKGEN-W509" && strings.Contains(value.Message, "target path") {
			found = true
		}
	}
	if !found {
		t.Fatalf("external target path relocation not isolated: %#v", diagnostics)
	}
	compileTypeScriptArtifactsWithProbe(t, document, "relocated-mount.probe.ts", `import {createClient} from './index.js';declare const api:ReturnType<typeof createClient>;
api.$operations.getSource;
api.$operations.getItem;
// @ts-expect-error helper cannot faithfully call the external target path
api.$links.getSource.item;
`)
}

func TestExternalResponseLinkMultipleMountIsAmbiguous(t *testing.T) {
	input, err := filepath.Abs("../../../test/fixtures/support-gaps/links/root.json")
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	for _, name := range []string{"source.json", "target.json"} {
		body, err := os.ReadFile(filepath.Join(filepath.Dir(input), name))
		if err != nil {
			t.Fatal(err)
		}
		if name == "target.json" {
			body = []byte(strings.ReplaceAll(string(body), `"operationId": "getItem",`, ""))
		}
		if err := os.WriteFile(filepath.Join(directory, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	root := `{"openapi":"3.2.0","info":{"title":"Ambiguous mount","version":"1"},"paths":{
 "/source":{"$ref":"source.json#/paths/~1source"},
 "/items/{id}":{"$ref":"target.json#/paths/~1items~1{id}"},
 "/second/{id}":{"$ref":"target.json#/paths/~1items~1{id}"}}}`
	path := filepath.Join(directory, "root.json")
	if err := os.WriteFile(path, []byte(root), 0600); err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.CompileFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, value := range diagnostics {
		if value.Code == "SDKGEN-W509" && strings.Contains(value.Message, "does not uniquely name") {
			found = true
		}
	}
	if !found {
		t.Fatalf("ambiguous source target not isolated: %#v", diagnostics)
	}
}
