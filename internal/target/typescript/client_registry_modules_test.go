package typescript

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestRegistryModulesKeepCrossChunkLinksAndOrderedPublicMaps(t *testing.T) {
	raw := registrySelectionDocument(t).Raw
	paths := raw["paths"].(map[string]any)
	paths["/zzTarget"], paths["/yyLast"] = paths["/b"], paths["/c"]
	delete(paths, "/b")
	delete(paths, "/c")
	// A complete chunk can consist of private Link dependencies. Its optional
	// public exposure method and imports must remain absent.
	last := paths["/yyLast"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)
	last["links"] = map[string]any{"next": map[string]any{"operationId": "private000"}}
	for i := 0; i < 300; i++ {
		response := map[string]any{"description": "ok", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Item"}}}}
		if i < 299 {
			response["links"] = map[string]any{"next": map[string]any{"operationId": fmt.Sprintf("private%03d", i+1)}}
		}
		paths[fmt.Sprintf("/zPrivate%03d", i)] = map[string]any{"get": map[string]any{"operationId": fmt.Sprintf("private%03d", i), "responses": map[string]any{"200": response}}}
	}
	ids := []string{"a", "pages", "events"}
	special := []string{"__proto__", "constructor", "10", "2", "é", "𝄞", "\ue000"}
	for i := 0; i < 700; i++ {
		id := fmt.Sprintf("op%04d", 699-i)
		if i < len(special) {
			id = special[i]
		}
		ids = append(ids, id)
		paths[fmt.Sprintf("/r%04d", i)] = map[string]any{"get": map[string]any{"operationId": id, "responses": map[string]any{"204": map[string]any{"description": "ok"}}}}
	}
	for _, extra := range []struct{ path, source string }{
		{"/sStream", `{"get":{"operationId":"events","responses":{"200":{"description":"ok","content":{"application/x-ndjson":{"itemSchema":{"type":"string"}}}}}}}`},
		{"/pPages", `{"get":{"operationId":"pages","parameters":[
{"name":"cursor","in":"query","schema":{"type":"string"}},
{"name":"limit","in":"query","schema":{"type":"integer","minimum":1}}
],"x-pagination":"cursor","responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object","properties":{"items":{"type":"array","items":{"type":"string"}},"pagination":{"type":"object","properties":{"nextCursor":{"type":["string","null"]}}}}}}}}}}}`},
	} {
		var value any
		if err := json.Unmarshal([]byte(extra.source), &value); err != nil {
			t.Fatal(err)
		}
		paths[extra.path] = value
	}
	input, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, selected := range []bool{false, true} {
		t.Run(fmt.Sprintf("private=%t", selected), func(t *testing.T) {
			options := generator.Options{}
			if selected {
				options.Selection = &generator.Selection{Operations: ids, Routes: []string{"GET /idless/{task-id}"}}
				options.Clients = map[string]generator.Client{"other": {Selection: &generator.Selection{Operations: []string{"b"}}}}
			}
			plan, values, err := (Generator{}).Prepare(document, options)
			if err != nil || diagnostic.HasErrors(values) {
				t.Fatalf("prepare: %v %v", err, values)
			}
			value, _ := plan.Value("typescript")
			prepared := value.(*sourcePlan)
			root := prepared
			if root.root != nil {
				root = root.root
			}
			if !usesRegistryModules(*root.manifest, root.modules) {
				t.Fatal("large fixture did not use registry modules")
			}
			for _, module := range root.modules.registryModules {
				if len(module.entries) > registryChunkSize {
					t.Fatalf("unbounded module: %s", module.path)
				}
			}
			artifacts, err := (Generator{}).Emit(plan)
			if err != nil {
				t.Fatal(err)
			}
			again, err := (Generator{}).Emit(plan)
			if err != nil || !reflect.DeepEqual(artifacts, again) {
				t.Fatalf("unstable registry emission: %v", err)
			}
			probe := `import type {Client, RouteInput, RouteOutput} from './index.js';
import type {Client as SelectedClient} from './selective/types.js';
import type {OperationReference} from './selective/index.js';
declare const api: Client;
const output: RouteOutput<'GET /a'> = {id:'one'};
output.id='mutable';
// @ts-expect-error decoded output preserves its value type
output.id=42;
const input: RouteInput<'GET /pPages'> = {query:{cursor:'one'}};
// @ts-expect-error projected input stays readonly
input.query!.cursor='changed';
void input;void output;
api.$operations['__proto__']();api.$operations.a;api.$links.a.next;
api.$operations.pages.paginate({});api.$operations.events.stream();
api.$routes['GET /idless/{task-id}']({path:{'task-id':'one'}});
// @ts-expect-error hidden operation
api.$operations.hidden;
// @ts-expect-error unknown operation
api.$operations.missing;
// @ts-expect-error this operation accepts no body
api.$operations['__proto__']({body:true});
declare const chosen: SelectedClient<readonly [OperationReference<'GET /a'>]>;
chosen.a.get();
// @ts-expect-error unselected resource remains absent
chosen.r0000.get();
declare const maybe: SelectedClient<{readonly route?: OperationReference<'GET /a'>}>;
maybe.a?.get?.();
// @ts-expect-error optional selection does not guarantee a resource
maybe.a.get();
declare const empty: SelectedClient<readonly []>;
// @ts-expect-error empty selection has no resources
empty.a.get();
`
			if selected {
				probe += `// @ts-expect-error dependency-only operation
api.$operations.b;
// @ts-expect-error dependency-only route
api.$routes['GET /zzTarget'];
// @ts-expect-error dependency-only Link source
api.$links.b;
`
			}
			output := compileTypeScriptArtifactSet(t, artifacts, "registry.probe.ts", probe)
			if !selected {
				compileDeclarationConsumers(t, artifacts, probe)
				compiler, err := filepath.Abs("../../../test/typescript/node_modules/typescript-5-9/lib/typescript.js")
				if err != nil {
					t.Fatal(err)
				}
				hover := `
import {pathToFileURL} from 'node:url';
import path from 'node:path';
const {default:ts}=await import(pathToFileURL(process.argv[2]));
const source=path.join(path.dirname(process.argv[1]),'source');
const loaded=ts.readConfigFile(path.join(source,'tsconfig.json'),ts.sys.readFile);
const parsed=ts.parseJsonConfigFileContent(loaded.config,ts.sys,source);
const probe=path.join(source,'registry.probe.ts');
const program=ts.createProgram([probe],parsed.options),checker=program.getTypeChecker();
const expected=new Map([['__proto__','GET /r0000'],['GET /idless/{task-id}','GET /idless/{task-id}']]),found=new Set();
const visit=node=>{
  if(ts.isElementAccessExpression(node)&&ts.isStringLiteral(node.argumentExpression)&&expected.has(node.argumentExpression.text)){
    const key=node.argumentExpression.text,symbol=checker.getPropertyOfType(checker.getTypeAtLocation(node.expression),key);
    if(!symbol||!ts.displayPartsToString(symbol.getDocumentationComment(checker)).includes(expected.get(key))||!symbol.getJsDocTags(checker).some(tag=>tag.name==='example'))throw Error('public hover documentation lost: '+key);
    found.add(key);
  }
  ts.forEachChild(node,visit);
};
visit(program.getSourceFile(probe));if(found.size!==expected.size)throw Error('hover coverage missing');
`
				if result, err := exec.Command("node", "--input-type=module", "--eval", hover, output, compiler).CombinedOutput(); err != nil {
					t.Fatalf("public map hover: %v\n%s", err, result)
				}
			}
			public := root.selection.publicManifest(*root.manifest)
			routes, operations := []string{}, []string{}
			byID := map[string]string{}
			for _, item := range public.Operations {
				if item.Visibility == "hidden" {
					continue
				}
				route := manifestRouteKey(item)
				routes = append(routes, route)
				if item.OperationID != "" {
					operations = append(operations, item.OperationID)
					byID[item.OperationID] = route
				}
			}
			sort.Strings(routes)
			sort.Strings(operations)
			expected, err := json.Marshal(struct {
				Routes, Operations []string
				ByID               map[string]string
				Private            bool
			}{routes, operations, byID, selected})
			if err != nil {
				t.Fatal(err)
			}
			script := `
import {pathToFileURL} from 'node:url';
const {createClient}=await import(pathToFileURL(process.argv[1]+'/index.js'));
const expected=JSON.parse(process.argv[2]),seen=[];
const fetch=async(url)=>{
  seen.push(String(url));
  const pathname=new URL(url).pathname;
  const body=pathname==='/sStream'?'"event"\n':pathname==='/pPages'?'{"items":["row"],"pagination":{"nextCursor":null}}':'{"id":"one"}';
  const response=new Response(body,{headers:{'content-type':pathname==='/sStream'?'application/x-ndjson':'application/json'}});
  Object.defineProperty(response,'url',{value:String(url)});return response;
};
const first=createClient({baseURL:'https://first.test',fetch}),second=createClient({baseURL:'https://second.test',fetch});
const keys=names=>Object.keys(Object.fromEntries(names.map(name=>[name,null])));
if(JSON.stringify(Object.keys(first.$routes))!==JSON.stringify(keys(expected.Routes)))throw Error('route order/scope');
if(JSON.stringify(Object.keys(first.$operations))!==JSON.stringify(keys(expected.Operations)))throw Error('ID order/scope');
if(!Object.hasOwn(first.$operations,'__proto__')||Object.getPrototypeOf(first.$operations)!==Object.prototype)throw Error('special own key');
for(const [id,route] of Object.entries(expected.ByID)){
  if(first.$operations[id]!==first.$routes[route])throw Error('callable identity '+id);
  if(first.$operations[id]===second.$operations[id])throw Error('client state leaked '+id);
}
await first.$links.a.next(await first.$operations.a.raw());
await second.$links.a.next(await second.$operations.a.raw());
if(seen.slice(0,4).join(',')!=='https://first.test/a,https://first.test/zzTarget,https://second.test/a,https://second.test/zzTarget')throw Error('cross-chunk Link/client binding');
if(expected.Private){
  if(Object.hasOwn(first.$operations,'b')||Object.hasOwn(first.$routes,'GET /zzTarget')||Object.hasOwn(first.$links,'b'))throw Error('private dependency exposed');
}else{await first.$links.b.back(await first.$operations.b.raw());}
const events=[];for await(const item of first.$operations.events.stream())events.push(item);
if(events.join(',')!=='event')throw Error('stream capability');
const pages=[];for await(const item of first.$operations.pages.paginate({}))pages.push(item);
if(pages.join(',')!=='row')throw Error('pagination capability');
`
			if result, err := exec.Command("node", "--input-type=module", "--eval", script, output, string(expected)).CombinedOutput(); err != nil {
				t.Fatalf("chunked registry runtime: %v\n%s", err, result)
			}
			if strings.Contains(string(artifactByPath(t, artifacts, "internal/client/registry.ts")), "bindBase as") {
				t.Fatal("large registry kept unbounded factory imports")
			}
		})
	}
}
