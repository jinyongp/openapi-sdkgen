package golang

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
)

type modelFixtureCatalog struct {
	Local []modelFixtureEntry `json:"local"`
}
type modelFixtureEntry struct {
	ID, Input string
	Addons    []string `json:"addons"`
}

// Schema consumers exercise the shared existing fixture corpus independently
// of the HTTP/media work units. They do not claim that those operation flows run.
func TestExistingFixtureSchemaConsumers(t *testing.T) {
	base := filepath.Join("..", "..", "..", "test", "typescript", "fixtures")
	data, err := os.ReadFile(filepath.Join(base, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var catalog modelFixtureCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Local {
		if fixture.Input == "diagnostics.openapi.json" {
			continue
		} // Deliberate compiler failures are covered by the existing diagnostic suite.
		t.Run(fixture.ID, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(base, fixture.Input))
			if err != nil {
				t.Fatal(err)
			}
			document, err := compiler.Compile(data)
			if err != nil {
				t.Fatal(err)
			}
			view := *document
			view.Operations = nil
			// This projection exercises schema codecs, without the removed HTTP
			// operations' extension declarations or error-category reachability.
			view.Raw = nil
			view.Schemas = make(map[string]ir.Schema, len(document.Schemas))
			for name, schema := range document.Schemas {
				view.Schemas[name] = schema
			}
			index := 0
			add := func(value any) {
				if value == nil {
					return
				}
				name := fmt.Sprintf("FixtureSchema%d", index)
				index++
				for {
					if _, exists := view.Schemas[name]; !exists {
						break
					}
					name += "X"
				}
				view.Schemas[name] = ir.Schema{Name: name, Pointer: "#/fixture/" + name, Value: value}
			}
			media := func(content []ir.MediaType) {
				for _, item := range content {
					add(item.Schema)
					add(item.ItemSchema)
				}
			}
			for _, operation := range document.Operations {
				for _, parameter := range operation.Parameters {
					add(parameter.Schema)
					media(parameter.Content)
				}
				if operation.RequestBody != nil {
					media(operation.RequestBody.Content)
				}
				for _, response := range operation.Responses {
					media(response.Content)
				}
			}
			plan, diagnostics, err := (Generator{}).Prepare(&view, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
			if err != nil || len(diagnostics) != 0 {
				t.Fatalf("prepare: %v %v", err, diagnostics)
			}
			testGeneratedModule(t, plan, map[string]string{})
			t.Logf("%d normalized schema roots", len(view.Schemas))
		})
	}
}
