package typescript

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/openapiwalk"
)

type projection string

const (
	projectionInput  projection = "input"
	projectionOutput projection = "output"
)

func reachableComponentSchemas(document *ir.Document) map[string]bool {
	input, output := reachableComponentSchemaProjections(document, false)
	return publicReachableComponentSchemas(document, input, output)
}

func publicReachableComponentSchemas(document *ir.Document, input, output map[string]bool) map[string]bool {
	result := make(map[string]bool, len(input)+len(output))
	for name := range input {
		result[name] = true
	}
	for name := range output {
		result[name] = true
	}
	if len(result) == 0 && len(document.Operations) == 0 {
		for name := range document.ComponentSchemas {
			result[name] = true
		}
		for name := range document.Schemas {
			result[name] = true
		}
	}
	return result
}

type componentSchemaReachabilityRoot struct {
	value any
	path  []string
}

func reachableComponentSchemaProjections(document *ir.Document, includeServer bool) (map[string]bool, map[string]bool) {
	input := make(map[string]bool)
	output := make(map[string]bool)
	inputRoots := make([]componentSchemaReachabilityRoot, 0, len(document.Operations)*3)
	outputRoots := make([]componentSchemaReachabilityRoot, 0, len(document.Operations))
	for _, operation := range document.Operations {
		if operation.Visibility == "hidden" {
			continue
		}
		base := []string{"paths", operation.Path, strings.ToLower(operation.Method)}
		if operation.Parameters != nil {
			for index, parameter := range operation.Parameters {
				inputRoots = append(inputRoots, componentSchemaReachabilityRoot{value: parameter.Raw, path: appendPath(base, "parameters", fmt.Sprint(index))})
			}
			if operation.RequestBody != nil {
				inputRoots = append(inputRoots, componentSchemaReachabilityRoot{value: operation.RequestBody.Raw, path: appendPath(base, "requestBody")})
			}
		} else {
			inputRoots = append(inputRoots,
				componentSchemaReachabilityRoot{value: operation.PathItemRaw["parameters"], path: []string{"paths", operation.Path, "parameters"}},
				componentSchemaReachabilityRoot{value: operation.Raw["parameters"], path: appendPath(base, "parameters")},
				componentSchemaReachabilityRoot{value: operation.Raw["requestBody"], path: appendPath(base, "requestBody")},
			)
		}
		if operation.Responses != nil {
			for _, response := range operation.Responses {
				outputRoots = append(outputRoots, componentSchemaReachabilityRoot{value: response.Raw, path: appendPath(base, "responses", response.Status)})
			}
		} else {
			outputRoots = append(outputRoots, componentSchemaReachabilityRoot{value: operation.Raw["responses"], path: appendPath(base, "responses")})
		}
		if includeServer {
			callbacks, _ := operation.Raw["callbacks"].(map[string]any)
			appendCallbackSchemaReachabilityRoots(document, callbacks, appendPath(base, "callbacks"), &inputRoots, &outputRoots)
		}
	}
	if includeServer {
		webhooks, _ := document.Raw["webhooks"].(map[string]any)
		for name, value := range webhooks {
			if openapiwalk.IsExtensionKey([]string{"webhooks"}, name) {
				continue
			}
			pathItem, _ := value.(map[string]any)
			appendServerPathItemSchemaReachabilityRoots(document, pathItem, []string{"webhooks", name}, &inputRoots, &outputRoots)
		}
		components, _ := document.Raw["components"].(map[string]any)
		callbacks, _ := components["callbacks"].(map[string]any)
		appendCallbackSchemaReachabilityRoots(document, callbacks, []string{"components", "callbacks"}, &inputRoots, &outputRoots)
	}
	visitComponentSchemaReferences(document, input, inputRoots...)
	visitComponentSchemaReferences(document, output, outputRoots...)
	return input, output
}

func appendCallbackSchemaReachabilityRoots(document *ir.Document, callbacks map[string]any, path []string, input, output *[]componentSchemaReachabilityRoot) {
	for name, value := range callbacks {
		if openapiwalk.IsExtensionKey(path, name) {
			continue
		}
		callback, _ := value.(map[string]any)
		resolved, err := resolveComponentObject(document, callback, "callbacks")
		if err != nil {
			continue
		}
		callbackPath := appendPath(path, name)
		for expression, item := range resolved {
			if openapiwalk.IsExtensionKey(callbackPath, expression) {
				continue
			}
			pathItem, _ := item.(map[string]any)
			appendServerPathItemSchemaReachabilityRoots(document, pathItem, appendPath(callbackPath, expression), input, output)
		}
	}
}

func appendServerPathItemSchemaReachabilityRoots(document *ir.Document, pathItem map[string]any, path []string, input, output *[]componentSchemaReachabilityRoot) {
	resolved, err := ir.ResolvePathItem(document.Raw, pathItem)
	if err != nil {
		return
	}
	*input = append(*input, componentSchemaReachabilityRoot{value: resolved["parameters"], path: appendPath(path, "parameters")})
	for _, method := range serverHTTPMethods {
		operation, _ := resolved[method].(map[string]any)
		appendServerOperationSchemaReachabilityRoots(operation, appendPath(path, method), input, output)
	}
	additional, _ := resolved["additionalOperations"].(map[string]any)
	for method, value := range additional {
		operation, _ := value.(map[string]any)
		appendServerOperationSchemaReachabilityRoots(operation, appendPath(appendPath(path, "additionalOperations"), method), input, output)
	}
}

func appendServerOperationSchemaReachabilityRoots(operation map[string]any, path []string, input, output *[]componentSchemaReachabilityRoot) {
	if operation == nil {
		return
	}
	*input = append(*input,
		componentSchemaReachabilityRoot{value: operation["parameters"], path: appendPath(path, "parameters")},
		componentSchemaReachabilityRoot{value: operation["requestBody"], path: appendPath(path, "requestBody")},
	)
	*output = append(*output, componentSchemaReachabilityRoot{value: operation["responses"], path: appendPath(path, "responses")})
}

func appendPath(path []string, values ...string) []string {
	result := make([]string, 0, len(path)+len(values))
	result = append(result, path...)
	return append(result, values...)
}

func visitComponentSchemaReferences(document *ir.Document, found map[string]bool, roots ...componentSchemaReachabilityRoot) {
	seenReferences := make(map[string]bool)
	var visit func(any, []string)
	visitReference := func(reference string) {
		if reference == "" || seenReferences[reference] {
			return
		}
		seenReferences[reference] = true
		if name, err := componentSchemaReferenceName(reference); err == nil {
			found[name] = true
			visit(componentSchemaValue(document, name), []string{"components", "schemas", name})
			return
		}
		components, _ := document.Raw["components"].(map[string]any)
		for component, values := range components {
			objects, _ := values.(map[string]any)
			name, err := componentReferenceName(reference, component)
			if err == nil {
				visit(objects[name], []string{"components", component, name})
				return
			}
		}
	}
	visit = func(value any, path []string) {
		switch typed := value.(type) {
		case map[string]any:
			for _, keyword := range []string{"$ref", "$dynamicRef"} {
				reference, _ := typed[keyword].(string)
				visitReference(reference)
			}
			if dynamic, _ := typed["x-sdkgen-dynamic-reference"].(map[string]any); dynamic != nil {
				reference, _ := dynamic["reference"].(string)
				visitReference(reference)
			}
			for key, item := range typed {
				if key == "$ref" || key == "$dynamicRef" || key == "x-sdkgen-dynamic-reference" ||
					openapiwalk.IsExtensionKey(path, key) ||
					(!openapiwalk.IsNamedMap(path) && openapiwalk.IsOpaqueDataField(key, item)) {
					continue
				}
				visit(item, appendPath(path, key))
			}
		case []any:
			for index, item := range typed {
				visit(item, appendPath(path, fmt.Sprint(index)))
			}
		}
	}
	for _, root := range roots {
		visit(root.value, root.path)
	}
}

func componentSchemaValue(document *ir.Document, name string) any {
	if schema, ok := document.Schemas[name]; ok {
		return schema.Value
	}
	return document.ComponentSchemas[name]
}

func schemaIsAlwaysDeprecated(document *ir.Document, value any) bool {
	return schemaIsAlwaysDeprecatedSeen(document, value, make(map[string]bool))
}

func schemaIsAlwaysDeprecatedSeen(document *ir.Document, value any, seen map[string]bool) bool {
	schema, ok := value.(map[string]any)
	if !ok {
		return false
	}
	if boolValue(schema, "deprecated") {
		return true
	}
	if document != nil {
		if reference, _ := schema["$ref"].(string); reference != "" {
			if name, err := componentSchemaReferenceName(reference); err == nil && !seen[name] {
				seen[name] = true
				deprecated := schemaIsAlwaysDeprecatedSeen(document, componentSchemaValue(document, name), seen)
				delete(seen, name)
				if deprecated {
					return true
				}
			}
		}
	}
	if branches, ok := schema["allOf"].([]any); ok {
		for _, branch := range branches {
			if schemaIsAlwaysDeprecatedSeen(document, branch, seen) {
				return true
			}
		}
	}
	for _, keyword := range []string{"oneOf", "anyOf"} {
		branches, ok := schema[keyword].([]any)
		if !ok || len(branches) == 0 {
			continue
		}
		allDeprecated := true
		for _, branch := range branches {
			if !schemaIsAlwaysDeprecatedSeen(document, branch, seen) {
				allDeprecated = false
				break
			}
		}
		if allDeprecated {
			return true
		}
	}
	return false
}

func emitSchemaValueJSDoc(output *bytes.Buffer, document *ir.Document, indent string, schema map[string]any, fallback string) {
	description, _ := schema["description"].(string)
	format, _ := schema["format"].(string)
	defaultValue, hasDefault := schema["default"]
	deprecated := schemaIsAlwaysDeprecated(document, schema)
	fmt.Fprintf(output, "%s/**\n", indent)
	if description != "" {
		fmt.Fprintf(output, "%s * %s\n", indent, sanitizeComment(description))
	} else {
		fmt.Fprintf(output, "%s * %s\n", indent, fallback)
	}
	if format != "" {
		fmt.Fprintf(output, "%s * Format: `%s`.\n", indent, sanitizeComment(format))
	}
	if boolValue(schema, "readOnly") {
		fmt.Fprintf(output, "%s * Present in responses; omitted from generated input projections.\n", indent)
	}
	if boolValue(schema, "writeOnly") {
		fmt.Fprintf(output, "%s * Accepted in requests; omitted from generated output projections.\n", indent)
	}
	if hasDefault {
		fmt.Fprintf(output, "%s * @default %s\n", indent, literalTS(defaultValue))
	}
	if deprecated {
		fmt.Fprintf(output, "%s * @deprecated This OpenAPI value is deprecated.\n", indent)
	}
	if constraints := schemaConstraintSummary(schema); constraints != "" {
		fmt.Fprintf(output, "%s * Constraints: %s.\n", indent, constraints)
	}
	fmt.Fprintf(output, "%s */\n", indent)
}

// schemaConstraintSummary keeps validation-only Schema Object keywords visible
// in generated source. TypeScript cannot encode every JSON Schema predicate in
// a static type, so the generated declaration records the contract rather than
// silently discarding it.
func schemaConstraintSummary(schema map[string]any) string {
	keys := []string{
		"multipleOf", "maximum", "exclusiveMaximum", "minimum", "exclusiveMinimum",
		"maxLength", "minLength", "pattern", "maxItems", "minItems", "uniqueItems",
		"contains", "minContains", "maxContains", "maxProperties", "minProperties",
		"dependentRequired", "propertyNames", "unevaluatedItems", "unevaluatedProperties",
		"contentEncoding", "contentMediaType", "contentSchema",
	}
	items := make([]string, 0, len(keys))
	for _, key := range keys {
		if value, ok := schema[key]; ok {
			items = append(items, "`"+key+"="+sanitizeComment(literalTS(value))+"`")
		}
	}
	return strings.Join(items, ", ")
}

func schemaType(document *ir.Document, value any, direction projection) (string, error) {
	return schemaTypeForScope(document, value, direction, typeRenderLocal)
}

func schemaTypeForScope(document *ir.Document, value any, direction projection, scope typeRenderScope) (string, error) {
	if boolean, ok := value.(bool); ok {
		if boolean {
			return "unknown", nil
		}
		return "never", nil
	}
	schema, ok := value.(map[string]any)
	if !ok {
		return "unknown", nil
	}
	// OpenAPI 3.0 expresses nullability independently of `type`, unlike the
	// JSON Schema type array used by OpenAPI 3.1 and 3.2.
	if document.OpenAPIVersionLine != "3.1" && document.OpenAPIVersionLine != "3.2" && boolValue(schema, "nullable") {
		withoutNullable := make(map[string]any, len(schema)-1)
		for key, value := range schema {
			if key != "nullable" {
				withoutNullable[key] = value
			}
		}
		value, err := schemaTypeForScope(document, withoutNullable, direction, scope)
		if err != nil {
			return "", err
		}
		return addUnionMember(value, "null"), nil
	}
	if dynamicReference, ok := schema["x-sdkgen-dynamic-reference"].(map[string]any); ok {
		reference, _ := dynamicReference["reference"].(string)
		name, err := componentSchemaReferenceName(reference)
		if err != nil {
			return "", err
		}
		referenced, err := referencedType(document, name, direction, scope)
		if err != nil || len(schema) == 1 {
			return referenced, err
		}
		siblings := make(map[string]any, len(schema)-1)
		for key, value := range schema {
			if key != "x-sdkgen-dynamic-reference" && isTypeAffectingSchemaKeyword(key) {
				siblings[key] = value
			}
		}
		if len(siblings) == 0 {
			return referenced, nil
		}
		siblingType, err := schemaTypeForScope(document, siblings, direction, scope)
		if err != nil {
			return "", err
		}
		return "(" + referenced + ") & (" + siblingType + ")", nil
	}
	if reference, _ := schema["$ref"].(string); reference != "" {
		name, err := componentSchemaReferenceName(reference)
		if err != nil {
			return "", err
		}
		referenced, err := referencedType(document, name, direction, scope)
		if err != nil || len(schema) == 1 {
			return referenced, err
		}
		siblings := make(map[string]any, len(schema)-1)
		for key, value := range schema {
			if key != "$ref" && isTypeAffectingSchemaKeyword(key) {
				siblings[key] = value
			}
		}
		if len(siblings) == 0 {
			return referenced, nil
		}
		siblingType, err := schemaTypeForScope(document, siblings, direction, scope)
		if err != nil {
			return "", err
		}
		return "(" + referenced + ") & (" + siblingType + ")", nil
	}
	// Apply composition and sibling constraints together.
	own := make(map[string]any, len(schema))
	for key, value := range schema {
		own[key] = value
	}
	var composed []string
	for _, keyword := range []string{"oneOf", "anyOf", "allOf"} {
		if variants, ok := schema[keyword].([]any); ok {
			delete(own, keyword)
			parts, err := schemaListTypes(document, variants, direction, scope)
			if err != nil {
				return "", err
			}
			part := strings.Join(uniqueStrings(parts), " | ")
			if keyword == "allOf" {
				part = intersectionType(parts)
			} else if len(parts) == 0 {
				part = "never"
			}
			composed = append(composed, part)
		}
	}
	if len(composed) > 0 {
		base, err := schemaTypeForScope(document, own, direction, scope)
		if err != nil {
			return "", err
		}
		if base != "unknown" {
			composed = append([]string{base}, composed...)
		}
		return intersectionType(composed), nil
	}
	if value, exists := schema["const"]; exists {
		return literalTS(value), nil
	}
	if values, ok := schema["enum"].([]any); ok && len(values) > 0 {
		quoted := make([]string, 0, len(values))
		for _, value := range values {
			quoted = append(quoted, literalTS(value))
		}
		return strings.Join(quoted, " | "), nil
	}

	types := schemaTypes(schema["type"])
	if len(types) > 1 {
		parts := make([]string, 0, len(types))
		for _, value := range types {
			part, err := scalarOrCompositeType(document, value, schema, direction, scope)
			if err != nil {
				return "", err
			}
			parts = append(parts, part)
		}
		return strings.Join(uniqueStrings(parts), " | "), nil
	}
	if len(types) == 0 {
		if required, _ := schema["required"].([]any); len(required) > 0 {
			return objectTypeForScope(document, schema, direction, scope)
		}
		if _, exists := schema["properties"]; exists {
			return objectTypeForScope(document, schema, direction, scope)
		}
		if _, exists := schema["additionalProperties"]; exists {
			return objectTypeForScope(document, schema, direction, scope)
		}
		if _, exists := schema["prefixItems"]; exists {
			return arrayType(document, schema, direction, scope)
		}
		return "unknown", nil
	}
	return scalarOrCompositeType(document, types[0], schema, direction, scope)
}

func isTypeAffectingSchemaKeyword(key string) bool {
	switch key {
	case "type", "const", "enum", "oneOf", "anyOf", "allOf", "properties", "required", "additionalProperties", "items", "prefixItems":
		return true
	default:
		return false
	}
}

func scalarOrCompositeType(document *ir.Document, kind string, schema map[string]any, direction projection, scope typeRenderScope) (string, error) {
	switch kind {
	case "string":
		return "string", nil
	case "integer", "number":
		return "number", nil
	case "boolean":
		return "boolean", nil
	case "null":
		return "null", nil
	case "array":
		return arrayType(document, schema, direction, scope)
	case "object":
		return objectTypeForScope(document, schema, direction, scope)
	default:
		return "unknown", nil
	}
}

func arrayType(document *ir.Document, schema map[string]any, direction projection, scope typeRenderScope) (string, error) {
	if prefixItems, ok := schema["prefixItems"].([]any); ok && len(prefixItems) > 0 {
		parts, err := schemaListTypes(document, prefixItems, direction, scope)
		if err != nil {
			return "", err
		}
		if items, exists := schema["items"]; exists {
			itemType, err := schemaTypeForScope(document, items, direction, scope)
			if err != nil {
				return "", err
			}
			parts = append(parts, "...("+itemType+")[]")
		} else {
			// JSON Schema allows unconstrained trailing items when `items` is
			// absent. Preserve that openness instead of producing a closed tuple.
			parts = append(parts, "...unknown[]")
		}
		return "readonly [" + strings.Join(parts, ", ") + "]", nil
	}
	items, exists := schema["items"]
	if !exists {
		return "readonly unknown[]", nil
	}
	itemType, err := schemaTypeForScope(document, items, direction, scope)
	if err != nil {
		return "", err
	}
	return "readonly (" + itemType + ")[]", nil
}

func objectType(document *ir.Document, schema map[string]any, direction projection) (string, error) {
	return objectTypeForScope(document, schema, direction, typeRenderLocal)
}

func objectTypeForScope(document *ir.Document, schema map[string]any, direction projection, scope typeRenderScope) (string, error) {
	declared, _ := schema["properties"].(map[string]any)
	properties := make(map[string]any, len(declared))
	for name, value := range declared {
		properties[name] = value
	}
	if values, ok := schema["required"].([]any); ok {
		for _, value := range values {
			if name, ok := value.(string); ok {
				if _, exists := properties[name]; !exists {
					properties[name] = true
				}
			}
		}
	}
	if len(properties) == 0 {
		dynamic, err := objectAdditionalType(document, schema, direction, scope)
		if err != nil {
			return "", err
		}
		if dynamic != "" {
			return "Readonly<Record<string, " + dynamic + ">>", nil
		}
		if additional, ok := schema["additionalProperties"].(bool); ok && !additional {
			return "Readonly<Record<string, never>>", nil
		}
		return "Readonly<Record<string, unknown>>", nil
	}
	required := make(map[string]bool)
	if values, ok := schema["required"].([]any); ok {
		for _, value := range values {
			if name, ok := value.(string); ok {
				required[name] = true
			}
		}
	}
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var output bytes.Buffer
	propertyIndexTypes := make([]string, 0, len(keys))
	output.WriteString("{\n")
	for _, wireName := range keys {
		propertyValue := properties[wireName]
		propertySchema, _ := propertyValue.(map[string]any)
		if direction == projectionInput && boolValue(propertySchema, "readOnly") {
			continue
		}
		if direction == projectionOutput && boolValue(propertySchema, "writeOnly") {
			continue
		}
		propertyType, err := schemaTypeForScope(document, propertyValue, direction, scope)
		if err != nil {
			return "", err
		}
		// A required-only constraint must also remove undefined contributed by
		// an optional input property in another intersection operand.
		if _, declaredHere := declared[wireName]; !declaredHere {
			propertyType = "{} | null"
		}
		propertyName := quoteTS(wireName)
		optional := ""
		if !required[wireName] {
			optional = "?"
			if direction == projectionInput {
				propertyType += " | undefined"
			}
		}
		propertyIndexTypes = append(propertyIndexTypes, propertyType)
		emitSchemaValueJSDoc(&output, document, "  ", propertySchema, "OpenAPI property `"+sanitizeComment(wireName)+"`.")
		fmt.Fprintf(&output, "  readonly %s%s: %s\n", propertyName, optional, propertyType)
	}
	output.WriteString("}")
	additional, err := objectAdditionalType(document, schema, direction, scope)
	if err != nil {
		return "", err
	}
	if additional == "" {
		return output.String(), nil
	}
	indexType := typeUnion(append([]string{additional}, propertyIndexTypes...))
	return "(" + output.String() + ") & (Readonly<Record<string, " + indexType + ">>)", nil
}

// objectAdditionalType is intentionally conservative for patternProperties:
// TypeScript has no regex-key type, so every additional key is represented by
// the union of the applicable pattern schemas. The exact patterns stay in the
// generated documentation/contract metadata.
func objectAdditionalType(document *ir.Document, schema map[string]any, direction projection, scope typeRenderScope) (string, error) {
	var values []string
	additional, hasAdditional := schema["additionalProperties"]
	if hasAdditional {
		if boolean, ok := additional.(bool); !ok || boolean {
			value, err := schemaTypeForScope(document, additional, direction, scope)
			if err != nil {
				return "", err
			}
			values = append(values, value)
		}
	}
	patterns, _ := schema["patternProperties"].(map[string]any)
	if len(patterns) > 0 && !hasAdditional {
		// JSON Schema permits keys that match no pattern when additionalProperties
		// is omitted. TypeScript cannot express regex-key constraints, so unknown
		// is the only non-rejecting index approximation for that open key space.
		return "unknown", nil
	}
	patternNames := make([]string, 0, len(patterns))
	for pattern := range patterns {
		patternNames = append(patternNames, pattern)
	}
	sort.Strings(patternNames)
	for _, pattern := range patternNames {
		typeValue, err := schemaTypeForScope(document, patterns[pattern], direction, scope)
		if err != nil {
			return "", err
		}
		values = append(values, typeValue)
	}
	return typeUnion(values), nil
}

func referencedType(document *ir.Document, name string, direction projection, scope typeRenderScope) (string, error) {
	_ = document
	if scope.componentReference != nil {
		return scope.componentReference(name, direction), nil
	}
	return componentProjectionTypeExpression(name, direction).render(scope), nil
}

func isSuccessResponseStatus(status string) bool {
	return status == "default" || strings.HasPrefix(status, "2")
}

func operationOutputType(document *ir.Document, operation ir.Operation) (string, error) {
	return operationOutputTypeForScope(document, operation, typeRenderLocal)
}

func operationOutputTypeExpression(document *ir.Document, operation ir.Operation) (typeExpression, error) {
	local, err := operationOutputTypeForScope(document, operation, typeRenderLocal)
	if err != nil {
		return typeExpression{}, err
	}
	contract, err := operationOutputTypeForScope(document, operation, typeRenderContract)
	if err != nil {
		return typeExpression{}, err
	}
	return scopedTypeExpression(local, contract), nil
}

func operationOutputTypeForScope(document *ir.Document, operation ir.Operation, scope typeRenderScope) (string, error) {
	sets, err := operationResponseMediaSets(document, operation)
	if err != nil {
		return "", err
	}
	responses := sets.normal
	var result []string
	for _, response := range responses {
		if !isSuccessResponseStatus(response.Status) {
			continue
		}
		if len(response.Content) == 0 {
			result = append(result, "void")
			continue
		}
		for _, media := range response.Content {
			if _, hasSchema := media.Raw["schema"]; !hasSchema {
				// A Media Type Object without a Schema Object still has a body.
				// Its shape is unconstrained, not absent.
				result = append(result, "unknown")
				continue
			}
			if media.Schema == false {
				result = append(result, "never")
				continue
			}
			schema, _ := media.Schema.(map[string]any)
			if !media.Stream.IsStreaming() && isBinaryMedia(media.ContentType, schema) {
				result = append(result, "ReadableStream<Uint8Array>")
				continue
			}
			if !media.Stream.IsStreaming() && isTextMedia(media.ContentType) {
				result = append(result, "string")
				continue
			}
			if operation.Envelope == "data" {
				if dataSchema := envelopeDataSchema(document, schema, make(map[string]bool)); len(dataSchema) > 0 {
					schema = dataSchema
				}
			}
			valueType, err := schemaTypeForScope(document, schema, projectionOutput, scope)
			if err != nil {
				return "", err
			}
			result = append(result, valueType)
		}
	}
	if len(result) == 0 {
		return "void", nil
	}
	return strings.Join(uniqueStrings(result), " | "), nil
}

func operationRawResponseType(document *ir.Document, operation ir.Operation) (string, error) {
	return operationRawResponseTypeForScope(document, operation, typeRenderLocal)
}

func operationRawResponseTypeExpression(document *ir.Document, operation ir.Operation) (typeExpression, error) {
	local, err := operationRawResponseTypeForScope(document, operation, typeRenderLocal)
	if err != nil {
		return typeExpression{}, err
	}
	contract, err := operationRawResponseTypeForScope(document, operation, typeRenderContract)
	if err != nil {
		return typeExpression{}, err
	}
	return scopedTypeExpression(local, contract), nil
}

func operationRawResponseTypeForScope(document *ir.Document, operation ir.Operation, scope typeRenderScope) (string, error) {
	responses, err := operationResponses(document, operation)
	if err != nil {
		return "", err
	}
	var result []string
	for _, response := range responses {
		if !isSuccessResponseStatus(response.Status) {
			continue
		}
		statusType := "number"
		status := response.Status
		if len(status) == 3 && status[0] >= '0' && status[0] <= '9' && status[1] >= '0' && status[1] <= '9' && status[2] >= '0' && status[2] <= '9' {
			statusType = status
		}
		headerType, err := responseHeaderType(document, response.Raw, scope)
		if err != nil {
			return "", err
		}
		if len(response.Content) == 0 {
			result = append(result, "RawResponseFor<"+statusType+", undefined, void, "+headerType+">")
			continue
		}
		for _, media := range response.Content {
			schemaObject, _ := media.Schema.(map[string]any)
			_, hasItemSchema := media.Raw["itemSchema"]
			valueType := "void"
			if !hasItemSchema && media.Schema != nil {
				if media.Schema == false {
					valueType = "never"
				} else if !media.Stream.IsStreaming() && isBinaryMedia(media.ContentType, schemaObject) {
					valueType = "ReadableStream<Uint8Array>"
				} else if !media.Stream.IsStreaming() && isTextMedia(media.ContentType) {
					valueType = "string"
				} else {
					valueType, err = schemaTypeForScope(document, media.Schema, projectionOutput, scope)
					if err != nil {
						return "", err
					}
				}
			}
			result = append(result, "RawResponseFor<"+statusType+", "+quoteTS(media.ContentType)+", "+valueType+", "+headerType+">")
		}
	}
	if len(result) == 0 {
		return "RawResponseFor<number, string | undefined, void>", nil
	}
	return strings.Join(uniqueStrings(result), " | "), nil
}

func responseHeaderType(document *ir.Document, response map[string]any, scope typeRenderScope) (string, error) {
	headers, _ := response["headers"].(map[string]any)
	if len(headers) == 0 {
		return "Readonly<Record<string, never>>", nil
	}
	names := sortedAnyKeys(headers)
	fields := make([]string, 0, len(names))
	for _, name := range names {
		header, _ := headers[name].(map[string]any)
		resolved, err := resolveComponentObject(document, header, "headers")
		if err != nil {
			return "", err
		}
		schema, _, err := responseHeaderSchema(document, resolved)
		if err != nil {
			return "", err
		}
		valueType, err := schemaTypeForScope(document, schema, projectionOutput, scope)
		if err != nil {
			return "", err
		}
		optional := "?"
		if boolValue(resolved, "required") {
			optional = ""
		}
		field := "readonly " + quoteTS(name) + optional + ": " + valueType
		if boolValue(resolved, "deprecated") {
			field = "/** @deprecated This OpenAPI response header is deprecated. */ " + field
		}
		fields = append(fields, field)
	}
	return "{ " + strings.Join(fields, "; ") + " }", nil
}

func responseHeaderSchema(document *ir.Document, header map[string]any) (any, string, error) {
	content, _ := header["content"].(map[string]any)
	if len(content) == 0 {
		return header["schema"], "", nil
	}
	if len(content) != 1 {
		return nil, "", fmt.Errorf("Header Object content must define exactly one media type")
	}
	mediaType := sortedAnyKeys(content)[0]
	media, _ := content[mediaType].(map[string]any)
	media, err := resolveMediaTypeObject(document, media)
	if err != nil {
		return nil, "", err
	}
	return media["schema"], mediaType, nil
}

func operationMediaOutputTypes(document *ir.Document, operation ir.Operation) (map[string]string, error) {
	return operationMediaOutputTypesForScope(document, operation, typeRenderLocal)
}

func operationMediaOutputTypeExpressions(document *ir.Document, operation ir.Operation) (map[string]typeExpression, error) {
	local, err := operationMediaOutputTypesForScope(document, operation, typeRenderLocal)
	if err != nil {
		return nil, err
	}
	contract, err := operationMediaOutputTypesForScope(document, operation, typeRenderContract)
	if err != nil {
		return nil, err
	}
	result := make(map[string]typeExpression, len(local))
	for mediaType, localType := range local {
		result[mediaType] = scopedTypeExpression(localType, contract[mediaType])
	}
	return result, nil
}

func operationMediaOutputTypesForScope(document *ir.Document, operation ir.Operation, scope typeRenderScope) (map[string]string, error) {
	sets, err := operationResponseMediaSets(document, operation)
	if err != nil {
		return nil, err
	}
	responses := sets.normal
	byMedia := make(map[string][]string)
	for _, response := range responses {
		if !isSuccessResponseStatus(response.Status) {
			continue
		}
		for _, media := range response.Content {
			schemaObject, _ := media.Schema.(map[string]any)
			valueType := "void"
			if media.Schema != nil {
				schema := media.Schema
				if schema == false {
					valueType = "never"
				} else if !media.Stream.IsStreaming() && isBinaryMedia(media.ContentType, schemaObject) {
					valueType = "ReadableStream<Uint8Array>"
				} else if !media.Stream.IsStreaming() && isTextMedia(media.ContentType) {
					valueType = "string"
				} else {
					if operation.Envelope == "data" {
						if dataSchema := envelopeDataSchema(document, schemaObject, make(map[string]bool)); len(dataSchema) > 0 {
							schema = dataSchema
						}
					}
					valueType, err = schemaTypeForScope(document, schema, projectionOutput, scope)
					if err != nil {
						return nil, err
					}
				}
			}
			byMedia[media.ContentType] = append(byMedia[media.ContentType], valueType)
		}
	}
	result := make(map[string]string, len(byMedia))
	for mediaType, values := range byMedia {
		result[mediaType] = strings.Join(uniqueStrings(values), " | ")
	}
	return result, nil
}

func isBinaryMedia(mediaType string, schema map[string]any) bool {
	mediaType = strings.ToLower(mediaType)
	if strings.HasPrefix(mediaType, "text/") || isJSONMediaType(mediaType) || strings.Contains(mediaType, "xml") {
		return false
	}
	format, _ := schema["format"].(string)
	contentEncoding, _ := schema["contentEncoding"].(string)
	return format == "binary" || contentEncoding == "binary" || mediaType == "application/octet-stream"
}

func isTextMedia(mediaType string) bool {
	mediaType = strings.ToLower(mediaType)
	return strings.HasPrefix(mediaType, "text/") && !strings.Contains(mediaType, "xml")
}

func isJSONMediaType(mediaType string) bool {
	mediaType = strings.TrimSpace(strings.SplitN(strings.ToLower(mediaType), ";", 2)[0])
	return mediaType == "application/json" || strings.HasSuffix(mediaType, "+json")
}

func envelopeDataSchema(document *ir.Document, schema map[string]any, seen map[string]bool) map[string]any {
	if reference, _ := schema["$ref"].(string); reference != "" {
		name, err := componentSchemaReferenceName(reference)
		if err != nil {
			return nil
		}
		if seen[name] {
			return nil
		}
		seen[name] = true
		return envelopeDataSchema(document, document.ComponentSchemas[name], seen)
	}
	properties, _ := schema["properties"].(map[string]any)
	if data, ok := properties["data"].(map[string]any); ok {
		return data
	}
	for _, keyword := range []string{"allOf", "oneOf", "anyOf"} {
		variants, _ := schema[keyword].([]any)
		if len(variants) == 0 {
			continue
		}
		dataSchemas := make([]any, 0, len(variants))
		for _, variant := range variants {
			item, _ := variant.(map[string]any)
			data := envelopeDataSchema(document, item, copyStringBoolMap(seen))
			if len(data) == 0 {
				if keyword == "allOf" {
					continue
				}
				return nil
			}
			dataSchemas = append(dataSchemas, data)
		}
		if len(dataSchemas) == 1 {
			data, _ := dataSchemas[0].(map[string]any)
			return data
		}
		if len(dataSchemas) > 1 {
			return map[string]any{keyword: dataSchemas}
		}
	}
	return nil
}

func copyStringBoolMap(source map[string]bool) map[string]bool {
	result := make(map[string]bool, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func operationSuccessSchema(document *ir.Document, operation ir.Operation) (map[string]any, bool, error) {
	responses, err := operationResponses(document, operation)
	if err != nil {
		return nil, false, err
	}
	for _, response := range responses {
		if !isSuccessResponseStatus(response.Status) {
			continue
		}
		for _, media := range response.Content {
			schema, _ := media.Schema.(map[string]any)
			if len(schema) == 0 {
				continue
			}
			return schema, true, nil
		}
		return nil, false, nil
	}
	return nil, false, nil
}

func operationItemType(document *ir.Document, operation ir.Operation) (string, error) {
	return operationItemTypeForScope(document, operation, typeRenderLocal)
}

func operationItemTypeForScope(document *ir.Document, operation ir.Operation, scope typeRenderScope) (string, error) {
	if operation.PaginationPlan != nil && len(operation.PaginationPlan.ItemSchema) > 0 {
		return schemaTypeForScope(document, operation.PaginationPlan.ItemSchema, projectionOutput, scope)
	}
	schema, found, err := operationSuccessSchema(document, operation)
	if err != nil {
		return "", err
	}
	if !found {
		return "unknown", nil
	}
	items := findItemsSchema(document, schema, make(map[string]bool))
	if len(items) == 0 {
		return "unknown", nil
	}
	return schemaTypeForScope(document, items, projectionOutput, scope)
}

func findItemsSchema(document *ir.Document, schema map[string]any, seen map[string]bool) map[string]any {
	if reference, _ := schema["$ref"].(string); reference != "" {
		name, err := componentSchemaReferenceName(reference)
		if err != nil {
			return nil
		}
		if seen[name] {
			return nil
		}
		seen[name] = true
		return findItemsSchema(document, document.ComponentSchemas[name], seen)
	}
	properties, _ := schema["properties"].(map[string]any)
	if itemsProperty, _ := properties["items"].(map[string]any); len(itemsProperty) > 0 {
		if items, _ := itemsProperty["items"].(map[string]any); len(items) > 0 {
			return items
		}
	}
	for _, key := range []string{"data", "result"} {
		if nested, _ := properties[key].(map[string]any); len(nested) > 0 {
			if result := findItemsSchema(document, nested, seen); len(result) > 0 {
				return result
			}
		}
	}
	return nil
}

func operationInputTypes(document *ir.Document, operation ir.Operation) ([]string, error) {
	prepared, err := prepareOperation(document, operation)
	if err != nil {
		return nil, err
	}
	return operationInputTypesFromPrepared(document, operation, prepared)
}

func operationInputTypesFromPrepared(document *ir.Document, operation ir.Operation, prepared preparedOperation) ([]string, error) {
	var result []string
	name := operationTypeName(operationRouteKey(operation))
	if len(prepared.clientParametersByLocation["path"]) > 0 {
		result = append(result, name+"PathInput")
	}
	if len(prepared.clientParametersByLocation["query"]) > 0 || operation.Pagination != "" || len(operation.SortParameters) > 0 {
		result = append(result, name+"QueryInput")
	}
	if len(prepared.clientParametersByLocation["querystring"]) > 0 {
		result = append(result, name+"QuerystringInput")
	}
	if len(prepared.clientParametersByLocation["header"]) > 0 {
		result = append(result, name+"HeaderInput")
	}
	if len(prepared.clientParametersByLocation["cookie"]) > 0 {
		result = append(result, name+"CookieInput")
	}
	body, err := operationRequestBody(document, operation)
	if err != nil {
		return nil, err
	}
	if body != nil {
		result = append(result, name+"BodyInput")
	}
	return result, nil
}

func schemaTypes(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []any:
		result := make([]string, 0, len(typed))
		for _, value := range typed {
			if text, ok := value.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

func addUnionMember(value, member string) string {
	for _, item := range strings.Split(value, " | ") {
		if item == member {
			return value
		}
	}
	return value + " | " + member
}

func schemaEnum(schema map[string]any) []string {
	values, _ := schema["enum"].([]any)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func intersectionType(parts []string) string {
	if len(parts) == 0 {
		return "unknown"
	}
	if len(parts) == 1 {
		return parts[0]
	}
	wrapped := make([]string, len(parts))
	for index, part := range parts {
		wrapped[index] = "(" + part + ")"
	}
	return strings.Join(wrapped, " & ")
}

func schemaListTypes(document *ir.Document, variants []any, direction projection, scope typeRenderScope) ([]string, error) {
	parts := make([]string, 0, len(variants))
	for _, variant := range variants {
		part, err := schemaTypeForScope(document, variant, direction, scope)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	return parts, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func typeUnion(values []string) string {
	values = uniqueStrings(values)
	for _, value := range values {
		if value == "unknown" {
			return "unknown"
		}
	}
	return strings.Join(values, " | ")
}

func literalTS(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "unknown"
	}
	return string(data)
}

func quoteTS(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func sanitizeComment(value string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(value), " "), "*/", "* /")
}
