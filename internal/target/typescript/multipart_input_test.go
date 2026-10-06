package typescript

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestMultipartBinaryInputContracts(t *testing.T) {
	for _, version := range []string{"3.0.3", "3.1.1", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			encoded := `{"type":"string","contentEncoding":"base64"}`
			if version == "3.0.3" {
				encoded = `{"type":"string","format":"byte"}`
			}
			raw := `{"type":"string","format":"binary"}`
			if version != "3.0.3" {
				raw = `{"contentMediaType":"image/png"}`
			}
			document, err := sdkgen.Compile([]byte(fmt.Sprintf(`{
"openapi":%q,"info":{"title":"Multipart input","version":"1"},
"components":{"schemas":{
  "File":{"type":"string","format":"binary"},
  "Upload":{"type":"object","required":["file","files","encoded"],"properties":{
    "file":{"$ref":"#/components/schemas/File"},
    "files":{"type":"array","items":{"$ref":"#/components/schemas/File"}},
    "encoded":%s}}}},
"paths":{
  "/upload":{"post":{"operationId":"upload","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"$ref":"#/components/schemas/Upload"}}}},"responses":{"204":{"description":"ok"}}}},
  "/raw":{"post":{"operationId":"rawUpload","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["file"],"properties":{"file":%s}},"encoding":{"file":{"contentType":"application/octet-stream"}}}}},"responses":{"204":{"description":"ok"}}}},
  "/json":{"post":{"operationId":"jsonUpload","requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Upload"}}}},"responses":{"204":{"description":"ok"}}}}
}}`, version, encoded, raw)))
			if err != nil {
				t.Fatal(err)
			}
			artifacts, err := SourceArtifacts(document)
			if err != nil {
				t.Fatal(err)
			}
			compileDeclarationConsumers(t, artifacts, `import {createClient, type RouteInput} from './index.js';
type BinaryBody = Blob | ArrayBuffer | ArrayBufferView;
type Equal<A,B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false;
type Expect<T extends true> = T;
export type Contracts = [
 Expect<Equal<RouteInput<'POST /upload'>['body']['file'], BinaryBody | string>>,
 Expect<Equal<RouteInput<'POST /json'>['body']['file'], string>>
];
const api: ReturnType<typeof createClient> = createClient({baseURL:'https://api.test'});
for (const file of [new File(['one'],'one.txt'), new Blob(['two']), new ArrayBuffer(2), new Uint8Array([3]), '문자열']) {
 void api.$operations.upload({body:{file,files:[file],encoded:'aGk='}});
 void api.$operations.rawUpload({body:{file}});
}
// @ts-expect-error unrelated objects are not file input
void api.$operations.upload({body:{file:{},files:[],encoded:'aGk='}});
// @ts-expect-error base64 schema stays a string
void api.$operations.upload({body:{file:'one',files:[],encoded:new Blob()}});
// @ts-expect-error shared JSON component stays a string
void api.$operations.jsonUpload({body:{file:new Blob(),files:[],encoded:'aGk='}});
`)
			output := compileTypeScriptArtifacts(t, document)
			script := `import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
const {createClient} = await import(pathToFileURL(process.argv[1]));
let calls = 0;
let expected;
let filename;
const api = createClient({baseURL:'https://api.test',fetch:async(input,init)=>{
 calls++;
 const request = new Request(input,init);
 const form = await request.formData();
 for (const name of new URL(request.url).pathname === '/raw' ? ['file'] : ['file','files']) {
  const part = form.get(name);
  assert.equal(part.type,'application/octet-stream');
  assert.deepEqual(new Uint8Array(await part.arrayBuffer()),expected);
  if(filename !== undefined) assert.equal(part.name,filename);
 }
 return new Response(null,{status:204});
}});
const bytes = new Uint8Array([0,127,255]);
for (const file of [new File([bytes],'original.bin'),new Blob([bytes]),bytes.buffer,new Uint8Array([9,0,127,255,9]).subarray(1,4),new DataView(bytes.buffer),'한글']) {
 expected = typeof file === 'string' ? new TextEncoder().encode(file) : bytes;
 filename = file instanceof File ? file.name : undefined;
 await api.$operations.upload({body:{file,files:[file],encoded:'aGk='}});
 await api.$operations.rawUpload({body:{file}});
}
const before = calls;
for(const file of [{},42,true,[],null]) {
 await assert.rejects(api.$operations.upload({body:{file,files:[],encoded:'aGk='}}),error=>error.code === 'REQUEST_ENCODE_FAILED');
}
await assert.rejects(api.$operations.jsonUpload({body:{file:new Blob(),files:[],encoded:'aGk='}}),error=>error.code === 'REQUEST_ENCODE_FAILED');
assert.equal(calls,before);
`
			if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
				t.Fatalf("generated multipart input contracts: %v\n%s", err, result)
			}
		})
	}
}

func TestMultipartBinaryCompositionAndReferenceAssertions(t *testing.T) {
	for _, version := range []string{"3.1.1", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			document, err := sdkgen.Compile([]byte(fmt.Sprintf(`{
"openapi":%q,"info":{"title":"Multipart composition","version":"1"},
"components":{"schemas":{
"Binary":{"type":"string","format":"binary"},
"Base":{"type":"object","required":["file","count"],"properties":{"file":{"$ref":"#/components/schemas/Binary"},"count":{"type":"integer"}}}}},
"paths":{
"/refs":{"post":{"operationId":"refs","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"$ref":"#/components/schemas/Base","required":["extra"],"properties":{"file":{"type":"string","maxLength":100},"extra":{"type":"string","format":"binary"}},"anyOf":[{"properties":{"file":{"type":"string","maxLength":100}}},{"properties":{"file":{"type":"string","maxLength":200}}}]}}}},"responses":{"204":{"description":"ok"}}}},
"/composed":{"post":{"operationId":"composed","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["file"],"properties":{"file":{"allOf":[{"$ref":"#/components/schemas/Binary"},{"type":"string","maxLength":100}]}}}}}},"responses":{"204":{"description":"ok"}}}},
"/alternative":{"post":{"operationId":"alternative","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"anyOf":[{"type":"object","required":["file"],"properties":{"file":{"contentMediaType":"image/png"}}}]}}}},"responses":{"204":{"description":"ok"}}}},
"/pdf":{"post":{"operationId":"pdf","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["file"],"properties":{"file":{"type":"string","format":"binary"}}},"encoding":{"file":{"contentType":"application/pdf"}}}}},"responses":{"204":{"description":"ok"}}}},
"/jsonfile":{"post":{"operationId":"jsonfile","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["file"],"properties":{"file":{"type":"string","format":"binary"}}},"encoding":{"file":{"contentType":"application/json"}}}}},"responses":{"204":{"description":"ok"}}}}
}}`, version)))
			if err != nil {
				t.Fatal(err)
			}
			artifacts, err := SourceArtifacts(document)
			if err != nil {
				t.Fatal(err)
			}
			compileDeclarationConsumers(t, artifacts, `import {createClient} from './index.js';
const api: ReturnType<typeof createClient> = createClient({baseURL:'https://api.test'});
void api.$operations.refs({body:{file:new File(['one'],'one.bin'),count:1,extra:new Blob()}});
void api.$operations.composed({body:{file:new Blob()}});
void api.$operations.alternative({body:{file:new Blob()}});
// @ts-expect-error $ref target requirements remain required
void api.$operations.refs({body:{extra:new Blob()}});
// @ts-expect-error occurrence sibling requirements remain required
void api.$operations.refs({body:{file:new Blob(),count:1}});
`)
			output := compileTypeScriptArtifacts(t, document)
			script := `import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
const {createClient} = await import(pathToFileURL(process.argv[1]));
let calls = 0;
const text = '{"name":"한글"}';
const api = createClient({baseURL:'https://api.test',fetch:async(input,init)=>{
 calls++;
 const request = new Request(input,init);
 const path = new URL(request.url).pathname;
 const form = await request.formData();
 const file = form.get('file');
 assert.ok(file instanceof File);
 assert.equal(await file.text(),text);
 assert.equal(file.type,path==='/alternative'?'image/png':path==='/pdf'?'application/pdf':path==='/jsonfile'?'application/json':'application/octet-stream');
 if(path==='/refs') { assert.equal(form.get('count'),'1'); assert.equal(await form.get('extra').text(),text); }
 return new Response(null,{status:204});
}});
for(const file of [text,new File([text],'original.bin'),new TextEncoder().encode(text)]) {
 await api.$operations.refs({body:{file,count:1,extra:file}});
 for(const name of ['composed','alternative','pdf','jsonfile']) await api.$operations[name]({body:{file}});
}
const before=calls;
await assert.rejects(api.$operations.refs({body:{extra:text}}),error=>error.code==='REQUEST_ENCODE_FAILED');
await assert.rejects(api.$operations.refs({body:{file:text,count:1}}),error=>error.code==='REQUEST_ENCODE_FAILED');
assert.equal(calls,before);
`
			if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
				t.Fatalf("multipart composition contracts: %v\n%s", err, result)
			}

		})
	}
}
