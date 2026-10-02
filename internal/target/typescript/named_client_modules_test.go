package typescript

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestNamedClientsExposeIndependentTypesAndBindPrivateLinksLazily(t *testing.T) {
	registry, _ := generator.NewAddonRegistry(generator.AddonServer, generator.AddonMetadata)
	options, _ := registry.Resolve([]string{"server", "metadata"})
	options.Selection = &generator.Selection{Routes: []string{"GET /idless/{task-id}"}}
	options.Clients = map[string]generator.Client{
		"a": {Selection: &generator.Selection{Operations: []string{"a"}}},
		"b": {Selection: &generator.Selection{Operations: []string{"b"}}},
		"c": {Selection: &generator.Selection{Operations: []string{"c"}}},
	}
	probe := `import {createClient as root} from './index.js';
import {createClient as a} from './clients/a/index.js';
import {createClient as c, type Components, type Client} from './clients/c/index.js';
const options={baseURL:'https://example.test'};
root(options).idless('id').get();
// @ts-expect-error Named APIs stay outside the independent root SDK selection.
root(options).$operations.a();
a(options).a.get();
a(options).$links.a.next;
// @ts-expect-error Link targets are private in the source client.
a(options).$operations.b();
// @ts-expect-error Other clients' resource methods are unavailable.
c(options).b.get();
// @ts-expect-error Other clients' routes are unavailable.
c(options).$routes['GET /a']();
// @ts-expect-error Unrelated schemas stay outside the named component catalog.
type Unused = Components['Unused'];
type Same = Client['$routes']['GET /c'];
`
	output := compileSelectedTypeScriptArtifacts(t, selectedFixtureDocument(t), options, probe)
	if err := filepath.WalkDir(output, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".js") {
			return err
		}
		relative, _ := filepath.Rel(output, path)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.WriteString("\nglobalThis.evaluations.add(" + quoteTS(filepath.ToSlash(relative)) + ");\n")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	script := `import assert from 'node:assert/strict';import {pathToFileURL} from 'node:url';
globalThis.evaluations=new Set();
const load=path=>import(pathToFileURL(process.argv[1]+'/'+path));
const {createClient}=await load('clients/a/index.js');
assert(!evaluations.has('internal/executions/b/get.js'));
assert(!evaluations.has('internal/executions/c/get.js'));
const seen=[];const options=name=>({baseURL:'https://'+name+'.test',authorization:name,fetch:async(url,init)=>{seen.push([String(url),new Headers(init.headers).get('authorization')]);const response=Response.json({id:'one'});Object.defineProperty(response,'url',{value:String(url)});return response;}});
const a=createClient(options('a'));const a2=createClient(options('a2'));
assert.deepEqual(Object.keys(a.$routes),['GET /a']);assert.deepEqual(Object.keys(a.$operations),['a']);assert(!('b' in a));assert.notEqual(a.a.get,a2.a.get);
await a.$links.a.next(await a.a.get.raw());
assert(evaluations.has('internal/executions/b/get.js'));assert(!evaluations.has('internal/executions/c/get.js'));
assert.equal(seen[1][0],'https://a.test/b');assert.equal(seen[1][1],'a');
await a2.$links.a.next(await a2.a.get.raw());assert.equal(seen[3][1],'a2');
const {createClient:b}=await load('clients/b/index.js');const apiB=b(options('b'));await apiB.b.get();assert.equal(seen[4][1],'b');assert.deepEqual(Object.keys(apiB.$routes),['GET /b']);
const {createClient:root}=await load('index.js');assert.deepEqual(Object.keys(root(options('root')).$routes),['GET /idless/{task-id}']);
const selective=await load('selective/index.js');await assert.rejects(selective.loadOperations([selective.operations.a]));
assert(!('server/runtime.js' in Object.fromEntries([...evaluations].map(path=>[path,true]))));assert(!evaluations.has('metadata.js'));
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
		t.Fatalf("named clients runtime: %v\n%s", err, result)
	}
}

func TestNamedClientsPreserveExactRoutesAndParameterBuilders(t *testing.T) {
	input := `{"openapi":"3.2.1","info":{"title":"Exact client routes","version":"1"},"paths":{
"/items/{id}":{"get":{"parameters":[{"in":"path","name":"id","required":true,"schema":{"type":"integer"}}],"responses":{"204":{"description":"OK"}}}},
"/literal/:id":{"get":{"operationId":"literal","responses":{"204":{"description":"OK"}}}},
"/items/$count":{"get":{"operationId":"count","responses":{"200":{"description":"OK","content":{"text/plain":{"schema":{"$ref":"#/components/schemas/Count"}}}}}}},
"/lookup":{"query":{"operationId":"lookup","responses":{"204":{"description":"OK"}}}},
"/custom":{"additionalOperations":{"PurGe":{"operationId":"purge","responses":{"204":{"description":"OK"}}}}}
},"components":{"schemas":{"Count":{"type":"integer"}}}}`
	document, err := sdkgen.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	routes := []string{"GET /items/{id}", "GET /literal/:id", "GET /items/$count", "QUERY /lookup", "PurGe /custom"}
	options := generator.Options{Selection: &generator.Selection{Operations: []string{"count"}}, Clients: map[string]generator.Client{"page": {Selection: &generator.Selection{Routes: routes, Operations: []string{"count"}}}}}
	probe := `import {createClient} from './clients/page/index.js';
const api=createClient({baseURL:'https://example.test'});
api.items(7).get();
const count:Promise<number>=api.$routes['GET /items/$count']();
api.$routes['PurGe /custom']();
// @ts-expect-error Custom method identities retain their original case.
api.$routes['PURGE /custom']();
// @ts-expect-error Literal colon paths stay literal.
api.$routes['GET /literal/{id}']();
// @ts-expect-error The parameter builder preserves the declared parameter type.
api.items('seven').get();
`
	output := compileSelectedTypeScriptArtifacts(t, document, options, probe)
	script := `import assert from 'node:assert/strict';import {pathToFileURL} from 'node:url';
const {createClient}=await import(pathToFileURL(process.argv[1]+'/clients/page/index.js'));
const seen=[];const api=createClient({baseURL:'https://example.test',fetch:async(url,init)=>{seen.push([String(url),init.method]);return String(url).endsWith('/$count')?new Response('42',{headers:{'content-type':'text/plain'}}):new Response(null,{status:204});}});
await api.items(7).get();await api.$routes['GET /literal/:id']();assert.equal(await api.$operations.count(),42);await api.$operations.lookup();await api.$operations.purge();
assert.deepEqual(seen,[['https://example.test/items/7','GET'],['https://example.test/literal/:id','GET'],['https://example.test/items/$count','GET'],['https://example.test/lookup','QUERY'],['https://example.test/custom','PurGe']]);
assert.equal(Object.keys(api.$routes).length,5);assert.equal(Object.keys(api.$operations).length,4);
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, output).CombinedOutput(); err != nil {
		t.Fatalf("named route execution: %v\n%s", err, result)
	}
}
