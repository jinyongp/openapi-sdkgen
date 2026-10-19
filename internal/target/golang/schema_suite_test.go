package golang

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

type schemaSuiteGroup struct {
	Description string            `json:"description"`
	Schema      any               `json:"schema"`
	Tests       []schemaSuiteCase `json:"tests"`
}
type schemaSuiteCase struct {
	Description string          `json:"description"`
	Data        json.RawMessage `json:"data"`
	Valid       bool            `json:"valid"`
}

// These fixtures retain the upstream revision, hashes and license beside them.
// Expected results come from the JSON Schema suite, independently of either target.
func TestGeneratedOfficialUnevaluatedSchemaCases(t *testing.T) {
	for _, keyword := range []string{"unevaluatedProperties", "unevaluatedItems"} {
		t.Run(keyword, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "typescript", "fixtures", "json-schema-"+keyword+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var groups []schemaSuiteGroup
			if err := json.Unmarshal(data, &groups); err != nil {
				t.Fatal(err)
			}
			paths := make(map[string]any)
			components := make(map[string]any)
			var source strings.Builder
			source.WriteString("package schema\nimport(\"context\";\"testing\";\"example.test/consumer/api/internal/wire\")\nfunc TestOfficialCases(t *testing.T){\n")
			count := 0
			for index, group := range groups {
				schema := group.Schema
				if object, ok := schema.(map[string]any); ok {
					if _, exists := object["$id"]; !exists {
						object["$id"] = fmt.Sprintf("https://schemas.example.test/%s/%d", keyword, index)
					}
				}
				name := fmt.Sprintf("Case%d", index)
				components[name] = schema
				paths[fmt.Sprintf("/case%d", index)] = map[string]any{"get": map[string]any{"operationId": fmt.Sprintf("getCase%d", index), "responses": map[string]any{"200": map[string]any{"description": group.Description, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/" + name}}}}}}}
				for _, test := range group.Tests {
					fmt.Fprintf(&source, "t.Run(%q,func(t *testing.T){value,err:=wire.Parse([]byte(%q));if err!=nil{t.Fatal(err)};_,err=ValidateHTTP__GetCase%dOutput__(context.Background(),value,\"\");if (err==nil)!=%t{t.Fatalf(\"valid=%%t error=%%v\",%t,err)}})\n", group.Description+"/"+test.Description, string(test.Data), index, test.Valid, test.Valid)
					count++
				}
			}
			source.WriteString("}\n")
			input, err := json.Marshal(map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Official suite", "version": "1"}, "paths": paths, "components": map[string]any{"schemas": components}})
			if err != nil {
				t.Fatal(err)
			}
			document, err := compiler.Compile(input)
			if err != nil {
				t.Fatal(err)
			}
			plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
			if err != nil || len(diagnostics) != 0 {
				t.Fatalf("prepare: %v %v", err, diagnostics)
			}
			testGeneratedModule(t, plan, map[string]string{"api/internal/schema/official_test.go": source.String()})
			t.Logf("%d official cases across %d schema groups", count, len(groups))
		})
	}
}
