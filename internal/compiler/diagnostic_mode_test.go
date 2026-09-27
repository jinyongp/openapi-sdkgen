package sdkgen

import (
	"path/filepath"
	"testing"

	"openapi-sdkgen/internal/diagnostic"
)

func TestStructuredCompilerDefaultsAndPreservesDiagnosticMode(t *testing.T) {
	input := []byte(`{"openapi":"3.1.0","info":{"title":"Mode","version":"1"},"paths":{}}`)

	baseline, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if baseline.DiagnosticMode != diagnostic.ModeFailFast {
		t.Fatalf("default mode = %q", baseline.DiagnosticMode)
	}

	collected, err := CompileResultWithOptions(input, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if collected.DiagnosticMode != diagnostic.ModeCollect {
		t.Fatalf("collect mode = %q", collected.DiagnosticMode)
	}

	if _, err := CompileResultWithOptions(input, CompileOptions{DiagnosticMode: "continue"}); err == nil {
		t.Fatal("invalid diagnostic mode was accepted")
	}
}

func TestInputLoadDiagnosticIdentityIsCheckoutIndependent(t *testing.T) {
	first, err := CompileInputResultWithOptions(filepath.Join(t.TempDir(), "checkout-a", "openapi.yaml"), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := CompileInputResultWithOptions(filepath.Join(t.TempDir(), "checkout-b", "openapi.yaml"), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	firstReport := diagnostic.NewReport(first.Diagnostics, first.SkippedPhases, first.Coverage...)
	secondReport := diagnostic.NewReport(second.Diagnostics, second.SkippedPhases, second.Coverage...)
	if len(firstReport.Diagnostics) != 1 || len(secondReport.Diagnostics) != 1 {
		t.Fatalf("reports = %#v %#v", firstReport, secondReport)
	}
	if firstReport.Diagnostics[0].ID != secondReport.Diagnostics[0].ID {
		t.Fatalf("root input issue id changed across checkout roots: %q != %q", firstReport.Diagnostics[0].ID, secondReport.Diagnostics[0].ID)
	}
}
