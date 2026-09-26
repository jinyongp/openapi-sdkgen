package sdkgen

import (
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
