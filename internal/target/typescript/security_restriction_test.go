package typescript

import (
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/compatibility"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/generator"
)

func TestCompilerSecurityRestrictionOmitsOperationWithoutTargetE508(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Security restriction target consumption","version":"1"},
  "paths":{
    "/bad":{"get":{
      "operationId":"getBad",
      "security":[{"missing":[]}],
      "responses":{"204":{"description":"OK"}}
    }},
    "/good":{"get":{
      "operationId":"getGood",
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.SemanticRestrictions) != 1 ||
		document.SemanticRestrictions[0].RuleID != compatibility.RuleSecurityRequirement {
		t.Fatalf("security restrictions = %#v", document.SemanticRestrictions)
	}

	prepared, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.HasErrors(diagnostics) {
		t.Fatalf("target re-promoted compiler security restriction: %#v", diagnostics)
	}
	for _, value := range diagnostics {
		if value.Code == "SDKGEN-E508" {
			t.Fatalf("compiler-omitted security route produced duplicate E508: %#v", diagnostics)
		}
	}
	value, err := prepared.Value("typescript")
	if err != nil {
		t.Fatal(err)
	}
	plan := value.(*sourcePlan)
	if !plan.omittedOperations["GET /bad"] {
		t.Fatalf("security omission state = %#v", plan.omittedOperations)
	}
	if plan.resourceReservationExcluded["GET /bad"] {
		t.Fatalf("security omission incorrectly removed valid resource reservation: %#v", plan.resourceReservationExcluded)
	}

	probe := `import { createClient } from "./index.js"
declare const api: ReturnType<typeof createClient>
api.$operations.getGood()
// @ts-expect-error unknown-scheme operation is omitted
api.$operations.getBad()
// @ts-expect-error omitted operation leaves no resource namespace
api.bad
`
	compileTypeScriptArtifactsWithProbe(t, document, "security-restriction.probe.ts", probe)
}

func TestSecurityRestrictionStillBlocksEmptyEntrySurface(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Only omitted security operation","version":"1"},
  "paths":{
    "/bad":{"get":{
      "operationId":"getBad",
      "security":[{"missing":[]}],
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var foundE512 bool
	for _, value := range diagnostics {
		if value.Code == "SDKGEN-E508" {
			t.Fatalf("compiler security omission duplicated as E508: %#v", diagnostics)
		}
		if value.Code == "SDKGEN-E512" &&
			value.Scope == failure.ScopeDocument &&
			value.Effect == failure.EffectBlock {
			foundE512 = true
		}
	}
	if !foundE512 {
		t.Fatalf("empty security omission did not preserve E512: %#v", diagnostics)
	}
}

func TestNonObjectComponentsContainerDoesNotBecomeUndeclaredSecurityOmission(t *testing.T) {
	result, err := sdkgen.CompileResultWithOptions([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Non-object components","version":"1"},
  "paths":{
    "/items":{"get":{
      "operationId":"getItems",
      "security":[{"declared":[]}],
      "responses":{"204":{"description":"OK"}}
    }}
  },
  "components":"malformed"
}`), sdkgen.CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil {
		t.Fatalf("structured compile lost document: %#v", result)
	}
	for _, value := range result.Diagnostics {
		if value.Rule == compatibility.RuleSecurityRequirement {
			t.Fatalf("malformed shared components container was misclassified as undeclared: %#v", result.Diagnostics)
		}
	}
	if len(result.Document.SemanticRestrictions) != 0 {
		t.Fatalf("malformed shared components created operation restriction: %#v", result.Document.SemanticRestrictions)
	}

	_, diagnostics, err := (Generator{}).Prepare(result.Document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasTargetSecurityBlocker(diagnostics) {
		t.Fatalf("malformed shared components did not remain target-blocking: %#v", diagnostics)
	}
}

func TestNonObjectDeclaredSecuritySchemeRemainsTargetOwned(t *testing.T) {
	result, err := sdkgen.CompileResultWithOptions([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Non-object security scheme","version":"1"},
  "paths":{
    "/items":{"get":{
      "operationId":"getItems",
      "security":[{"declared":[]}],
      "responses":{"204":{"description":"OK"}}
    }}
  },
  "components":{"securitySchemes":{"declared":"malformed"}}
}`), sdkgen.CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil {
		t.Fatalf("structured compile lost document: %#v", result)
	}
	for _, value := range result.Diagnostics {
		if value.Rule == compatibility.RuleSecurityRequirement {
			t.Fatalf("declared malformed scheme was misclassified as undeclared: %#v", result.Diagnostics)
		}
	}
	if len(result.Document.SemanticRestrictions) != 0 {
		t.Fatalf("declared malformed scheme created operation restriction: %#v", result.Document.SemanticRestrictions)
	}

	_, diagnostics, err := (Generator{}).Prepare(result.Document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasTargetSecurityBlocker(diagnostics) {
		t.Fatalf("declared malformed scheme did not remain target-blocking: %#v", diagnostics)
	}
}

func TestSecuritySchemeURIsAndDeclaredMalformedSchemesRemainTargetOwned(t *testing.T) {
	uriDocument, err := sdkgen.Compile([]byte(`{
  "openapi":"3.2.0",
  "info":{"title":"Security URI target ownership","version":"1"},
  "paths":{
    "/items":{"get":{
      "operationId":"getItems",
      "security":[{"https://security.example.test/schemes/bearer":[]}],
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(uriDocument.SemanticRestrictions) != 0 {
		t.Fatalf("3.2 Security Scheme URI received compiler restriction: %#v", uriDocument.SemanticRestrictions)
	}
	_, uriDiagnostics, err := (Generator{}).Prepare(uriDocument, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasTargetSecurityBlocker(uriDiagnostics) {
		t.Fatalf("3.2 URI target ownership diagnostics = %#v", uriDiagnostics)
	}

	malformedDocument, err := sdkgen.Compile([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Declared malformed security scheme","version":"1"},
  "paths":{
    "/items":{"get":{
      "operationId":"getItems",
      "security":[{"declared":[]}],
      "responses":{"204":{"description":"OK"}}
    }}
  },
  "components":{"securitySchemes":{"declared":{"type":"http"}}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(malformedDocument.SemanticRestrictions) != 0 {
		t.Fatalf("declared malformed scheme received compiler missing-scheme restriction: %#v", malformedDocument.SemanticRestrictions)
	}
	_, malformedDiagnostics, err := (Generator{}).Prepare(malformedDocument, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasTargetSecurityBlocker(malformedDiagnostics) {
		t.Fatalf("declared malformed scheme target diagnostics = %#v", malformedDiagnostics)
	}
}

func hasTargetSecurityBlocker(values []diagnostic.Diagnostic) bool {
	for _, value := range values {
		if value.Code == "SDKGEN-E508" &&
			value.Scope == failure.ScopeDocument &&
			value.Effect == failure.EffectBlock {
			return true
		}
	}
	return false
}
