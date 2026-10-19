package typescript

import (
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

func operationHTTPErrorType(document *ir.Document, operation ir.Operation, scope typeRenderScope) (string, error) {
	responses, err := operationResponses(document, operation)
	if err != nil {
		return "", err
	}
	var result []string
	for _, branch := range responseStatusBranches(responses, false) {
		data, media := "void", "undefined"
		if branch.Media != nil {
			media = quoteTS(branch.Media.ContentType)
			data, err = schemaTypeForScope(document, branch.Media.Schema, projectionOutput, scope)
			if err != nil {
				return "", err
			}
		}
		result = append(result, "HTTPErrorFor<"+responseStatusUnion(branch.Statuses)+", "+data+", "+media+">")
	}
	if len(result) == 0 {
		return "never", nil
	}
	return strings.Join(uniqueStrings(result), " | "), nil
}
