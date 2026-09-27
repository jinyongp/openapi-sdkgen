package compatibility

import (
	"reflect"
	"testing"

	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"openapi-sdkgen/internal/openapiwalk"
)

func TestConsumerPolicyNormalizesProvedOpenAPI30SchemaForms(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  any
		rules []string
	}{
		{
			name:  "boolean true",
			input: true,
			want:  map[string]any{},
			rules: []string{RuleSchemaBoolean30},
		},
		{
			name:  "boolean false",
			input: false,
			want:  map[string]any{"not": map[string]any{}},
			rules: []string{RuleSchemaBoolean30},
		},
		{
			name:  "const",
			input: map[string]any{"type": "string", "const": "ready"},
			want:  map[string]any{"type": "string", "enum": []any{"ready"}},
			rules: []string{RuleSchemaConst30},
		},
		{
			name:  "nullable type array",
			input: map[string]any{"type": []any{"string", "null"}, "minLength": 2},
			want:  map[string]any{"type": "string", "nullable": true, "minLength": 2},
			rules: []string{RuleSchemaNullableTypes30},
		},
		{
			name:  "exclusive minimum",
			input: map[string]any{"type": "number", "exclusiveMinimum": 2.5},
			want:  map[string]any{"type": "number", "minimum": 2.5, "exclusiveMinimum": true},
			rules: []string{RuleSchemaExclusive30},
		},
		{
			name: "combined proved rewrites",
			input: map[string]any{
				"type":             []any{"integer", "null"},
				"const":            4,
				"exclusiveMinimum": 1,
				"$comment":         "source-only",
			},
			want: map[string]any{
				"type":             "integer",
				"nullable":         true,
				"enum":             []any{4},
				"minimum":          1,
				"exclusiveMinimum": true,
			},
			rules: []string{RuleSchemaNullableTypes30, RuleSchemaConst30, RuleSchemaExclusive30, RuleSchemaKeywords30},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := (ConsumerPolicy{}).Apply(Context{
				Version: openapidoc.Version30,
				Object:  openapiwalk.ObjectSchema,
				Source:  "openapi.yaml",
				Pointer: "#/components/schemas/Value",
			}, test.input)
			if result.Reject || !result.Changed || !reflect.DeepEqual(result.Value, test.want) {
				t.Fatalf("result = %#v", result)
			}
			if len(result.Ledger) != len(test.rules) || len(result.Findings) != len(test.rules) {
				t.Fatalf("evidence = findings %#v ledger %#v", result.Findings, result.Ledger)
			}
			for index, rule := range test.rules {
				if result.Ledger[index].RuleID != rule {
					t.Fatalf("ledger[%d] = %#v, want %s", index, result.Ledger[index], rule)
				}
			}
		})
	}
}

func TestConsumerPolicyPreservesNativeOpenAPI30BooleanAdditionalProperties(t *testing.T) {
	for _, allowed := range []bool{true, false} {
		result := (ConsumerPolicy{}).Apply(Context{
			Version: openapidoc.Version30,
			Object:  openapiwalk.ObjectSchema,
			Keyword: "additionalProperties",
			Source:  "openapi.yaml",
			Pointer: "#/components/schemas/Value/additionalProperties",
		}, allowed)
		if result.Reject || result.Changed || result.Value != allowed ||
			len(result.Findings) != 0 || len(result.Ledger) != 0 {
			t.Fatalf("additionalProperties %t result = %#v", allowed, result)
		}
	}

	propertyNamedAdditional := (ConsumerPolicy{}).Apply(Context{
		Version: openapidoc.Version30,
		Object:  openapiwalk.ObjectSchema,
		Keyword: "properties",
		Source:  "openapi.yaml",
		Pointer: "#/components/schemas/Value/properties/additionalProperties",
	}, false)
	if propertyNamedAdditional.Reject || !propertyNamedAdditional.Changed ||
		len(propertyNamedAdditional.Findings) != 1 ||
		propertyNamedAdditional.Findings[0].RuleID != RuleSchemaBoolean30 {
		t.Fatalf("property named additionalProperties result = %#v", propertyNamedAdditional)
	}
}

func TestConsumerPolicyRejectsUnprovedOpenAPI30SchemaForms(t *testing.T) {
	tests := []struct {
		name  string
		value any
		rule  string
	}{
		{name: "general type array", value: map[string]any{"type": []any{"string", "integer"}}, rule: RuleSchemaNullableTypes30},
		{name: "overlapping numeric array", value: map[string]any{"type": []any{"number", "integer", "null"}}, rule: RuleSchemaNullableTypes30},
		{name: "const with enum", value: map[string]any{"const": "a", "enum": []any{"a", "b"}}, rule: RuleSchemaConst30},
		{name: "exclusive with bound", value: map[string]any{"minimum": 1, "exclusiveMinimum": 2}, rule: RuleSchemaExclusive30},
		{name: "dialect keyword", value: map[string]any{"$schema": "https://json-schema.org/draft/2020-12/schema"}, rule: RuleSchemaKeywords30},
		{name: "applicator keyword", value: map[string]any{"contains": map[string]any{"type": "string"}}, rule: RuleSchemaKeywords30},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := (ConsumerPolicy{}).Apply(Context{
				Version: openapidoc.Version30,
				Object:  openapiwalk.ObjectSchema,
				Pointer: "#/components/schemas/Value",
			}, test.value)
			if !result.Reject || len(result.Findings) != 1 || result.Findings[0].RuleID != test.rule || result.Findings[0].Action != ActionReject {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}
