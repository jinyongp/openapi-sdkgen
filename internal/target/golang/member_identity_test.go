package golang

import (
	"go/ast"
	"go/parser"
	"go/token"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"strings"
	"testing"
)

func TestHTTPMemberNamesUseOriginalSchemaOccurrence(t *testing.T) {
	const source = `{"openapi":"3.1.0","info":{"title":"Names","version":"1"},"paths":{"/names":{"get":{"operationId":"names","parameters":[{"name":"a-b","in":"query","schema":{"type":"string"}},{"name":"a_b","in":"query","description":"Second parameter","deprecated":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"OK","headers":{"X-A":{"schema":{"type":"string"}},"X_A":{"schema":{"type":"string"}}},"content":{"application/json":{"schema":{"type":"boolean"}}}}}}}}}`
	document, err := compiler.Compile([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	artifacts, err := (Generator{}).Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{"NamesQuery": "AB_" + nameDigest("#/paths/~1names/get/parameters/1/schema"), "Names200Headers": "XA_" + nameDigest("#/paths/~1names/get/responses/200/headers/X_A/schema")}
	found := make(map[string]bool)
	for _, artifact := range artifacts {
		if !strings.HasPrefix(artifact.Path, "models/") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), artifact.Path, artifact.Data, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			declaration, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			want, present := expected[declaration.Name.Name]
			if !present {
				return true
			}
			structure, ok := declaration.Type.(*ast.StructType)
			if !ok {
				t.Fatal("group is not a named struct", declaration.Name.Name)
			}
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					if name.Name == want {
						found[declaration.Name.Name] = true
						if declaration.Name.Name == "NamesQuery" && (field.Doc == nil || !strings.Contains(field.Doc.Text(), "Second parameter") || !strings.Contains(field.Doc.Text(), "Deprecated:")) {
							t.Fatal("parameter documentation is not attached to its field")
						}
					}
				}
			}
			return false
		})
	}
	for name, want := range expected {
		if !found[name] {
			t.Errorf("%s lacks source-derived field %s", name, want)
		}
	}
}
