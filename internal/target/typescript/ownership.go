package typescript

import (
	"openapi-sdkgen/internal/compiler/ir"
	httpplan "openapi-sdkgen/internal/http/plan"
)

type sourceOwnershipIndex struct{ shared *httpplan.OwnershipIndex }

func newSourceOwnershipIndex(document *ir.Document) *sourceOwnershipIndex {
	return &sourceOwnershipIndex{shared: httpplan.NewOwnershipIndex(document)}
}
func (index *sourceOwnershipIndex) operationByExactPointer(pointer string) (ir.Operation, bool) {
	if index == nil {
		return ir.Operation{}, false
	}
	return index.shared.OperationByExactPointer(pointer)
}
func (index *sourceOwnershipIndex) operationAt(pointer string) (ir.Operation, bool) {
	if index == nil {
		return ir.Operation{}, false
	}
	return index.shared.OperationAt(pointer)
}
func (index *sourceOwnershipIndex) operationAtLocation(location ir.SourceLocation) (ir.Operation, bool) {
	if index == nil {
		return ir.Operation{}, false
	}
	return index.shared.OperationAtLocation(location)
}
func (index *sourceOwnershipIndex) linkTarget(link map[string]any, pointer string) (ir.Operation, error) {
	var shared *httpplan.OwnershipIndex
	if index != nil {
		shared = index.shared
	}
	return shared.LinkTarget(link, pointer)
}
func (index *sourceOwnershipIndex) restrictionsAt(document *ir.Document, pointer string) []ir.SemanticRestriction {
	if index == nil {
		return nil
	}
	return index.shared.RestrictionsAt(document, pointer)
}
