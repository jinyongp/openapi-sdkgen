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

// StructuralPosition describes the grammar-owned meaning of one OpenAPI path.
// Keyword is the structural field or named-map collection that owns the value.
// NamedMapContainer identifies maps whose keys are author-defined names;
// NamedMapEntry identifies one value selected from such a map.
type StructuralPosition struct {
	Object            ObjectContext
	Keyword           string
	NamedMapContainer bool
	NamedMapEntry     bool

	kind        structuralPositionKind
	entryObject ObjectContext
}

type structuralPositionKind uint8

const (
	structuralObject structuralPositionKind = iota
	structuralComponents
	structuralNamedMap
	structuralSequence
)

func objectPosition(object ObjectContext, keyword string) StructuralPosition {
	return StructuralPosition{Object: object, Keyword: keyword, kind: structuralObject}
}

func namedMapPosition(keyword string, entryObject ObjectContext) StructuralPosition {
	return StructuralPosition{
		Object:            ObjectUnknown,
		Keyword:           keyword,
		NamedMapContainer: true,
		kind:              structuralNamedMap,
		entryObject:       entryObject,
	}
}

func sequencePosition(keyword string, entryObject ObjectContext) StructuralPosition {
	return StructuralPosition{
		Object:      ObjectUnknown,
		Keyword:     keyword,
		kind:        structuralSequence,
		entryObject: entryObject,
	}
}

// StructuralPositionAt resolves an RFC 6901 token path from the OpenAPI root.
// Resolution follows the grammar from parent to child so a user-defined name
// such as "links" or "properties" cannot acquire structural meaning merely
// because its token matches an OpenAPI or JSON Schema keyword.
func StructuralPositionAt(path []string) StructuralPosition {
	return StructuralPositionAtRoot(ObjectOpenAPI, path)
}

// StructuralPositionAtRoot resolves a path whose physical source is already
// known to represent one reusable OpenAPI object or Schema root.
func StructuralPositionAtRoot(root ObjectContext, path []string) StructuralPosition {
	if root == "" {
		root = ObjectOpenAPI
	}
	position := objectPosition(root, "")
	for _, token := range path {
		position = childStructuralPosition(position, token)
	}
	return position
}

// ObjectContextAt classifies common OpenAPI object locations from an RFC 6901
// token path. Reference Object detection remains a value-level compatibility
// decision and must not be inferred from path alone.
func ObjectContextAt(path []string) ObjectContext {
	return StructuralPositionAt(path).Object
}

func childStructuralPosition(parent StructuralPosition, token string) StructuralPosition {
	switch parent.kind {
	case structuralComponents:
		return componentsChildPosition(token)
	case structuralNamedMap, structuralSequence:
		return StructuralPosition{
			Object:        parent.entryObject,
			Keyword:       parent.Keyword,
			NamedMapEntry: parent.kind == structuralNamedMap,
			kind:          structuralObject,
		}
	}

	switch parent.Object {
	case ObjectOpenAPI:
		switch token {
		case "info":
			return objectPosition(ObjectInfo, token)
		case "components":
			return StructuralPosition{Object: ObjectUnknown, Keyword: token, kind: structuralComponents}
		case "paths", "webhooks":
			return namedMapPosition(token, ObjectPathItem)
		}
	case ObjectPathItem:
		if isOperationToken(token) {
			return objectPosition(ObjectOperation, token)
		}
		switch token {
		case "parameters":
			return sequencePosition(token, ObjectParameter)
		case "additionalOperations":
			return namedMapPosition(token, ObjectOperation)
		}
	case ObjectOperation:
		switch token {
		case "parameters":
			return sequencePosition(token, ObjectParameter)
		case "requestBody":
			return objectPosition(ObjectRequestBody, token)
		case "responses":
			return namedMapPosition(token, ObjectResponse)
		case "callbacks":
			return namedMapPosition(token, ObjectCallback)
		}
	case ObjectParameter, ObjectHeader:
		switch token {
		case "schema":
			return objectPosition(ObjectSchema, token)
		case "content":
			return namedMapPosition(token, ObjectMediaType)
		case "examples":
			return namedMapPosition(token, ObjectExample)
		}
	case ObjectRequestBody:
		if token == "content" {
			return namedMapPosition(token, ObjectMediaType)
		}
	case ObjectResponse:
		switch token {
		case "headers":
			return namedMapPosition(token, ObjectHeader)
		case "links":
			return namedMapPosition(token, ObjectLink)
		case "content":
			return namedMapPosition(token, ObjectMediaType)
		}
	case ObjectMediaType:
		switch token {
		case "schema":
			return objectPosition(ObjectSchema, token)
		case "encoding":
			return namedMapPosition(token, ObjectEncoding)
		case "examples":
			return namedMapPosition(token, ObjectExample)
		}
	case ObjectEncoding:
		if token == "headers" {
			return namedMapPosition(token, ObjectHeader)
		}
	case ObjectCallback:
		if !strings.HasPrefix(token, "x-") {
			return objectPosition(ObjectPathItem, "callbacks")
		}
	case ObjectSchema:
		return schemaChildPosition(token)
	}
	return objectPosition(ObjectUnknown, token)
}

func componentsChildPosition(token string) StructuralPosition {
	switch token {
	case "schemas":
		return namedMapPosition(token, ObjectSchema)
	case "parameters":
		return namedMapPosition(token, ObjectParameter)
	case "headers":
		return namedMapPosition(token, ObjectHeader)
	case "requestBodies":
		return namedMapPosition(token, ObjectRequestBody)
	case "responses":
		return namedMapPosition(token, ObjectResponse)
	case "links":
		return namedMapPosition(token, ObjectLink)
	case "callbacks":
		return namedMapPosition(token, ObjectCallback)
	case "examples":
		return namedMapPosition(token, ObjectExample)
	case "securitySchemes":
		return namedMapPosition(token, ObjectSecurityScheme)
	case "pathItems":
		return namedMapPosition(token, ObjectPathItem)
	default:
		return objectPosition(ObjectUnknown, token)
	}
}

func schemaChildPosition(token string) StructuralPosition {
	switch token {
	case "items", "not", "additionalProperties", "contains", "contentSchema",
		"else", "if", "propertyNames", "then", "unevaluatedItems", "unevaluatedProperties":
		return objectPosition(ObjectSchema, token)
	case "properties", "patternProperties", "$defs", "definitions", "dependentSchemas":
		return namedMapPosition(token, ObjectSchema)
	case "allOf", "anyOf", "oneOf", "prefixItems":
		return sequencePosition(token, ObjectSchema)
	default:
		return objectPosition(ObjectUnknown, token)
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
