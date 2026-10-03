package typescript

import (
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler/ir"
)

func TestSemanticModulePlanBoundsLongResourceParameterArtifact(t *testing.T) {
	t.Parallel()
	const parameter = "portableArtifactBoundaryParameterIdentifierWithDeliberatelyLongName"
	operation := pathOperation(
		"getPolicy",
		"GET",
		"/resources/{"+parameter+"}",
		parameter,
		map[string]any{"type": "string"},
	)
	operation.RouteKey = "GET " + operation.Path
	operation.Visibility = "public"
	document := &ir.Document{Raw: map[string]any{}, Operations: []ir.Operation{operation}}
	manifest := Manifest{Operations: []ManifestOperation{{
		RouteKey:    operation.RouteKey,
		OperationID: operation.OperationID,
		Method:      operation.Method,
		Path:        operation.Path,
		Visibility:  "public",
	}}}
	tree, err := buildResourceTree(document, manifest)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := buildSemanticModulePlan(document, manifest, manifest, tree, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, resource := range plan.resources {
		if !strings.Contains(resource.identity, "{"+parameter+"}") {
			continue
		}
		found = true
		if err := validateArtifactPath(resource.path); err != nil {
			t.Fatalf("long resource parameter artifact = %q: %v", resource.path, err)
		}
	}
	if !found {
		t.Fatalf("long parameter resource was not planned: %#v", plan.resources)
	}
}

func TestValidateGeneratedArtifactsChecksHeaderAndPortableUniqueness(t *testing.T) {
	t.Parallel()
	valid := []Artifact{{Path: "internal/user.ts", Data: generatedSource([]byte("export {}\n"))}}
	if err := validateGeneratedArtifacts(valid); err != nil {
		t.Fatal(err)
	}
	withoutHeader := []Artifact{{Path: "internal/user.ts", Data: []byte("export {}\n")}}
	if err := validateGeneratedArtifacts(withoutHeader); err == nil || !strings.Contains(err.Error(), "header") {
		t.Fatalf("missing-header error = %v", err)
	}
	duplicate := append(valid, Artifact{Path: "INTERNAL/USER.ts", Data: generatedSource(nil)})
	if err := validateGeneratedArtifacts(duplicate); err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("duplicate error = %v", err)
	}
}
