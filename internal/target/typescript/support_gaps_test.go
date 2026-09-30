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

func supportGapFixture(t *testing.T, name string) []byte {
	t.Helper()
	input, err := os.ReadFile(filepath.Join("../../../test/fixtures/support-gaps", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func TestSupportGapSSEEventDefaultClientContract(t *testing.T) {
	document, err := sdkgen.Compile(supportGapFixture(t, "sse-event"))
	if err != nil {
		t.Fatal(err)
	}
	probe := `
import { createClient } from "./index.js";
declare const api: ReturnType<typeof createClient>;
const all = await api.$operations.buffered();
const data: string = all[0]!.data;
// @ts-expect-error schema-only provides buffered output
api.$operations.buffered.stream();
for await (const event of api.$operations.incremental.stream()) { const text: string=event.data; void text; }
const raw = await api.$operations.buffered.raw();
const empty: void = raw.data;
void data; void empty;
`
	output := compileTypeScriptArtifactsWithProbe(t, document, "sse-event-probe.ts", probe)
	script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
const wire='event: message\ndata: plain\nid: evt1\nretry: 4\n\nevent: json\ndata: {"a":1}\nid: \nretry: 0\n\n';
const expected=[{data:"plain",event:"message",id:"evt1",retry:4},{data:'{"a":1}',event:"json",id:"",retry:0}];
const sent=[];
const api=createClient({baseURL:"https://example.test",fetch:async(url,init)=>{
  if(init.body!==undefined) sent.push(await new Response(init.body).text());
  return new URL(url).pathname.startsWith('/publish-')?new Response(null,{status:204}):new Response(wire,{headers:{"content-type":"text/event-stream"}});
}});
const same=value=>{if(JSON.stringify(value)!==JSON.stringify(expected)) throw new Error("Event fields changed: "+JSON.stringify(value));};
same(await api.$operations.buffered());
same(await api.$operations.both({body:expected}));
const incremental=[];for await(const event of api.$operations.incremental.stream())incremental.push(event);same(incremental);
async function* events(){yield* expected;}
await api.$operations.publishBuffered({body:expected});
await api.$operations.publishIncremental({body:events()});
same(await api.$operations.both({body:events()}));
if(sent.some(body=>body!==wire)) throw new Error("Event encode fields changed");
const raw=await api.$operations.buffered.raw();
if(raw.data!==undefined||raw.response.bodyUsed||await raw.response.text()!==wire)throw new Error("raw Event body consumed");
try{await api.$operations.publishBuffered({body:[{data:3}]});throw new Error("invalid Event data accepted");}
catch(error){if(error.code!=="REQUEST_ENCODE_FAILED")throw error;}
const invalid=createClient({baseURL:"https://example.test",fetch:async()=>new Response('data: \n\n',{headers:{'content-type':'text/event-stream'}})});
try{await invalid.$operations.buffered();throw new Error("invalid buffered Event accepted");}
catch(error){if(error.code!=="RESPONSE_DECODE_FAILED")throw error;}
try{for await(const event of invalid.$operations.incremental.stream()){}throw new Error("invalid incremental Event accepted");}
catch(error){if(error.code!=="RESPONSE_DECODE_FAILED")throw error;}
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, result)
	}
}

func TestSupportGapSSEEventDefaultInboundContract(t *testing.T) {
	var input map[string]any
	if err := json.Unmarshal(supportGapFixture(t, "sse-event"), &input); err != nil {
		t.Fatal(err)
	}
	paths := input["paths"].(map[string]any)
	webhooks := map[string]any{}
	for _, pair := range [][2]string{{"batch", "/publish-buffered"}, {"stream", "/publish-incremental"}, {"both", "/both"}} {
		op := paths[pair[1]].(map[string]any)["post"].(map[string]any)
		webhooks[pair[0]] = map[string]any{"post": map[string]any{"requestBody": op["requestBody"], "responses": map[string]any{"204": map[string]any{"description": "OK"}}}}
	}
	input["paths"] = map[string]any{}
	input["webhooks"] = webhooks
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(encoded)
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
	artifacts, err := (Generator{}).Generate(document, options)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	writeTargetArtifacts(t, source, artifacts)
	for name, body := range map[string]string{"package.json": `{"type":"module"}`, "tsconfig.json": serverTSConfig} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tsc := filepath.Join("../../../test/typescript/node_modules/typescript/lib/tsc.js")
	if _, err := os.Stat(tsc); err != nil {
		t.Skipf("TypeScript compiler unavailable: %v", err)
	}
	if result, err := exec.Command("node", tsc, "--project", filepath.Join(source, "tsconfig.json")).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, result)
	}
	output := filepath.Join(directory, "output")
	if err := os.WriteFile(filepath.Join(output, "package.json"), []byte(`{"type":"module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `
import { pathToFileURL } from "node:url";
const { createWebhookRouter } = await import(pathToFileURL(process.argv[1]).href);
const wire='event: message\ndata: plain\nid: evt1\nretry: 4\n\nevent: json\ndata: {"a":1}\nid: \nretry: 0\n\n';
const expected=[{data:"plain",event:"message",id:"evt1",retry:4},{data:'{"a":1}',event:"json",id:"",retry:0}];
const seen=[];
const handler=async({body})=>{const values=[];for await(const event of body)values.push(event);seen.push(values);return {status:204};};
const router=createWebhookRouter({batch:{POST:handler},stream:{POST:handler},both:{POST:handler}},{routes:{batch:"/batch",stream:"/stream",both:"/both"}});
for(const path of ['batch','stream','both']){
 const response=await router.fetch(new Request('https://example.test/'+path,{method:'POST',headers:{'content-type':'text/event-stream'},body:wire}));
 if(response.status!==204)throw new Error(path+" inbound failed: "+response.status);
}
if(seen.length!==3||seen.some(value=>JSON.stringify(value)!==JSON.stringify(expected)))throw new Error("inbound Event mapping changed");
for(const path of ['batch','stream','both']){
 const invalid=await router.fetch(new Request('https://example.test/'+path,{method:'POST',headers:{'content-type':'text/event-stream'},body:'data: \n\n'}));
 if(invalid.status!==400)throw new Error("invalid inbound Event accepted: "+path+" "+invalid.status);
}
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "server/webhooks.js")).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, result)
	}
}

func TestSupportGapReferenceFixtures(t *testing.T) {
	for _, name := range []string{"schema-locations", "schema-array", "schema-percent"} {
		t.Run(name, func(t *testing.T) {
			for _, version := range []string{"3.0.3", "3.1.1", "3.2.0"} {
				input := supportGapFixture(t, name)
				input = []byte(strings.ReplaceAll(strings.ReplaceAll(string(input), "3.2.0", version), "3.1.1", version))
				document, err := sdkgen.Compile(input)
				if err == nil {
					_, err = SourceArtifacts(document)
				}
				if err != nil {
					t.Fatalf("%s %s: %v", name, version, err)
				}
			}
		})
	}
	if document, err := sdkgen.Compile(supportGapFixture(t, "sse-event")); err != nil {
		t.Fatal(err)
	} else if _, err := SourceArtifacts(document); err != nil {
		t.Fatal(err)
	}
}

func TestSupportGapUntypedSequentialFixture(t *testing.T) {
	document, err := sdkgen.Compile(supportGapFixture(t, "sequential-untyped"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SourceArtifacts(document); err != nil {
		t.Fatal(err)
	}
}

func TestSupportGapUntypedSequentialRuntimeAndTypes(t *testing.T) {
	for _, version := range []string{"3.0.3", "3.1.1", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			document, err := sdkgen.Compile([]byte(strings.ReplaceAll(string(supportGapFixture(t, "sequential-untyped")), "3.2.0", version)))
			if err != nil {
				t.Fatal(err)
			}
			probe := `
import { createClient } from "./index.js";
const api = createClient({baseURL:"https://example.test"});
for (const operation of [api.$operations.sse,api.$operations.ndjson,api.$operations.jsonl,api.$operations.jsonseq]) {
  const value: unknown = await operation();
  // @ts-expect-error decoded untyped media has no known string shape
  const text: string = value;
  // @ts-expect-error incremental calls require itemSchema
  operation.stream();
  const raw = await operation.raw();
  const empty: void = raw.data;
  void text; void empty;
}
`
			output := compileTypeScriptArtifactsWithProbe(t, document, "unknown-probe.ts", probe)
			script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
const bodies={sse:['text/event-stream','data: {"id":"one"}\n\n'],ndjson:['application/x-ndjson','{"id":"one"}\n'],jsonl:['application/jsonl','{"id":"one"}\n'],jsonseq:['application/json-seq','\x1e{"id":"one"}\n']};
let bad=false;
const api=createClient({baseURL:"https://example.test",fetch:async url=>{const [media,body]=bodies[new URL(url).pathname.slice(1)];return new Response(bad?'not-json\n':body,{headers:{"content-type":media}})}});
for (const [key,[media,body]] of Object.entries(bodies)) {
  if (api.$operations[key].stream !== undefined) throw new Error("untyped incremental API appeared");
  const raw=await api.$operations[key].raw();
  if(raw.data!==undefined||raw.response.bodyUsed||raw.response.body?.locked) throw new Error("raw consumed body");
  if(await raw.response.text()!==body) throw new Error("raw bytes changed");
  const decoded=await api.$operations[key]();
  if(!Array.isArray(decoded)||decoded.length!==1) throw new Error("built-in sequential codec absent: "+key);
}
bad=true;
try { await api.$operations.ndjson(); throw new Error("invalid JSON accepted"); }
catch(error){ if(error.code!=="RESPONSE_DECODE_FAILED") throw error; }
`
			if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}

func TestSupportGapReferencesValidateGeneratedRuntime(t *testing.T) {
	for _, name := range []string{"schema-locations", "schema-array", "schema-percent"} {
		t.Run(name, func(t *testing.T) {
			for _, version := range []string{"3.0.3", "3.1.1", "3.2.0"} {
				input := strings.ReplaceAll(strings.ReplaceAll(string(supportGapFixture(t, name)), "3.2.0", version), "3.1.1", version)
				document, err := sdkgen.Compile([]byte(input))
				if err != nil {
					t.Fatal(err)
				}
				output := compileTypeScriptArtifacts(t, document)
				script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
let body = '"valid"';
const api = createClient({baseURL:"https://example.test",fetch:async()=>new Response(body,{headers:{"content-type":"application/json"}})});
if (await api.$operations.use() !== "valid") throw new Error("reference changed output");
for (const invalid of ['"x"','7']) {
  body=invalid;
  try { await api.$operations.use(); throw new Error("invalid target value accepted"); }
  catch (error) { if (String(error).includes("invalid target value accepted")) throw error; }
}
`
				if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
					t.Fatalf("%s: %v\n%s", version, err, result)
				}
			}
		})
	}
}
