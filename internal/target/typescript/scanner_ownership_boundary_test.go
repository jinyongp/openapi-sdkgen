package typescript

import (
	"fmt"
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

func TestOwnershipIndexConstructionIsOperationLinearAndLookupPointerBounded(t *testing.T) {
	const operations = 4096
	document := &ir.Document{Operations: make([]ir.Operation, 0, operations)}
	for index := 0; index < operations; index++ {
		path := fmt.Sprintf("/items/%d", index)
		document.Operations = append(document.Operations, ir.Operation{
			Pointer:     "#/paths/~1items~1" + fmt.Sprint(index) + "/get",
			OperationID: fmt.Sprintf("getItem%d", index),
			Method:      "GET",
			Path:        path,
		})
	}
	index := newSourceOwnershipIndex(document)
	if len(index.operationsByPointer) != operations || len(index.operationsByID) != operations || len(index.operationsByPath) != operations {
		t.Fatalf("ownership index cardinality = pointer:%d id:%d path:%d", len(index.operationsByPointer), len(index.operationsByID), len(index.operationsByPath))
	}
	target := document.Operations[operations-1]
	nested := target.Pointer + "/responses/200/content/application~1json/schema/properties/id"
	value, found := index.operationAt(nested)
	if !found || value.OperationID != target.OperationID {
		t.Fatalf("nested ownership = %#v, found=%v", value, found)
	}
	if _, found := index.operationAt("#/components/schemas/Unowned/properties/id"); found {
		t.Fatal("component pointer unexpectedly acquired operation ownership")
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
