package typescript

import (
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/compatibility"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/generator"
)

func TestCompilerPathRestrictionOmitsOperationBeforeResourceReservation(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.0.3",
  "info":{"title":"Path restriction target consumption","version":"1"},
  "paths":{
    "/bad/{id}":{"get":{
      "operationId":"getBad",
      "parameters":[{"name":"other","in":"path","required":true,"schema":{"type":"string"}}],
      "responses":{"204":{"description":"OK"}}
    }},
    "/good/{id}":{"get":{
      "operationId":"getGood",
      "parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.SemanticRestrictions) != 1 {
		t.Fatalf("restrictions = %#v", document.SemanticRestrictions)
	}

	prepared, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.HasErrors(diagnostics) {
		t.Fatalf("target re-promoted compiler restriction to an error: %#v", diagnostics)
	}
	for _, value := range diagnostics {
		if value.Code == "SDKGEN-E507" {
			t.Fatalf("path restriction duplicated as E507: %#v", diagnostics)
		}
	}
	value, err := prepared.Value("typescript")
	if err != nil {
		t.Fatal(err)
	}
	plan := value.(*sourcePlan)
	badRoute := "GET /bad/{id}"
	if !plan.omittedOperations[badRoute] || !plan.resourceReservationExcluded[badRoute] {
		t.Fatalf("path restriction state = omitted %#v resourceExcluded %#v", plan.omittedOperations, plan.resourceReservationExcluded)
	}
	foundReservation := false
	for _, operation := range plan.reservationManifest.Operations {
		if operation.RouteKey == badRoute {
			foundReservation = true
		}
	}
	if !foundReservation {
		t.Fatal("path-invalid operation did not retain flat/artifact reservation")
	}
	for _, operation := range plan.manifest.Operations {
		if operation.RouteKey == badRoute {
			t.Fatalf("path-invalid operation remained in emission manifest: %#v", plan.manifest.Operations)
		}
	}
	if plan.resourceTree != nil && plan.resourceTree.children["bad"] != nil {
		t.Fatalf("path-invalid operation entered resource reservation tree: %#v", plan.resourceTree.children["bad"])
	}

	probe := `import { createClient } from "./index.js"
declare const api: ReturnType<typeof createClient>
api.$operations.getGood({ path: { id: "ok" } })
// @ts-expect-error path-invalid operation is omitted
api.$operations.getBad({ path: { other: "bad" } })
// @ts-expect-error path-invalid resource namespace is omitted
api.bad
`
	compileTypeScriptArtifactsWithProbe(t, document, "path-restriction.probe.ts", probe)
}

func TestSemanticRestrictionExplicitOwnerBeatsAmbiguousSourceFallback(t *testing.T) {
	shared := ir.SourceLocation{Source: "shared.yaml", Pointer: "#/Shared/get"}
	operations := []ir.Operation{
		{Pointer: "#/paths/~1a/get", OperationID: "getA", Method: "GET", Path: "/a"},
		{Pointer: "#/paths/~1b/get", OperationID: "getB", Method: "GET", Path: "/b"},
	}
	baseRestriction := ir.SemanticRestriction{
		RuleID:      compatibility.RulePathParameterBinding,
		Conformance: string(compatibility.ConformanceNonconforming),
		Disposition: string(compatibility.DispositionInvalid),
		Action:      string(compatibility.ActionReject),
		Impact:      string(compatibility.ImpactRouting),
		Scope:       failure.ScopeOperation,
		Effect:      failure.EffectOmitOperation,
		Location:    shared,
		Message:     "shared source restriction",
	}
	newDocument := func(restriction ir.SemanticRestriction) *ir.Document {
		return &ir.Document{
			Operations:           operations,
			SemanticRestrictions: []ir.SemanticRestriction{restriction},
			Provenance: map[string]ir.Provenance{
				operations[0].Pointer: {Primary: shared},
				operations[1].Pointer: {Primary: shared},
			},
		}
	}

	legacyDocument := newDocument(baseRestriction)
	legacyPlan := &sourcePlan{document: legacyDocument, ownership: newSourceOwnershipIndex(legacyDocument)}
	legacyDiagnostics := semanticOperationRestrictionDiagnostics(legacyPlan)
	if len(legacyDiagnostics) != 1 || legacyDiagnostics[0].Code != "SDKGEN-E507" ||
		legacyDiagnostics[0].Scope != failure.ScopeDocument || legacyDiagnostics[0].Effect != failure.EffectBlock {
		t.Fatalf("ambiguous legacy restriction diagnostics = %#v", legacyDiagnostics)
	}
	if len(legacyPlan.omittedOperations) != 0 {
		t.Fatalf("ambiguous legacy restriction guessed an owner: %#v", legacyPlan.omittedOperations)
	}

	explicit := baseRestriction
	explicit.OwnerPointer = operations[0].Pointer
	explicitDocument := newDocument(explicit)
	explicitPlan := &sourcePlan{document: explicitDocument, ownership: newSourceOwnershipIndex(explicitDocument)}
	explicitDiagnostics := semanticOperationRestrictionDiagnostics(explicitPlan)
	if len(explicitDiagnostics) != 0 {
		t.Fatalf("explicit owner diagnostics = %#v", explicitDiagnostics)
	}
	if !explicitPlan.omittedOperations["GET /a"] || explicitPlan.omittedOperations["GET /b"] {
		t.Fatalf("explicit owner omissions = %#v", explicitPlan.omittedOperations)
	}
	if !explicitPlan.resourceReservationExcluded["GET /a"] || explicitPlan.resourceReservationExcluded["GET /b"] {
		t.Fatalf("explicit owner resource exclusions = %#v", explicitPlan.resourceReservationExcluded)
	}

	unresolved := baseRestriction
	unresolved.OwnerPointer = "#/paths/~1missing/get"
	unresolvedDocument := newDocument(unresolved)
	unresolvedPlan := &sourcePlan{document: unresolvedDocument, ownership: newSourceOwnershipIndex(unresolvedDocument)}
	unresolvedDiagnostics := semanticOperationRestrictionDiagnostics(unresolvedPlan)
	if len(unresolvedDiagnostics) != 1 || unresolvedDiagnostics[0].Code != "SDKGEN-E507" {
		t.Fatalf("unresolved explicit owner diagnostics = %#v", unresolvedDiagnostics)
	}
}
