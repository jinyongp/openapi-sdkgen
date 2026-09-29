package openapiwalk

import "testing"

func TestObjectContextAt(t *testing.T) {
	tests := []struct {
		name string
		path []string
		want ObjectContext
	}{
		{name: "root", want: ObjectOpenAPI},
		{name: "info", path: []string{"info"}, want: ObjectInfo},
		{name: "path item", path: []string{"paths", "/items"}, want: ObjectPathItem},
		{name: "operation", path: []string{"paths", "/items", "get"}, want: ObjectOperation},
		{name: "additional operation", path: []string{"paths", "/items", "additionalOperations", "PURGE"}, want: ObjectOperation},
		{name: "parameter", path: []string{"components", "parameters", "Limit"}, want: ObjectParameter},
		{name: "component request body", path: []string{"components", "requestBodies", "Body"}, want: ObjectRequestBody},
		{name: "operation request body", path: []string{"paths", "/items", "post", "requestBody"}, want: ObjectRequestBody},
		{name: "response", path: []string{"paths", "/items", "get", "responses", "200"}, want: ObjectResponse},
		{name: "header", path: []string{"components", "headers", "Trace"}, want: ObjectHeader},
		{name: "link", path: []string{"paths", "/items", "get", "responses", "200", "links", "next"}, want: ObjectLink},
		{name: "callback", path: []string{"paths", "/items", "post", "callbacks", "done"}, want: ObjectCallback},
		{name: "example", path: []string{"components", "examples", "Sample"}, want: ObjectExample},
		{name: "security scheme", path: []string{"components", "securitySchemes", "Bearer"}, want: ObjectSecurityScheme},
		{name: "component media type", path: []string{"components", "mediaTypes", "JSON"}, want: ObjectMediaType},
		{name: "media type", path: []string{"paths", "/items", "get", "responses", "200", "content", "application/json"}, want: ObjectMediaType},
		{name: "encoding", path: []string{"paths", "/items", "post", "requestBody", "content", "multipart/form-data", "encoding", "file"}, want: ObjectEncoding},
		{name: "parameter example", path: []string{"paths", "/items", "get", "parameters", "0", "examples", "sample"}, want: ObjectExample},
		{name: "header example", path: []string{"components", "headers", "Trace", "examples", "sample"}, want: ObjectExample},
		{name: "link parameter expression is not parameter object", path: []string{"components", "links", "Next", "parameters", "id"}, want: ObjectUnknown},
		{name: "component schema", path: []string{"components", "schemas", "Thing"}, want: ObjectSchema},
		{name: "property schema", path: []string{"components", "schemas", "Thing", "properties", "id"}, want: ObjectSchema},
		{name: "inline schema", path: []string{"paths", "/items", "get", "parameters", "0", "schema"}, want: ObjectSchema},
		{name: "callback path item", path: []string{"paths", "/items", "post", "callbacks", "done", "{$request.body#/url}"}, want: ObjectPathItem},
		{name: "schema property named links", path: []string{"components", "schemas", "Thing", "properties", "links"}, want: ObjectSchema},
		{name: "schema properties container under property named links", path: []string{"components", "schemas", "Thing", "properties", "links", "properties"}, want: ObjectUnknown},
		{name: "schema scalar under property named properties", path: []string{"components", "schemas", "Thing", "properties", "properties", "nullable"}, want: ObjectUnknown},
		{name: "schema property named responses", path: []string{"components", "schemas", "Thing", "properties", "responses"}, want: ObjectSchema},
		{name: "schema property named headers", path: []string{"components", "schemas", "Thing", "properties", "headers"}, want: ObjectSchema},
		{name: "schema property named content", path: []string{"components", "schemas", "Thing", "properties", "content"}, want: ObjectSchema},
		{name: "schema property named callbacks", path: []string{"components", "schemas", "Thing", "properties", "callbacks"}, want: ObjectSchema},
		{name: "sequence item schema", path: []string{"paths", "/events", "get", "responses", "200", "content", "application/x-ndjson", "itemSchema"}, want: ObjectSchema},
		{name: "sequence nested item schema", path: []string{"components", "mediaTypes", "Events", "itemSchema", "properties", "value"}, want: ObjectSchema},
		{name: "opaque example itemSchema", path: []string{"components", "mediaTypes", "Events", "example", "itemSchema"}, want: ObjectUnknown},
		{name: "unknown", path: []string{"x-custom"}, want: ObjectUnknown},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ObjectContextAt(test.path); got != test.want {
				t.Fatalf("context = %q, want %q", got, test.want)
			}
		})
	}
}

func TestStructuralPositionAtRootKeepsAmbiguousRootUnknown(t *testing.T) {
	if got := StructuralPositionAtRoot(ObjectUnknown, nil); got.Object != ObjectUnknown {
		t.Fatalf("ambiguous root context = %#v, want unknown", got)
	}
	if got := StructuralPositionAtRoot(ObjectUnknown, []string{"properties", "value"}); got.Object != ObjectUnknown {
		t.Fatalf("ambiguous nested context = %#v, want unknown", got)
	}
}

func TestStructuralPositionAtDistinguishesSchemaKeywordsFromNamedEntries(t *testing.T) {
	tests := []struct {
		name              string
		path              []string
		object            ObjectContext
		keyword           string
		namedMapContainer bool
		namedMapEntry     bool
	}{
		{
			name:    "additional properties keyword",
			path:    []string{"components", "schemas", "Thing", "additionalProperties"},
			object:  ObjectSchema,
			keyword: "additionalProperties",
		},
		{
			name:          "property named additional properties",
			path:          []string{"components", "schemas", "Thing", "properties", "additionalProperties"},
			object:        ObjectSchema,
			keyword:       "properties",
			namedMapEntry: true,
		},
		{
			name:              "properties collection",
			path:              []string{"components", "schemas", "Thing", "properties"},
			object:            ObjectUnknown,
			keyword:           "properties",
			namedMapContainer: true,
		},
		{
			name:          "component link entry",
			path:          []string{"components", "links", "Next"},
			object:        ObjectLink,
			keyword:       "links",
			namedMapEntry: true,
		},
		{
			name:          "response link entry",
			path:          []string{"paths", "/items", "get", "responses", "200", "links", "next"},
			object:        ObjectLink,
			keyword:       "links",
			namedMapEntry: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := StructuralPositionAt(test.path)
			if got.Object != test.object || got.Keyword != test.keyword ||
				got.NamedMapContainer != test.namedMapContainer || got.NamedMapEntry != test.namedMapEntry {
				t.Fatalf("position = %#v", got)
			}
		})
	}
}
