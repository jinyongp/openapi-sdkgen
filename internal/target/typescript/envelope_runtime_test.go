package typescript

import (
	"os/exec"
	"path/filepath"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
)

func TestOptionalEnvelopeRuntimeFallback(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Envelope views","version":"1"},"paths":{"/optional":{"get":{"operationId":"optionalData","responses":{"200":{"description":"Done","content":{"application/json":{"schema":{"type":"object","required":["meta"],"properties":{"data":{"type":"boolean"},"meta":{"type":"string"}}}}}}},"x-envelope":"data"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `import {pathToFileURL} from 'node:url';const {createClient}=await import(pathToFileURL(process.argv[1]).href);let calls=0;const api=createClient({baseURL:'https://example.test',fetch:async()=>new Response(JSON.stringify(++calls===2?{meta:'missing'}:{data:false,meta:'whole'}),{headers:{'content-type':'application/json'}})});if(await api.$operations.optionalData()!==false)throw new Error('false data treated as absent');const fallback=await api.$operations.optionalData();if(JSON.stringify(fallback)!=='{"meta":"missing"}')throw new Error('missing data lost complete fallback');const raw=await api.$operations.optionalData.raw();if(JSON.stringify(raw.data)!=='{"data":false,"meta":"whole"}')throw new Error('Raw projected envelope');`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("envelope consumer: %v\n%s", err, result)
	}
}

func TestClosedOutputWriteOnlyRuntimeContract(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Output projection","version":"1"},"paths":{"/thing":{"get":{"operationId":"readThing","responses":{"400":{"description":"Invalid","content":{"application/json":{"schema":{"type":"object","required":["field"],"properties":{"field":{"type":"string"},"secret":{"type":"string","writeOnly":true}},"additionalProperties":false}}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `import {pathToFileURL} from 'node:url';const {createClient}=await import(pathToFileURL(process.argv[1]).href);let secret=true;const api=createClient({baseURL:'https://example.test',fetch:async()=>Response.json(secret?{field:'title',secret:'hidden'}:{field:'title'},{status:400})});try{await api.$operations.readThing();throw new Error('unexpected success')}catch(error){if(error.code!=='HTTP_400'||error.data.secret!=='hidden')throw error};secret=false;try{await api.$operations.readThing();throw new Error('unexpected success')}catch(error){if(error.code!=='HTTP_400'||error.data.field!=='title')throw error}`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("writeOnly output consumer: %v\n%s", err, result)
	}
}

func TestResponsePropertyProfileRuntime(t *testing.T) {
	document, err := compiler.CompileFile("../../../test/fixtures/response-properties.json")
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `import {pathToFileURL} from 'node:url';const {createClient}=await import(pathToFileURL(process.argv[1]).href);const api=createClient({baseURL:'https://example.test',fetch:async(url)=>{const path=new URL(url).pathname;if(path==='/closed')return Response.json({id:'known',secret:'preserve',extra:'from-extra'});if(path==='/choice')return Response.json({left:'selected',right:2});if(path==='/echo/from-extra')return new Response('linked',{headers:{'content-type':'text/plain'}});throw new Error('unexpected path')}});const raw=await api.$operations.readClosed.raw();if(raw.data.id!=='known'||raw.data.secret!=='preserve')throw new Error('unknown response values lost');if(await api.$links.readClosed.Extra(raw)!=='linked')throw new Error('Link lost undeclared source body');const choice=await api.$operations.readChoice();if(choice.left!=='selected'||choice.right!==2)throw new Error('strict branch preference lost');`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("response property consumer: %v\n%s", err, result)
	}
}

func TestPaginationItemVariantsRuntime(t *testing.T) {
	document, err := compiler.CompileFile("../../../test/fixtures/pagination-item-variants.json")
	if err != nil {
		t.Fatal(err)
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `import {pathToFileURL} from 'node:url';const {createClient}=await import(pathToFileURL(process.argv[1]).href);const api=createClient({baseURL:'https://example.test',fetch:async(url)=>{const parsed=new URL(url),second=parsed.searchParams.has('cursor');return Response.json({data:parsed.pathname==='/names'?[second?'x':'long']:[second?[]:['first']],meta:{pagination:{nextCursor:second?null:'next'}}},{status:second?206:200})}});const names=[];for await(const item of api.$operations.listNames.paginate({})){names.push(item);JSON.stringify(item)};if(names.join(',')!=='long,x')throw new Error('first response constraints applied to second item');const tuples=[];for await(const item of api.$operations.listTuples.paginate({})){tuples.push(item);JSON.stringify(item)};if(JSON.stringify(tuples)!=='[["first"],[]]')throw new Error('optional tuple prefix rejected');`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("pagination item consumer: %v\n%s", err, result)
	}
}
