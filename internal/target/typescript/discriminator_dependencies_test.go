package typescript

import (
	"os"
	"reflect"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
)

func TestDiscriminatorOnlyDependenciesPreserveProjectionAndCycles(t *testing.T) {
	input, err := os.ReadFile("../../../test/typescript/fixtures/discriminator-dependencies.openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	document, err := sdkgen.Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	in, out := reachableComponentSchemaProjections(document, false)
	wantIn := map[string]bool{"Pet": true, "Cat": true, "OtherPet": true, "InputBase": true, "InputOnly": true}
	wantOut := map[string]bool{"Pet": true, "Cat": true, "OtherPet": true, "OutputBase": true, "OutputOnly": true}
	if !reflect.DeepEqual(in, wantIn) || !reflect.DeepEqual(out, wantOut) {
		t.Fatalf("projection closure: input=%v output=%v", in, out)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"pet", "cat", "other-pet", "input-only", "output-only"} {
		_ = artifactByPath(t, artifacts, "internal/schemas/"+name+".ts")
	}
	for _, artifact := range artifacts {
		if strings.Contains(artifact.Path, "/schemas/unused.") {
			t.Fatalf("unused schema retained: %s", artifact.Path)
		}
	}
	inputOnly := string(artifactByPath(t, artifacts, "internal/schemas/input-only.ts"))
	outputOnly := string(artifactByPath(t, artifacts, "internal/schemas/output-only.ts"))
	if strings.Contains(inputOnly, "export const outputWireSchema") || strings.Contains(outputOnly, "export const inputWireSchema") {
		t.Fatal("discriminator dependencies crossed input/output projections")
	}
}

func TestDiscriminatorReferencesUseCanonicalNames(t *testing.T) {
	for _, tt := range []struct{ name, reference string }{
		{"Cat", "Cat"}, {"Cat", "#/components/schemas/Cat"},
		{"Cat", "#%2Fcomponents%2Fschemas%2FCat"},
		{"Cat/~", "Cat/~"}, {"Cat/~", "#/components/schemas/Cat~1~0"},
	} {
		for _, field := range []string{"mapping", "defaultMapping"} {
			t.Run(field+"/"+tt.reference, func(t *testing.T) {
				document := &ir.Document{ComponentSchemas: map[string]map[string]any{tt.name: {"type": "string"}}}
				discriminator := map[string]any{"propertyName": "kind", field: tt.reference}
				if field == "mapping" {
					discriminator[field] = map[string]any{"value": tt.reference}
				}
				found := make(map[string]bool)
				visitComponentSchemaReferences(document, found, componentSchemaReachabilityRoot{
					value: map[string]any{"discriminator": discriminator}, path: []string{"components", "schemas", "Root"},
				})
				if !reflect.DeepEqual(found, map[string]bool{tt.name: true}) {
					t.Fatalf("references=%v", found)
				}
			})
		}
	}
}

func TestDiscriminatorDependenciesRespectStructuralLocations(t *testing.T) {
	document := &ir.Document{ComponentSchemas: map[string]map[string]any{"Used": {"type": "string"}, "Unused": {"type": "boolean"}}}
	dispatch := map[string]any{"propertyName": "kind", "mapping": map[string]any{"value": "Used"}}
	opaque := map[string]any{"discriminator": map[string]any{"propertyName": "kind", "mapping": map[string]any{"value": "Unused"}, "defaultMapping": "Unused"}}
	for _, location := range [][]string{
		{"paths", "/pets", "get", "responses", "200", "content", "application/json", "schema"},
		{"paths", "/pets", "post", "requestBody", "content", "application/json", "schema"},
		{"paths", "/pets", "get", "parameters", "0", "schema"},
		{"components", "headers", "Header", "schema"},
		{"components", "mediaTypes", "Events", "itemSchema"},
		{"paths", "/events", "get", "responses", "200", "content", "application/x-ndjson", "itemSchema"},
	} {
		found := make(map[string]bool)
		visitComponentSchemaReferences(document, found, componentSchemaReachabilityRoot{value: map[string]any{"discriminator": dispatch}, path: location})
		if !reflect.DeepEqual(found, map[string]bool{"Used": true}) {
			t.Fatalf("location %v: references=%v", location, found)
		}
	}
	for _, nested := range []string{"properties", "patternProperties", "$defs", "dependentSchemas", "allOf", "anyOf", "oneOf", "prefixItems", "items", "contentSchema", "if", "then", "else", "not", "contains"} {
		t.Run(nested, func(t *testing.T) {
			schema := map[string]any{"discriminator": dispatch}
			var child any = schema
			switch nested {
			case "properties", "patternProperties", "$defs", "dependentSchemas":
				child = map[string]any{"discriminator": schema}
			case "allOf", "anyOf", "oneOf", "prefixItems":
				child = []any{schema}
			}
			root := map[string]any{nested: child, "example": opaque, "const": opaque, "default": opaque, "enum": []any{opaque}, "examples": []any{opaque}, "x-private": opaque}
			found := make(map[string]bool)
			visitComponentSchemaReferences(document, found, componentSchemaReachabilityRoot{value: root, path: []string{"components", "schemas", "Root"}})
			if !reflect.DeepEqual(found, map[string]bool{"Used": true}) {
				t.Fatalf("references=%v", found)
			}
		})
	}
	// A property-map entry named discriminator is itself a schema, not a
	// discriminator object on its parent. Its arbitrary keywords are not refs.
	found := make(map[string]bool)
	visitComponentSchemaReferences(document, found, componentSchemaReachabilityRoot{
		value: map[string]any{"properties": map[string]any{"discriminator": opaque["discriminator"]}},
		path:  []string{"components", "schemas", "Root"},
	})
	if len(found) != 0 {
		t.Fatalf("named property interpreted as discriminator: %v", found)
	}
}
