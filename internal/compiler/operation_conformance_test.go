package sdkgen

import (
	"sort"
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
