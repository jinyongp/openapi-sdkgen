package sdkgen

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaLocationNormalizationPreservesRecursiveIdentityAndOpaqueData(t *testing.T) {
	var input map[string]any
	if err := json.Unmarshal([]byte(`{"openapi":"3.1.1","info":{"title":"Refs","version":"1"},"paths":{},"components":{"schemas":{"Root":{"type":"object","$defs":{"Node":{"type":"object","properties":{"next":{"$ref":"#/components/schemas/Root/$defs/Node"}}}},"properties":{"one":{"$ref":"#/components/schemas/Root/$defs/Node"},"two":{"$ref":"#/components/schemas/Root/$defs/Node","description":"sibling"},"literal":{"const":{"$ref":"#/not-a-schema"}}}}}}}`), &input); err != nil {
		t.Fatal(err)
	}
	value, err := normalizeNestedSchemaReferences(input)
	if err != nil {
		t.Fatal(err)
	}
	root := value.(map[string]any)
	schemas := root["components"].(map[string]any)["schemas"].(map[string]any)
	properties := schemas["Root"].(map[string]any)["properties"].(map[string]any)
	ref := properties["one"].(map[string]any)["$ref"].(string)
	if properties["two"].(map[string]any)["$ref"] != ref || properties["two"].(map[string]any)["description"] != "sibling" {
		t.Fatal("identity or sibling lost")
	}
	alias := schemas[strings.TrimPrefix(ref, "#/components/schemas/")].(map[string]any)
	if alias["properties"].(map[string]any)["next"].(map[string]any)["$ref"] != ref {
		t.Fatal("recursive identity lost")
	}
	if properties["literal"].(map[string]any)["const"].(map[string]any)["$ref"] != "#/not-a-schema" {
		t.Fatal("literal changed")
	}
	if len(schemas) != 2 {
		t.Fatalf("unbounded identities: %d", len(schemas))
	}
}

func TestSchemaLocationNormalizationRejectsInvalidTargets(t *testing.T) {
	for _, ref := range []string{"#/info", "#/components/schemas/Root/description", "#/components/schemas/Root/allOf/01", "#/components/schemas/Root/allOf/1", "#/components/schemas/Root/properties/bad~2", "#/components/schemas/Root/properties/%xx"} {
		t.Run(ref, func(t *testing.T) {
			root := map[string]any{"openapi": "3.1.1", "info": map[string]any{"title": "Refs"}, "components": map[string]any{"schemas": map[string]any{"Root": map[string]any{"description": "text", "allOf": []any{map[string]any{}}, "properties": map[string]any{"use": map[string]any{"$ref": ref}}}}}}
			if _, err := normalizeNestedSchemaReferences(root); err == nil {
				t.Fatal("invalid Schema target accepted")
			}
		})
	}
}

func TestSchemaLocationNormalizationDecodesPercentOnceAndResourcePointers(t *testing.T) {
	var input map[string]any
	if err := json.Unmarshal([]byte(`{"openapi":"3.1.1","paths":{},"components":{"schemas":{"Root":{"$id":"https://example.test/schema","$defs":{"Token":{"type":"string"}},"properties":{"%69d":{"type":"integer"},"resource":{"$ref":"#/$defs/Token"},"literal-percent":{"$ref":"#/components/schemas/Root/properties/%2569d"}}}}}}`), &input); err != nil {
		t.Fatal(err)
	}
	value, err := normalizeNestedSchemaReferences(input)
	if err != nil {
		t.Fatal(err)
	}
	schemas := value.(map[string]any)["components"].(map[string]any)["schemas"].(map[string]any)
	props := schemas["Root"].(map[string]any)["properties"].(map[string]any)
	for key, expected := range map[string]string{"resource": "string", "literal-percent": "integer"} {
		ref := props[key].(map[string]any)["$ref"].(string)
		if schemas[strings.TrimPrefix(ref, "#/components/schemas/")].(map[string]any)["type"] != expected {
			t.Fatalf("%s reference changed", key)
		}
	}
}

func TestSchemaLocationsDistinguishNamedSchemasFromLiteralFields(t *testing.T) {
	var input map[string]any
	if err := json.Unmarshal([]byte(`{"openapi":"3.1.1","paths":{},"components":{"schemas":{"example":{"type":"string","$anchor":"named"},"Root":{"type":"object","$defs":{"default":{"type":"integer","$anchor":"number"}},"properties":{"value":{"$ref":"#named"},"example":{"$ref":"#number"},"enum":{"$ref":"#/components/schemas/Root/$defs/default"},"default":{"$ref":"#/components/schemas/example"},"literal":{"const":{"$ref":"#/not-schema"},"examples":[{"$id":"https://bad.example/","$ref":"#bad"}]}}}}}}`), &input); err != nil {
		t.Fatal(err)
	}
	normalized, err := normalizeNestedSchemaReferences(input)
	if err != nil {
		t.Fatal(err)
	}
	schemas := normalized.(map[string]any)["components"].(map[string]any)["schemas"].(map[string]any)
	properties := schemas["Root"].(map[string]any)["properties"].(map[string]any)
	if properties["value"].(map[string]any)["$ref"] != "#/components/schemas/example" {
		t.Fatal("named component anchor skipped")
	}
	if properties["example"].(map[string]any)["$ref"] != properties["enum"].(map[string]any)["$ref"] {
		t.Fatal("named property/$defs normalization skipped")
	}
	if properties["literal"].(map[string]any)["const"].(map[string]any)["$ref"] != "#/not-schema" {
		t.Fatal("literal interpreted as Schema")
	}
}

func TestSchemaLocationReferencesKeepExplicitResourceIdentity(t *testing.T) {
	var input map[string]any
	if err := json.Unmarshal([]byte(`{"openapi":"3.1.1","paths":{},"components":{"schemas":{"Token":{"type":"string"},"Resource":{"$id":"https://example.test/resource","$defs":{"Token":{"type":"integer"}}},"Use":{"properties":{"missing":{"$ref":"https://unloaded.example/schema#/components/schemas/Token"},"known":{"$ref":"https://example.test/resource#/$defs/Token"}}}}}}`), &input); err != nil {
		t.Fatal(err)
	}
	value, err := normalizeNestedSchemaReferences(input)
	if err != nil {
		t.Fatal(err)
	}
	schemas := value.(map[string]any)["components"].(map[string]any)["schemas"].(map[string]any)
	properties := schemas["Use"].(map[string]any)["properties"].(map[string]any)
	if properties["missing"].(map[string]any)["$ref"] != "https://unloaded.example/schema#/components/schemas/Token" {
		t.Fatal("foreign resource silently rebound to local component")
	}
	reference := properties["known"].(map[string]any)["$ref"].(string)
	if schemas[strings.TrimPrefix(reference, "#/components/schemas/")].(map[string]any)["type"] != "integer" {
		t.Fatal("known resource selected local Token")
	}
}
