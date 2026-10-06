package typescript

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestOperationInputDiagnosticsAndHints(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
 "openapi":"3.2.0","info":{"title":"Input DX","version":"1"},
 "paths":{
  "/orders":{"get":{"operationId":"listOrders","parameters":[{"name":"status","in":"query","schema":{"type":"array","items":{"type":"string","enum":["open","closed"]}}}],"responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"type":"object","required":["items"],"properties":{"items":{"type":"array","items":{"type":"string"}}}}}}}}}},
  "/orders/{id}":{"get":{"operationId":"getOrder","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}},{"name":"status","in":"query","schema":{"type":"array","items":{"type":"string","enum":["open","closed"]}}}],"responses":{"204":{"description":"OK"}}}},
  "/events":{"get":{"operationId":"watchEvents","parameters":[{"name":"status","in":"query","schema":{"type":"array","items":{"type":"string","enum":["open","closed"]}}}],"responses":{"200":{"description":"OK","content":{"application/x-ndjson":{"itemSchema":{"type":"string"}}}}}}}
 }
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	valid := `import {createClient, type RequestOptions, type RouteInput, type OperationInput} from './index.js';
type Equal<A,B> = (<T>()=>T extends A ? 1 : 2) extends (<T>()=>T extends B ? 1 : 2) ? true : false;
type Expect<T extends true> = T;
const api: ReturnType<typeof createClient> = createClient({baseURL:'https://api.test'});
const options: RequestOptions = {headers:new Headers(),signal:new AbortController().signal};
export type Contracts = [
 Expect<Equal<NonNullable<Parameters<typeof api.orders.list>[1]>['headers'],RequestOptions['headers']>>,
 Expect<Equal<NonNullable<Parameters<typeof api.orders.list.raw>[1]>['signal'],AbortSignal | undefined>>,
 Expect<Equal<OperationInput<typeof api.orders.list>,RouteInput<'GET /orders'>>>
];
void api.orders.list(); void api.orders.list(options); void api.orders.list(undefined,options);
void api.orders.list(undefined); void api.orders.list({},undefined);
const args: Parameters<typeof api.orders.list> = [{query:{status:['open']}},options];
void api.orders.list(...args);
void api.orders.list({query:{status:['open']}},options);
void api.orders.list.raw(options); void api.orders('one').get(options);
void api.orders('one').get({query:{status:['closed']}},options);
void api.$operations.listOrders({query:{status:['open']}});
void api.$routes['GET /orders'](options);
void api.$operations.watchEvents.stream(options);
void api.$operations.watchEvents.stream({query:{status:['open']}},options);
// @ts-expect-error options belong in their own argument
void api.orders.list({query:{status:['open']},headers:new Headers()});
// @ts-expect-error options-only calls have only one argument
void api.orders.list(options,options);
// @ts-expect-error raw calls keep the same input/options boundary
void api.orders.list.raw({query:{status:['open']},headers:new Headers()});
// @ts-expect-error resource-bound input keeps the same boundary
void api.orders('one').get({query:{status:['open']},headers:new Headers()});
// @ts-expect-error streams keep the same boundary
void api.$operations.watchEvents.stream({query:{status:['open']},headers:new Headers()});
// @ts-expect-error stream options-only calls have only one argument
void api.$operations.watchEvents.stream(options,options);
`
	compileDeclarationConsumers(t, artifacts, valid)
	source := t.TempDir()
	writeTargetArtifacts(t, source, artifacts)
	consumer := `import {createClient} from './index.js';
const api: ReturnType<typeof createClient> = createClient({baseURL:'https://api.test'});
api.orders.list({query:{status:['bogus']}});
api.orders.list.raw({query:{status:['bogus']}});
api.orders('one').get({query:{status:['bogus']}});
api.$operations.watchEvents.stream({query:{status:['bogus']}});
`
	if err := os.WriteFile(filepath.Join(source, "consumer.ts"), []byte(consumer), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "package.json"), []byte(`{"type":"module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
const {default:ts}=await import(pathToFileURL(process.argv[2]));
const root=process.argv[1],file=root+'/consumer.ts';
const options={strict:true,module:ts.ModuleKind.NodeNext,moduleResolution:ts.ModuleResolutionKind.NodeNext,target:ts.ScriptTarget.ES2022,noEmit:true};
const host={getScriptFileNames:()=>[file],getScriptVersion:()=> '0',getScriptSnapshot:(path)=>{const text=ts.sys.readFile(path);return text===undefined?undefined:ts.ScriptSnapshot.fromString(text)},getCurrentDirectory:()=>root,getCompilationSettings:()=>options,getDefaultLibFileName:(o)=>ts.getDefaultLibFilePath(o),fileExists:ts.sys.fileExists,readFile:ts.sys.readFile,readDirectory:ts.sys.readDirectory};
const service=ts.createLanguageService(host);
const diagnostics=service.getSemanticDiagnostics(file);
assert.equal(diagnostics.length,4);
for(const diagnostic of diagnostics){
 const message=ts.flattenDiagnosticMessageText(diagnostic.messageText,'\n');
 assert.ok(message.includes('bogus') && message.includes('open') && message.includes('closed'),message);
 assert.ok(!message.includes('No overload matches') && !message.includes('does not exist') && !message.includes('Promise<'),message);
}
const text=ts.sys.readFile(file),position=text.indexOf('list(');
const quick=ts.displayPartsToString(service.getQuickInfoAtPosition(file,position).displayParts);
assert.ok(quick.includes('ListOrdersInput') && !quick.includes('OperationPublicType<Input>'),quick);
const help=service.getSignatureHelpItems(file,position+5,undefined);
const input=help.items.find(item=>item.parameters.some(p=>ts.displayPartsToString(p.displayParts).includes('ListOrdersInput')));
assert.ok(input,'signature help must show the named input');
const inputFile=root+'/internal/operations/orders/get.ts';
const declaration=ts.sys.readFile(inputFile),reference=declaration.indexOf('ListOrdersInput',declaration.indexOf('interface ListOrdersInput')+'interface ListOrdersInput'.length);
const definitions=service.getDefinitionAtPosition(inputFile,reference);
assert.ok(definitions.some(d=>d.name==='ListOrdersInput'),JSON.stringify(definitions));
const queryPosition=text.indexOf('query:')+1;
const completions=service.getCompletionsAtPosition(file,queryPosition,{});
assert.ok(completions?.entries.some(entry=>entry.name==='query'),'input field completions remain available');
service.dispose();
`
	for _, version := range []string{"typescript-5-7", "typescript-5-9", "typescript-6"} {
		t.Run(version, func(t *testing.T) {
			compiler, err := filepath.Abs("../../../test/typescript/node_modules/" + version + "/lib/typescript.js")
			if err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command("node", "--input-type=module", "--eval", script, source, compiler).CombinedOutput(); err != nil {
				t.Fatalf("input diagnostics and editor hints: %v\n%s", err, output)
			}
		})
	}
}

func TestNamedOperationInputsPreserveRequiredOptionsAndSchemaImports(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
 "openapi":"3.1.1","info":{"title":"Named input contracts","version":"1"},
 "components":{
  "securitySchemes":{"first":{"type":"apiKey","in":"header","name":"X-First"},"second":{"type":"apiKey","in":"header","name":"X-Second"}},
  "schemas":{"ListOrders":{"type":"object","properties":{"next":{"$ref":"#/components/schemas/ListOrders"}}}}
 },
 "paths":{
  "/orders":{"post":{"operationId":"listOrders","security":[{"first":[]},{"second":[]}],"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/ListOrders"}}}},"responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"type":"string"}},"text/plain":{"schema":{"type":"string"}}}}}}},
  "/unnamed":{"get":{"parameters":[{"name":"enabled","in":"query","schema":{"type":"boolean"}}],"responses":{"204":{"description":"OK"}}}},
  "/resources/{id}":{"get":{"operationId":"resource","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}},{"name":"enabled","in":"query","schema":{"type":"boolean"}}],"responses":{"204":{"description":"OK"}}}},
  "/pages":{"get":{"operationId":"paginate","parameters":[{"name":"cursor","in":"query","schema":{"type":"string"}},{"name":"limit","in":"query","schema":{"type":"integer","minimum":1}}],"x-pagination":"cursor","responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"type":"object","properties":{"items":{"type":"array","items":{"type":"string"}},"pagination":{"type":"object","properties":{"nextCursor":{"type":"string"}}}}}}}}}}}
 }
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	compileDeclarationConsumers(t, artifacts, `import {createClient, type RouteInput, type RouteOptions} from './index.js';
const api: ReturnType<typeof createClient> = createClient({baseURL:'https://api.test'});
const options: RouteOptions<'POST /orders'> = {securityRequirement:'first',accept:'application/json'};
const input: RouteInput<'POST /orders'> = {body:{next:{next:{}}}};
void api.$operations.listOrders(options);
void api.$operations.listOrders(undefined,options);
void api.$operations.listOrders(input,options);
void api.$operations.listOrders.raw(input,{securityRequirement:'second',accept:'text/plain'});
void api.$routes['GET /unnamed']({query:{enabled:true}});
void api.$operations.resource({path:{id:'one'},query:{enabled:true}});
void api.resources('one').get({query:{enabled:true}});
void api.resources('one').get({headers:new Headers()});
void api.$operations.paginate({query:{cursor:'next'}});
void api.$operations.paginate.paginate({query:{cursor:'next'}});
// @ts-expect-error required security selection is preserved
void api.$operations.listOrders();
// @ts-expect-error an input is not a security selection
void api.$operations.listOrders(input);
// @ts-expect-error invalid accept is rejected
void api.$operations.listOrders(input,{securityRequirement:'first',accept:'application/xml'});
// @ts-expect-error recursive schema fields keep their input contract
void api.$operations.listOrders({body:{next:123}},options);
`)
}
