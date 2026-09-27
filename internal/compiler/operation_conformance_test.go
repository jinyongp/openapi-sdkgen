package sdkgen

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler/compatibility"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
)

func TestPathParameterConformanceCollectsIndependentOperationMismatches(t *testing.T) {
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Path binding conformance","version":"1"},
  "paths":{
    "/a/{id}":{"get":{
      "operationId":"getA",
      "parameters":[{"name":"other","in":"path","required":true,"schema":{"type":"string"}}],
      "responses":{"204":{"description":"OK"}}
    }},
    "/b/{slug}":{"post":{
      "operationId":"postB",
      "parameters":[{"name":"wrong","in":"path","required":true,"schema":{"type":"string"}}],
      "responses":{"204":{"description":"OK"}}
    }},
    "/ok/{id}":{"get":{
      "operationId":"getOK",
      "parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`)

	result, err := CompileResultWithOptions(input, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil {
		t.Fatalf("collect result has no document: %#v", result)
	}
	var findings []diagnostic.Diagnostic
	for _, value := range result.Diagnostics {
		if value.Rule == compatibility.RulePathParameterBinding {
			findings = append(findings, value)
		}
	}
	if len(findings) != 2 {
		t.Fatalf("path conformance diagnostics = %#v", result.Diagnostics)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Route < findings[j].Route })
	for index, route := range []string{"GET /a/{id}", "POST /b/{slug}"} {
		value := findings[index]
		if value.Code != "SDKGEN-W140" ||
			value.Severity != diagnostic.SeverityWarning ||
			value.Scope != failure.ScopeOperation ||
			value.Effect != failure.EffectOmitOperation ||
			value.Action != string(compatibility.ActionReject) ||
			value.Route != route {
			t.Fatalf("finding[%d] = %#v", index, value)
		}
	}
	if len(result.Document.SemanticRestrictions) != 2 {
		t.Fatalf("semantic restrictions = %#v", result.Document.SemanticRestrictions)
	}
	owners := map[string]bool{}
	for _, restriction := range result.Document.SemanticRestrictions {
		if restriction.RuleID != compatibility.RulePathParameterBinding ||
			restriction.Scope != failure.ScopeOperation ||
			restriction.Effect != failure.EffectOmitOperation ||
			restriction.OwnerPointer == "" {
			t.Fatalf("restriction = %#v", restriction)
		}
		owners[restriction.OwnerPointer] = true
	}
	if !owners["#/paths/~1a~1{id}/get"] || !owners["#/paths/~1b~1{slug}/post"] {
		t.Fatalf("restriction owners = %#v", owners)
	}

	var coverage diagnostic.AnalysisCoverage
	for _, item := range result.Coverage {
		if item.Analyzer == pathParameterConformanceAnalyzer {
			coverage = item
			break
		}
	}
	if coverage.Status != diagnostic.CoverageComplete ||
		len(coverage.Prerequisites) != 1 ||
		coverage.Prerequisites[0].Name != "compiler-document" ||
		!coverage.Prerequisites[0].Available {
		t.Fatalf("path conformance coverage = %#v", coverage)
	}
}

func TestDirectCompilerKeepsScopedPathMismatchNonFatal(t *testing.T) {
	input := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Direct scoped conformance","version":"1"},
  "paths":{
    "/items/{id}":{"get":{
      "operationId":"getItem",
      "parameters":[{"name":"other","in":"path","required":true,"schema":{"type":"string"}}],
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`)
	document, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.SemanticRestrictions) != 1 ||
		document.SemanticRestrictions[0].RuleID != compatibility.RulePathParameterBinding ||
		document.SemanticRestrictions[0].OwnerPointer != "#/paths/~1items~1{id}/get" {
		t.Fatalf("direct compile restrictions = %#v", document.SemanticRestrictions)
	}
}

func TestSecurityRequirementConformanceScopesOperationAndRootFailures(t *testing.T) {
	operationInput := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Operation security conformance","version":"1"},
  "paths":{
    "/bad":{"get":{
      "operationId":"getBad",
      "security":[{"missing":[]}],
      "responses":{"204":{"description":"OK"}}
    }},
    "/good":{"get":{
      "operationId":"getGood",
      "security":[{"good":[]}],
      "responses":{"204":{"description":"OK"}}
    }}
  },
  "components":{"securitySchemes":{"good":{"type":"http","scheme":"bearer"}}}
}`)
	result, err := CompileResultWithOptions(operationInput, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil {
		t.Fatalf("operation security result has no document: %#v", result)
	}
	var securityDiagnostics []diagnostic.Diagnostic
	for _, value := range result.Diagnostics {
		if value.Rule == compatibility.RuleSecurityRequirement {
			securityDiagnostics = append(securityDiagnostics, value)
		}
	}
	if len(securityDiagnostics) != 1 {
		t.Fatalf("security diagnostics = %#v", result.Diagnostics)
	}
	value := securityDiagnostics[0]
	if value.Code != "SDKGEN-W140" ||
		value.Scope != failure.ScopeOperation ||
		value.Effect != failure.EffectOmitOperation ||
		value.Route != "GET /bad" ||
		value.Operation != "getBad" ||
		value.Location.Pointer != "#/paths/~1bad/get/security/0/missing" {
		t.Fatalf("operation security diagnostic = %#v", value)
	}
	if len(result.Document.SemanticRestrictions) != 1 ||
		result.Document.SemanticRestrictions[0].RuleID != compatibility.RuleSecurityRequirement ||
		result.Document.SemanticRestrictions[0].OwnerPointer != "#/paths/~1bad/get" {
		t.Fatalf("operation security restriction = %#v", result.Document.SemanticRestrictions)
	}
	coverage := map[string]diagnostic.AnalysisCoverage{}
	for _, item := range result.Coverage {
		coverage[item.Analyzer] = item
	}
	if coverage[securityRequirementConformanceAnalyzer].Status != diagnostic.CoverageComplete {
		t.Fatalf("security coverage = %#v", result.Coverage)
	}

	rootInput := []byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Root security conformance","version":"1"},
  "security":[{"missing":[]}],
  "paths":{"/items":{"get":{"operationId":"getItems","responses":{"204":{"description":"OK"}}}}}
}`)
	rootResult, err := CompileResultWithOptions(rootInput, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if rootResult.Document == nil {
		t.Fatalf("root security structured result lost canonical IR: %#v", rootResult)
	}
	var rootDiagnostic *diagnostic.Diagnostic
	for index := range rootResult.Diagnostics {
		if rootResult.Diagnostics[index].Rule == compatibility.RuleSecurityRequirement {
			rootDiagnostic = &rootResult.Diagnostics[index]
			break
		}
	}
	if rootDiagnostic == nil ||
		rootDiagnostic.Code != "SDKGEN-E140" ||
		rootDiagnostic.Scope != failure.ScopeDocument ||
		rootDiagnostic.Effect != failure.EffectBlock ||
		rootDiagnostic.Location.Pointer != "#/security/0/missing" {
		t.Fatalf("root security diagnostic = %#v", rootDiagnostic)
	}
	if len(rootResult.Document.SemanticRestrictions) != 0 {
		t.Fatalf("root security blocker created scoped restriction: %#v", rootResult.Document.SemanticRestrictions)
	}
	if _, err := Compile(rootInput); err == nil || !strings.Contains(err.Error(), "undeclared Security Scheme") {
		t.Fatalf("direct root security error = %v", err)
	}
}

func TestSecurityRequirementConformanceIsVersionAware(t *testing.T) {
	input := []byte(`{
  "openapi":"3.2.0",
  "info":{"title":"Security URI","version":"1"},
  "paths":{
    "/items":{"get":{
      "operationId":"getItems",
      "security":[{"https://security.example.test/schemes/bearer":[]}],
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`)
	result, err := CompileResultWithOptions(input, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil {
		t.Fatalf("3.2 security URI result = %#v", result)
	}
	for _, value := range result.Diagnostics {
		if value.Rule == compatibility.RuleSecurityRequirement {
			t.Fatalf("3.2 Security Scheme URI was misclassified: %#v", result.Diagnostics)
		}
	}
	if len(result.Document.SemanticRestrictions) != 0 {
		t.Fatalf("3.2 Security Scheme URI created restriction: %#v", result.Document.SemanticRestrictions)
	}
}

func TestRootSecurityBlockerDoesNotPublishReferenceLock(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(input, []byte(`openapi: 3.0.3
info: {title: Blocked lock, version: "1"}
security:
  - missing: []
paths:
  /items:
    get:
      operationId: getItems
      responses:
        "204": {description: OK}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := CompileFileResultWithOptions(input, CompileOptions{
		DiagnosticMode: diagnostic.ModeCollect,
		UpdateRefLock:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !diagnostic.HasErrors(result.Diagnostics) {
		t.Fatalf("blocked security result = %#v", result)
	}
	if _, err := os.Stat(defaultReferenceLockPath(input)); !os.IsNotExist(err) {
		t.Fatalf("blocked compile published reference lock: %v", err)
	}
}
