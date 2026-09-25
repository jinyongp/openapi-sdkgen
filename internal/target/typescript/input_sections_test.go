package typescript

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
)

func TestSemanticInputSectionMappings(t *testing.T) {
	for _, test := range []struct {
		section                operationInputSection
		suffix, slot, property string
	}{
		{inputSectionPath, "PathInput", "path", "path"},
		{inputSectionQuery, "QueryInput", "query", "query"},
		{inputSectionQuerystring, "QuerystringInput", "querystring", "querystring"},
		{inputSectionHeader, "HeaderInput", "header", "headerParams"},
		{inputSectionCookie, "CookieInput", "cookie", "cookieParams"},
		{inputSectionBody, "BodyInput", "body", "body"},
	} {
		descriptor, err := requestInputSection(test.section)
		if err != nil || descriptor.suffix != test.suffix || descriptor.sectionKey != test.slot || descriptor.inputProperty != test.property {
			t.Fatalf("section %q: %#v, %v", test.section, descriptor, err)
		}
	}
}

func TestInputSectionPresenceAndRequirednessAreIndependent(t *testing.T) {
	prepared := preparedOperation{requiredByLocation: map[string]bool{"path": true}}
	for _, test := range []struct {
		sections                      operationInputSectionList
		pathBound, hasInput, required bool
	}{
		{nil, false, false, false},
		{operationInputSectionList{inputSectionPath}, false, true, true},
		{operationInputSectionList{inputSectionPath}, true, false, false},
		{operationInputSectionList{inputSectionPath, inputSectionBody}, true, true, false},
		{operationInputSectionList{inputSectionBody}, true, true, false},
		{operationInputSectionList{inputSectionHeader, inputSectionCookie}, false, true, false},
	} {
		if got := test.sections.hasInput(test.pathBound); got != test.hasInput {
			t.Fatalf("sections %v bound=%v: hasInput=%v", test.sections, test.pathBound, got)
		}
		required, err := prepared.clientInputRequired(nil, ir.Operation{}, test.sections, test.pathBound)
		if err != nil || required != test.required {
			t.Fatalf("sections %v bound=%v: required=%v, %v", test.sections, test.pathBound, required, err)
		}
	}
	prepared.bodyRequired = true
	required, err := prepared.clientInputRequired(nil, ir.Operation{}, operationInputSectionList{inputSectionBody}, true)
	if err != nil || !required {
		t.Fatalf("required body: %v, %v", required, err)
	}
	if _, err := prepared.clientInputRequired(nil, ir.Operation{}, operationInputSectionList{"invalid"}, false); err == nil {
		t.Fatal("unknown section was accepted")
	}
}

func TestInputSectionPlannerKeepsOptionalBodyAndSyntheticQuery(t *testing.T) {
	document, err := compiler.Compile([]byte(`{
  "openapi":"3.1.0", "info":{"title":"Section planning","version":"1"},
  "paths":{"/optional":{"post":{
    "operationId":"optionalBody",
    "requestBody":{"content":{"application/json":{"schema":{"type":"string"}}}},
    "responses":{"204":{"description":"ok"}}
  }}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	operation := document.Operations[0]
	sections, err := operationInputSections(document, operation)
	if err != nil || !reflect.DeepEqual(sections, operationInputSectionList{inputSectionBody}) {
		t.Fatalf("optional body sections=%v, %v", sections, err)
	}
	required, err := operationInputRequired(document, operation, sections, false)
	if err != nil || required {
		t.Fatalf("optional body required=%v, %v", required, err)
	}
	operation.Pagination = "cursor"
	sections, err = operationInputSections(document, operation)
	if err != nil || !reflect.DeepEqual(sections, operationInputSectionList{inputSectionQuery, inputSectionBody}) {
		t.Fatalf("pagination sections=%v, %v", sections, err)
	}
}

func TestLinkTargetReferencesDoNotRewriteLiteralValues(t *testing.T) {
	const route = "GET /target"
	legacyName := operationValueName(route)
	link := generatedLink{
		SourceOperation: ir.Operation{Method: "GET", Path: "/source"},
		TargetOperation: ir.Operation{Method: "GET", Path: "/target"},
		Status:          "200", Name: "follow",
		Definition: "{ value: " + quoteTS(legacyName) + " }",
	}
	var output bytes.Buffer
	calls := 0
	err := emitLinkValuesForGroups(&output, &ir.Document{}, []generatedLink{link}, nil, func(target string) (string, error) {
		calls++
		if target != route {
			t.Fatalf("target=%q", target)
		}
		return "targets[" + quoteTS(target) + "]", nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || !strings.Contains(output.String(), "return targets["+quoteTS(route)+"](invocation.options)") || !strings.Contains(output.String(), "{ value: "+quoteTS(legacyName)+" }") {
		t.Fatalf("target binding or authored literal changed:\n%s", output.String())
	}
	if err := emitLinkValuesForGroups(&bytes.Buffer{}, &ir.Document{}, []generatedLink{link}, nil, nil, nil); err == nil {
		t.Fatal("missing resolver was accepted")
	}
	missing := errors.New("target not owned")
	err = emitLinkValuesForGroups(&bytes.Buffer{}, &ir.Document{}, []generatedLink{link}, nil, func(string) (string, error) { return "", missing }, nil)
	if !errors.Is(err, missing) {
		t.Fatalf("target error=%v", err)
	}
	err = emitLinkValuesForGroups(&bytes.Buffer{}, &ir.Document{}, []generatedLink{link}, nil, func(string) (string, error) { return "", nil }, nil)
	if err == nil {
		t.Fatal("empty reference was accepted")
	}
}

func TestSchemaReferenceReplayRejectsUnplannedProjection(t *testing.T) {
	plan := &semanticModulePlan{
		schemas:            []schemaModulePlan{{name: "Hidden", path: "internal/schemas/hidden.ts"}},
		schemaByQuotedName: map[string]string{quoteTS("Hidden"): "Hidden"},
	}
	module := operationModulePlan{routeKey: "GET /source", path: "internal/operations/source/get.ts"}
	_, err := localizeOperationSchemaReferences("type Probe = ContractSchemas.ComponentInput<\"Hidden\">\n", module, plan, "../../schemas/index.js", newLocalIdentifierPlan(module.path))
	if err == nil || !strings.Contains(err.Error(), "projection") {
		t.Fatalf("error=%v, want missing exact projection diagnostic", err)
	}
}
