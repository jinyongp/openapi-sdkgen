package ir

import (
	"fmt"
	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"strings"
)

// ReadInboundOperations applies the same inheritance, component resolution,
// media, method and security semantics as outbound operations. The host supplies
// the eventual HTTP path; pointers retain the declared inbound identity.
func ReadInboundOperations(document *Document, pathItem map[string]any, pointer string) ([]Operation, error) {
	if document == nil {
		return nil, fmt.Errorf("inbound document is nil")
	}
	resolved, err := ResolvePathItem(document.Raw, pathItem)
	if err != nil {
		return nil, err
	}
	headers, err := readOperationHeadersAt(pointer, resolved, openapidoc.VersionLine(document.OpenAPIVersionLine), pointer)
	if err != nil {
		return nil, err
	}
	result := make([]Operation, 0, len(headers))
	for _, header := range headers {
		operation, err := buildOperationAt(document.Raw, "/", header.Method, strings.TrimPrefix(header.Pointer, "#"), pointer, resolved, header.Raw)
		if err != nil {
			return nil, err
		}
		operation.RouteKey = header.Method + " " + header.Pointer
		result = append(result, operation)
	}
	return result, nil
}
