package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInternalPreparationFailureRetainsInputAndAllowsFollowingDocument(t *testing.T) {
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
		data := `{"openapi":"3.0.3","info":{"title":"Preparation failure","version":"1"},"paths":{"` + input.path + `":{"get":{"operationId":"listItems","responses":{"200":{"description":"ok"}}}}}}`
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
	failed := results[0]
	if failed.Generation.Status != "fail" || !strings.Contains(failed.Generation.Detail, "path exceeds 240 bytes") {
		t.Fatalf("generation = %#v", failed.Generation)
	}
	if failed.ID != "long-path" || len(failed.InputSHA256) != 64 || failed.OpenAPIVersion != "3.0.3" {
		t.Fatalf("failed input identity = %#v", failed)
	}
	if failed.DocumentSuccess || failed.CapabilityAdjustedSuccess || failed.DiscoveryComplete || failed.Typecheck.Status != "not-run" {
		t.Fatalf("preparation failure was recorded as verified: %#v", failed)
	}
	if failed.OperationEmission.Available || failed.OperationEmission.Count != 0 || failed.Generation.DurationMillis != nil {
		t.Fatalf("failure recorded a successful emission or timing: %#v", failed)
	}
	if !results[1].DocumentSuccess || typechecks != 1 {
		t.Fatalf("following result = %#v; typechecks = %d", results[1], typechecks)
	}
	summary := summarizeDocuments("fixture", results)
	if summary.Documents != 2 || summary.SuccessfulDocuments != 1 || summary.OperationEmission.AvailableDocuments != 1 || summary.OperationEmission.Count != 1 {
		t.Fatalf("summary = %#v", summary)
	}
}
