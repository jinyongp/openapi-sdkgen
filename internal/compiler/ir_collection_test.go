package sdkgen

import (
	"testing"

	"openapi-sdkgen/internal/diagnostic"
)

func TestCollectModeReportsIndependentIRPrerequisiteFailures(t *testing.T) {
	input := []byte(`{
  "openapi":"3.1.1",
  "info":{"title":"IR prerequisites","version":"1"},
  "security":{"root":[]},
  "paths":{
    "/a":{"get":{
      "security":[{"first":"not-an-array"}],
      "responses":{"204":{"description":"OK"}}
    }},
    "/b":{"post":{
      "security":[{"second":[true]}],
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`)

	failFast, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if failFast.Document != nil || len(failFast.Diagnostics) != 1 {
		t.Fatalf("fail-fast result = %#v", failFast)
	}
	if got := failFast.Diagnostics[0]; got.Code != "SDKGEN-E150" || got.Location.Pointer != "#" {
		t.Fatalf("fail-fast diagnostic = %#v", got)
	}

	collected, err := CompileResultWithOptions(input, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if collected.Document != nil {
		t.Fatalf("collect mode built poisoned IR: %#v", collected.Document)
	}
	var irFindings []diagnostic.Diagnostic
	for _, value := range collected.Diagnostics {
		if value.Code == "SDKGEN-E150" {
			irFindings = append(irFindings, value)
		}
	}
	if len(irFindings) != 3 {
		t.Fatalf("diagnostics = %#v, want 3 independent IR findings", collected.Diagnostics)
	}
	wantPointers := map[string]bool{
		"#/security":                           true,
		"#/paths/~1a/get/security/0/first":     true,
		"#/paths/~1b/post/security/0/second/0": true,
	}
	for _, value := range irFindings {
		if !wantPointers[value.Location.Pointer] {
			t.Fatalf("unexpected IR finding location: %#v", value)
		}
		delete(wantPointers, value.Location.Pointer)
	}
	if len(wantPointers) != 0 {
		t.Fatalf("missing IR finding pointers: %#v", wantPointers)
	}

	var prerequisiteComplete, buildSkipped bool
	for _, coverage := range collected.Coverage {
		switch coverage.Analyzer {
		case "ir.prerequisite":
			prerequisiteComplete = coverage.Status == diagnostic.CoverageComplete
		case "ir.build":
			buildSkipped = coverage.Status == diagnostic.CoverageSkipped
		}
	}
	if !prerequisiteComplete || !buildSkipped {
		t.Fatalf("coverage = %#v", collected.Coverage)
	}
}
