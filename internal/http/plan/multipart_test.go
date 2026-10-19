package plan

import (
	"reflect"
	"testing"

	"openapi-sdkgen/internal/compiler/ir"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func TestMultipartRootMaterializationKeepsOriginalReference(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{"label": map[string]any{"type": "string"}}}
	document := &ir.Document{OpenAPIVersionLine: "3.2", ComponentSchemas: map[string]map[string]any{"Envelope": schema}}
	reference := map[string]any{"$ref": "#/components/schemas/Envelope"}
	if got := MultipartInputSchema(document, "multipart/form-data", reference, nil); !reflect.DeepEqual(got, reference) {
		t.Fatalf("ordinary projection changed reference: %#v", got)
	}
	if got := MultipartInputSchema(document, "multipart/form-data", reference, nil, true); !reflect.DeepEqual(got, schema) {
		t.Fatalf("materialized occurrence = %#v", got)
	}
	if got := MultipartInputSchema(document, "application/json", reference, nil, true); !reflect.DeepEqual(got, reference) {
		t.Fatalf("JSON occurrence changed: %#v", got)
	}
}

func TestMultipartNestedPrefixInheritsItemEncoding(t *testing.T) {
	document := &ir.Document{OpenAPIVersionLine: "3.2"}
	part := map[string]any{"type": "string", "format": "binary"}
	schema := map[string]any{"type": "object", "properties": map[string]any{"bundle": map[string]any{"type": "array", "prefixItems": []any{part, part}}}}
	media := map[string]any{"encoding": map[string]any{"bundle": map[string]any{"contentType": "multipart/mixed", "prefixEncoding": []any{map[string]any{"contentType": "image/png"}}, "itemEncoding": map[string]any{"contentType": "application/pdf"}}}}
	projected := MultipartInputSchema(document, "multipart/form-data", schema, media).(map[string]any)
	bundle := projected["properties"].(map[string]any)["bundle"].(map[string]any)
	items := bundle["prefixItems"].([]any)
	for index, expected := range []string{"image/png", "application/pdf"} {
		binary, ok := items[index].(schemaplan.BinaryInput)
		if !ok || binary.ContentType != expected {
			t.Fatalf("part %d: %#v, want %s", index, items[index], expected)
		}
	}
	if _, changed := schema["properties"].(map[string]any)["bundle"].(map[string]any)["prefixItems"].([]any)[0].(schemaplan.BinaryInput); changed {
		t.Fatal("projection mutated the input schema")
	}
}

func TestMultipartPartSchemaPreservesFieldCorrelation(t *testing.T) {
	document := &ir.Document{OpenAPIVersionLine: "3.2"}
	stringPart := map[string]any{"type": "string"}
	numberPart := map[string]any{"type": "number"}
	schema := map[string]any{
		"oneOf": []any{
			map[string]any{"properties": map[string]any{"value": stringPart}},
			map[string]any{"properties": map[string]any{"value": stringPart}},
		},
	}
	got, err := MultipartPartSchema(document, schema, "value", false)
	if err != nil || !reflect.DeepEqual(got, map[string]any{"anyOf": []any{stringPart, stringPart}}) {
		t.Fatalf("whole-body exclusivity leaked into one part: %#v, %v", got, err)
	}
	schema = map[string]any{"properties": map[string]any{"field": stringPart}, "patternProperties": map[string]any{"^f": numberPart}, "additionalProperties": false}
	got, err = MultipartPartSchema(document, schema, "field", false)
	if err != nil || !reflect.DeepEqual(got, map[string]any{"allOf": []any{stringPart, numberPart}}) {
		t.Fatalf("direct and pattern assertions: %#v, %v", got, err)
	}
	schema = map[string]any{"properties": map[string]any{"0": stringPart}, "prefixItems": []any{numberPart}}
	for _, positional := range []bool{false, true} {
		expected := stringPart
		if positional {
			expected = numberPart
		}
		got, err := MultipartPartSchema(document, schema, "0", positional)
		if err != nil || !reflect.DeepEqual(got, expected) {
			t.Fatalf("numeric name and position conflated (%t): %#v, %v", positional, got, err)
		}
	}
}
