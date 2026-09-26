package sdkgen

import (
	"testing"

	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
)

func TestVersionFeatureRejectCarriesStructuredCompatibilityDiagnostic(t *testing.T) {
	result, err := CompileResult([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Reject","version":"1"},
  "paths":{},
  "webhooks":{"event":{}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil || len(result.Diagnostics) != 1 {
		t.Fatalf("result = %#v", result)
	}
	value := result.Diagnostics[0]
	if value.Code != "SDKGEN-E140" || value.Location.Pointer != "#/webhooks" ||
		value.Rule != "COMP-VERSION-003" || value.Action != "reject" ||
		value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock {
		t.Fatalf("diagnostic = %#v", value)
	}
}

func TestCollectModeAccumulatesVersionFeatureRejectsWithoutChangingFailFastFirstFinding(t *testing.T) {
	input := []byte(`{
  "openapi":"3.1.0",
  "info":{"title":"Version collection","version":"1"},
  "$self":"https://example.test/openapi.yaml",
  "paths":{
    "/a":{"query":{"responses":{"200":{"description":"OK"}}}},
    "/b":{"additionalOperations":{"PURGE":{"responses":{"200":{"description":"OK"}}}}}
  },
  "components":{"mediaTypes":{}}
}`)

	failFast, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if failFast.Document != nil || len(failFast.Diagnostics) != 1 {
		t.Fatalf("fail-fast result = %#v", failFast)
	}
	first := failFast.Diagnostics[0]
	if first.Code != "SDKGEN-E140" || first.Location.Pointer != "#/$self" ||
		first.Rule != "COMP-VERSION-003" || first.Action != "reject" ||
		first.Scope != failure.ScopeDocument || first.Effect != failure.EffectBlock {
		t.Fatalf("fail-fast diagnostic = %#v", first)
	}

	collected, err := CompileResultWithOptions(input, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if collected.Document != nil {
		t.Fatalf("collect mode built poisoned IR: %#v", collected.Document)
	}
	var pointers []string
	for _, value := range collected.Diagnostics {
		if value.Rule != "COMP-VERSION-003" {
			continue
		}
		if value.Code != "SDKGEN-E140" || value.Phase != diagnostic.PhaseOpenAPI ||
			value.Action != "reject" || value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock {
			t.Fatalf("version diagnostic = %#v", value)
		}
		pointers = append(pointers, value.Location.Pointer)
	}
	wantPointers := []string{
		"#/$self",
		"#/components/mediaTypes",
		"#/paths/~1a/query",
		"#/paths/~1b/additionalOperations",
	}
	if len(pointers) != len(wantPointers) {
		t.Fatalf("version pointers = %#v, want %#v", pointers, wantPointers)
	}
	for index, want := range wantPointers {
		if pointers[index] != want {
			t.Fatalf("version pointers = %#v, want %#v", pointers, wantPointers)
		}
	}

	coverage := map[string]diagnostic.CoverageStatus{}
	for _, item := range collected.Coverage {
		coverage[item.Analyzer] = item.Status
	}
	if coverage["source.version-identity"] != diagnostic.CoverageComplete ||
		coverage["source.version-features"] != diagnostic.CoverageComplete {
		t.Fatalf("coverage = %#v", collected.Coverage)
	}
}

func TestCollectModeSkipsVersionFeatureAnalyzerWhenVersionIdentityIsUnavailable(t *testing.T) {
	input := []byte(`{
  "openapi":"4.0.0",
  "info":{"title":"Unknown version","version":"1"},
  "paths":{}
}`)
	result, err := CompileResultWithOptions(input, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil || !diagnostic.HasErrors(result.Diagnostics) {
		t.Fatalf("result = %#v", result)
	}
	var identityErrors int
	for _, value := range result.Diagnostics {
		if value.Code == "SDKGEN-E140" && value.Phase == diagnostic.PhaseOpenAPI {
			identityErrors++
		}
		if value.Rule == "COMP-VERSION-003" {
			t.Fatalf("version features ran without a version identity: %#v", result.Diagnostics)
		}
	}
	if identityErrors != 1 {
		t.Fatalf("diagnostics = %#v, want one version-identity error", result.Diagnostics)
	}
	for _, item := range result.Coverage {
		if item.Analyzer != "source.version-features" {
			continue
		}
		if item.Status != diagnostic.CoverageSkipped {
			t.Fatalf("version feature coverage = %#v", item)
		}
		var missing bool
		for _, prerequisite := range item.Prerequisites {
			if prerequisite.Name == "version-identity" && !prerequisite.Available {
				missing = true
			}
		}
		if !missing {
			t.Fatalf("version feature coverage lacks unavailable version identity: %#v", item)
		}
		return
	}
	t.Fatalf("coverage = %#v, missing source.version-features", result.Coverage)
}
