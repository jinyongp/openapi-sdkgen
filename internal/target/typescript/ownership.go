package typescript

import (
	"errors"
	"fmt"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

var errMissingOwnershipIndex = errors.New("source ownership index is unavailable")

type sourceRestrictionKey struct {
	source  string
	pointer string
}

type sourceOwnershipIndex struct {
	operations           []ir.Operation
	operationsByPointer  map[string]int
	operationsBySource   map[sourceRestrictionKey]int
	operationsByID       map[string]int
	operationsByPath     map[string]int
	restrictionsByPoint  map[string][]ir.SemanticRestriction
	restrictionsBySource map[sourceRestrictionKey][]ir.SemanticRestriction
}

func newSourceOwnershipIndex(document *ir.Document) *sourceOwnershipIndex {
	index := &sourceOwnershipIndex{
		operations:           document.Operations,
		operationsByPointer:  make(map[string]int, len(document.Operations)),
		operationsBySource:   make(map[sourceRestrictionKey]int, len(document.Operations)),
		operationsByID:       make(map[string]int, len(document.Operations)),
		operationsByPath:     make(map[string]int, len(document.Operations)),
		restrictionsByPoint:  make(map[string][]ir.SemanticRestriction),
		restrictionsBySource: make(map[sourceRestrictionKey][]ir.SemanticRestriction),
	}
	for operationIndex := range document.Operations {
		operation := document.Operations[operationIndex]
		pointer := operation.Pointer
		if pointer == "" {
			pointer = "#/paths/" + escapePointerToken(operation.Path) + "/" + strings.ToLower(operation.Method)
		}
		index.operationsByPointer[pointer] = operationIndex
		if provenance, found := document.LookupProvenance(pointer); found {
			locations := append([]ir.SourceLocation{provenance.Primary}, provenance.Related...)
			for _, location := range locations {
				key := sourceRestrictionKey{source: location.Source, pointer: location.Pointer}
				if existing, exists := index.operationsBySource[key]; exists && existing != operationIndex {
					index.operationsBySource[key] = -1
					continue
				}
				if _, exists := index.operationsBySource[key]; !exists {
					index.operationsBySource[key] = operationIndex
				}
			}
		}
		if operation.OperationID != "" {
			index.operationsByID[operation.OperationID] = operationIndex
		}
		index.operationsByPath[operationPathMethodKey(operation.Path, operation.Method)] = operationIndex
	}
	for _, restriction := range document.SemanticRestrictions {
		index.restrictionsByPoint[restriction.Location.Pointer] = append(index.restrictionsByPoint[restriction.Location.Pointer], restriction)
		key := sourceRestrictionKey{source: restriction.Location.Source, pointer: restriction.Location.Pointer}
		index.restrictionsBySource[key] = append(index.restrictionsBySource[key], restriction)
	}
	return index
}

func (index *sourceOwnershipIndex) operation(operationIndex int) (ir.Operation, bool) {
	if index == nil || operationIndex < 0 || operationIndex >= len(index.operations) {
		return ir.Operation{}, false
	}
	return index.operations[operationIndex], true
}

func (index *sourceOwnershipIndex) operationAt(pointer string) (ir.Operation, bool) {
	if index == nil {
		return ir.Operation{}, false
	}
	for candidate := pointer; candidate != ""; candidate = parentSourcePointer(candidate) {
		if operationIndex, exists := index.operationsByPointer[candidate]; exists {
			return index.operation(operationIndex)
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
		if operationIndex, exists := index.operationsBySource[key]; exists {
			if operationIndex < 0 {
				return ir.Operation{}, false
			}
			return index.operation(operationIndex)
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
	if operationID, _ := link["operationId"].(string); operationID != "" {
		operationIndex, ok := index.operationsByID[operationID]
		if !ok {
			return ir.Operation{}, fmt.Errorf("operationId %q does not name a generated operation", operationID)
		}
		operation, _ := index.operation(operationIndex)
		return operation, nil
	}
	operationRef, _ := link["operationRef"].(string)
	if !strings.HasPrefix(operationRef, "#/paths/") {
		return ir.Operation{}, fmt.Errorf("requires operationId or a local operationRef")
	}
	tokens := strings.Split(strings.TrimPrefix(operationRef, "#/"), "/")
	if len(tokens) != 3 || tokens[0] != "paths" {
		return ir.Operation{}, fmt.Errorf("operationRef %q must target one path operation", operationRef)
	}
	path, err := linkJSONPointerToken(tokens[1])
	if err != nil {
		return ir.Operation{}, err
	}
	method, err := linkJSONPointerToken(tokens[2])
	if err != nil {
		return ir.Operation{}, err
	}
	if operationIndex, exists := index.operationsByPath[operationPathMethodKey(path, method)]; exists {
		operation, _ := index.operation(operationIndex)
		return operation, nil
	}
	return ir.Operation{}, fmt.Errorf("operationRef %q does not name a generated operation", operationRef)
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
