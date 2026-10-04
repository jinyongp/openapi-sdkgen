package typescript

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
)

func TestResponseTypeReachabilityRegression(t *testing.T) {
	ref := func(name string) map[string]any { return map[string]any{"$ref": "#/components/schemas/" + name} }
	response := func(media, name string) map[string]any {
		value := map[string]any{"description": "response"}
		if media != "" {
			value["content"] = map[string]any{media: map[string]any{"schema": ref(name)}}
		}
		return value
	}
	paths := map[string]any{}
	add := func(id, status, media string) {
		paths["/"+id] = map[string]any{"get": map[string]any{"operationId": id, "responses": map[string]any{status: response("application/json", "Item"), "default": response(media, "Problem")}}}
	}
	add("exact", "200", "application/json")
	add("covered", "2XX", "application/json")
	add("different", "200", "application/xml")
	add("wildcard", "200", "*/*")
	paths["/fallback"] = map[string]any{"get": map[string]any{"operationId": "fallback", "responses": map[string]any{"default": response("application/json", "Problem")}}}
	paths["/bodyless"] = map[string]any{"get": map[string]any{"operationId": "bodyless", "responses": map[string]any{"200": response("", ""), "default": response("application/json", "Problem")}}}
	paths["/pages"] = map[string]any{"get": map[string]any{"operationId": "pages", "x-pagination": "cursor", "parameters": []any{map[string]any{"name": "cursor", "in": "query", "schema": map[string]any{"type": "string"}}, map[string]any{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 1}}}, "responses": map[string]any{"2XX": response("application/json", "Page"), "default": response("application/json", "Problem")}}}
	paths["/envelope"] = map[string]any{"get": map[string]any{"operationId": "envelope", "x-envelope": "data", "responses": map[string]any{"2XX": response("application/json", "Envelope"), "default": response("application/json", "Problem")}}}
	paths["/stream"] = map[string]any{"get": map[string]any{"operationId": "stream", "responses": map[string]any{"2XX": map[string]any{"description": "stream", "content": map[string]any{"application/x-ndjson": map[string]any{"itemSchema": ref("Item")}}}, "default": response("application/x-ndjson", "Problem")}}}
	data, err := json.Marshal(map[string]any{"openapi": "3.2.0", "info": map[string]any{"title": "Reachability", "version": "1"}, "paths": paths, "components": map[string]any{"schemas": map[string]any{
		"Item":     map[string]any{"type": "object", "required": []string{"id"}, "properties": map[string]any{"id": map[string]any{"type": "string"}}},
		"Problem":  map[string]any{"type": "object", "xml": map[string]any{"name": "problem"}, "required": []string{"problem"}, "properties": map[string]any{"problem": map[string]any{"type": "string"}}},
		"Envelope": map[string]any{"type": "object", "required": []string{"data"}, "properties": map[string]any{"data": ref("Item")}},
		"Page":     map[string]any{"type": "object", "required": []string{"items", "pagination"}, "properties": map[string]any{"items": map[string]any{"type": "array", "items": ref("Item")}, "pagination": map[string]any{"type": "object", "properties": map[string]any{"nextCursor": map[string]any{"type": []string{"string", "null"}}}}}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(data)
	if err != nil {
		t.Fatal(err)
	}
	registry, _ := generator.NewAddonRegistry(generator.AddonServer)
	options, _ := registry.Resolve([]string{"server"})
	artifacts, err := (Generator{}).Generate(document, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range document.Operations {
		if operation.OperationID != "covered" {
			continue
		}
		output, err := operationOutputType(document, operation)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(output, "Problem") || !strings.Contains(output, "Item") {
			t.Fatalf("covered default leaked: %s", output)
		}
	}
	probe := `import {createClient} from './index.js';
const api:ReturnType<typeof createClient>=createClient({baseURL:'https://api.test'});
export async function check():Promise<void>{
 const exact=await api.$operations.exact.raw();
 if(exact.status===200){exact.data.id='new';
 // @ts-expect-error exact status excludes same-media default
 exact.data.problem;}
 else {const problem:string=exact.data.problem;void problem;}
 const covered=await api.$operations.covered();covered.id='new';
 // @ts-expect-error fully covered default is not a successful output
 covered.problem;
 const raw=await api.$operations.covered.raw();const status:200|201|202|203|204|205|206|207|208|209|210|211|212|213|214|215|216|217|218|219|220|221|222|223|224|225|226|227|228|229|230|231|232|233|234|235|236|237|238|239|240|241|242|243|244|245|246|247|248|249|250|251|252|253|254|255|256|257|258|259|260|261|262|263|264|265|266|267|268|269|270|271|272|273|274|275|276|277|278|279|280|281|282|283|284|285|286|287|288|289|290|291|292|293|294|295|296|297|298|299=raw.status;void status;
 // @ts-expect-error raw success cannot carry HTTP 400
 const failure:400=raw.status;void failure;
 const fallback=await api.$operations.fallback();fallback.problem='fallback';
 const media=await api.$operations.different.raw();if(media.contentType==='application/xml'){media.data.problem='xml';}else{media.data.id='json';}
 const wildcard=await api.$operations.wildcard.raw();if('problem' in wildcard.data)wildcard.data.problem='custom';
 const absent=await api.$operations.bodyless.raw();if(absent.contentType===undefined){const value:void=absent.data;void value;}else{absent.data.problem='body';}
 const envelope=await api.$operations.envelope();envelope.id='envelope';
 for await(const item of api.$operations.pages.paginate({})){item.id='page';}
 for await(const item of api.$operations.stream.stream()){item.id='stream';}
 // @ts-expect-error exact item stream shadows buffered default at every success status
 api.$operations.stream();
}
`
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", probe)
	script := `import assert from 'node:assert/strict';import {pathToFileURL} from 'node:url';
const {createClient}=await import(pathToFileURL(process.argv[1]+'/index.js'));
let status=200,body={id:'item'},media='application/json';
const api=createClient({baseURL:'https://api.test',fetch:async()=>new Response(typeof body==='string'?body:JSON.stringify(body),{status,headers:{'content-type':media}})});
assert.deepEqual(await api.$operations.exact(),{id:'item'});assert.equal((await api.$operations.exact.raw()).status,200);
status=202;body={problem:'fallback'};assert.deepEqual(await api.$operations.exact(),body);const fallback=await api.$operations.exact.raw();assert.equal(fallback.status,202);assert.deepEqual(fallback.data,body);
assert.deepEqual(await api.$operations.fallback(),body);
body={id:'range'};assert.deepEqual(await api.$operations.covered(),body);assert.deepEqual((await api.$operations.covered.raw()).data,body);
status=200;media='application/xml';body='<problem><problem>xml</problem></problem>';assert.deepEqual(await api.$operations.different(),{problem:'xml'});assert.equal((await api.$operations.different.raw()).contentType,'application/xml');
media='application/vendor+json';body={problem:'custom'};assert.deepEqual(await api.$operations.wildcard(),body);
media='application/json';assert.deepEqual(await api.$operations.bodyless(),body);
body={data:{id:'envelope'}};assert.deepEqual(await api.$operations.envelope(),body.data);
body={items:[{id:'page'}],pagination:{nextCursor:null}};const rows=[];for await(const item of api.$operations.pages.paginate({}))rows.push(item);assert.deepEqual(rows,body.items);
media='application/x-ndjson';body='{"id":"stream"}\n';const items=[];for await(const item of api.$operations.stream.stream())items.push(item);assert.deepEqual(items,[{id:'stream'}]);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
		t.Fatalf("native response reachability: %v\n%s", err, result)
	}
	compileDeclarationConsumers(t, artifacts, probe)
	verifyResponseStatusSelector(t, output)
}

func verifyResponseStatusSelector(t *testing.T, output string) {
	t.Helper()
	response := func(status, media string) ir.Response {
		value := ir.Response{Status: status, Raw: map[string]any{}}
		if media != "" {
			value.Content = []ir.MediaType{{ContentType: media, Schema: map[string]any{}, Raw: map[string]any{}}}
		}
		return value
	}
	fixtures := [][]ir.Response{
		{response("200", "application/json"), response("default", "application/json")},
		{response("2XX", "application/json"), response("default", "application/json")},
		{response("default", "application/json")},
		{response("200", "application/json"), response("default", "application/xml")},
		{response("200", "application/json"), response("default", "*/*")},
		{response("200", ""), response("default", "application/json")},
		{response("201", "application/problem+json"), response("2XX", "application/*+json"), response("default", "*/*")},
	}
	var sources []string
	for _, responses := range fixtures {
		wire, _, err := newWireRenderContext(wirePropertiesLiteral).operationResponseWireBodies(&ir.Document{}, ir.Operation{Responses: responses})
		if err != nil {
			t.Fatal(err)
		}
		allowed := map[string][]int{}
		for _, branch := range responseStatusBranches(responses, true) {
			media := ""
			if branch.media != nil {
				media = branch.media.ContentType
			}
			allowed[branch.response.Status+" "+media] = branch.statuses
		}
		data, err := json.Marshal(allowed)
		if err != nil {
			t.Fatal(err)
		}
		sources = append(sources, "{responses:"+wire+",allowed:"+string(data)+"}")
	}
	script := `import assert from 'node:assert/strict';import {pathToFileURL} from 'node:url';
const {selectResponseDefinition}=await import(pathToFileURL(process.argv[1]+'/internal/runtime/http/http-execution-support.js'));
const fixtures=[` + strings.Join(sources, ",") + `];let comparisons=0;
for(const fixture of fixtures)for(let status=200;status<300;status++){
 const witnessed=new Set();
 for(const media of [undefined,'application/json','application/xml','text/plain','application/problem+json','application/vendor+json']){
  const response=new Response(null,{status,headers:media===undefined?{}:{'content-type':media+'; charset=utf-8'}});
  const winner=selectResponseDefinition(fixture,response,true);comparisons++;
  if(winner!==undefined){const key=winner.status+' '+winner.contentType;assert.ok(fixture.allowed[key]?.includes(status),'selector winner excluded: '+key+' '+status);witnessed.add(key);}
 }
 for(const [key,statuses]of Object.entries(fixture.allowed))if(statuses.includes(status))assert.ok(witnessed.has(key),'planned branch has no runtime witness: '+key+' '+status);
}
assert.equal(comparisons,4200);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
		t.Fatalf("Go status plan / real runtime selector: %v\n%s", err, result)
	}
}
