package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectivePathBoundaryGeneratesAndAllowsFollowingDocument(t *testing.T) {
	root := t.TempDir()
	longPath := "/" + strings.Repeat("segment/", 26) + "items"
	inputs := []struct {
		id, path string
	}{
		{"long-path", longPath},
		{"following", "/items"},
	}
	var results []documentResult
	typechecks := 0
	for _, input := range inputs {
		path := filepath.Join(root, input.id+".json")
		data := `{"openapi":"3.0.3","info":{"title":"Path boundary","version":"1"},"paths":{"` + input.path + `":{"get":{"operationId":"listItems","responses":{"200":{"description":"ok"}}}}}}`
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		result, err := benchmarkDocument(corpusSpec{ID: input.id, Cohort: "fixture", Input: input.id + ".json"}, path, func(string) verificationResult {
			typechecks++
			return verificationResult{Status: "pass"}
		})
		if err != nil {
			t.Fatalf("%s aborted measurement: %v", input.id, err)
		}
		results = append(results, result)
	}
	boundary := results[0]
	if boundary.Generation.Status != "pass" {
		t.Fatalf("generation = %#v", boundary.Generation)
	}
	if boundary.ID != "long-path" || len(boundary.InputSHA256) != 64 || boundary.OpenAPIVersion != "3.0.3" {
		t.Fatalf("boundary input identity = %#v", boundary)
	}
	if !boundary.DocumentSuccess || !boundary.CapabilityAdjustedSuccess || !boundary.DiscoveryComplete || boundary.Typecheck.Status != "pass" {
		t.Fatalf("boundary generation was not verified: %#v", boundary)
	}
	if !boundary.OperationEmission.Available || boundary.OperationEmission.Count != 1 || boundary.Generation.DurationMillis == nil {
		t.Fatalf("boundary generation lost emission or timing: %#v", boundary)
	}
	if !results[1].DocumentSuccess || typechecks != 2 {
		t.Fatalf("following result = %#v; typechecks = %d", results[1], typechecks)
	}
	summary := summarizeDocuments("fixture", results)
	if summary.Documents != 2 || summary.SuccessfulDocuments != 2 || summary.OperationEmission.AvailableDocuments != 2 || summary.OperationEmission.Count != 2 {
		t.Fatalf("summary = %#v", summary)
	}
}
