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
		{name: "media type", path: []string{"paths", "/items", "get", "responses", "200", "content", "application/json"}, want: ObjectMediaType},
		{name: "encoding", path: []string{"paths", "/items", "post", "requestBody", "content", "multipart/form-data", "encoding", "file"}, want: ObjectEncoding},
		{name: "component schema", path: []string{"components", "schemas", "Thing"}, want: ObjectSchema},
		{name: "property schema", path: []string{"components", "schemas", "Thing", "properties", "id"}, want: ObjectSchema},
		{name: "inline schema", path: []string{"paths", "/items", "get", "parameters", "0", "schema"}, want: ObjectSchema},
		{name: "callback path item", path: []string{"paths", "/items", "post", "callbacks", "done", "{$request.body#/url}"}, want: ObjectPathItem},
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
