package sdkgen

import (
	"os"
	"path/filepath"
	"testing"

	"openapi-sdkgen/internal/diagnostic"
)

func TestCollectModeAccumulatesIndependentSourceSafeBlockers(t *testing.T) {
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Collected source blockers","version":"1"},
  "x-sdkgen-owned":true,
  "paths":{
    "/items":{"get":{"responses":{"200":{
      "description":"OK",
      "content":{"application/json":{"schema":{"$ref":"#/components/schemas/Missing"}}}
    }}}}
  },
  "components":{"schemas":{"Bad":{"type":["string","integer"]}}}
}`)

	failFast, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if failFast.Document != nil || len(failFast.Diagnostics) != 1 ||
		failFast.Diagnostics[0].Code != "SDKGEN-E140" {
		t.Fatalf("fail-fast result = %#v", failFast)
	}

	collected, err := CompileResultWithOptions(input, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if collected.Document != nil {
		t.Fatalf("collect mode built poisoned IR: %#v", collected.Document)
	}
	counts := map[string]int{}
	for _, value := range collected.Diagnostics {
		counts[value.Code]++
	}
	for _, code := range []string{"SDKGEN-E120", "SDKGEN-E140", "SDKGEN-E160"} {
		if counts[code] != 1 {
			t.Fatalf("diagnostics = %#v, want exactly one %s", collected.Diagnostics, code)
		}
	}
	if len(collected.Coverage) != 3 {
		t.Fatalf("coverage = %#v", collected.Coverage)
	}
	for _, item := range collected.Coverage {
		if item.Status != diagnostic.CoverageComplete {
			t.Fatalf("coverage = %#v", collected.Coverage)
		}
	}
}

func TestCollectModeKeepsCompatibilityQuarantinedReferenceUnreachable(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "openapi.json")
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Quarantined external body","version":"1"},
  "paths":{
    "/items":{"get":{
      "operationId":"unsafeBody",
      "requestBody":{"$ref":"./missing.yaml#/components/requestBodies/Body"},
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`)
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := CompileFileResultWithOptions(path, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil {
		t.Fatalf("quarantined source failed collect compilation: %#v", result)
	}
	for _, value := range result.Diagnostics {
		if value.Code == "SDKGEN-E120" {
			t.Fatalf("quarantined reference was traversed: %#v", result.Diagnostics)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, "missing.yaml")); !os.IsNotExist(err) {
		t.Fatalf("unexpected missing reference file state: %v", err)
	}
}
