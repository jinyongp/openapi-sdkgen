package sdkgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler/compatibility"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
)

const duplicateOperationInput = `{"openapi":"3.2.1","info":{"title":"Identity","version":"1"},"paths":{
"/a":{"get":{"operationId":"same","x-sdk-visibility":"hidden","responses":{"204":{"description":"OK"}}}},
"/b":{"post":{"operationId":"same","responses":{"204":{"description":"OK"}}}}
}}`

func TestOperationIdentityIsCompilerDocumentError(t *testing.T) {
	for _, mode := range []diagnostic.Mode{diagnostic.ModeCollect, diagnostic.ModeFailFast} {
		t.Run(string(mode), func(t *testing.T) {
			result, err := CompileResultWithOptions([]byte(duplicateOperationInput), CompileOptions{DiagnosticMode: mode})
			if err != nil {
				t.Fatal(err)
			}
			var found []diagnostic.Diagnostic
			for _, value := range result.Diagnostics {
				if value.Rule == compatibility.RuleOperationIDUnique {
					found = append(found, value)
				}
			}
			if len(found) != 1 {
				t.Fatalf("diagnostics = %#v", result.Diagnostics)
			}
			value := found[0]
			if value.Phase != diagnostic.PhaseOpenAPI || value.Target != "" || value.Severity != diagnostic.SeverityError ||
				value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock || value.Route != "POST /b" ||
				value.Location.Pointer != "#/paths/~1b/post/operationId" || len(value.Related) != 1 ||
				value.Related[0].Pointer != "#/paths/~1a/get/operationId" {
				t.Fatalf("identity diagnostic = %#v", value)
			}
			var complete bool
			for _, coverage := range result.Coverage {
				if coverage.Analyzer == operationIdentityConformanceAnalyzer && coverage.Status == diagnostic.CoverageComplete {
					complete = true
				}
			}
			if !complete {
				t.Fatalf("identity coverage = %#v", result.Coverage)
			}
		})
	}
}

func TestDirectCompilerRejectsDuplicateOperationIDs(t *testing.T) {
	if _, err := Compile([]byte(duplicateOperationInput)); err == nil || !strings.Contains(err.Error(), "operationId") {
		t.Fatalf("direct compile error = %v", err)
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "openapi.json")
	if err := os.WriteFile(input, []byte(duplicateOperationInput), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := CompileFile(input); err == nil || !strings.Contains(err.Error(), "operationId") {
		t.Fatalf("file compile error = %v", err)
	}
	result, err := CompileFileResultWithOptions(input, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range result.Diagnostics {
		if value.Rule == compatibility.RuleOperationIDUnique {
			if value.Location.Source != input || len(value.Related) != 1 || value.Related[0].Source != input {
				t.Fatalf("source = %#v", value)
			}
			return
		}
	}
	t.Fatal("file result omitted duplicate ID diagnostic")
}

func TestCompilerOperationIdentityUsesExactDeclaredIDs(t *testing.T) {
	input := strings.Replace(duplicateOperationInput, `"operationId":"same","responses"`, `"operationId":"Same","responses"`, 1)
	if _, err := Compile([]byte(input)); err != nil {
		t.Fatal(err)
	}
	input = strings.ReplaceAll(duplicateOperationInput, `"operationId":"same",`, "")
	if _, err := Compile([]byte(input)); err != nil {
		t.Fatal(err)
	}
}
