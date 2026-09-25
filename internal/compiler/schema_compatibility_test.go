package sdkgen

import (
	"strings"
	"testing"

	"openapi-sdkgen/internal/diagnostic"
)

func TestCompatibilityNormalizesProvedOpenAPI30SchemasAndPreservesSourceMetadata(t *testing.T) {
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Schema compatibility","version":"1"},
  "paths":{},
  "components":{"schemas":{
    "Allowed":true,
    "Denied":false,
    "State":{"type":"string","const":"ready"},
    "Optional":{"type":["string","null"],"minLength":2},
    "Positive":{"type":"number","exclusiveMinimum":2.5},
    "Annotated":{"type":"string","$comment":"source-only","examples":["a"]}
  }}
}`)
	result, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil {
		t.Fatalf("result = %#v", result)
	}
	if len(result.Diagnostics) != 7 {
		t.Fatalf("diagnostics = %#v", result.Diagnostics)
	}
	for _, value := range result.Diagnostics {
		if value.Severity != diagnostic.SeverityWarning || value.Code != "SDKGEN-W140" ||
			value.Action == "" || value.Rule == "" {
			t.Fatalf("diagnostic = %#v", value)
		}
	}
	schemas := result.Document.ComponentSchemas
	if got := schemas["Allowed"]; len(got) != 0 {
		t.Fatalf("Allowed = %#v", got)
	}
	if got := schemas["Denied"]; len(got) != 1 {
		t.Fatalf("Denied = %#v", got)
	} else if negated, ok := got["not"].(map[string]any); !ok || len(negated) != 0 {
		t.Fatalf("Denied not = %#v", got["not"])
	}
	if got := schemas["State"]; got["const"] != nil || len(got["enum"].([]any)) != 1 || got["enum"].([]any)[0] != "ready" {
		t.Fatalf("State = %#v", got)
	}
	if got := schemas["Optional"]; got["type"] != "string" || got["nullable"] != true || got["minLength"] != 2 {
		t.Fatalf("Optional = %#v", got)
	}
	if got := schemas["Positive"]; got["minimum"] != 2.5 || got["exclusiveMinimum"] != true {
		t.Fatalf("Positive = %#v", got)
	}
	if got := schemas["Annotated"]; got["$comment"] != nil || got["examples"] != nil {
		t.Fatalf("Annotated = %#v", got)
	}
	metadata := string(result.Document.SourceMetadataJSON)
	for _, source := range []string{
		`"Allowed":true`,
		`"Denied":false`,
		`"const":"ready"`,
		`"type":["string","null"]`,
		`"exclusiveMinimum":2.5`,
		`"$comment":"source-only"`,
	} {
		if !strings.Contains(metadata, source) {
			t.Fatalf("source metadata lost %q: %s", source, metadata)
		}
	}
}

func TestCompatibilityRejectsUnprovedOpenAPI30SchemasPrecisely(t *testing.T) {
	tests := []struct {
		name    string
		schema  string
		rule    string
		pointer string
	}{
		{name: "general type array", schema: `{"type":["string","integer"]}`, rule: "COMP-SCHEMA-003", pointer: "#/components/schemas/Value"},
		{name: "exclusive bound algebra", schema: `{"minimum":1,"exclusiveMinimum":2}`, rule: "COMP-SCHEMA-004", pointer: "#/components/schemas/Value"},
		{name: "dialect keyword", schema: `{"$schema":"https://json-schema.org/draft/2020-12/schema"}`, rule: "COMP-SCHEMA-005", pointer: "#/components/schemas/Value"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := []byte(`{"openapi":"3.0.3","info":{"title":"Reject","version":"1"},"paths":{},"components":{"schemas":{"Value":` + test.schema + `}}}`)
			result, err := CompileResult(input)
			if err != nil {
				t.Fatal(err)
			}
			if result.Document != nil || len(result.Diagnostics) != 1 {
				t.Fatalf("result = %#v", result)
			}
			value := result.Diagnostics[0]
			if value.Code != "SDKGEN-E140" || value.Rule != test.rule || value.Action != "reject" ||
				value.Location.Pointer != test.pointer {
				t.Fatalf("diagnostic = %#v", value)
			}
		})
	}
}
