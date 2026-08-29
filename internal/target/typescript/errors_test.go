package typescript

import (
	"reflect"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
)

func TestSourceArtifactsEmitInlineOneOfErrorContracts(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi": "3.1.0",
  "info": {"title": "Inline errors", "version": "1"},
  "paths": {"/query": {"get": {
    "operationId": "getQuery",
    "responses": {
      "200": {"description": "OK"},
      "400": {"description": "Bad query", "content": {"application/json": {
        "schema": {"$ref": "#/components/schemas/QueryParameterError"}
      }}}
    }
  }}},
  "components": {"schemas": {
    "QueryParameterError": {
      "oneOf": [
        {
          "type": "object", "required": ["error"],
          "properties": {"error": {
            "type": "object", "required": ["code"],
            "properties": {"code": {"const": "query_parameter_unknown"}}
          }}
        },
        {
          "type": "object", "required": ["error"],
          "properties": {"error": {
            "type": "object", "required": ["code"],
            "properties": {"code": {"const": "query_parameter_duplicate"}}
          }}
        }
      ]
    }
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	errorsSource := string(artifactByPath(t, artifacts, "internal/errors.ts"))
	operationSource := string(artifactByPath(t, artifacts, "internal/operations/query/get.ts"))
	schemaSource := string(artifactByPath(t, artifacts, "internal/schemas/query-parameter-error.ts"))
	for _, expected := range []string{
		`export type ServerErrorCode = "query_parameter_duplicate" | "query_parameter_unknown"`,
		`readonly "query_parameter_duplicate": unknown`,
		`readonly "query_parameter_unknown": unknown`,
	} {
		if !strings.Contains(errorsSource, expected) {
			t.Errorf("errors missing %q:\n%s", expected, errorsSource)
		}
	}
	for _, expected := range []string{
		`Errors.ServerError<"query_parameter_duplicate", unknown>`,
		`Errors.ServerError<"query_parameter_unknown", unknown>`,
		`| TransportError`,
	} {
		if !strings.Contains(operationSource, expected) {
			t.Errorf("operation error type missing %q:\n%s", expected, operationSource)
		}
	}
	if !strings.Contains(schemaSource, "oneOf:") {
		t.Errorf("runtime union schema missing:\n%s", schemaSource)
	}
}

func TestErrorContractsCollectInlineUnionVariantsAndNarrowOperations(t *testing.T) {
	document := &ir.Document{ComponentSchemas: map[string]map[string]any{
		"StringDetails": {"type": "string"},
		"ObjectDetails": {
			"type": "object", "properties": map[string]any{"parameter": map[string]any{"type": "string"}},
		},
		"QueryParameterError": {
			"oneOf": []any{
				errorEnvelopeSchema("query_parameter_unknown", "StringDetails", ""),
				map[string]any{"anyOf": []any{
					errorEnvelopeSchema("query_parameter_duplicate", "ObjectDetails", ""),
				}},
			},
		},
		"QueryParameterErrorAlias": {"$ref": "#/components/schemas/QueryParameterError"},
		"InlineAnyError": {
			"anyOf": []any{
				errorEnvelopeSchema("query_parameter_invalid", "StringDetails", ""),
				errorEnvelopeSchema("query_parameter_missing", "ObjectDetails", ""),
			},
		},
		"ReferencedUnknownError":   errorEnvelopeSchema("referenced_unknown", "StringDetails", ""),
		"ReferencedDuplicateError": errorEnvelopeSchema("referenced_duplicate", "ObjectDetails", ""),
		"ReferencedUnionError": {
			"oneOf": []any{
				map[string]any{"$ref": "#/components/schemas/ReferencedUnknownError"},
				map[string]any{"$ref": "#/components/schemas/ReferencedDuplicateError"},
			},
		},
	}}
	document.Operations = []ir.Operation{
		operationWithErrorSchema("QueryParameterErrorAlias"),
		operationWithErrorSchema("InlineAnyError"),
		operationWithErrorSchema("ReferencedUnionError"),
	}

	contracts, bySchema, err := errorContracts(document)
	if err != nil {
		t.Fatal(err)
	}
	wantDetails := map[string]string{
		"query_parameter_unknown":   `Contract.ComponentOutput<"StringDetails">`,
		"query_parameter_duplicate": `Contract.ComponentOutput<"ObjectDetails">`,
		"query_parameter_invalid":   `Contract.ComponentOutput<"StringDetails">`,
		"query_parameter_missing":   `Contract.ComponentOutput<"ObjectDetails">`,
		"referenced_unknown":        `Contract.ComponentOutput<"StringDetails">`,
		"referenced_duplicate":      `Contract.ComponentOutput<"ObjectDetails">`,
	}
	if len(contracts) != len(wantDetails) {
		t.Fatalf("contracts = %#v", contracts)
	}
	for _, contract := range contracts {
		if want := wantDetails[contract.Code]; len(contract.Details) != 1 || contract.Details[0] != want {
			t.Errorf("contract %q details = %#v, want %q", contract.Code, contract.Details, want)
		}
	}

	assertOperationErrorCodes(t, document, operationWithErrorSchema("QueryParameterErrorAlias"), bySchema,
		"query_parameter_duplicate", "query_parameter_unknown")
	assertOperationErrorCodes(t, document, operationWithErrorSchema("InlineAnyError"), bySchema,
		"query_parameter_invalid", "query_parameter_missing")
	assertOperationErrorCodes(t, document, operationWithErrorSchema("ReferencedUnionError"), bySchema,
		"referenced_duplicate", "referenced_unknown")
}

func TestInlineUnionErrorContractsEmitCodesDetailsAndCategories(t *testing.T) {
	document := &ir.Document{
		ComponentSchemas: map[string]map[string]any{
			"FirstDetails":  {"type": "string"},
			"SecondDetails": {"type": "integer"},
			"QueryParameterError": {
				"x-error-category": "query-parameter",
				"oneOf": []any{
					errorEnvelopeSchema("query_parameter_unknown", "FirstDetails", ""),
					errorEnvelopeSchema("query_parameter_duplicate", "SecondDetails", ""),
				},
			},
		},
		Operations: []ir.Operation{operationWithErrorSchema("QueryParameterError")},
	}

	prepared, diagnostics, err := prepareKnownExtensions(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	source, err := emitErrors(prepared)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`export type ServerErrorCode = "query_parameter_duplicate" | "query_parameter_unknown"`,
		`readonly "query_parameter_duplicate": Contract.ComponentOutput<"SecondDetails">`,
		`readonly "query_parameter_unknown": Contract.ComponentOutput<"FirstDetails">`,
		`readonly "query-parameter": "query_parameter_duplicate" | "query_parameter_unknown"`,
	} {
		if !strings.Contains(string(source), expected) {
			t.Errorf("errors missing %q:\n%s", expected, source)
		}
	}
}

func TestInlineUnionErrorContractsPreserveBranchWireCategories(t *testing.T) {
	first := errorEnvelopeSchema("first_failure", "FirstDetails", "")
	setWireErrorCategory(first, "first")
	second := errorEnvelopeSchema("second_failure", "SecondDetails", "")
	setWireErrorCategory(second, "second")
	document := &ir.Document{
		ComponentSchemas: map[string]map[string]any{
			"FirstDetails":  {"type": "string"},
			"SecondDetails": {"type": "integer"},
			"CategorizedError": {
				"oneOf": []any{first, second},
			},
		},
		Operations: []ir.Operation{operationWithErrorSchema("CategorizedError")},
	}

	prepared, diagnostics, err := prepareKnownExtensions(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	source, err := emitErrors(prepared)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`readonly "first": "first_failure"`,
		`readonly "second": "second_failure"`,
	} {
		if !strings.Contains(string(source), expected) {
			t.Errorf("errors missing %q:\n%s", expected, source)
		}
	}
}

func setWireErrorCategory(schema map[string]any, category string) {
	properties := schema["properties"].(map[string]any)
	errorSchema := properties["error"].(map[string]any)
	errorSchema["required"] = append(errorSchema["required"].([]any), "category")
	errorProperties := errorSchema["properties"].(map[string]any)
	errorProperties["category"] = map[string]any{"const": category}
}

func assertOperationErrorCodes(t *testing.T, document *ir.Document, operation ir.Operation, bySchema map[string][]errorContract, codes ...string) {
	t.Helper()
	types, err := operationErrorTypes(document, operation, bySchema)
	if err != nil {
		t.Fatal(err)
	}
	if len(types) != len(codes) {
		t.Fatalf("operation error types = %#v", types)
	}
	for index, code := range codes {
		if !strings.Contains(types[index], `ServerError<"`+code+`", `) {
			t.Errorf("operation error type %d = %q, want code %q", index, types[index], code)
		}
	}
}

func TestErrorContractsPropagateAndDeduplicateComposedErrorSchemas(t *testing.T) {
	document := &ir.Document{ComponentSchemas: map[string]map[string]any{
		"BaseError": {
			"type": "object", "required": []any{"error"},
			"properties": map[string]any{
				"error": map[string]any{"required": []any{"code"}, "properties": map[string]any{
					"code":    map[string]any{"enum": []any{"invalid_widget", "missing_widget"}},
					"details": map[string]any{"type": "object", "properties": map[string]any{}},
				}},
			},
		},
		"CombinedError": {
			"allOf": []any{
				map[string]any{"$ref": "#/components/schemas/BaseError"},
				map[string]any{"$ref": "#/components/schemas/BaseError"},
			},
		},
	}}
	document.Operations = []ir.Operation{operationWithErrorSchema("CombinedError")}
	contracts, bySchema, err := errorContracts(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(contracts) != 2 || len(bySchema["CombinedError"]) != 2 {
		t.Fatalf("contracts = %#v, combined = %#v", contracts, bySchema["CombinedError"])
	}
	operation := operationWithErrorSchema("CombinedError")
	types, err := operationErrorTypes(document, operation, bySchema)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(types, []string{
		`ServerError<"invalid_widget", Readonly<Record<string, unknown>>>`,
		`ServerError<"missing_widget", Readonly<Record<string, unknown>>>`,
	}) {
		t.Fatalf("operation error types = %#v", types)
	}
}

func TestErrorContractsAggregateCodeDetailsAndNarrowOperations(t *testing.T) {
	document := &ir.Document{ComponentSchemas: map[string]map[string]any{
		"AlphaError": errorEnvelopeSchema("shared-code", "AlphaDetails", ""),
		"BetaError":  errorEnvelopeSchema("shared-code", "BetaDetails", "request-errors"),
		"OtherError": errorEnvelopeSchema("other-code", "OtherDetails", "request-errors"),
		"AlphaDetails": {
			"type": "object", "properties": map[string]any{"alpha": map[string]any{"type": "string"}},
		},
		"BetaDetails": {
			"type": "object", "properties": map[string]any{"beta": map[string]any{"type": "string"}},
		},
		"OtherDetails": {
			"type": "object", "properties": map[string]any{"other": map[string]any{"type": "string"}},
		},
	}, ErrorCategories: map[string]string{"BetaError": "request-errors", "OtherError": "request-errors"}}
	document.Operations = []ir.Operation{
		operationWithErrorSchema("AlphaError"),
		operationWithErrorSchema("BetaError"),
		operationWithErrorSchema("OtherError"),
	}
	contracts, bySchema, err := errorContracts(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(contracts) != 2 {
		t.Fatalf("contracts = %#v", contracts)
	}
	shared := contracts[1]
	if shared.Code != "shared-code" || shared.Category != "request-errors" || !reflect.DeepEqual(shared.Details, []string{
		`Contract.ComponentOutput<"AlphaDetails">`,
		`Contract.ComponentOutput<"BetaDetails">`,
	}) {
		t.Fatalf("shared contract = %#v", shared)
	}
	operation := operationWithErrorSchema("AlphaError")
	types, err := operationErrorTypes(document, operation, bySchema)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(types, []string{`ServerError<"shared-code", Contract.ComponentOutput<"AlphaDetails">>`}) {
		t.Fatalf("operation error types = %#v", types)
	}
	expression, err := operationErrorTypeExpression(document, operation, bySchema)
	if err != nil {
		t.Fatal(err)
	}
	if got := expression.render(typeRenderContract); got != `Errors.ServerError<"shared-code", Contract.ComponentOutput<"AlphaDetails">> | TransportError` {
		t.Fatalf("operation error expression = %q", got)
	}
}

func TestErrorContractsRejectConflictingNonEmptyCategories(t *testing.T) {
	document := &ir.Document{ComponentSchemas: map[string]map[string]any{
		"AlphaError": errorEnvelopeSchema("shared", "AlphaDetails", "alpha"),
		"BetaError":  errorEnvelopeSchema("shared", "BetaDetails", "beta"),
		"AlphaDetails": {
			"type": "string",
		},
		"BetaDetails": {
			"type": "string",
		},
	}, ErrorCategories: map[string]string{"AlphaError": "alpha", "BetaError": "beta"}}
	document.Operations = []ir.Operation{operationWithErrorSchema("AlphaError"), operationWithErrorSchema("BetaError")}
	_, _, err := errorContracts(document)
	if err == nil || !containsAll(err.Error(), `"alpha"`, `"beta"`, "AlphaError", "BetaError") {
		t.Fatalf("error = %v", err)
	}
}

func TestErrorContractsExcludeHiddenOnlySchemasAndRestoreVisibleReachability(t *testing.T) {
	schema := errorEnvelopeSchema("hidden-code", "HiddenDetails", "")
	document := &ir.Document{
		ComponentSchemas: map[string]map[string]any{
			"HiddenError":   schema,
			"HiddenDetails": {"type": "string"},
		},
		Operations: []ir.Operation{{
			Visibility: "hidden",
			Raw:        operationWithErrorSchema("HiddenError").Raw,
		}},
	}
	contracts, _, err := errorContracts(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(contracts) != 0 {
		t.Fatalf("hidden contracts = %#v", contracts)
	}
	document.Operations = append(document.Operations, ir.Operation{
		Visibility: "public",
		Raw:        operationWithErrorSchema("HiddenError").Raw,
	})
	contracts, _, err = errorContracts(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(contracts) != 1 || contracts[0].Code != "hidden-code" {
		t.Fatalf("visible contracts = %#v", contracts)
	}
}

func errorEnvelopeSchema(code, details, category string) map[string]any {
	schema := map[string]any{
		"type":     "object",
		"required": []any{"error"},
		"properties": map[string]any{
			"error": map[string]any{
				"type":     "object",
				"required": []any{"code"},
				"properties": map[string]any{
					"code":    map[string]any{"const": code},
					"details": map[string]any{"$ref": "#/components/schemas/" + details},
				},
			},
		},
	}
	if category != "" {
		schema["x-error-category"] = category
	}
	return schema
}

func operationWithErrorSchema(schema string) ir.Operation {
	return ir.Operation{Raw: map[string]any{"responses": map[string]any{
		"400": map[string]any{"content": map[string]any{
			"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/" + schema}},
		}},
	}}}
}

func TestErrorContractsResolveEscapedSchemaReferences(t *testing.T) {
	document := &ir.Document{ComponentSchemas: map[string]map[string]any{
		"Base/Error": {
			"type": "object", "required": []any{"error"},
			"properties": map[string]any{
				"error": map[string]any{"required": []any{"code"}, "properties": map[string]any{
					"code": map[string]any{"const": "invalid_widget"},
				}},
			},
		},
	}}
	operation := ir.Operation{Raw: map[string]any{"responses": map[string]any{
		"400": map[string]any{"content": map[string]any{
			"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Base~1Error"}},
		}},
	}}}
	document.Operations = []ir.Operation{operation}
	contracts, bySchema, err := errorContracts(document)
	if err != nil {
		t.Fatal(err)
	}
	types, err := operationErrorTypes(document, operation, bySchema)
	if err != nil {
		t.Fatal(err)
	}
	if len(contracts) != 1 || !reflect.DeepEqual(types, []string{`ServerError<"invalid_widget", unknown>`}) {
		t.Fatalf("contracts = %#v, types = %#v", contracts, types)
	}
}
