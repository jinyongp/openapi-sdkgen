package typescript

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
)

func registrySelectionDocument(t *testing.T) *ir.Document {
	t.Helper()
	input := strings.Replace(generationSelectionFixture, `,"x-envelope":false`, "", 1)
	document, err := sdkgen.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func TestCallableRegistrySelectionReusesEquivalentFullRegistry(t *testing.T) {
	document := registrySelectionDocument(t)
	for _, test := range []struct {
		name      string
		selection generator.Selection
		keep      func(ir.Operation) bool
	}{
		{
			name: "all including Link cycles and ID-less routes",
			selection: generator.Selection{
				Operations: []string{"a", "b", "c", "unused"},
				Routes:     []string{"GET /idless/{task-id}"},
			},
			keep: func(ir.Operation) bool { return true },
		},
		{
			name:      "partial without private Link dependencies",
			selection: generator.Selection{Operations: []string{"c"}},
			keep:      func(operation ir.Operation) bool { return operation.OperationID == "c" },
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			input, err := json.Marshal(document.Raw)
			if err != nil {
				t.Fatal(err)
			}
			var raw map[string]any
			if err := json.Unmarshal(input, &raw); err != nil {
				t.Fatal(err)
			}
			paths := raw["paths"].(map[string]any)
			for _, operation := range document.Operations {
				if !test.keep(operation) {
					delete(paths, operation.Path)
				}
			}
			input, err = json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			view, err := sdkgen.Compile(input)
			if err != nil {
				t.Fatal(err)
			}
			full, err := (Generator{}).Generate(view, generator.Options{})
			if err != nil {
				t.Fatal(err)
			}
			selected, err := (Generator{}).Generate(document, generator.Options{Selection: &test.selection})
			if err != nil {
				t.Fatal(err)
			}
			path := "internal/client/registry.ts"
			if !bytes.Equal(artifactByPath(t, full, path), artifactByPath(t, selected, path)) {
				t.Fatal("equivalent public/execution scopes generated different callable registries")
			}
		})
	}
}

func TestCallableRegistrySelectionReusedMapRuntimeAndStrictTypes(t *testing.T) {
	for _, test := range []struct {
		name       string
		selection  generator.Selection
		probe      string
		routes     string
		operations string
		link       bool
	}{
		{
			name: "all",
			selection: generator.Selection{
				Operations: []string{"a", "b", "c", "unused"},
				Routes:     []string{"GET /idless/{task-id}"},
			},
			probe: `import {createClient} from "./index.js";
declare const api: ReturnType<typeof createClient>;
api.$routes["GET /idless/{task-id}"]({path:{"task-id":"one"}});
api.$operations.a; api.$operations.b; api.$links.a.next; api.$links.b.next;
// @ts-expect-error hidden route
api.$routes["GET /hidden"];
`,
			routes:     "GET /a,GET /b,GET /c,GET /idless/{task-id},GET /unused",
			operations: "a,b,c,unused",
			link:       true,
		},
		{
			name:      "partial",
			selection: generator.Selection{Operations: []string{"c"}},
			probe: `import {createClient} from "./index.js";
declare const api: ReturnType<typeof createClient>;
api.$operations.c;
// @ts-expect-error excluded operation
api.$operations.a;
// @ts-expect-error excluded route
api.$routes["GET /a"];
`,
			routes:     "GET /c",
			operations: "c",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			output := compileSelectedTypeScriptArtifacts(t, registrySelectionDocument(t), generator.Options{Selection: &test.selection}, test.probe)
			script := `import {pathToFileURL} from 'node:url';
const {createClient}=await import(pathToFileURL(process.argv[1]+"/index.js"));
const seen=[];
const api=createClient({baseURL:'https://api.test',fetch:async(url)=>{
  seen.push(String(url));
  const response=new Response('{"id":"one"}',{headers:{'content-type':'application/json'}});
  Object.defineProperty(response,'url',{value:String(url)});return response;
}});
if(Object.keys(api.$routes).sort().join(',')!==process.argv[2])throw Error('route scope');
if(Object.keys(api.$operations).sort().join(',')!==process.argv[3])throw Error('operation scope');
if(api.$routes['GET /c']!==api.$operations.c)throw Error('callable identity');
await api.$operations.c();
if(process.argv[4]==='true'){
  await api.$links.a.next(await api.$operations.a.raw());
  if(seen.join(',')!=='https://api.test/c,https://api.test/a,https://api.test/b')throw Error('Link target binding');
}else if(seen.join(',')!=='https://api.test/c')throw Error('partial call');
`
			link := "false"
			if test.link {
				link = "true"
			}
			if result, err := exec.Command("node", "--input-type=module", "--eval", script, output, test.routes, test.operations, link).CombinedOutput(); err != nil {
				t.Fatalf("runtime: %v\n%s", err, result)
			}
		})
	}
}
