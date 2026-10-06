package typescript

import (
	"os/exec"
	"path/filepath"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestMultipartBinaryPositionalAndStreamingContracts(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
"openapi":"3.2.0","info":{"title":"Sequential file parts","version":"1"},
"paths":{
"/positional":{"post":{"operationId":"positional","requestBody":{"required":true,"content":{"multipart/mixed":{"schema":{"type":"array","items":{"allOf":[{"contentMediaType":"image/png"}]}},"itemEncoding":{}}}},"responses":{"204":{"description":"ok"}}}},
"/streaming":{"post":{"operationId":"streaming","requestBody":{"required":true,"content":{"multipart/mixed":{"itemSchema":{"allOf":[{"contentMediaType":"image/png"}]},"itemEncoding":{}}}},"responses":{"204":{"description":"ok"}}}}
}}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	compileDeclarationConsumers(t, artifacts, `import {createClient} from './index.js';
const api: ReturnType<typeof createClient> = createClient({baseURL:'https://api.test'});
void api.$operations.positional({body:[new File(['one'],'one.png'),new Blob(),new Uint8Array([1]),'한글']});
async function* files(): AsyncGenerator<Blob | string, void, unknown> { yield new Blob(); yield '한글'; }
void api.$operations.streaming({body:files()});
// @ts-expect-error raw streaming file parts reject unrelated objects
void api.$operations.streaming({body:(async function* (): AsyncGenerator<object, void, unknown> { yield {}; })()});
`)
	output := compileTypeScriptArtifacts(t, document)
	script := `import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
const {createClient} = await import(pathToFileURL(process.argv[1]));
let calls=0;
const text='한글';
const api=createClient({baseURL:'https://api.test',fetch:async(input,init)=>{
 calls++;
 const request=new Request(input,init);
 const body=await request.text();
 assert.equal((body.match(/Content-Type: image\/png/g)??[]).length,3,body);
 assert.equal((body.match(/\r\n\r\n한글\r\n/g)??[]).length,3,body);
 assert.ok(body.includes('filename="original.png"'),body);
 return new Response(null,{status:204});
}});
const files=[text,new File([text],'original.png',{type:'text/plain'}),new TextEncoder().encode(text)];
await api.$operations.positional({body:files});
await api.$operations.streaming({body:(async function*(){yield* files;})()});
assert.equal(calls,2);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("sequential multipart file contracts: %v\n%s", err, result)
	}
}
