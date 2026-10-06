package typescript

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestOperationOptionsPreservePlatformContractsAndCallTypes(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1","info":{"title":"Options contracts","version":"1"},
  "paths":{
    "/orders":{"get":{"operationId":"listOrders","parameters":[{"name":"status","in":"query","schema":{"type":"array","items":{"type":"string","enum":["open","closed"]}}}],"responses":{"204":{"description":"OK"}}}},
    "/files/{id}":{"get":{"operationId":"getFile","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}},"text/plain":{"schema":{"type":"string"}}}}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	probe := `import {createClient, type RequestOptions, type RouteOptions, type OperationContract} from './index.js';
type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false;
type Expect<T extends true> = T;
const api: ReturnType<typeof createClient> = createClient({baseURL:'https://api.test'});
export type OptionsContracts = [
  Expect<Equal<RouteOptions<'GET /orders'>['signal'], AbortSignal | undefined>>,
  Expect<Equal<RouteOptions<'GET /orders'>['headers'], RequestOptions['headers']>>,
  Expect<Equal<OperationContract<typeof api.orders.list>['options']['signal'], AbortSignal | undefined>>,
  Expect<Equal<NonNullable<Parameters<typeof api.orders.list.raw>[1]>['headers'], RequestOptions['headers']>>,
  Expect<Equal<RouteOptions<'GET /files/{id}'>['accept'], 'application/json' | 'text/plain' | undefined>>
];
const options: RouteOptions<'GET /orders'> = {signal: new AbortController().signal, headers: new Headers({'x-trace':'one'})};
void api.orders.list();
void api.orders.list(options);
void api.orders.list(undefined, options);
void api.orders.list({query:{status:['open']}}, options);
void api.orders.list.raw(options);
void api.$operations.listOrders(options);
void api.$routes['GET /orders']({query:{status:['closed']}}, options);
// @ts-expect-error invalid enum input remains rejected
void api.orders.list({query:{status:['bogus']}});
// @ts-expect-error transport options still require a native AbortSignal contract
void api.orders.list({signal:{aborted:false}});
// @ts-expect-error generated single-media options do not expose arbitrary accept values
void api.orders.list({accept:'application/xml'});
const json: Promise<{readonly id:string}> = api.files('one').get({accept:'application/json'});
const text: Promise<string> = api.$operations.getFile({path:{id:'one'}}, {accept:'text/plain'});
void json; void text;
// @ts-expect-error declared media types retain exact literal choices
void api.files('one').get({accept:'application/xml'});
`
	compileDeclarationConsumers(t, artifacts, probe)

	// Check the diagnostic produced for a real caller mistake, rather than an
	// emitted source snapshot or a compiler-dependent byte-count threshold.
	source := t.TempDir()
	writeTargetArtifacts(t, source, artifacts)
	invalid := `import {createClient} from './index.js';
const api: ReturnType<typeof createClient> = createClient({baseURL:'https://api.test'});
api.orders.list({query:{status:['bogus']}});
`
	if err := os.WriteFile(filepath.Join(source, "consumer.ts"), []byte(invalid), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "package.json"), []byte(`{"type":"module"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	compiler, err := filepath.Abs("../../../test/typescript/node_modules/typescript-6/lib/typescript.js")
	if err != nil {
		t.Fatal(err)
	}
	script := `import assert from 'node:assert/strict';
import {pathToFileURL} from 'node:url';
const {default:ts}=await import(pathToFileURL(process.argv[2]));
const file=process.argv[1]+'/consumer.ts';
const program=ts.createProgram([file],{strict:true,module:ts.ModuleKind.NodeNext,moduleResolution:ts.ModuleResolutionKind.NodeNext,target:ts.ScriptTarget.ES2022,noEmit:true});
const diagnostics=program.getSemanticDiagnostics(program.getSourceFile(file));
assert.equal(diagnostics.length,1);
assert.equal(diagnostics[0].code,2769);
const message=ts.flattenDiagnosticMessageText(diagnostics[0].messageText,'\n');
assert.ok(/options\?: (?:[A-Za-z_$][A-Za-z0-9_$]*Options|Options)(?: \| undefined)?\)/.test(message),message);
assert.ok(message.includes('bogus'),message);
`
	if output, err := exec.Command("node", "--input-type=module", "--eval", script, source, compiler).CombinedOutput(); err != nil {
		t.Fatalf("caller diagnostic should preserve the named options contract: %v\n%s", err, output)
	}
}
