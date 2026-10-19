package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/generator"
	"strings"
	"testing"
)

func TestCompilerOperationOmissionIsPreserved(t *testing.T) {
	const source = `{"openapi":"3.0.3","info":{"title":"Unsafe body","version":"1"},"paths":{"/unsafe":{"get":{"operationId":"unsafe","requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"boolean"}}}},"responses":{"204":{"description":"No content"}}}},"/safe":{"get":{"operationId":"safe","responses":{"204":{"description":"No content"}}}}}}`
	document, err := compiler.Compile([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.SemanticRestrictions) == 0 {
		t.Fatal("fixture did not exercise compiler omission")
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	artifacts, err := (Generator{}).Emit(plan)
	if err != nil {
		t.Fatal(err)
	}
	var sourceText strings.Builder
	for _, artifact := range artifacts {
		sourceText.Write(artifact.Data)
	}
	if strings.Contains(sourceText.String(), ") Unsafe(") || !strings.Contains(sourceText.String(), ") Safe(") {
		t.Fatal("unsafe operation exposed or safe neighbor omitted")
	}
}

func TestRestrictionOwnerUsesExactSourceIdentity(t *testing.T) {
	document := &ir.Document{Operations: []ir.Operation{{RouteKey: "GET /a", Pointer: "#/paths/~1a/get"}, {RouteKey: "GET /b", Pointer: "#/paths/~1b/get"}}, Provenance: map[string]ir.Provenance{
		"#/paths/~1a/get": {Primary: ir.SourceLocation{Source: "a.yaml", Pointer: "#/paths/~1same/get"}},
		"#/paths/~1b/get": {Primary: ir.SourceLocation{Source: "b.yaml", Pointer: "#/paths/~1same/get"}},
	}, SemanticRestrictions: []ir.SemanticRestriction{{Scope: failure.ScopeOperation, Effect: failure.EffectOmitOperation, Location: ir.SourceLocation{Source: "b.yaml", Pointer: "#/paths/~1same/get/parameters/0"}}}}
	omitted, diagnostics := operationOmissions(document)
	if len(diagnostics) != 0 || omitted["GET /a"] || !omitted["GET /b"] {
		t.Fatalf("wrong source owner: %v %v", omitted, diagnostics)
	}
	document.SemanticRestrictions[0].Location.Source = "foreign.yaml"
	if _, diagnostics := operationOmissions(document); len(diagnostics) != 1 || diagnostics[0].Code != "GO_RESTRICTION_OWNER" {
		t.Fatal("unknown owner was silently guessed", diagnostics)
	}
	document.Provenance["#/paths/~1a/get"] = document.Provenance["#/paths/~1b/get"]
	document.SemanticRestrictions[0].Location.Source = "b.yaml"
	if _, diagnostics := operationOmissions(document); len(diagnostics) != 1 {
		t.Fatal("ambiguous mounted owner was guessed", diagnostics)
	}
}

func TestCompilerSecurityOmissionIsNotPromotedByNameReservation(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.0.3","info":{"title":"Security omission","version":"1"},"paths":{"/bad":{"get":{"operationId":"bad","security":[{"unknown":[]}],"responses":{"204":{"description":"Done"}}}},"/safe":{"get":{"operationId":"safe","responses":{"204":{"description":"Done"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.SemanticRestrictions) == 0 {
		t.Fatal("fixture did not produce a security omission")
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": `package consumer_test
import("testing";sdk "example.test/consumer/api")
func TestRetainedOperationCompiles(t *testing.T){var client sdk.Client;_ = client.Operations.Safe()}
`})
}

func TestDuplicateOperationIDDoesNotBecomeAnInventedPublicIdentity(t *testing.T) {
	document := &ir.Document{OpenAPIVersionLine: "3.1", Operations: []ir.Operation{{RouteKey: "GET /a", OperationID: "same", Method: "GET", Path: "/a"}, {RouteKey: "GET /b", OperationID: "same", Method: "GET", Path: "/b"}}}
	_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 1 || diagnostics[0].Code != "GO_DUPLICATE_OPERATION_ID" {
		t.Fatalf("duplicate identity hidden: %v %v", err, diagnostics)
	}
}
