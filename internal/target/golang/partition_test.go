package golang

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"reflect"
	"strconv"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestPartitionPrivateSymbolCollisions(t *testing.T) {
	var previous map[string][]byte
	for repeat := 0; repeat < 2; repeat++ {
		artifacts := make(map[string][]byte)
		files := []partitionFile{
			{artifact: generator.Artifact{Path: "models/a.go", Data: []byte("package models\ntype SdkgenPrivate__foo int\ntype hidden int\nfunc local()hidden{return 1}\nfunc foo()int{return int(local())}\n")}},
			{artifact: generator.Artifact{Path: "models/b.go", Data: []byte("package models\nfunc foo_()int{return foo()+1}\n")}},
			{artifact: generator.Artifact{Path: "models/c.go", Data: []byte("package models\nfunc Value()int{return foo_()}\n")}},
		}
		if err := partitionPackage(files, "example.test/consumer/api", "models", generator.ArtifactSinkFunc(func(artifact generator.Artifact) error {
			artifacts[artifact.Path] = artifact.Data
			return nil
		}), 1); err != nil {
			t.Fatal(err)
		}
		for name, data := range artifacts {
			if bytes.Contains(data, []byte("SdkgenPrivate__local")) || bytes.Contains(data, []byte("SdkgenPrivate__hidden")) {
				t.Fatal("package-local helpers leaked into export data", name)
			}
		}
		if repeat == 1 {
			for name, data := range artifacts {
				if !bytes.Equal(data, previous[name]) {
					t.Fatal("partition emission is not deterministic", name)
				}
			}
			continue
		}
		previous = artifacts
		var prepared []filePlan
		for name, data := range artifacts {
			prepared = append(prepared, filePlan{Path: name, Source: "partition-test", Prepared: string(data)})
		}
		testGeneratedModule(t, generator.NewPlan("golang", sourcePlan{Files: prepared}), map[string]string{"names_test.go": `package consumer_test
import("testing";"example.test/consumer/api/models")
func TestPrivateBindings(t *testing.T){if models.Value()!=2{t.Fatal("private symbols were merged")}}
`})
	}
}

func TestPartitionedModelAndValidatorContracts(t *testing.T) {
	document, err := compiler.Compile([]byte(`{
"openapi":"3.1.0","info":{"title":"Partitions","version":"1"},
"components":{"schemas":{
"Node":{"type":"object","required":["id","mode"],"properties":{"id":{"type":"integer","minimum":1},"mode":{"type":"string","enum":["ready","done"]},"next":{"$ref":"#/components/schemas/Other"}}},
"Other":{"type":"object","properties":{"node":{"$ref":"#/components/schemas/Node"}}}
}},
"paths":{"/items":{"post":{"operationId":"send","requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Node"}}}},"responses":{"200":{"description":"Item","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Node"}}}}}}}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	testPartitionedModule(t, plan, map[string]string{"partition_test.go": partitionConsumer})
}

func TestPartitionBindingsPreserveImportAndLocalNames(t *testing.T) {
	const source = `package models
import schema "example.test/consumer/api/internal/schema"
var sdkgenSchema0 = 7
func Value() int { return schema.Value()+sdkgenSchema0 }
type Own interface { Value() int }
func Shadow(schema Own) int { return schema.Value() }
type local int
func (value local) Value() int { return int(value) }
func ShadowResult() int { return Shadow(local(3)) }
`
	artifact, err := bindPartitionImports(generator.Artifact{Path: "models/names.go", Data: []byte(source)}, "example.test/consumer/api/internal/schema", map[string]partitionReference{
		"Value": {path: "example.test/consumer/api/internal/schemaimpl/p1", name: "Value"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(artifact.Data, []byte(`"example.test/consumer/api/internal/schema"`)) {
		t.Fatal("caller still loads the complete schema facade")
	}
	files := []filePlan{
		{Path: artifact.Path, Source: "partition-test", Prepared: string(artifact.Data)},
		{Path: "internal/schemaimpl/p1/value.go", Source: "partition-test", Prepared: "package p1\nfunc Value()int{return 2}\n"},
	}
	testGeneratedModule(t, generator.NewPlan("golang", sourcePlan{Files: files}), map[string]string{"binding_test.go": `package consumer_test
import("testing";"example.test/consumer/api/models")
func TestBindings(t *testing.T){if models.Value()!=9||models.ShadowResult()!=3{t.Fatal("import, collision or local binding changed")}}
`})
}

func TestPartitionBindingsPreserveCommentsAndCanonicalFormatting(t *testing.T) {
	const source = `// Package models documents the generated API.
package models

import (
	// Keep the standard formatting dependency.
	"fmt"
	schema "example.test/consumer/api/internal/schema"
)

// Value returns the schema value.
func Value() string {
	// Keep this body comment and comment-like string data.
	return fmt.Sprint(schema.Value(), "// unchanged /* literal */")
}
`
	artifact, err := bindPartitionImports(generator.Artifact{Path: "models/value.go", Data: []byte(source)}, "example.test/consumer/api/internal/schema", map[string]partitionReference{
		"Value": {path: "example.test/consumer/api/internal/schemaimpl/p1", name: "Value"},
	})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := format.Source(artifact.Data)
	if err != nil || !bytes.Equal(canonical, artifact.Data) {
		t.Fatal("partition output is not canonical Go source", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), artifact.Path, artifact.Data, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	if file.Doc.Text() != "Package models documents the generated API.\n" {
		t.Fatal("package documentation changed")
	}
	var value *ast.FuncDecl
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok && function.Name.Name == "Value" {
			value = function
		}
	}
	if value == nil || value.Doc.Text() != "Value returns the schema value.\n" {
		t.Fatal("function documentation changed ownership")
	}
	for _, text := range []string{"// Keep the standard formatting dependency.", "// Keep this body comment and comment-like string data.", `"// unchanged /* literal */"`} {
		if bytes.Count(artifact.Data, []byte(text)) != 1 {
			t.Fatalf("source text was changed or duplicated: %s", text)
		}
	}
}

func TestPartitionRewrittenSignaturesRemainCanonical(t *testing.T) {
	files := []partitionFile{
		{artifact: packTestArtifact(t, "models/state.go", `type state struct{}
func (value *state) use(item *state) *state { return item }
`)},
		{artifact: packTestArtifact(t, "models/caller.go", `func consume(value *state) *state { return value }
`)},
	}
	implementations := 0
	if err := partitionPackage(files, "example.test/consumer/api", "models", generator.ArtifactSinkFunc(func(artifact generator.Artifact) error {
		if strings.HasPrefix(artifact.Path, "internal/modelsimpl/") {
			implementations++
		}
		canonical, err := format.Source(artifact.Data)
		if err != nil || !bytes.Equal(canonical, artifact.Data) {
			t.Errorf("rewritten signature is not canonical in %s: %v\n%s", artifact.Path, err, artifact.Data)
		}
		return nil
	}), 1); err != nil {
		t.Fatal(err)
	}
	if implementations != len(files) {
		t.Fatal("partitioned implementations were not checked", implementations)
	}
}

func TestPartitionFacadePreservesDocumentation(t *testing.T) {
	const source = `// Package models documents the generated API.
package models

// Count is the default item count.
const Count = 3

// Name identifies an item.
type Name string

// Names returns its arguments.
func Names(values ...Name) []Name {
	// This implementation comment must not enter the facade.
	return values
}
`
	var facade []byte
	if err := partitionPackage([]partitionFile{{artifact: generator.Artifact{
		Path: "models/names.go", Data: []byte(source),
	}}}, "example.test/consumer/api", "models", generator.ArtifactSinkFunc(func(artifact generator.Artifact) error {
		if artifact.Path == "models/names.go" {
			facade = artifact.Data
		}
		return nil
	}), 1); err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "names.go", facade, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	docs := map[string]string{"package": file.Doc.Text()}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			docs[declaration.Name.Name] = declaration.Doc.Text()
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					docs[spec.Name.Name] = declaration.Doc.Text()
				case *ast.ValueSpec:
					docs[spec.Names[0].Name] = declaration.Doc.Text()
				}
			}
		}
	}
	for name, want := range map[string]string{
		"package": "Package models documents the generated API.\n",
		"Count":   "Count is the default item count.\n",
		"Name":    "Name identifies an item.\n",
		"Names":   "Names returns its arguments.\n",
	} {
		if docs[name] != want {
			t.Errorf("documentation for %s: got %q, want %q", name, docs[name], want)
		}
	}
	if bytes.Contains(facade, []byte("This implementation comment")) {
		t.Fatal("detached implementation comment entered the facade")
	}
	canonical, err := format.Source(facade)
	if err != nil || !bytes.Equal(canonical, facade) {
		t.Fatal("facade is not canonical Go source", err)
	}
}

func TestPartitionFacadeViewDoesNotMutateImplementation(t *testing.T) {
	const source = `// Package models preserves its API documentation.
package models
import (
"fmt"
"strings"
)
// Count is retained by the facade.
const Count = 3
// Name identifies a model.
type Name string
func Names(_ Name, values ...Name) []Name {
// Implementation-only comment.
return values
}
func Unnamed(Name, ...Name) {}
func Reference(value fmt.Stringer) fmt.Stringer { return value }
func private() string { return strings.TrimSpace(" body ") }
`
	for _, mutable := range []bool{false, true} {
		text := source
		if mutable {
			text += "var Mutable int\n"
		}
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, "models/names.go", text, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		scope := file.Scope
		objects := make(map[string]*ast.Object, len(scope.Objects))
		for name, object := range scope.Objects {
			objects[name] = object
		}
		snapshot := func() []byte {
			var buffer bytes.Buffer
			// ast.Fprint visits map keys in unspecified order. Check the scope
			// map separately while retaining object links in the syntax snapshot.
			if err := ast.Fprint(&buffer, set, file, func(name string, _ reflect.Value) bool { return name != "Objects" }); err != nil {
				t.Fatal(err)
			}
			return buffer.Bytes()
		}
		before := snapshot()
		partition := packagePartition{root: "example.test/consumer/api", packageName: "models", symbols: map[string]partitionSymbol{"Name": {name: "Name"}}}
		first, err := partition.facade(set, file, 0)
		if !bytes.Equal(before, snapshot()) || file.Scope != scope || !reflect.DeepEqual(objects, scope.Objects) {
			t.Fatal("facade modified parser-owned implementation syntax or scope")
		}
		if mutable {
			if err == nil || !strings.Contains(err.Error(), "cannot alias mutable public variable Mutable") {
				t.Fatal("mutable variable contract changed", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		second, err := partition.facade(set, file, 0)
		if err != nil || !bytes.Equal(first, second) || !bytes.Equal(before, snapshot()) {
			t.Fatal("facade view is not repeatable and isolated", err)
		}
		canonical, err := format.Source(first)
		if err != nil || !bytes.Equal(canonical, first) {
			t.Fatal("facade view is not canonical Go", err)
		}
		for _, expected := range []string{
			"func Names(argument0 Name, values ...Name) []Name",
			"return sdkgenimpl.Names(argument0, values...)",
			"func Unnamed(argument0 Name, argument1 ...Name)",
			"sdkgenimpl.Unnamed(argument0, argument1...)",
			"func Reference(value fmt.Stringer) fmt.Stringer",
			"// Name identifies a model.",
		} {
			if !bytes.Contains(first, []byte(expected)) {
				t.Fatalf("facade lost its public signature or forwarding behavior: %s\n%s", expected, first)
			}
		}
		if bytes.Contains(first, []byte("Implementation-only comment")) || bytes.Contains(first, []byte(`"strings"`)) {
			t.Fatal("facade retained a discarded body or its import")
		}
	}
}

func TestPartitionForwardReferencesCyclesAndLocalBindings(t *testing.T) {
	files := []partitionFile{
		{artifact: generator.Artifact{Path: "models/first.go", Data: []byte(`package models
type First struct { Other *Second }
func Value() int { return later() + shadow(3) }
func shadow(later int) int { return later }
`)}},
		{artifact: generator.Artifact{Path: "models/second.go", Data: []byte(`package models
type Second struct { First *First }
`)}},
		{artifact: generator.Artifact{Path: "models/last.go", Data: []byte(`package models
func later() int { return 7 }
`)}},
	}
	var prepared []filePlan
	if err := partitionPackage(files, "example.test/consumer/api", "models", generator.ArtifactSinkFunc(func(artifact generator.Artifact) error {
		prepared = append(prepared, filePlan{Path: artifact.Path, Source: "partition-test", Prepared: string(artifact.Data)})
		return nil
	}), 1); err != nil {
		t.Fatal(err)
	}
	testGeneratedModule(t, generator.NewPlan("golang", sourcePlan{Files: prepared}), map[string]string{"forward_test.go": `package consumer_test
import("testing";"example.test/consumer/api/models")
func TestForwardReferences(t *testing.T) {
 if models.Value() != 10 { t.Fatal("forward reference or local binding changed") }
 first := models.First{}
 second := models.Second{First: &first}
 first.Other = &second
 if first.Other.First != &first { t.Fatal("recursive type ownership changed") }
}
`})
}

func testPartitionedModule(t *testing.T, plan generator.Plan, consumers map[string]string) {
	t.Helper()
	value, err := plan.Value("golang")
	if err != nil {
		t.Fatal(err)
	}
	prepared := value.(sourcePlan)
	shared := make(map[string]bool)
	for _, file := range prepared.Files {
		shared[file.Path] = file.Source != "" || strings.HasPrefix(file.Template, "runtime/")
	}
	var files []filePlan
	packages := make(map[string][]partitionFile)
	sink := generator.ArtifactSinkFunc(func(artifact generator.Artifact) error {
		files = append(files, filePlan{Path: artifact.Path, Package: "unused", Source: "partition-test", Prepared: string(artifact.Data)})
		return nil
	})
	err = (Generator{}).EmitTo(plan, generator.ArtifactSinkFunc(func(artifact generator.Artifact) error {
		directory := path.Dir(artifact.Path)
		if directory == "models" || directory == "internal/schema" {
			packages[directory] = append(packages[directory], partitionFile{artifact: artifact, shared: shared[artifact.Path]})
			return nil
		}
		return sink.WriteArtifact(artifact)
	}))
	if err != nil {
		t.Fatal(err)
	}
	bindings := make(map[string]partitionReference)
	modelBindings := make(map[string]partitionReference)
	for _, directory := range []string{"internal/schema", "models"} {
		var exports map[string]partitionReference
		if directory == "internal/schema" {
			exports = bindings
		} else {
			exports = modelBindings
		}
		if err := partitionPackageBound(packages[directory], prepared.ImportPath, directory, sink, 512, exports); err != nil {
			t.Fatal(err)
		}
	}
	for index, file := range files {
		if strings.HasPrefix(file.Path, "internal/schema/") {
			t.Fatal("unused internal schema facade was emitted", file.Path)
		}
		if strings.HasPrefix(file.Path, "internal/schemaimpl/") {
			continue
		}
		artifact, err := bindPartitionImports(generator.Artifact{Path: file.Path, Data: []byte(file.Prepared)}, prepared.ImportPath+"/internal/schema", bindings)
		if err != nil {
			t.Fatal(err)
		}
		artifact, err = bindPartitionImports(artifact, prepared.ImportPath+"/models", modelBindings)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(file.Path, ".go") {
			parsed, err := parser.ParseFile(token.NewFileSet(), file.Path, artifact.Data, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imported := range parsed.Imports {
				if imported.Path.Value == strconv.Quote(prepared.ImportPath+"/models") {
					t.Fatal("generated caller still loads the public model facade", file.Path)
				}
			}
		}
		files[index].Prepared = string(artifact.Data)
	}
	testGeneratedModule(t, generator.NewPlan("golang", sourcePlan{Files: files}), consumers)
}

const partitionConsumer = `package consumer_test
import("context";"net/http";"net/http/httptest";"testing";"example.test/consumer/api/models";"example.test/consumer/api/types";sdk "example.test/consumer/api")
func TestPartitions(t *testing.T){
 raw,err:=types.ParseValue([]byte("{\"id\":1,\"mode\":\"ready\",\"next\":{\"node\":{\"id\":2,\"mode\":\"done\"}}}"));if err!=nil{t.Fatal(err)}
 model,err:=models.DecodeSendBody(context.Background(),raw);if err!=nil{t.Fatal(err)}
 encoded,err:=model.MarshalJSON();if err!=nil{t.Fatal(err)}
 var restored models.SendBody;if err:=restored.UnmarshalJSON(encoded);err!=nil{t.Fatal(err)}
 invalid,_:=types.ParseValue([]byte("{\"id\":0,\"mode\":\"bad\"}"));if _,err:=models.DecodeSendBody(context.Background(),invalid);err==nil{t.Fatal("validation lost")}
 calls:=0;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){calls++;w.Header().Set("Content-Type","application/json");w.Write(encoded)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 body,err:=models.DecodeSendBody(context.Background(),raw);if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.Send(sdk.SendInput{Body:body}).Do(context.Background());err!=nil{t.Fatal(err)};if calls!=1{t.Fatal(calls)}
}
`
