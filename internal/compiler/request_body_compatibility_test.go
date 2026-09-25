package sdkgen

import (
	"strings"
	"testing"

	"openapi-sdkgen/internal/diagnostic"
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

func TestCompatibilityPreservesMeaningfulOAS30DeleteBodyWithWarning(t *testing.T) {
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Delete body","version":"1"},
  "paths":{"/items":{"delete":{
    "operationId":"deleteItem",
    "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}},
    "responses":{"204":{"description":"Deleted"}}
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
		value.Location.Pointer != "#/paths/~1items/delete/requestBody" {
		t.Fatalf("diagnostic = %#v", value)
	}
}

func TestCompatibilityRejectsMeaningfulOAS30UnsafeBodiesBeforeReferenceResolution(t *testing.T) {
	for _, method := range []string{"get", "head", "options", "trace"} {
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
			if result.Document != nil || len(result.Diagnostics) != 1 {
				t.Fatalf("compile result = %#v", result)
			}
			value := result.Diagnostics[0]
			if value.Code != "SDKGEN-E140" || value.Phase != diagnostic.PhaseOpenAPI ||
				value.Location.Pointer != "#/paths/~1items/"+method+"/requestBody" {
				t.Fatalf("diagnostic = %#v", value)
			}
		})
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
