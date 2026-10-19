package golang

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedAnnotatedEnumsAndDocumentation(t *testing.T) {
	schemas := map[string]any{
		"State": map[string]any{"description": "Available states.", "anyOf": []any{
			map[string]any{"const": "open", "title": "Open state", "description": "Accepts work."},
			map[string]any{"const": "closed", "description": "Use open instead.", "deprecated": true},
		}},
		"Mixed":     map[string]any{"oneOf": []any{map[string]any{"const": nil}, map[string]any{"const": map[string]any{"a": float64(1)}, "deprecated": true}}},
		"Old":       map[string]any{"type": "string", "enum": []any{"legacy"}, "deprecated": true},
		"Alias":     map[string]any{"$ref": "#/components/schemas/Old"},
		"Partial":   map[string]any{"anyOf": []any{map[string]any{"const": "old", "deprecated": true}, map[string]any{"const": "new"}}},
		"Asserted":  map[string]any{"oneOf": []any{map[string]any{"const": "x", "minLength": 2}, map[string]any{"const": "y"}}},
		"Duplicate": map[string]any{"oneOf": []any{map[string]any{"const": "x"}, map[string]any{"const": "x"}}},
		"Record":    map[string]any{"type": "object", "properties": map[string]any{"old": map[string]any{"$ref": "#/components/schemas/Old", "description": "Old field."}, "text": map[string]any{"type": "string", "description": "First\r\n//go:linkname unsafe\x00\nLast"}}},
		"Merged": map[string]any{"allOf": []any{
			map[string]any{"type": "object", "properties": map[string]any{"field": map[string]any{"type": "string", "deprecated": true}}},
			map[string]any{"type": "object", "properties": map[string]any{"field": map[string]any{"type": "string", "description": "Merged field."}}},
		}},
	}
	for _, version := range []string{"3.1.0", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			input, err := json.Marshal(map[string]any{"openapi": version, "info": map[string]any{"title": "Annotations", "version": "1"}, "paths": map[string]any{}, "components": map[string]any{"schemas": schemas}})
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
			artifacts, err := (Generator{}).Emit(plan)
			if err != nil {
				t.Fatal(err)
			}
			docs := make(map[string]string)
			for _, artifact := range artifacts {
				if !strings.HasPrefix(artifact.Path, "models/") {
					continue
				}
				file, err := parser.ParseFile(token.NewFileSet(), artifact.Path, artifact.Data, parser.ParseComments)
				if err != nil {
					t.Fatal(err)
				}
				for _, decl := range file.Decls {
					switch declaration := decl.(type) {
					case *ast.GenDecl:
						for _, spec := range declaration.Specs {
							switch item := spec.(type) {
							case *ast.TypeSpec:
								docs[item.Name.Name] = declaration.Doc.Text()
								if structure, ok := item.Type.(*ast.StructType); ok {
									for _, field := range structure.Fields.List {
										for _, name := range field.Names {
											docs[item.Name.Name+"."+name.Name] = field.Doc.Text()
										}
									}
								}
							case *ast.ValueSpec:
								for _, name := range item.Names {
									docs[name.Name] = item.Doc.Text()
								}
							}
						}
					case *ast.FuncDecl:
						if declaration.Recv == nil {
							docs[declaration.Name.Name] = declaration.Doc.Text()
						}
					}
				}
			}
			for name, want := range map[string]string{"StateOutput": "Available states.", "StateOutputStringOpen": "Open state", "StateOutputStringClosed": "Deprecated:", "MixedOutputObject1": "Deprecated:", "OldOutput": "Deprecated:", "AliasOutput": "Deprecated:", "RecordOutput.Old": "Deprecated:", "RecordOutput.Text": "Last", "MergedOutput.Field": "Deprecated:"} {
				if !strings.Contains(docs[name], want) {
					t.Errorf("%s documentation %q missing %q", name, docs[name], want)
				}
			}
			if strings.Contains(docs["PartialOutput"], "Deprecated:") {
				t.Fatal("partial deprecation applied to entire enum")
			}
			testGeneratedModule(t, plan, map[string]string{"api/models/annotations_test.go": annotatedEnumConsumer})
		})
	}
}

const annotatedEnumConsumer = `package models
import("context";"testing";"example.test/consumer/api/types")
func TestAnnotatedEnums(t *testing.T){
 if len(StateOutputValues())!=2||StateOutputStringOpen!=StateOutput("open"){t.Fatal("annotated scalar values missing")}
 if len(MixedOutputValues())!=2||!MixedOutputValues()[1].Equal(MixedOutputObject1()){t.Fatal("annotated mixed values missing")}
 x,_:=types.ValueFrom("x")
 if _,err:=DecodeAssertedOutput(context.Background(),x);err==nil{t.Fatal("enum promotion discarded branch assertion")}
 if _,err:=DecodeDuplicateOutput(context.Background(),x);err==nil{t.Fatal("enum promotion discarded oneOf duplicate semantics")}
 invalid,_:=types.ValueFrom("invalid");if IsStateOutput(invalid){t.Fatal("unknown enum accepted")}
}
`
