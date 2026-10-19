package golang

import (
	"encoding/json"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedContractAndValidatorNamespaces(t *testing.T) {
	boolean := map[string]any{"type": "boolean"}
	response := func(media string, schema any) map[string]any {
		return map[string]any{"description": "OK", "content": map[string]any{media: map[string]any{"schema": schema}}}
	}
	nested := response("application/json", map[string]any{"$ref": "#/components/schemas/Outer"})
	nested["headers"] = map[string]any{"X-Ok": map[string]any{"schema": boolean}}
	input := map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Namespaces", "version": "1"}, "components": map[string]any{"schemas": map[string]any{
		"Outer":                  map[string]any{"type": "object", "properties": map[string]any{"GetItem200Headers": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}}}},
		"GetThing200Application": map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string"}}},
	}}, "paths": map[string]any{
		"/nested":      map[string]any{"get": map[string]any{"operationId": "outerOutputGetItem", "responses": map[string]any{"200": nested}}},
		"/root":        map[string]any{"get": map[string]any{"operationId": "getThing", "responses": map[string]any{"200": response("application/output", map[string]any{"$ref": "#/components/schemas/GetThing200Application"}), "201": response("application/json", boolean)}}},
		"/names":       map[string]any{"get": map[string]any{"operationId": "collisionMedia", "responses": map[string]any{"200": map[string]any{"description": "OK", "content": map[string]any{"application/a-b": map[string]any{"schema": boolean}, "application/a_b": map[string]any{"schema": map[string]any{"type": "string"}}}}}}},
		"/component":   map[string]any{"get": map[string]any{"operationId": "schema0", "responses": map[string]any{"200": response("application/json", map[string]any{"$ref": "#/components/schemas/Outer"})}}},
		"/helper":      map[string]any{"get": map[string]any{"operationId": "foo200A", "responses": map[string]any{"200": response("application/json", map[string]any{"type": "object", "properties": map[string]any{"x": boolean}})}}},
		"/helper-root": map[string]any{"get": map[string]any{"operationId": "foo", "responses": map[string]any{"200": response("a/OutputProperty0", boolean), "201": response("application/json", boolean)}}},
	}}
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	document, err := compiler.Compile(encoded)
	if err != nil {
		t.Fatal(err)
	}
	names, err := planNames(document)
	if err != nil {
		t.Fatal(err)
	}
	if want := "GetThing_" + nameDigest("#/paths/~1root/get"); names.operations["GET /root"] != want {
		t.Fatalf("root collision name %s, want %s", names.operations["GET /root"], want)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": "package consumer_test\nimport \"testing\"\nfunc TestGeneratedContractsCompile(t *testing.T){}\n"})
}
