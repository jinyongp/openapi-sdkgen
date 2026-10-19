package golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"strings"
	"testing"
)

func TestGoMetadataAnnotationsWithoutAddon(t *testing.T) {
	for _, version := range []string{"3.0.4", "3.1.2", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			document, err := compiler.Compile([]byte(fmt.Sprintf(`{"openapi":%q,"info":{"title":"Annotations","version":"1"},"paths":{"/legacy":{"get":{"operationId":"legacy","summary":"Legacy endpoint","description":"Use the replacement.","deprecated":true,"parameters":[{"in":"query","name":"old","deprecated":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"OK","headers":{"X-Legacy":{"$ref":"#/components/headers/Legacy"}},"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Contract"}}}}}}}},"components":{"headers":{"Legacy":{"description":"Legacy header.","deprecated":true,"schema":{"type":"string"}}},"schemas":{"Contract":{"type":"string","title":"Contract title","description":"Contract documentation.","format":"email","default":"old","minLength":3,"maxLength":9,"pattern":".*","example":"sample"}}}}`, version)))
			if err != nil {
				t.Fatal(err)
			}
			plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
			if err != nil || len(diagnostics) != 0 {
				t.Fatal(err, diagnostics)
			}
			artifacts, err := (Generator{}).Emit(plan)
			if err != nil {
				t.Fatal(err)
			}
			var schemaDoc, parameterDoc, headerDoc string
			var clientDeprecated, referenceDeprecated, resourceDeprecated bool
			for _, artifact := range artifacts {
				if strings.HasPrefix(artifact.Path, "metadata/") {
					t.Fatal("document addon unexpectedly required")
				}
				if !strings.HasSuffix(artifact.Path, ".go") {
					continue
				}
				file, err := parser.ParseFile(token.NewFileSet(), artifact.Path, artifact.Data, parser.ParseComments)
				if err != nil {
					t.Fatal(err)
				}
				for _, declaration := range file.Decls {
					switch node := declaration.(type) {
					case *ast.FuncDecl:
						deprecated := strings.Contains(node.Doc.Text(), "Deprecated:")
						clientDeprecated = clientDeprecated || artifact.Path == "client.go" && node.Name.Name == "Legacy" && deprecated
						referenceDeprecated = referenceDeprecated || strings.HasPrefix(artifact.Path, "operations/") && node.Name.Name == "Legacy" && deprecated
						resourceDeprecated = resourceDeprecated || strings.HasPrefix(artifact.Path, "internal/resources/") && node.Name.Name == "Get" && deprecated
					case *ast.GenDecl:
						for _, specification := range node.Specs {
							typeSpec, ok := specification.(*ast.TypeSpec)
							if !ok {
								continue
							}
							if typeSpec.Name.Name == "ContractOutput" {
								schemaDoc += node.Doc.Text()
							}
							structure, ok := typeSpec.Type.(*ast.StructType)
							if !ok {
								continue
							}
							for _, field := range structure.Fields.List {
								if typeSpec.Name.Name == "LegacyQuery" {
									parameterDoc += field.Doc.Text()
								}
								if typeSpec.Name.Name == "Legacy200Headers" {
									headerDoc += field.Doc.Text()
								}
							}
						}
					}
				}
			}
			for _, text := range []string{"Contract title", "Contract documentation.", `format="email"`, `default="old"`, "minLength=3", "maxLength=9", `pattern=".*"`, `example="sample"`} {
				if !strings.Contains(schemaDoc, text) {
					t.Fatalf("schema annotation %q missing: %s", text, schemaDoc)
				}
			}
			if !clientDeprecated || !referenceDeprecated || !resourceDeprecated || !strings.Contains(parameterDoc, "Deprecated:") || !strings.Contains(headerDoc, "Deprecated:") || !strings.Contains(headerDoc, "Legacy header.") {
				t.Fatalf("deprecation metadata: client=%t reference=%t resource=%t parameter=%q header=%q", clientDeprecated, referenceDeprecated, resourceDeprecated, parameterDoc, headerDoc)
			}
		})
	}
}
