package openapiwalk

import "strings"

// ObjectContext identifies the OpenAPI object type expected at one structural
// path. It is intentionally target-neutral and conservative: unknown or
// ambiguous locations remain ObjectUnknown rather than being guessed.
type ObjectContext string

const (
	ObjectUnknown        ObjectContext = "unknown"
	ObjectOpenAPI        ObjectContext = "openapi"
	ObjectInfo           ObjectContext = "info"
	ObjectPathItem       ObjectContext = "path-item"
	ObjectOperation      ObjectContext = "operation"
	ObjectParameter      ObjectContext = "parameter"
	ObjectRequestBody    ObjectContext = "request-body"
	ObjectResponse       ObjectContext = "response"
	ObjectHeader         ObjectContext = "header"
	ObjectLink           ObjectContext = "link"
	ObjectCallback       ObjectContext = "callback"
	ObjectExample        ObjectContext = "example"
	ObjectSecurityScheme ObjectContext = "security-scheme"
	ObjectMediaType      ObjectContext = "media-type"
	ObjectEncoding       ObjectContext = "encoding"
	ObjectSchema         ObjectContext = "schema"
)

// ObjectContextAt classifies common OpenAPI object locations from an RFC 6901
// token path. Reference Object detection remains a value-level compatibility
// decision and must not be inferred from path alone.
func ObjectContextAt(path []string) ObjectContext {
	if len(path) == 0 {
		return ObjectOpenAPI
	}
	if len(path) == 1 && path[0] == "info" {
		return ObjectInfo
	}
	if isSchemaObjectPath(path) {
		return ObjectSchema
	}
	if len(path) >= 2 {
		switch path[len(path)-2] {
		case "parameters":
			return ObjectParameter
		case "requestBodies":
			return ObjectRequestBody
		case "responses":
			return ObjectResponse
		case "headers":
			return ObjectHeader
		case "links":
			return ObjectLink
		case "callbacks":
			return ObjectCallback
		case "examples":
			return ObjectExample
		case "securitySchemes":
			return ObjectSecurityScheme
		case "content":
			return ObjectMediaType
		case "encoding":
			return ObjectEncoding
		case "paths", "webhooks", "pathItems":
			return ObjectPathItem
		case "additionalOperations":
			return ObjectOperation
		}
	}
	if isOperationToken(path[len(path)-1]) {
		return ObjectOperation
	}
	if isCallbackPathItem(path) {
		return ObjectPathItem
	}
	return ObjectUnknown
}

func isSchemaObjectPath(path []string) bool {
	if len(path) == 0 {
		return false
	}
	last := path[len(path)-1]
	switch last {
	case "schema", "items", "not", "additionalProperties", "contains", "contentSchema",
		"else", "if", "propertyNames", "then", "unevaluatedItems", "unevaluatedProperties":
		return true
	}
	if len(path) < 2 {
		return false
	}
	switch path[len(path)-2] {
	case "schemas", "properties", "patternProperties", "$defs", "definitions", "dependentSchemas":
		return true
	case "allOf", "anyOf", "oneOf", "prefixItems":
		return true
	default:
		return false
	}
}

func isOperationToken(value string) bool {
	switch strings.ToLower(value) {
	case "get", "put", "post", "delete", "options", "head", "patch", "trace", "query":
		return true
	default:
		return false
	}
}

func isCallbackPathItem(path []string) bool {
	if len(path) < 3 {
		return false
	}
	for index := len(path) - 3; index >= 0; index-- {
		if path[index] == "callbacks" {
			return len(path) == index+3
		}
	}
	return false
}
