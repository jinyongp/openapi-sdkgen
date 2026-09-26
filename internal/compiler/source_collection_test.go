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
	rules := map[string]int{}
	for _, value := range collected.Diagnostics {
		counts[value.Code]++
		if value.Rule != "" {
			rules[value.Rule]++
		}
	}
	if counts["SDKGEN-E120"] != 1 || counts["SDKGEN-E160"] != 1 || counts["SDKGEN-E140"] != 2 {
		t.Fatalf("diagnostics = %#v, want one E120, one E160, and two independently owned E140 findings", collected.Diagnostics)
	}
	if rules["COMP-SCHEMA-003"] != 1 || rules["COMP-VERSION-003"] != 1 {
		t.Fatalf("diagnostic rules = %#v, want compatibility and version ownership preserved", rules)
	}
	coverage := map[string]diagnostic.CoverageStatus{}
	for _, item := range collected.Coverage {
		coverage[item.Analyzer] = item.Status
		if item.Status != diagnostic.CoverageComplete {
			t.Fatalf("coverage = %#v", collected.Coverage)
		}
	}
	for _, analyzer := range []string{
		"source.compatibility",
		"source.reserved-extensions",
		"source.local-references",
		"source.version-identity",
		"source.version-features",
	} {
		if coverage[analyzer] != diagnostic.CoverageComplete {
			t.Fatalf("coverage = %#v, missing complete analyzer %q", collected.Coverage, analyzer)
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
