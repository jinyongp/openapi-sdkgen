package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedSelectedMetadataAndInspection(t *testing.T) {
	document, err := compiler.Compile([]byte(selectionFixture))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := generator.NewAddonRegistry(generator.AddonMetadata)
	if err != nil {
		t.Fatal(err)
	}
	options, err := registry.Resolve([]string{"metadata"})
	if err != nil {
		t.Fatal(err)
	}
	options.Golang = &generator.GoOptions{ImportPath: "example.test/consumer/api"}
	options.Selection = &generator.Selection{Operations: []string{"source"}}
	plan, diagnostics, err := (Generator{}).Prepare(document, options)
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	inspection, err := (Generator{}).InspectPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	source := inspection["GET /source"]
	if source.Status != "exposed" || !source.Routes || !source.Operations || source.ResourceCall == nil || *source.ResourceCall != "api.Source.Get().Do(ctx)" {
		t.Fatal(source)
	}
	*source.ResourceCall = "mutated"
	again, err := (Generator{}).InspectPlan(plan)
	if err != nil || *again["GET /source"].ResourceCall == "mutated" {
		t.Fatal("inspection mutated plan", err)
	}
	if inspection["GET /target/{id}"].Status != "hidden" {
		t.Fatal("Link dependency exposed")
	}
	testGeneratedModule(t, plan, map[string]string{"metadata_test.go": `package consumer_test
import("encoding/json";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/operations";"example.test/consumer/api/metadata")
func TestFullSourceSnapshot(t *testing.T){raw:=metadata.Document();var doc struct{Paths map[string]json.RawMessage};if err:=json.Unmarshal(raw,&doc);err!=nil{t.Fatal(err)};if len(doc.Paths)!=4||doc.Paths["/unused"]==nil||metadata.Version!="3.1.0"||metadata.VersionLine!="3.1"||sdk.OpenAPIVersion!=metadata.Version||operations.OpenAPIVersionLine!=metadata.VersionLine{t.Fatal(string(raw))};raw[0]='!';if !json.Valid(metadata.Document()){t.Fatal("shared mutable metadata")}}
`})
	options, _ = registry.Resolve(nil)
	options.Golang = &generator.GoOptions{ImportPath: "example.test/consumer/api"}
	plain, diagnostics, err := (Generator{}).Prepare(document, options)
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	artifacts, err := (Generator{}).Emit(plain)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range artifacts {
		if artifact.Path == "metadata/metadata.go" {
			t.Fatal("metadata emitted without addon")
		}
	}
}
