package ir

import (
	"strings"
	"testing"

	openapidoc "openapi-sdkgen/internal/compiler/openapi"
)

func TestValidatePrerequisitesCollectsIndependentSecurityFailures(t *testing.T) {
	document := &openapidoc.Document{
		Version: openapidoc.Version31,
		Raw: map[string]any{
			"openapi":  "3.1.1",
			"security": map[string]any{"root": []any{}},
			"paths": map[string]any{
				"/a": map[string]any{
					"get": map[string]any{
						"security": []any{map[string]any{"first": "not-an-array"}},
					},
				},
				"/b": map[string]any{
					"post": map[string]any{
						"security": []any{map[string]any{"second": []any{true}}},
					},
				},
			},
		},
	}

	findings := ValidatePrerequisites(document)
	if len(findings) != 3 {
		t.Fatalf("findings = %#v, want 3 independent failures", findings)
	}
	pointers := []string{findings[0].Pointer, findings[1].Pointer, findings[2].Pointer}
	want := []string{"#/security", "#/paths/~1a/get/security/0/first", "#/paths/~1b/post/security/0/second/0"}
	for index := range want {
		if pointers[index] != want[index] {
			t.Fatalf("pointers = %#v, want %#v", pointers, want)
		}
	}

	_, err := Build(document)
	if err == nil || !strings.Contains(err.Error(), "root security: security must be an array") {
		t.Fatalf("Build error = %v", err)
	}
}

func TestBuildPreservesReferenceFailureOrderingAheadOfLaterCollectibleIRFailure(t *testing.T) {
	document := &openapidoc.Document{
		Version: openapidoc.Version31,
		Raw: map[string]any{
			"openapi": "3.1.1",
			"paths": map[string]any{
				"/a": map[string]any{"$ref": "#/paths/~1missing"},
				"/b": map[string]any{
					"get": map[string]any{
						"security": []any{map[string]any{"broken": "not-an-array"}},
					},
				},
			},
		},
	}
	findings := ValidatePrerequisites(document)
	if len(findings) != 1 || findings[0].Pointer != "#/paths/~1b/get/security/0/broken" {
		t.Fatalf("collectible findings = %#v", findings)
	}
	_, err := Build(document)
	if err == nil || !IsReferenceError(err) {
		t.Fatalf("Build error = %v, want earlier reference failure", err)
	}
}

func TestValidatePrerequisitesKeepsReferenceFailuresOutOfIRCollection(t *testing.T) {
	document := &openapidoc.Document{
		Version: openapidoc.Version31,
		Raw: map[string]any{
			"openapi": "3.1.1",
			"paths": map[string]any{
				"/items": map[string]any{"$ref": "#/paths/~1missing"},
			},
		},
	}
	if findings := ValidatePrerequisites(document); len(findings) != 0 {
		t.Fatalf("reference-owned findings leaked into IR prerequisites: %#v", findings)
	}
	_, err := Build(document)
	if err == nil || !IsReferenceError(err) {
		t.Fatalf("Build error = %v, want reference-owned failure", err)
	}
}
