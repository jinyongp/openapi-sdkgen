package compatibility

import (
	"testing"

	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/openapiwalk"
)

func TestConsumerPolicyClassifiesOAS30RequestBodiesByMethodAndImpact(t *testing.T) {
	policy := ConsumerPolicy{}
	empty := map[string]any{
		"content": map[string]any{
			"application/x-www-form-urlencoded": map[string]any{
				"schema": map[string]any{"type": "object", "properties": map[string]any{}},
			},
		},
	}
	meaningful := map[string]any{
		"required": true,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{
					"type":       "object",
					"properties": map[string]any{"id": map[string]any{"type": "string"}},
				},
			},
		},
	}

	for _, method := range []string{"get", "head", "delete", "options", "trace"} {
		t.Run("empty-"+method, func(t *testing.T) {
			result := policy.Apply(Context{
				Version: openapidoc.Version30,
				Object:  openapiwalk.ObjectRequestBody,
				Pointer: "#/paths/~1items/" + method + "/requestBody",
			}, empty)
			if !result.Omit || result.Reject || len(result.Ledger) != 1 || result.Ledger[0].Action != ActionIgnore {
				t.Fatalf("result = %#v", result)
			}
		})
	}

	for _, method := range []string{"delete", "options"} {
		t.Run("meaningful-preserve-"+method, func(t *testing.T) {
			result := policy.Apply(Context{
				Version: openapidoc.Version30,
				Object:  openapiwalk.ObjectRequestBody,
				Pointer: "#/paths/~1items/" + method + "/requestBody",
			}, meaningful)
			if result.Omit || result.Reject || len(result.Findings) != 1 ||
				result.Findings[0].Action != ActionPreserveExtension {
				t.Fatalf("%s result = %#v", method, result)
			}
		})
	}

	for _, method := range []string{"get", "head", "trace"} {
		t.Run("meaningful-"+method, func(t *testing.T) {
			result := policy.Apply(Context{
				Version: openapidoc.Version30,
				Object:  openapiwalk.ObjectRequestBody,
				Pointer: "#/paths/~1items/" + method + "/requestBody",
			}, meaningful)
			if result.Omit || !result.Reject || result.Scope != failure.ScopeOperation || result.Effect != failure.EffectOmitOperation ||
				len(result.Findings) != 1 || result.Findings[0].Action != ActionReject ||
				result.Findings[0].Scope != failure.ScopeOperation || result.Findings[0].Effect != failure.EffectOmitOperation {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestConsumerPolicyTreatsReferencedOAS30UnsafeBodyAsMeaningfulWithoutResolution(t *testing.T) {
	result := (ConsumerPolicy{}).Apply(Context{
		Version: openapidoc.Version30,
		Object:  openapiwalk.ObjectRequestBody,
		Pointer: "#/paths/~1items/get/requestBody",
	}, map[string]any{"$ref": "#/components/requestBodies/Missing"})
	if !result.Reject || result.Scope != failure.ScopeOperation || result.Effect != failure.EffectOmitOperation ||
		len(result.Findings) != 1 || result.Findings[0].Action != ActionReject ||
		result.Findings[0].Scope != failure.ScopeOperation || result.Findings[0].Effect != failure.EffectOmitOperation {
		t.Fatalf("result = %#v", result)
	}
}

func TestConsumerPolicyPreservesLaterLineRequestBodies(t *testing.T) {
	for _, version := range []openapidoc.VersionLine{openapidoc.Version31, openapidoc.Version32} {
		result := (ConsumerPolicy{}).Apply(Context{
			Version: version,
			Object:  openapiwalk.ObjectRequestBody,
			Pointer: "#/paths/~1items/get/requestBody",
		}, map[string]any{
			"required": true,
			"content": map[string]any{
				"application/json": map[string]any{"schema": map[string]any{"type": "string"}},
			},
		})
		if result.Omit || result.Reject || result.Changed {
			t.Fatalf("%s result = %#v", version, result)
		}
	}
}

func TestConsumerPolicyDistinguishesStructurallyEmptyOAS30RequestBodySchemas(t *testing.T) {
	emptySchemas := map[string]any{
		"nil":                 nil,
		"annotations":         map[string]any{"title": "Payload", "description": "docs", "deprecated": true, "readOnly": true, "writeOnly": true, "example": map[string]any{"id": 1}, "examples": []any{1}, "x-note": "extension"},
		"object type":         map[string]any{"type": "object"},
		"empty properties":    map[string]any{"properties": map[string]any{}},
		"empty required":      map[string]any{"required": []any{}},
		"closed empty object": map[string]any{"type": "object", "properties": map[string]any{}, "required": []any{}, "additionalProperties": false, "nullable": false},
	}
	for name, schema := range emptySchemas {
		t.Run("empty-"+name, func(t *testing.T) {
			result := (ConsumerPolicy{}).Apply(Context{
				Version: openapidoc.Version30,
				Object:  openapiwalk.ObjectRequestBody,
				Pointer: "#/paths/~1items/get/requestBody",
			}, map[string]any{
				"content": map[string]any{
					"application/json": map[string]any{"schema": schema},
				},
			})
			if !result.Omit || result.Reject || !result.Changed || len(result.Findings) != 0 ||
				len(result.Ledger) != 1 || result.Ledger[0].Action != ActionIgnore {
				t.Fatalf("result = %#v", result)
			}
		})
	}

	meaningfulSchemas := map[string]any{
		"scalar type":                map[string]any{"type": "string"},
		"non-empty properties":       map[string]any{"properties": map[string]any{"id": map[string]any{"type": "string"}}},
		"required property":          map[string]any{"required": []any{"id"}},
		"additional properties":      map[string]any{"additionalProperties": true},
		"nullable":                   map[string]any{"nullable": true},
		"validation keyword":         map[string]any{"minProperties": 1},
		"boolean schema":             true,
		"malformed properties value": map[string]any{"properties": []any{}},
	}
	for name, schema := range meaningfulSchemas {
		t.Run("meaningful-"+name, func(t *testing.T) {
			result := (ConsumerPolicy{}).Apply(Context{
				Version: openapidoc.Version30,
				Object:  openapiwalk.ObjectRequestBody,
				Pointer: "#/paths/~1items/get/requestBody",
			}, map[string]any{
				"content": map[string]any{
					"application/json": map[string]any{"schema": schema},
				},
			})
			if result.Omit || !result.Reject || result.Scope != failure.ScopeOperation || result.Effect != failure.EffectOmitOperation {
				t.Fatalf("result = %#v", result)
			}
		})
	}

	for name, body := range map[string]map[string]any{
		"encoding": {
			"content": map[string]any{
				"application/x-www-form-urlencoded": map[string]any{
					"schema":   map[string]any{"type": "object", "properties": map[string]any{}},
					"encoding": map[string]any{"field": map[string]any{"style": "form"}},
				},
			},
		},
		"malformed media": {
			"content": map[string]any{"application/json": "unexpected"},
		},
	} {
		t.Run("meaningful-"+name, func(t *testing.T) {
			result := (ConsumerPolicy{}).Apply(Context{
				Version: openapidoc.Version30,
				Object:  openapiwalk.ObjectRequestBody,
				Pointer: "#/paths/~1items/get/requestBody",
			}, body)
			if result.Omit || !result.Reject {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}
