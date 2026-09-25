package typescript

import (
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestOpenAPI30SchemaNormalizationsMatchEquivalentTargetSemantics(t *testing.T) {
	tests := []struct {
		name       string
		candidate  string
		equivalent string
	}{
		{name: "boolean true", candidate: "true", equivalent: `{}`},
		{name: "const", candidate: `{"type":"string","const":"ready"}`, equivalent: `{"type":"string","enum":["ready"]}`},
		{name: "nullable type array", candidate: `{"type":["string","null"],"minLength":2}`, equivalent: `{"type":"string","nullable":true,"minLength":2}`},
		{name: "numeric exclusive minimum", candidate: `{"type":"number","exclusiveMinimum":2.5}`, equivalent: `{"type":"number","minimum":2.5,"exclusiveMinimum":true}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compile := func(schema string) *sdkgen.Result {
				result, err := sdkgen.CompileResult([]byte(`{"openapi":"3.0.3","info":{"title":"Equivalence","version":"1"},"paths":{},"components":{"schemas":{"Value":` + schema + `}}}`))
				if err != nil {
					t.Fatal(err)
				}
				if result.Document == nil {
					t.Fatalf("compile result = %#v", result)
				}
				return &result
			}
			candidate := compile(test.candidate)
			equivalent := compile(test.equivalent)
			candidateSchema := candidate.Document.ComponentSchemas["Value"]
			equivalentSchema := equivalent.Document.ComponentSchemas["Value"]

			candidateType, err := schemaType(candidate.Document, candidateSchema, projectionInput)
			if err != nil {
				t.Fatal(err)
			}
			equivalentType, err := schemaType(equivalent.Document, equivalentSchema, projectionInput)
			if err != nil {
				t.Fatal(err)
			}
			if candidateType != equivalentType {
				t.Fatalf("public type differs: candidate %q equivalent %q", candidateType, equivalentType)
			}

			candidateWire, err := newWireRenderContext(wirePropertiesLiteral).wireSchemaDescriptorForDocument(candidate.Document, candidateSchema, projectionInput)
			if err != nil {
				t.Fatal(err)
			}
			equivalentWire, err := newWireRenderContext(wirePropertiesLiteral).wireSchemaDescriptorForDocument(equivalent.Document, equivalentSchema, projectionInput)
			if err != nil {
				t.Fatal(err)
			}
			if candidateWire != equivalentWire {
				t.Fatalf("wire descriptor differs:\ncandidate: %s\nequivalent: %s", candidateWire, equivalentWire)
			}
		})
	}
}
