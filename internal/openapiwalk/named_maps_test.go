package openapiwalk

import "testing"

func TestIsNamedMapRespectsAuthorDefinedAndPatternedCollections(t *testing.T) {
	tests := []struct {
		name string
		path []string
		want bool
	}{
		{name: "root paths", path: []string{"paths"}, want: true},
		{name: "path entry", path: []string{"paths", "/items"}, want: false},
		{name: "component schemas", path: []string{"components", "schemas"}, want: true},
		{name: "component named paths", path: []string{"components", "schemas", "paths"}, want: false},
		{name: "schema properties", path: []string{"components", "schemas", "Thing", "properties"}, want: true},
		{name: "property named properties", path: []string{"components", "schemas", "Thing", "properties", "properties"}, want: false},
		{name: "callback collection", path: []string{"paths", "/jobs", "post", "callbacks"}, want: true},
		{name: "callback object", path: []string{"paths", "/jobs", "post", "callbacks", "done"}, want: true},
		{name: "callback runtime expression", path: []string{"paths", "/jobs", "post", "callbacks", "done", "{$request.body#/url}"}, want: false},
		{name: "root security requirement", path: []string{"security", "0"}, want: true},
		{name: "operation security requirement", path: []string{"paths", "/items", "get", "security", "1"}, want: true},
		{name: "security scheme name", path: []string{"components", "securitySchemes", "security"}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsNamedMap(test.path); got != test.want {
				t.Fatalf("IsNamedMap(%v) = %v, want %v", test.path, got, test.want)
			}
		})
	}
}

func TestIsExtensionKeyDistinguishesExtensionsFromNamedEntries(t *testing.T) {
	tests := []struct {
		name string
		path []string
		key  string
		want bool
	}{
		{name: "ordinary object extension", path: []string{"info"}, key: "x-owner", want: true},
		{name: "ordinary non extension", path: []string{"info"}, key: "owner", want: false},
		{name: "component name beginning x", path: []string{"components", "schemas"}, key: "x-model", want: false},
		{name: "schema property beginning x", path: []string{"components", "schemas", "Thing", "properties"}, key: "x-field", want: false},
		{name: "paths patterned extension", path: []string{"paths"}, key: "x-paths", want: true},
		{name: "webhooks patterned extension", path: []string{"webhooks"}, key: "x-webhooks", want: true},
		{name: "callback object extension", path: []string{"paths", "/jobs", "post", "callbacks", "done"}, key: "x-callback", want: true},
		{name: "operation responses extension", path: []string{"paths", "/items", "get", "responses"}, key: "x-response", want: true},
		{name: "component response name beginning x", path: []string{"components", "responses"}, key: "x-response", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsExtensionKey(test.path, test.key); got != test.want {
				t.Fatalf("IsExtensionKey(%v, %q) = %v, want %v", test.path, test.key, got, test.want)
			}
		})
	}
}

func TestReferenceChildOpaqueKeepsLiteralDataAndNamedMapKeysSeparate(t *testing.T) {
	for _, field := range []string{"const", "dataValue", "default", "enum", "example", "serializedValue", "value"} {
		t.Run(field, func(t *testing.T) {
			if !IsOpaqueDataField(field, map[string]any{"$ref": "literal"}) {
				t.Fatalf("%s was not classified as opaque data", field)
			}
		})
	}
	if !IsOpaqueDataField("examples", []any{map[string]any{"$ref": "literal"}}) {
		t.Fatal("schema examples array was not classified as opaque data")
	}
	if IsOpaqueDataField("examples", map[string]any{"sample": map[string]any{"$ref": "structural"}}) {
		t.Fatal("OpenAPI examples map was classified as literal schema data")
	}

	if !ReferenceChildOpaque([]string{"components", "schemas", "Thing"}, "default", map[string]any{"$ref": "literal"}) {
		t.Fatal("schema default descendants were not kept opaque")
	}
	if ReferenceChildOpaque([]string{"components", "schemas"}, "default", map[string]any{"$ref": "component"}) {
		t.Fatal("component named default was mistaken for a literal-data field")
	}
	if ReferenceChildOpaque([]string{"components", "schemas"}, "x-model", map[string]any{"$ref": "component"}) {
		t.Fatal("component name beginning x- was mistaken for an extension")
	}
	if !ReferenceChildOpaque([]string{"paths"}, "x-internal", map[string]any{"$ref": "literal"}) {
		t.Fatal("Paths extension descendants were not kept opaque")
	}
}
