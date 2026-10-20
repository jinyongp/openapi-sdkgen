package golang

import (
	"bytes"
	"path"
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
	for _, directory := range []string{"internal/schema", "models"} {
		var exports map[string]partitionReference
		if directory == "internal/schema" {
			exports = bindings
		}
		if err := partitionPackageBound(packages[directory], prepared.ImportPath, directory, sink, 512, exports); err != nil {
			t.Fatal(err)
		}
	}
	for index, file := range files {
		if strings.HasPrefix(file.Path, "internal/schema/") || strings.HasPrefix(file.Path, "internal/schemaimpl/") {
			continue
		}
		artifact, err := bindPartitionImports(generator.Artifact{Path: file.Path, Data: []byte(file.Prepared)}, prepared.ImportPath+"/internal/schema", bindings)
		if err != nil {
			t.Fatal(err)
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
