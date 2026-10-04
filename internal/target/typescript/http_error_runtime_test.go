package typescript

import (
	"encoding/json"
	"os/exec"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func httpErrorFixture(t *testing.T) []byte {
	t.Helper()
	ref := func(name string) map[string]any { return map[string]any{"$ref": "#/components/schemas/" + name} }
	response := func(schema string) map[string]any {
		if schema == "" {
			return map[string]any{"description": "bodyless"}
		}
		return map[string]any{"description": "body", "content": map[string]any{"application/json": map[string]any{"schema": ref(schema)}}}
	}
	operation := func(id string, responses map[string]any) map[string]any {
		return map[string]any{"get": map[string]any{"operationId": id, "responses": responses}}
	}
	schemas := map[string]any{
		"Validation": map[string]any{"type": "object", "required": []string{"fields"}, "properties": map[string]any{"fields": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "additionalProperties": false},
		"Problem":    map[string]any{"type": "object", "xml": map[string]any{"name": "problem"}, "required": []string{"message"}, "properties": map[string]any{"message": map[string]any{"type": "string"}}, "additionalProperties": false},
		"Fallback":   map[string]any{"type": "object", "required": []string{"fallback"}, "properties": map[string]any{"fallback": map[string]any{"type": "boolean"}}, "additionalProperties": false},
		"Rate":       map[string]any{"type": "object", "required": []string{"retry"}, "properties": map[string]any{"retry": map[string]any{"type": "integer"}}, "additionalProperties": false},
		"Alpha":      map[string]any{"type": "object", "required": []string{"alpha"}, "properties": map[string]any{"alpha": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}},
		"Beta":       map[string]any{"type": "object", "required": []string{"beta"}, "properties": map[string]any{"beta": map[string]any{"type": "integer"}}},
	}
	envelope := func(codes []string, details string) map[string]any {
		return map[string]any{"type": "object", "required": []string{"error"}, "properties": map[string]any{"error": map[string]any{"type": "object", "required": []string{"code", "details"}, "properties": map[string]any{"code": map[string]any{"type": "string", "enum": codes}, "details": ref(details)}}}}
	}
	schemas["Known"] = map[string]any{"oneOf": []any{envelope([]string{"SHARED"}, "Alpha"), envelope([]string{"OTHER", "SECOND"}, "Beta")}}
	schemas["OtherKnown"] = envelope([]string{"SHARED"}, "Beta")
	media := map[string]any{"description": "media", "content": map[string]any{"application/json": map[string]any{"schema": ref("Validation")}, "application/xml": map[string]any{"schema": ref("Problem")}}}
	paths := map[string]any{
		"/basic":          operation("basic", map[string]any{"200": response(""), "400": response("Validation"), "404": response("Problem"), "default": response("Fallback")}),
		"/range":          operation("ranges", map[string]any{"200": response(""), "400": response("Validation"), "4XX": response("Rate"), "default": response("Fallback")}),
		"/media":          operation("media", map[string]any{"200": response(""), "400": media}),
		"/empty":          operation("empty", map[string]any{"200": response(""), "405": response("")}),
		"/known":          operation("known", map[string]any{"200": response(""), "400": response("Known")}),
		"/other":          operation("other", map[string]any{"200": response(""), "400": response("OtherKnown")}),
		"/stream":         operation("stream", map[string]any{"200": map[string]any{"description": "items", "content": map[string]any{"application/x-ndjson": map[string]any{"itemSchema": ref("Problem")}}}, "400": response("Validation")}),
		"/resources/{id}": operation("resource", map[string]any{"200": response(""), "400": response("Validation")}),
	}
	paths["/resources/{id}"].(map[string]any)["parameters"] = []any{map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}}}
	data, err := json.Marshal(map[string]any{"openapi": "3.2.0", "info": map[string]any{"title": "HTTP errors", "version": "1"}, "components": map[string]any{"schemas": schemas}, "paths": paths})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDeclaredHTTPErrorGuardRuntimeAndTypes(t *testing.T) {
	document, err := sdkgen.Compile(httpErrorFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	options := generator.Options{Clients: map[string]generator.Client{"errors": {Selection: &generator.Selection{Operations: []string{"basic", "ranges", "media", "empty", "known", "other", "stream", "resource"}}}}}
	artifacts, err := (Generator{}).Generate(document, options)
	if err != nil {
		t.Fatal(err)
	}
	probe := `import {createClient,isOperationHTTPError,isErrorCode,type APIError,type HTTPErrorFor,type OperationHTTPError,type ErrorCode} from './index.js';
const api: ReturnType<typeof createClient> = createClient({baseURL:'https://api.test'});
export function basic(error: unknown): void {
  if (isOperationHTTPError(error,api.$operations.basic)) {
    if(error.status === 400) { error.data.fields.push('new'); }
    else if(error.status === 404) { const message: string = error.data.message; void message; }
    else { const fallback: boolean = error.data.fallback; void fallback; }
    // @ts-expect-error no transport failures in the declared HTTP guard
    const transport: 'NETWORK_ERROR' = error.code; void transport;
  }
  if(isOperationHTTPError(error,api.$operations.basic.raw) && error.status === 400) error.data.fields.push('raw');
  if(isOperationHTTPError(error,api.$operations.stream.stream)) error.data.fields.push('stream');
  if(isOperationHTTPError(error,api.resources('id').get)) error.data.fields.push('resource');
  if(isOperationHTTPError(error,api.$operations.ranges) && error.status === 400) error.data.fields.push('exact');
  if(isOperationHTTPError(error,api.$operations.media)) {
    if(error.contentType === 'application/json') error.data.fields.push('media');
    else { const message: string = error.data.message; void message; }
  }
  if(isOperationHTTPError(error,api.$operations.empty)) { const absent: void = error.data; const code:'HTTP_405'=error.code; void absent; void code; }
  if(isOperationHTTPError(error,api.$operations.known)) {
    if(error.code === 'SHARED') { if(error.details !== undefined) error.details.alpha.push('new'); error.data.error.details.alpha.push('new'); }
    else { if(error.details !== undefined) error.details.beta=3; }
  }
  if(isOperationHTTPError(error,api.$operations.other) && error.details !== undefined) { error.details.beta=4; }
  if(isErrorCode(error,'arbitrary-code')) { const code:'arbitrary-code'=error.code; void code; }
}
export type ByMethod = OperationHTTPError<typeof api.$operations.basic>;
export type ByBody = HTTPErrorFor<400,{fields:string[]},'application/json'>;
export type LegacyOne = APIError<'legacy'>;
export type LegacyTwo = APIError<'legacy',{field:string}>;
export const httpCode: ErrorCode = 'HTTP_404';
`
	output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", probe)
	script := `
import assert from 'node:assert/strict'; import {pathToFileURL} from 'node:url';
const load=file=>import(pathToFileURL(process.argv[1]+'/'+file));
const {createClient,isOperationHTTPError,APIError,isErrorCode}=await load('index.js');
const named=(await load('clients/errors/index.js')).createClient, selective=await load('selective/index.js');
const names=['basic','ranges','media','empty','known','other','stream','resource'];
const prepared=await selective.loadOperations(names.map(name=>selective.operations[name]));
const failure=async promise=>{try{await promise;throw Error('expected HTTP failure');}catch(error){if(!(error instanceof APIError))throw error;return error;}};
for(const create of [createClient,named,options=>selective.createClient({...options,operations:prepared})]) {
 let status=400,body={fields:['name']},media='application/json',network=false;
 const api=create({baseURL:'https://api.test',fetch:async()=>{if(network)throw Error('network');return new Response(body===undefined?null:typeof body==='string'?body:JSON.stringify(body),{status,headers:body===undefined?{}:{'content-type':media}});}});
 for(const call of [api.$operations.basic,api.$operations.basic.raw,api.$routes['GET /basic'],api.basic.get]) {
   const error=await failure(call());assert.equal(isOperationHTTPError(error,call),true);
   assert.equal(isOperationHTTPError(error,api.$operations.other),false);
 }
 const guarded=await failure(api.$operations.basic());
 assert.equal(isOperationHTTPError({...guarded},api.$operations.basic),false);
 assert.equal(isOperationHTTPError(new APIError({code:'HTTP_400',message:'forged',status:400,data:{fields:['name']}}),api.$operations.basic),false);
 assert.equal(isOperationHTTPError(guarded,()=>{}),false);
 for(const [field,value] of [['status',404],['code','OTHER'],['data',{fields:['name']}],['contentType','application/xml'],['details',['forged']],['response',new Response(null,{status:400})]]) {
   const fresh=await failure(api.$operations.basic());Reflect.set(fresh,field,value);assert.equal(isOperationHTTPError(fresh,api.$operations.basic),false,field);
 }
 const tampered=await failure(api.$operations.basic());tampered.data.fields.push(3);assert.equal(isOperationHTTPError(tampered,api.$operations.basic),false);
 const changedMedia=await failure(api.$operations.basic());changedMedia.response.headers.set('content-type','application/xml');assert.equal(isOperationHTTPError(changedMedia,api.$operations.basic),false);
 status=404;body={message:'missing'};assert.equal(isOperationHTTPError(await failure(api.$operations.basic()),api.$operations.basic),true);
 status=503;body={fallback:true};assert.equal(isOperationHTTPError(await failure(api.$operations.basic()),api.$operations.basic),true);
 status=401;body={retry:2};assert.equal(isOperationHTTPError(await failure(api.$operations.ranges()),api.$operations.ranges),true);
 status=400;body={fields:['name']};assert.equal(isOperationHTTPError(await failure(api.$operations.ranges()),api.$operations.ranges),true);
 body='<problem><message>missing</message></problem>';media='application/xml';const xml=await failure(api.$operations.media());assert.equal(xml.contentType,'application/xml');assert.equal(isOperationHTTPError(xml,api.$operations.media),true);
 status=405;body=undefined;const empty=await failure(api.$operations.empty());assert.equal(isOperationHTTPError(empty,api.$operations.empty),true,JSON.stringify({code:empty.code,status:empty.status,data:empty.data,details:empty.details,fields:empty.fields,contentType:empty.contentType,header:empty.response?.headers.get('content-type')}));
 status=400;body={error:{code:'SHARED',details:{alpha:['one']}}};media='application/json';const known=await failure(api.$operations.known());assert.equal(isOperationHTTPError(known,api.$operations.known),true);assert.equal(isOperationHTTPError(known,api.$operations.other),false);
 known.details.alpha.push(2);assert.equal(isOperationHTTPError(known,api.$operations.known),false);
 for(const code of ['OTHER','SECOND']) {body={error:{code,details:{beta:2}}};assert.equal(isOperationHTTPError(await failure(api.$operations.known()),api.$operations.known),true);}
 body={fields:['stream']};const stream=api.$operations.stream.stream();const streamError=await failure(stream[Symbol.asyncIterator]().next());assert.equal(isOperationHTTPError(streamError,api.$operations.stream.stream),true);
 const resource=api.resources('one').get;assert.equal(isOperationHTTPError(await failure(resource()),resource),true);assert.equal(isOperationHTTPError(await failure(resource.raw()),resource.raw),true);
 status=500;body={undeclared:true};assert.equal(isOperationHTTPError(await failure(api.$operations.other()),api.$operations.other),false);
 status=400;body={fields:[3]};const decode=await failure(api.$operations.basic());assert.equal(decode.code,'RESPONSE_DECODE_FAILED');assert.equal(isOperationHTTPError(decode,api.$operations.basic),false);
 network=true;const transport=await failure(api.$operations.basic());assert.equal(isOperationHTTPError(transport,api.$operations.basic),false);
 assert.equal(isErrorCode(new APIError({code:'arbitrary-code',message:'custom'}),'arbitrary-code'),true);
}
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
		t.Fatalf("execute HTTP guard: %v\n%s", err, result)
	}
	compileDeclarationConsumers(t, artifacts, probe)
}
