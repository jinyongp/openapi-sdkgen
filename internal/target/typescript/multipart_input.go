package typescript

import (
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	schemaplan "openapi-sdkgen/internal/target/typescript/schema/plan"
)

// multipartInputSchema projects only a request media occurrence. A component
// reused by JSON retains its ordinary JSON Schema input contract.
func multipartInputSchema(document *ir.Document, contentType string, value any, media map[string]any) any {
	if !strings.HasPrefix(strings.ToLower(contentType), "multipart/") {
		return value
	}
	projected, changed := projectMultipartRoot(document, value, media, make(map[string]bool), nil)
	if !changed {
		return value
	}
	return projected
}

func multipartItemInputSchema(document *ir.Document, contentType string, value any, encoding any) any {
	if !strings.HasPrefix(strings.ToLower(contentType), "multipart/") {
		return value
	}
	definition, _ := encoding.(map[string]any)
	projected, changed := projectMultipartPart(document, value, definition, make(map[string]bool))
	if !changed {
		return value
	}
	return projected
}

func multipartResolvedSchema(document *ir.Document, value any, seen map[string]bool) (map[string]any, bool) {
	schema, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	reference, _ := schema["$ref"].(string)
	if reference == "" {
		return schema, true
	}
	name, err := componentSchemaReferenceName(reference)
	if err != nil || seen[name] {
		return nil, false
	}
	target, exists := document.ComponentSchemas[name]
	if !exists {
		return nil, false
	}
	seen[name] = true
	resolved, ok := multipartResolvedSchema(document, target, seen)
	delete(seen, name)
	if !ok {
		return nil, false
	}
	result := cloneMultipartMap(resolved)
	// Reference siblings are assertions in JSON Schema dialects. OpenAPI 3.0
	// Reference Objects ignore siblings.
	if document.OpenAPIVersionLine == "3.1" || document.OpenAPIVersionLine == "3.2" {
		siblings := cloneMultipartMap(schema)
		delete(siblings, "$ref")
		if len(siblings) > 0 {
			branches, _ := result["allOf"].([]any)
			result["allOf"] = append(append([]any(nil), branches...), siblings)
		}
		for key, item := range schema {
			if key == "format" || key == "contentMediaType" || key == "contentEncoding" || key == "readOnly" || key == "writeOnly" {
				result[key] = item
			}
		}
	}
	return result, true
}

func cloneMultipartMap(schema map[string]any) map[string]any {
	result := make(map[string]any, len(schema))
	for key, value := range schema {
		result[key] = value
	}
	return result
}

func projectMultipartRoot(document *ir.Document, value any, media map[string]any, seen map[string]bool, inherited map[string]schemaplan.BinaryInput) (any, bool) {
	source, _ := value.(map[string]any)
	reference, _ := source["$ref"].(string)
	if reference != "" {
		if seen[reference] {
			return value, false
		}
		seen[reference] = true
		defer delete(seen, reference)
	}
	schema, ok := multipartResolvedSchema(document, value, make(map[string]bool))
	if !ok {
		return value, false
	}
	result := cloneMultipartMap(schema)
	changed := false
	hints := multipartRootBinaryInputs(document, schema, media, make(map[string]bool))
	for name, hint := range inherited {
		hints[name] = hint
	}
	if items, exists := schema["items"]; exists {
		encoding, _ := media["itemEncoding"].(map[string]any)
		part, partChanged := projectMultipartPart(document, items, encoding, seen)
		if partChanged {
			result["items"] = part
			changed = true
		}
	}
	if items, ok := schema["prefixItems"].([]any); ok {
		encodings, _ := media["prefixEncoding"].([]any)
		projected := append([]any(nil), items...)
		for index, item := range items {
			var encoding map[string]any
			if index < len(encodings) {
				encoding, _ = encodings[index].(map[string]any)
			}
			part, partChanged := projectMultipartPart(document, item, encoding, seen)
			if partChanged {
				projected[index] = part
				changed = true
			}
		}
		result["prefixItems"] = projected
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		projected := cloneMultipartMap(properties)
		encodings, _ := media["encoding"].(map[string]any)
		for name, property := range properties {
			encoding, _ := encodings[name].(map[string]any)
			part, partChanged := projectMultipartPartWithHint(document, property, encoding, seen, hints[name])
			if partChanged {
				projected[name] = part
				changed = true
			}
		}
		result["properties"] = projected
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		if branches, ok := schema[keyword].([]any); ok {
			projected := append([]any(nil), branches...)
			for index, branch := range branches {
				part, branchChanged := projectMultipartRoot(document, branch, media, seen, hints)
				if branchChanged {
					projected[index] = part
					changed = true
				}
			}
			result[keyword] = projected
		}
	}
	return result, changed
}

// Conjunctive assertions for one named part share its transport contract. This
// does not mix contracts between alternative root branches.
func multipartRootBinaryInputs(document *ir.Document, value any, media map[string]any, seen map[string]bool) map[string]schemaplan.BinaryInput {
	result := make(map[string]schemaplan.BinaryInput)
	source, _ := value.(map[string]any)
	reference, _ := source["$ref"].(string)
	if reference != "" {
		if seen[reference] {
			return result
		}
		seen[reference] = true
		defer delete(seen, reference)
	}
	schema, ok := multipartResolvedSchema(document, value, make(map[string]bool))
	if !ok {
		return result
	}
	properties, _ := schema["properties"].(map[string]any)
	encodings, _ := media["encoding"].(map[string]any)
	for name, property := range properties {
		encoding, _ := encodings[name].(map[string]any)
		projected, _ := projectMultipartPart(document, property, encoding, make(map[string]bool))
		if hint, ok := multipartProjectedBinaryInput(projected); ok {
			result[name] = hint
		}
	}
	branches, _ := schema["allOf"].([]any)
	for _, branch := range branches {
		for name, hint := range multipartRootBinaryInputs(document, branch, media, seen) {
			if _, exists := result[name]; !exists {
				result[name] = hint
			}
		}
	}
	return result
}

func multipartProjectedBinaryInput(value any) (schemaplan.BinaryInput, bool) {
	if input, ok := value.(schemaplan.BinaryInput); ok {
		return input, true
	}
	schema, _ := value.(map[string]any)
	if item, exists := schema["items"]; exists {
		if input, ok := multipartProjectedBinaryInput(item); ok {
			return input, true
		}
	}
	branches, _ := schema["allOf"].([]any)
	for _, branch := range branches {
		if input, ok := multipartProjectedBinaryInput(branch); ok {
			return input, true
		}
	}
	return schemaplan.BinaryInput{}, false
}

func projectMultipartPartWithHint(document *ir.Document, value any, encoding map[string]any, seen map[string]bool, hint schemaplan.BinaryInput) (any, bool) {
	if hint.ContentType != "" {
		schema, ok := multipartResolvedSchema(document, value, make(map[string]bool))
		kind, _ := schema["type"].(string)
		encoded, _ := schema["contentEncoding"].(string)
		if ok && kind == "string" && encoded == "" && !boolValue(schema, "readOnly") {
			hint.Schema = schema
			return hint, true
		}
		if ok && kind == "array" && encoded == "" {
			if items, exists := schema["items"]; exists {
				projected, changed := projectMultipartPartWithHint(document, items, encoding, seen, hint)
				if changed {
					result := cloneMultipartMap(schema)
					result["items"] = projected
					return result, true
				}
			}
		}
	}
	return projectMultipartPart(document, value, encoding, seen)
}

func projectMultipartPart(document *ir.Document, value any, encoding map[string]any, seen map[string]bool) (any, bool) {
	source, _ := value.(map[string]any)
	reference, _ := source["$ref"].(string)
	if reference != "" {
		if seen[reference] {
			return value, false
		}
		seen[reference] = true
		defer delete(seen, reference)
	}
	schema, ok := multipartResolvedSchema(document, value, make(map[string]bool))
	if !ok || boolValue(schema, "readOnly") {
		return value, false
	}
	if items, exists := schema["items"]; exists {
		projected, changed := projectMultipartPart(document, items, encoding, seen)
		if changed {
			result := cloneMultipartMap(schema)
			result["items"] = projected
			return result, true
		}
		return value, false
	}
	contentEncoding, _ := schema["contentEncoding"].(string)
	if contentEncoding != "" && contentEncoding != "binary" {
		return value, false
	}
	typeName, _ := schema["type"].(string)
	_, hasType := schema["type"]
	stringType := typeName == "string"
	nullable := boolValue(schema, "nullable") && document.OpenAPIVersionLine != "3.1" && document.OpenAPIVersionLine != "3.2"
	if types, ok := schema["type"].([]any); ok {
		stringType = true
		for _, kind := range types {
			if kind == "null" {
				nullable = true
			} else if kind != "string" {
				stringType = false
			}
		}
	}
	format, _ := schema["format"].(string)
	contentType, _ := encoding["contentType"].(string)
	if contentType == "" {
		contentType, _ = schema["contentMediaType"].(string)
	}
	raw := format == "binary" && (stringType || !hasType)
	if !hasType && (document.OpenAPIVersionLine == "3.1" || document.OpenAPIVersionLine == "3.2") {
		_, object := schema["properties"]
		_, composition := schema["allOf"]
		_, alternatives := schema["anyOf"]
		_, choices := schema["oneOf"]
		raw = raw || !object && !composition && !alternatives && !choices && (contentType == "" || !isJSONMediaType(contentType) && !isTextMedia(contentType) && !executionXMLMedia(contentType))
	}
	if !raw {
		for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
			if branches, ok := schema[keyword].([]any); ok {
				projected := append([]any(nil), branches...)
				changed := false
				for index, branch := range branches {
					part, partChanged := projectMultipartPart(document, branch, encoding, seen)
					if partChanged {
						projected[index] = part
						changed = true
					}
				}
				if changed {
					result := cloneMultipartMap(schema)
					if keyword == "allOf" {
						for _, branch := range projected {
							if hint, ok := branch.(schemaplan.BinaryInput); ok {
								for index, original := range branches {
									projected[index], _ = projectMultipartPartWithHint(document, original, encoding, seen, hint)
								}
								break
							}
						}
					}
					result[keyword] = projected
					return result, true
				}
			}
		}
		return value, false
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return schemaplan.BinaryInput{Schema: schema, ContentType: contentType, Nullable: nullable}, true
}
