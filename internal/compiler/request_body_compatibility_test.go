package sdkgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
)

func TestCompatibilityIgnoresEmptyOAS30RequestBodies(t *testing.T) {
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Empty bodies","version":"1"},
  "paths":{
    "/get":{"get":{"operationId":"emptyGet","requestBody":{"content":{"application/x-www-form-urlencoded":{"schema":{"type":"object","properties":{}}}}},"responses":{"204":{"description":"OK"}}}},
    "/head":{"head":{"operationId":"emptyHead","requestBody":{"content":{"application/x-www-form-urlencoded":{"schema":{"type":"object","properties":{}}}}},"responses":{"204":{"description":"OK"}}}},
    "/delete":{"delete":{"operationId":"emptyDelete","requestBody":{"content":{"application/x-www-form-urlencoded":{"schema":{"type":"object","properties":{}}}}},"responses":{"204":{"description":"OK"}}}},
    "/options":{"options":{"operationId":"emptyOptions","requestBody":{"content":{"application/x-www-form-urlencoded":{"schema":{"type":"object","properties":{}}}}},"responses":{"204":{"description":"OK"}}}}
  }
}`)
	result, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil || len(result.Diagnostics) != 0 {
		t.Fatalf("compile result = %#v", result)
	}
	if metadata := string(result.Document.SourceMetadataJSON); !strings.Contains(metadata, `"requestBody"`) || !strings.Contains(metadata, `"emptyGet"`) {
		t.Fatalf("source metadata lost ignored request bodies: %s", metadata)
	}
	for _, operation := range result.Document.Operations {
		if operation.RequestBody != nil {
			t.Fatalf("%s request body survived = %#v", operation.OperationID, operation.RequestBody)
		}
	}
}

func TestCompatibilityPreservesMeaningfulOAS30DeleteAndOptionsBodiesWithWarning(t *testing.T) {
	for _, method := range []string{"delete", "options"} {
		t.Run(method, func(t *testing.T) {
			input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Preserved body","version":"1"},
  "paths":{"/items":{"` + method + `":{
    "operationId":"preservedBody",
    "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}},
    "responses":{"204":{"description":"OK"}}
  }}}
}`)
			result, err := CompileResult(input)
			if err != nil {
				t.Fatal(err)
			}
			if result.Document == nil || len(result.Document.Operations) != 1 || result.Document.Operations[0].RequestBody == nil {
				t.Fatalf("compile result = %#v", result)
			}
			if len(result.Diagnostics) != 1 {
				t.Fatalf("diagnostics = %#v", result.Diagnostics)
			}
			value := result.Diagnostics[0]
			if value.Severity != diagnostic.SeverityWarning || value.Code != "SDKGEN-W140" ||
				value.Location.Pointer != "#/paths/~1items/"+method+"/requestBody" {
				t.Fatalf("diagnostic = %#v", value)
			}
		})
	}
}

func TestCompatibilityPreservedOAS30OptionsBodyStillResolvesReferences(t *testing.T) {
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"OPTIONS reference","version":"1"},
  "paths":{"/items":{"options":{
    "operationId":"optionsItems",
    "requestBody":{"$ref":"#/components/requestBodies/Missing"},
    "responses":{"204":{"description":"OK"}}
  }}}
}`)
	result, err := CompileResult(input)
	if err != nil {
		t.Fatal(err)
	}
	foundReferenceError := false
	for _, value := range result.Diagnostics {
		if value.Code == "SDKGEN-E120" {
			foundReferenceError = true
			break
		}
	}
	if result.Document != nil || !foundReferenceError {
		t.Fatalf("compile result = %#v", result)
	}
}

func TestCompatibilityRejectsMeaningfulOAS30UnsafeBodiesBeforeReferenceResolution(t *testing.T) {
	for _, method := range []string{"get", "head", "trace"} {
		t.Run(method, func(t *testing.T) {
			input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Unsafe body","version":"1"},
  "paths":{"/items":{"` + method + `":{
    "operationId":"unsafeBody",
    "requestBody":{"$ref":"#/components/requestBodies/Missing"},
    "responses":{"204":{"description":"OK"}}
  }}}
}`)
			result, err := CompileResult(input)
			if err != nil {
				t.Fatal(err)
			}
			if result.Document == nil || len(result.Document.Operations) != 1 || len(result.Diagnostics) != 1 {
				t.Fatalf("compile result = %#v", result)
			}
			if body := result.Document.Operations[0].RequestBody; body != nil {
				t.Fatalf("quarantined request body survived = %#v", body)
			}
			if metadata := string(result.Document.SourceMetadataJSON); !strings.Contains(metadata, `"requestBody"`) ||
				!strings.Contains(metadata, `"#/components/requestBodies/Missing"`) {
				t.Fatalf("source metadata lost quarantined request body: %s", metadata)
			}
			if len(result.Document.SemanticRestrictions) != 1 {
				t.Fatalf("semantic restrictions = %#v", result.Document.SemanticRestrictions)
			}
			restriction := result.Document.SemanticRestrictions[0]
			if restriction.RuleID != "COMP-BODY-001" || restriction.Scope != failure.ScopeOperation ||
				restriction.Effect != failure.EffectOmitOperation ||
				restriction.Location.Pointer != "#/paths/~1items/"+method+"/requestBody" {
				t.Fatalf("restriction = %#v", restriction)
			}
			value := result.Diagnostics[0]
			if value.Code != "SDKGEN-W140" || value.Severity != diagnostic.SeverityWarning || value.Phase != diagnostic.PhaseOpenAPI ||
				value.Scope != failure.ScopeOperation || value.Effect != failure.EffectOmitOperation ||
				value.Location.Pointer != "#/paths/~1items/"+method+"/requestBody" {
				t.Fatalf("diagnostic = %#v", value)
			}
		})
	}
}

func TestCompatibilityQuarantinedUnsafeBodyDoesNotResolveExternalReference(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "openapi.json")
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Unsafe external body","version":"1"},
  "paths":{"/items":{"get":{
    "operationId":"unsafeBody",
    "requestBody":{"$ref":"./missing.yaml#/components/requestBodies/Body"},
    "responses":{"204":{"description":"OK"}}
  }}}
}`)
	if err := os.WriteFile(path, input, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := CompileFileResultWithOptions(path, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil || len(result.Diagnostics) != 1 || result.ReusableInput == nil {
		t.Fatalf("compile result = %#v", result)
	}
	if len(result.Document.SemanticRestrictions) != 1 {
		t.Fatalf("semantic restrictions = %#v", result.Document.SemanticRestrictions)
	}
	value := result.Diagnostics[0]
	if value.Code != "SDKGEN-W140" || value.Severity != diagnostic.SeverityWarning || value.Phase != diagnostic.PhaseOpenAPI ||
		value.Scope != failure.ScopeOperation || value.Effect != failure.EffectOmitOperation {
		t.Fatalf("diagnostic = %#v", value)
	}
	if _, err := os.Stat(filepath.Join(directory, "missing.yaml")); !os.IsNotExist(err) {
		t.Fatalf("unexpected missing reference file state: %v", err)
	}
}

func TestCompatibilityPreservesLaterLineGETAndHEADBodiesForTargetPreflight(t *testing.T) {
	for _, version := range []string{"3.1.1", "3.2.0"} {
		for _, method := range []string{"get", "head"} {
			t.Run(version+"-"+method, func(t *testing.T) {
				input := []byte(`{
  "openapi":"` + version + `",
  "info":{"title":"Later body","version":"1"},
  "paths":{"/items":{"` + method + `":{
    "operationId":"laterBody",
    "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"string"}}}},
    "responses":{"204":{"description":"OK"}}
  }}}
}`)
				result, err := CompileResult(input)
				if err != nil {
					t.Fatal(err)
				}
				if result.Document == nil || len(result.Diagnostics) != 0 ||
					result.Document.Operations[0].RequestBody == nil {
					t.Fatalf("compile result = %#v", result)
				}
			})
		}
	}
}
