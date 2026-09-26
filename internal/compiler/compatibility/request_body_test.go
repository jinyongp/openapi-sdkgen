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
