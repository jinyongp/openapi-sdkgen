package golang

import (
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"strings"
)

// Compiler restrictions already removed unsafe source semantics from the IR.
// Preserve their owner/effect rather than exposing an incomplete operation.
func operationOmissions(document *ir.Document) (map[string]bool, []diagnostic.Diagnostic) {
	omitted := make(map[string]bool)
	byPointer := make(map[string]int)
	bySource := make(map[ir.SourceLocation]int)
	ids := make(map[string]bool)
	var diagnostics []diagnostic.Diagnostic
	for index, operation := range document.Operations {
		pointer := operation.Pointer
		if pointer == "" {
			pointer = "#/paths/" + pointerToken(operation.Path) + "/" + strings.ToLower(operation.Method)
		}
		byPointer[pointer] = index
		if provenance, found := document.LookupProvenance(pointer); found {
			for _, location := range append([]ir.SourceLocation{provenance.Primary}, provenance.Related...) {
				if previous, exists := bySource[location]; exists && previous != index {
					bySource[location] = -1
				} else if !exists {
					bySource[location] = index
				}
			}
		}
		if operation.OperationID != "" {
			if ids[operation.OperationID] {
				diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.SeverityError, Phase: diagnostic.PhaseTarget, Target: "golang", Code: "GO_DUPLICATE_OPERATION_ID", Scope: failure.ScopeDocument, Effect: failure.EffectBlock, Location: diagnostic.Location{Pointer: pointer}, Message: "operationId must uniquely identify an operation"})
			}
			ids[operation.OperationID] = true
		}
	}
	for _, restriction := range document.SemanticRestrictions {
		if restriction.Scope != failure.ScopeOperation || restriction.Effect != failure.EffectOmitOperation {
			continue
		}
		index, found := -1, false
		if restriction.OwnerPointer != "" {
			index, found = byPointer[restriction.OwnerPointer]
		} else {
			for pointer := restriction.Location.Pointer; pointer != ""; {
				if restriction.Location.Source == "" {
					index, found = byPointer[pointer]
				} else {
					index, found = bySource[ir.SourceLocation{Source: restriction.Location.Source, Pointer: pointer}]
				}
				if found {
					break
				}
				end := strings.LastIndex(pointer, "/")
				if end < 1 {
					break
				}
				pointer = pointer[:end]
			}
		}
		if !found || index < 0 {
			diagnostics = append(diagnostics, diagnostic.Diagnostic{Severity: diagnostic.SeverityError, Phase: diagnostic.PhaseTarget, Target: "golang", Code: "GO_RESTRICTION_OWNER", Scope: failure.ScopeDocument, Effect: failure.EffectBlock, Location: diagnostic.Location{Source: restriction.Location.Source, Pointer: restriction.Location.Pointer}, Message: "cannot prove the owner of an operation-scoped semantic restriction"})
			continue
		}
		omitted[document.Operations[index].RouteKey] = true
	}
	return omitted, diagnostics
}
