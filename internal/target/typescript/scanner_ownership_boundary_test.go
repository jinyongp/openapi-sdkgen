package typescript

import (
	"testing"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/generator"
)

func TestFlatSchemaScannerKeepsOperationOwnedFailureDocumentBlocking(t *testing.T) {
	pointer := "#/paths/~1items/post"
	document := &ir.Document{
		Raw: map[string]any{
			"paths": map[string]any{
				"/items": map[string]any{
					"post": map[string]any{
						"operationId": "createItem",
						"requestBody": map[string]any{
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{"$dynamicRef": "#node"},
								},
							},
						},
						"responses": map[string]any{"204": map[string]any{"description": "OK"}},
					},
				},
			},
		},
		Operations: []ir.Operation{{
			Pointer:     pointer,
			OperationID: "createItem",
			Method:      "POST",
			Path:        "/items",
			Raw: map[string]any{
				"requestBody": map[string]any{
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{"$dynamicRef": "#node"},
						},
					},
				},
				"responses": map[string]any{"204": map[string]any{"description": "OK"}},
			},
		}},
	}
	_, values, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	value := requireDiagnosticCode(t, values, "SDKGEN-E501")
	if value.Route != "POST /items" || value.Operation != "createItem" {
		t.Fatalf("operation ownership metadata = %#v", value)
	}
	if value.Severity != diagnostic.SeverityError || value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock {
		t.Fatalf("operation-owned flat scanner scope = %#v", value)
	}
}

func TestUnusedUnsupportedComponentRemainsDocumentBlocking(t *testing.T) {
	document := &ir.Document{
		ComponentSchemas: map[string]map[string]any{
			"Unused": {"$dynamicRef": "#node"},
		},
		Raw: map[string]any{
			"components": map[string]any{
				"schemas": map[string]any{
					"Unused": map[string]any{"$dynamicRef": "#node"},
				},
			},
			"paths": map[string]any{
				"/ok": map[string]any{
					"post": map[string]any{
						"operationId": "createOK",
						"responses":   map[string]any{"204": map[string]any{"description": "OK"}},
					},
				},
			},
		},
		Operations: []ir.Operation{{
			Pointer:     "#/paths/~1ok/post",
			OperationID: "createOK",
			Method:      "POST",
			Path:        "/ok",
			Raw:         map[string]any{"responses": map[string]any{"204": map[string]any{"description": "OK"}}},
		}},
	}
	_, values, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	value := requireDiagnosticCode(t, values, "SDKGEN-E501")
	if value.Route != "" || value.Operation != "" {
		t.Fatalf("unused component was assigned an operation owner: %#v", value)
	}
	if value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock {
		t.Fatalf("unused component scanner scope = %#v", value)
	}
}

func TestUnusedUnsupportedReusableOpenAPIComponentRemainsDocumentBlocking(t *testing.T) {
	document := &ir.Document{
		Raw: map[string]any{
			"components": map[string]any{
				"parameters": map[string]any{
					"Unused": map[string]any{
						"name": "filter",
						"in":   "query",
						"content": map[string]any{
							"application/json": map[string]any{},
							"text/plain":       map[string]any{},
						},
					},
				},
			},
			"paths": map[string]any{
				"/ok": map[string]any{
					"post": map[string]any{
						"operationId": "createOK",
						"responses":   map[string]any{"204": map[string]any{"description": "OK"}},
					},
				},
			},
		},
		Operations: []ir.Operation{{
			Pointer:     "#/paths/~1ok/post",
			OperationID: "createOK",
			Method:      "POST",
			Path:        "/ok",
			Raw:         map[string]any{"responses": map[string]any{"204": map[string]any{"description": "OK"}}},
		}},
	}
	_, values, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	value := requireDiagnosticCode(t, values, "SDKGEN-E502")
	if value.Route != "" || value.Operation != "" {
		t.Fatalf("unused reusable component was assigned an operation owner: %#v", value)
	}
	if value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock {
		t.Fatalf("unused reusable component scanner scope = %#v", value)
	}
}

func requireDiagnosticCode(t *testing.T, values []diagnostic.Diagnostic, code string) diagnostic.Diagnostic {
	t.Helper()
	for _, value := range values {
		if value.Code == code {
			return value
		}
	}
	t.Fatalf("diagnostic %s missing: %#v", code, values)
	return diagnostic.Diagnostic{}
}
