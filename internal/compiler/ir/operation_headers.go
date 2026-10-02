package ir

import (
	"fmt"
	"strings"

	openapidoc "openapi-sdkgen/internal/compiler/openapi"
)

// OperationHeader is the method and declared metadata of a Path Item operation.
// It does not read request, response, or schema contracts.
type OperationHeader struct {
	Method  string
	Pointer string
	Raw     map[string]any
}

// ReadOperationHeaders uses the compiler's method set and additional-operation
// shape checks without constructing the full HTTP intermediate representation.
func ReadOperationHeaders(path string, item map[string]any, version openapidoc.VersionLine) ([]OperationHeader, error) {
	var headers []OperationHeader
	for _, method := range standardMethods {
		value, exists := item[method]
		if !exists {
			continue
		}
		operation, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("operation %s %q must be an object", strings.ToUpper(method), path)
		}
		if method == "query" && version != openapidoc.Version32 {
			return nil, fmt.Errorf("query method requires OpenAPI 3.2")
		}
		headers = append(headers, OperationHeader{strings.ToUpper(method), "#" + jsonPointer("paths", path, method), operation})
	}
	value, exists := item["additionalOperations"]
	if !exists {
		return headers, nil
	}
	additional, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("additionalOperations at %q must be an object", path)
	}
	if version != openapidoc.Version32 {
		return nil, fmt.Errorf("additionalOperations requires OpenAPI 3.2")
	}
	for _, method := range sortedKeys(additional) {
		if method == "" || strings.ContainsFunc(method, func(r rune) bool {
			return r > 127 || !strings.ContainsRune("!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", r)
		}) {
			return nil, fmt.Errorf("invalid additional operation method %q", method)
		}
		operation, err := validatedAdditionalOperationObject(additional, method, path)
		if err != nil {
			return nil, err
		}
		headers = append(headers, OperationHeader{method, "#" + jsonPointer("paths", path, "additionalOperations", method), operation})
	}
	return headers, nil
}
