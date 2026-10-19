package plan

import (
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"testing"
)

func TestOwnershipIndexConstructionIsOperationLinearAndLookupPointerBounded(t *testing.T) {
	const operations = 4096
	document := &ir.Document{Operations: make([]ir.Operation, 0, operations)}
	for index := 0; index < operations; index++ {
		path := fmt.Sprintf("/items/%d", index)
		document.Operations = append(document.Operations, ir.Operation{
			Pointer:     "#/paths/~1items~1" + fmt.Sprint(index) + "/get",
			OperationID: fmt.Sprintf("getItem%d", index),
			Method:      "GET",
			Path:        path,
		})
	}
	index := NewOwnershipIndex(document)
	if len(index.operationsByPointer) != operations || len(index.operationsByID) != operations || len(index.operationsByPath) != operations {
		t.Fatalf("ownership index cardinality = pointer:%d id:%d path:%d", len(index.operationsByPointer), len(index.operationsByID), len(index.operationsByPath))
	}
	target := document.Operations[operations-1]
	nested := target.Pointer + "/responses/200/content/application~1json/schema/properties/id"
	value, found := index.OperationAt(nested)
	if !found || value.OperationID != target.OperationID {
		t.Fatalf("nested ownership = %#v, found=%v", value, found)
	}
	if _, found := index.OperationAt("#/components/schemas/Unowned/properties/id"); found {
		t.Fatal("component pointer unexpectedly acquired operation ownership")
	}
}
