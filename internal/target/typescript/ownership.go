package typescript

import (
	"errors"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

var errMissingOwnershipIndex = errors.New("source ownership index is unavailable")

type sourceRestrictionKey struct {
	source  string
	pointer string
}

type sourceOwnershipIndex struct {
	operationsByPointer  map[string]ir.Operation
	operationsBySource   map[sourceRestrictionKey]ir.Operation
	operationsByID       map[string]ir.Operation
	operationsByPath     map[string]ir.Operation
	restrictionsByPoint  map[string][]ir.SemanticRestriction
	restrictionsBySource map[sourceRestrictionKey][]ir.SemanticRestriction
}

func newSourceOwnershipIndex(document *ir.Document) *sourceOwnershipIndex {
	index := &sourceOwnershipIndex{
		operationsByPointer:  make(map[string]ir.Operation, len(document.Operations)),
		operationsBySource:   make(map[sourceRestrictionKey]ir.Operation, len(document.Operations)),
		operationsByID:       make(map[string]ir.Operation, len(document.Operations)),
		operationsByPath:     make(map[string]ir.Operation, len(document.Operations)),
		restrictionsByPoint:  make(map[string][]ir.SemanticRestriction),
		restrictionsBySource: make(map[sourceRestrictionKey][]ir.SemanticRestriction),
	}
	for _, operation := range document.Operations {
		pointer := operation.Pointer
		if pointer == "" {
			pointer = "#/paths/" + escapePointerToken(operation.Path) + "/" + strings.ToLower(operation.Method)
		}
		index.operationsByPointer[pointer] = operation
		if provenance, found := document.LookupProvenance(pointer); found {
			locations := append([]ir.SourceLocation{provenance.Primary}, provenance.Related...)
			for _, location := range locations {
				index.operationsBySource[sourceRestrictionKey{source: location.Source, pointer: location.Pointer}] = operation
			}
		}
		if operation.OperationID != "" {
			index.operationsByID[operation.OperationID] = operation
		}
		index.operationsByPath[operationPathMethodKey(operation.Path, operation.Method)] = operation
	}
	for _, restriction := range document.SemanticRestrictions {
		index.restrictionsByPoint[restriction.Location.Pointer] = append(index.restrictionsByPoint[restriction.Location.Pointer], restriction)
		key := sourceRestrictionKey{source: restriction.Location.Source, pointer: restriction.Location.Pointer}
		index.restrictionsBySource[key] = append(index.restrictionsBySource[key], restriction)
	}
	return index
}

func (index *sourceOwnershipIndex) operationAt(pointer string) (ir.Operation, bool) {
	if index == nil {
		return ir.Operation{}, false
	}
	for candidate := pointer; candidate != ""; candidate = parentSourcePointer(candidate) {
		if operation, exists := index.operationsByPointer[candidate]; exists {
			return operation, true
		}
		if candidate == "#" {
			break
		}
	}
	return ir.Operation{}, false
}

func (index *sourceOwnershipIndex) operationAtLocation(location ir.SourceLocation) (ir.Operation, bool) {
	if index == nil {
		return ir.Operation{}, false
	}
	for candidate := location.Pointer; candidate != ""; candidate = parentSourcePointer(candidate) {
		key := sourceRestrictionKey{source: location.Source, pointer: candidate}
		if operation, exists := index.operationsBySource[key]; exists {
			return operation, true
		}
		if candidate == "#" {
			break
		}
	}
	if location.Source == "" {
		return index.operationAt(location.Pointer)
	}
	return ir.Operation{}, false
}

func (index *sourceOwnershipIndex) linkTarget(link map[string]any) (ir.Operation, error) {
	if index == nil {
		return ir.Operation{}, errMissingOwnershipIndex
	}
	return linkTargetOperation(index.operationsByID, index.operationsByPath, link)
}

func (index *sourceOwnershipIndex) restrictionsAt(document *ir.Document, pointer string) []ir.SemanticRestriction {
	if index == nil {
		return nil
	}
	if document == nil {
		return append([]ir.SemanticRestriction(nil), index.restrictionsByPoint[pointer]...)
	}
	provenance, found := document.LookupProvenance(pointer)
	if !found {
		return append([]ir.SemanticRestriction(nil), index.restrictionsByPoint[pointer]...)
	}
	var result []ir.SemanticRestriction
	locations := append([]ir.SourceLocation{provenance.Primary}, provenance.Related...)
	for _, location := range locations {
		key := sourceRestrictionKey{source: location.Source, pointer: location.Pointer}
		for _, restriction := range index.restrictionsBySource[key] {
			if !containsSemanticRestriction(result, restriction) {
				result = append(result, restriction)
			}
		}
	}
	return result
}

func parentSourcePointer(pointer string) string {
	if pointer == "" || pointer == "#" {
		return ""
	}
	index := strings.LastIndex(pointer, "/")
	if index < 1 {
		return "#"
	}
	return pointer[:index]
}

func containsSemanticRestriction(values []ir.SemanticRestriction, wanted ir.SemanticRestriction) bool {
	for _, value := range values {
		if value.RuleID == wanted.RuleID && value.Scope == wanted.Scope && value.Effect == wanted.Effect &&
			value.Location == wanted.Location {
			return true
		}
	}
	return false
}
