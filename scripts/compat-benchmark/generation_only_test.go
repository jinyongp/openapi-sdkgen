package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"openapi-sdkgen/internal/generator"
)

func TestGraphGenerationNeverCallsTypechecker(t *testing.T) {
	input := filepath.Join(t.TempDir(), "api.json")
	data := []byte(`{"openapi":"3.2.1","info":{"title":"Generation-only control","version":"1"},"paths":{"/items":{"get":{"responses":{"204":{"description":"OK"}}}}},"webhooks":{"event":{"post":{"responses":{"204":{"description":"OK"}}}}}}`)
	if err := os.WriteFile(input, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, selected := range []bool{false, true} {
		t.Run(fmt.Sprint(selected), func(t *testing.T) {
			var selection *benchmarkSelection
			if selected {
				selection = &benchmarkSelection{Document: "microsoft-graph-beta", InputSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), options: &generator.Selection{Routes: []string{"GET /items"}}}
			}
			forbidden := func(string) verificationResult {
				t.Fatal("Graph must not invoke a typechecker or compiling runtime probe")
				return verificationResult{}
			}
			result, err := benchmarkDocumentWithOptions(corpusSpec{ID: "microsoft-graph-beta", Cohort: "fixture", Input: "api.json"}, input, forbidden, selection, forbidden, generator.Options{})
			if err != nil {
				t.Fatal(err)
			}
			if result.Typecheck.Status != "not-run" || result.DocumentSuccess || result.CapabilityAdjustedSuccess {
				t.Fatalf("Graph counted as verified: %#v", result)
			}
			if len(result.SupportProfiles) != 1 || result.SupportProfiles[0].Generation.Status != "pass" || result.SupportProfiles[0].Typecheck.Status != "not-run" || result.SupportProfiles[0].Success {
				t.Fatal("server generation-only evidence missing")
			}
		})
	}
	candidates, err := selectBenchmarkCorpora([]corpusSpec{{ID: "github"}, {ID: "microsoft-graph-beta"}}, "")
	if err != nil || len(candidates) != 1 || candidates[0].ID != "github" {
		t.Fatalf("Graph remained in typecheck corpus: %v, %v", candidates, err)
	}
}
