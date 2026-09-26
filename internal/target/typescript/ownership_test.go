package typescript

import (
	"testing"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/failure"
)

func TestSourceOwnershipIndexFindsNestedOperationByAncestorPointer(t *testing.T) {
	document := &ir.Document{
		Operations: []ir.Operation{{
			Pointer:     "#/paths/~1items/get",
			OperationID: "listItems",
			Method:      "GET",
			Path:        "/items",
		}},
	}
	index := newSourceOwnershipIndex(document)
	operation, found := index.operationAt("#/paths/~1items/get/responses/200/content/application~1json/schema")
	if !found || operation.OperationID != "listItems" {
		t.Fatalf("nested operation ownership = %#v, found=%v", operation, found)
	}
	if _, found := index.operationAt("#/components/schemas/Item"); found {
		t.Fatal("component pointer unexpectedly acquired operation ownership")
	}
}

func TestSourceOwnershipIndexKeepsRestrictionSourceIdentity(t *testing.T) {
	pointer := "#/paths/~1items/get/responses/200/links/follow"
	wanted := ir.SemanticRestriction{
		RuleID:   "wanted",
		Scope:    failure.ScopeCapability,
		Effect:   failure.EffectOmitCapability,
		Location: ir.SourceLocation{Source: "root.yaml", Pointer: pointer},
	}
	foreign := wanted
	foreign.RuleID = "foreign"
	foreign.Location.Source = "other.yaml"
	document := &ir.Document{
		SemanticRestrictions: []ir.SemanticRestriction{foreign, wanted},
		Provenance: map[string]ir.Provenance{
			pointer: {Primary: wanted.Location},
		},
	}
	index := newSourceOwnershipIndex(document)
	values := index.restrictionsAt(document, pointer)
	if len(values) != 1 || values[0].RuleID != wanted.RuleID {
		t.Fatalf("source-scoped restrictions = %#v", values)
	}
}

func TestSourceOwnershipIndexFollowsReferenceProvenance(t *testing.T) {
	pointer := "#/paths/~1items/get/responses/200/links/follow"
	location := ir.SourceLocation{Source: "links.yaml", Pointer: "#/Follow"}
	restriction := ir.SemanticRestriction{
		RuleID:   "external",
		Scope:    failure.ScopeCapability,
		Effect:   failure.EffectOmitCapability,
		Location: location,
	}
	document := &ir.Document{
		SemanticRestrictions: []ir.SemanticRestriction{restriction},
		Provenance: map[string]ir.Provenance{
			pointer: {
				Primary: location,
				Related: []ir.SourceLocation{{Source: "root.yaml", Pointer: pointer + "/$ref"}},
			},
		},
	}
	index := newSourceOwnershipIndex(document)
	values := index.restrictionsAt(document, pointer)
	if len(values) != 1 || values[0].RuleID != restriction.RuleID {
		t.Fatalf("reference-scoped restrictions = %#v", values)
	}
}
