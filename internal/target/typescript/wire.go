package typescript

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	schemaemit "openapi-sdkgen/internal/target/typescript/schema/emit"
	schemaplan "openapi-sdkgen/internal/target/typescript/schema/plan"
)

func (wire *wireRenderContext) emitWireComponents(output *bytes.Buffer, document *ir.Document, name string, direction projection) error {
	all := make(map[string]bool, len(document.ComponentSchemas)+len(document.Schemas))
	for schemaName := range document.ComponentSchemas {
		all[schemaName] = true
	}
	for schemaName := range document.Schemas {
		all[schemaName] = true
	}
	names := make([]string, 0, len(all))
	for schemaName := range all {
		names = append(names, schemaName)
	}
	sort.Strings(names)
	properties := make([]runtimeProperty, 0, len(names))
	for _, schemaName := range names {
		if wire.componentNames != nil && !wire.componentNames[direction][schemaName] {
			continue
		}
		value := any(document.ComponentSchemas[schemaName])
		if schema, ok := document.Schemas[schemaName]; ok {
			value = schema.Value
		}
		descriptor, err := wire.wireSchemaDescriptorForDocument(document, value, direction)
		if err != nil {
			return fmt.Errorf("component %s wire schema: %w", schemaName, err)
		}
		properties = append(properties, runtimeProperty{key: schemaName, value: descriptor})
	}
	fmt.Fprintf(output, "const %s: WireSchemas = %s\n\n", name, runtimeObjectExpression(properties))
	return nil
}

func (wire *wireRenderContext) wireSchemaDescriptor(value any, direction projection) (string, error) {
	return wire.wireSchemaDescriptorScoped(value, direction, schemaRequiresFormatAssertion(value), false)
}

func (wire *wireRenderContext) wireSchemaDescriptorForDocument(document *ir.Document, value any, direction projection) (string, error) {
	legacyNullable := document != nil && document.OpenAPIVersionLine == "3.0"
	return wire.wireSchemaDescriptorScoped(value, direction, schemaRequiresFormatAssertion(value), legacyNullable)
}

func (wire *wireRenderContext) wireMediaSchemaDescriptorForDocument(document *ir.Document, value any, direction projection, mediaType string) (string, error) {
	legacy := document != nil && document.OpenAPIVersionLine == "3.0"
	ignore := mediaRootContentMediaTypeConflicts(document, value, mediaType)
	return wire.lowerSchemaDescriptor(value, direction, schemaRequiresFormatAssertion(value), legacy, ignore)
}

func mediaRootContentMediaTypeConflicts(document *ir.Document, value any, mediaType string) bool {
	if document == nil || (document.OpenAPIVersionLine != "3.1" && document.OpenAPIVersionLine != "3.2") {
		return false
	}
	schema, ok := value.(map[string]any)
	if !ok {
		return false
	}
	contentMediaType, _ := schema["contentMediaType"].(string)
	if contentMediaType == "" {
		resolved := resolveSchemaReference(document, schema, make(map[string]bool))
		contentMediaType, _ = resolved["contentMediaType"].(string)
	}
	return contentMediaType != "" && !declaredMediaTypeMatches(mediaType, contentMediaType)
}

func declaredMediaTypeMatches(declared, nested string) bool {
	declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
	nested = strings.ToLower(strings.TrimSpace(strings.Split(nested, ";")[0]))
	if declared == nested {
		return true
	}
	parts := strings.SplitN(declared, "/", 2)
	nestedParts := strings.SplitN(nested, "/", 2)
	if len(parts) != 2 || len(nestedParts) != 2 {
		return false
	}
	if parts[0] != "*" && parts[0] != nestedParts[0] {
		return false
	}
	if parts[1] == "*" {
		return true
	}
	if strings.HasPrefix(parts[1], "*+") {
		return strings.HasSuffix(nestedParts[1], strings.TrimPrefix(parts[1], "*"))
	}
	return false
}

const formatAssertionVocabulary = schemaplan.FormatAssertionVocabulary

func schemaRequiresFormatAssertion(value any) bool { return schemaplan.RequiresFormatAssertion(value) }

type schemaPlanObserver struct{ wire *wireRenderContext }

func (observer schemaPlanObserver) Field(name string) { observer.wire.recordRuntimeSchemaField(name) }
func (observer schemaPlanObserver) Value(kind, value string) {
	observer.wire.recordRuntimeSchemaValue(kind, value)
}
func (observer schemaPlanObserver) Reference(name string, direction schemaplan.Projection) {
	observer.wire.recordExecutionReference(name, projection(direction))
}
func (observer schemaPlanObserver) Dynamic() {
	observer.wire.recordExecutionCapability(executionSchemaDynamic)
}
func (observer schemaPlanObserver) ContentMedia(media string) {
	observer.wire.recordRuntimeMedia("schema.content", media, ir.StreamFramingNone, false)
	if executionXMLMedia(media) {
		observer.wire.recordExecutionCapability(executionSchemaXML)
	}
}
func (wire *wireRenderContext) wireSchemaDescriptorScoped(value any, direction projection, formatAssertion, legacyNullable bool) (string, error) {
	return wire.lowerSchemaDescriptor(value, direction, formatAssertion, legacyNullable, false)
}
func (wire *wireRenderContext) lowerSchemaDescriptor(value any, direction projection, formatAssertion, legacyNullable, ignore bool) (string, error) {
	var node *schemaplan.Node
	var err error
	if wire.schemaPrograms != nil {
		node, err = wire.schemaPrograms.lower(value, direction, formatAssertion, legacyNullable, ignore, schemaPlanObserver{wire})
	} else {
		node, err = schemaplan.Lower(value, schemaplan.Options{Projection: schemaplan.Projection(direction), FormatAssertion: formatAssertion, LegacyNullable: legacyNullable, ReferenceName: componentSchemaReferenceName, Observer: schemaPlanObserver{wire}})
		if err == nil && ignore {
			node.Fields = append(node.Fields, schemaplan.Field{Name: "ignoreContentMediaType", Value: schemaplan.Literal{Data: true}})
		}
	}
	if err != nil {
		return "", err
	}
	if wire.trackSchemas {
		wire.schemaNodes = append(wire.schemaNodes, node)
	}
	if wire.semanticOnly {
		return "{}", nil
	}
	return wire.emitSchemaDescriptor(node)
}
func (wire *wireRenderContext) emitSchemaDescriptor(node *schemaplan.Node) (string, error) {
	var module *schemaProgramBinding
	var err error
	if wire.schemaPrograms != nil {
		module, err = wire.schemaPrograms.moduleFor(node)
		if err != nil {
			return "", err
		}
		shared, err := wire.schemaPrograms.sharedDescriptor(node, wire.properties, module)
		if err != nil {
			return "", err
		}
		if shared != nil {
			return wire.importDescriptor(shared)
		}
		if descriptor := module.descriptors[wire.properties]; descriptor != nil {
			return wire.renderDescriptor(descriptor)
		}
		if wire.schemaPrograms.sealed {
			return "", fmt.Errorf("unprepared schema descriptor mode %d", wire.properties)
		}
		descriptor, err := wire.schemaPrograms.prepareDescriptor(node, wire.properties, module)
		if err != nil {
			return "", err
		}
		module.descriptors[wire.properties] = descriptor
		return wire.renderDescriptor(descriptor)

	}
	return schemaemit.Descriptor(node, schemaemit.DescriptorOptions{Literal: schemaDescriptorLiteral, Properties: func(entries []schemaemit.PropertyExpression) (string, error) {
		properties := make([]runtimeProperty, 0, len(entries))
		for _, entry := range entries {
			properties = append(properties, runtimeProperty{key: entry.Name, value: entry.Expression})
		}
		return wire.propertyExpression(properties)
	}})

}

func schemaDescriptorLiteral(value any) (string, error) {
	switch typed := value.(type) {
	case []string:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			items = append(items, quoteTS(item))
		}
		return "[" + strings.Join(items, ", ") + "]", nil
	case map[string][]string:
		properties := make([]runtimeProperty, 0, len(typed))
		for key, items := range typed {
			source, err := schemaDescriptorLiteral(items)
			if err != nil {
				return "", err
			}
			properties = append(properties, runtimeProperty{key: key, value: source})
		}
		return runtimeObjectExpression(properties), nil
	default:
		return runtimeJSONExpression(value)
	}
}

func mergeWireSchemaDescriptors(reference, sibling string) string {
	if sibling == "{}" {
		return reference
	}
	reference = strings.TrimSuffix(strings.TrimPrefix(reference, "{ "), " }")
	sibling = strings.TrimSuffix(strings.TrimPrefix(sibling, "{ "), " }")
	if sibling == "" {
		return "{ " + reference + " }"
	}
	return "{ " + reference + ", " + sibling + " }"
}

func normalizedDiscriminatorReference(reference string) string {
	if strings.HasPrefix(reference, "#") || strings.Contains(reference, ":") || strings.HasPrefix(reference, "./") || strings.HasPrefix(reference, "../") {
		return reference
	}
	return "#/components/schemas/" + strings.ReplaceAll(strings.ReplaceAll(reference, "~", "~0"), "/", "~1")
}

func (wire *wireRenderContext) operationRequestWireBodies(document *ir.Document, operation ir.Operation) (string, bool, error) {
	body, err := operationRequestBody(document, operation)
	if err != nil {
		return "", false, err
	}
	if body == nil {
		return "", false, nil
	}
	entries := make([]string, 0, len(body.Content))
	for _, media := range body.Content {
		wire.recordRuntimeMedia("request", media.ContentType, media.Stream.Framing, media.ItemSchema != nil)
		schemaObject, _ := media.Schema.(map[string]any)
		booleanSchema, isBooleanSchema := media.Schema.(bool)
		schemaIsFalse := isBooleanSchema && !booleanSchema
		descriptor := "{}"
		if !schemaIsFalse && isBinaryMediaForDocument(document, media.ContentType, schemaObject) {
			descriptor, err = wire.wireSchemaDescriptorForDocument(document, nil, projectionInput)
			if err != nil {
				return "", false, err
			}
		}
		if schemaIsFalse || !isBinaryMediaForDocument(document, media.ContentType, schemaObject) {
			var err error
			descriptor, err = wire.wireMediaSchemaDescriptorForDocument(document, multipartInputSchema(document, media.ContentType, media.Schema, media.Raw), projectionInput, media.ContentType)
			if err != nil {
				return "", false, err
			}
		}
		entry := "{ contentType: " + quoteTS(media.ContentType) + ", schema: " + descriptor
		if !schemaIsFalse && !media.Stream.IsStreaming() && isBinaryMediaForDocument(document, media.ContentType, schemaObject) {
			entry += ", binary: true"
		}
		if _, exists := media.Raw["schema"]; exists {
			entry += ", schemaDeclared: true"
		}
		if media.Stream.IsStreaming() {
			entry += ", streamFraming: " + quoteTS(string(media.Stream.Framing))
		}
		if _, exists := media.Raw["itemSchema"]; exists {
			itemDescriptor, err := wire.wireSchemaDescriptorForDocument(document, multipartItemInputSchema(document, media.ContentType, media.ItemSchema, media.Raw["itemEncoding"]), projectionInput)
			if err != nil {
				return "", false, err
			}
			entry += ", itemSchema: " + itemDescriptor
		}
		encodings, err := wire.requestBodyWireEncodings(document, media.Raw)
		if err != nil {
			return "", false, err
		}
		if encodings != "" {
			entry += ", encoding: " + encodings
		}
		prefixEncoding, err := wire.positionalMultipartWireEncodings(document, media.Raw["prefixEncoding"])
		if err != nil {
			return "", false, err
		}
		if prefixEncoding != "" {
			entry += ", prefixEncoding: " + prefixEncoding
		}
		itemEncoding, err := wire.positionalMultipartWireEncoding(document, media.Raw["itemEncoding"])
		if err != nil {
			return "", false, err
		}
		if itemEncoding != "" {
			entry += ", itemEncoding: " + itemEncoding
		}
		entries = append(entries, entry+" }")
	}
	return "[" + strings.Join(entries, ", ") + "]", len(entries) > 0, nil
}

func (wire *wireRenderContext) requestBodyWireEncodings(document *ir.Document, media map[string]any) (string, error) {
	values, _ := media["encoding"].(map[string]any)
	if len(values) == 0 {
		return "", nil
	}
	names := sortedAnyKeys(values)
	entries := make([]string, 0, len(names))
	for _, name := range names {
		value, _ := values[name].(map[string]any)
		entry, err := wire.multipartWireEncoding(document, value, name, projectionInput)
		if err != nil {
			return "", err
		}
		entries = append(entries, entry)
	}
	return "[" + strings.Join(entries, ", ") + "]", nil
}

func (wire *wireRenderContext) positionalMultipartWireEncodings(document *ir.Document, value any) (string, error) {
	values, _ := value.([]any)
	if len(values) == 0 {
		return "", nil
	}
	entries := make([]string, 0, len(values))
	for _, item := range values {
		encoding, _ := item.(map[string]any)
		entry, err := wire.multipartWireEncoding(document, encoding, "", projectionInput)
		if err != nil {
			return "", err
		}
		entries = append(entries, entry)
	}
	return "[" + strings.Join(entries, ", ") + "]", nil
}

func (wire *wireRenderContext) positionalMultipartWireEncoding(document *ir.Document, value any) (string, error) {
	encoding, _ := value.(map[string]any)
	if len(encoding) == 0 {
		return "", nil
	}
	return wire.multipartWireEncoding(document, encoding, "", projectionInput)
}

func (wire *wireRenderContext) multipartWireEncoding(document *ir.Document, value map[string]any, name string, direction projection) (string, error) {
	fields := make([]string, 0, 9)
	if name != "" {
		fields = append(fields, "name: "+quoteTS(name))
	}
	if contentType, _ := value["contentType"].(string); contentType != "" {
		wire.recordRuntimeMedia("part", contentType, ir.StreamFramingNone, false)
		fields = append(fields, "contentType: "+quoteTS(contentType))
	}
	if style, _ := value["style"].(string); style != "" {
		fields = append(fields, "style: "+quoteTS(style))
	}
	if explode, exists := value["explode"].(bool); exists {
		fields = append(fields, fmt.Sprintf("explode: %t", explode))
	}
	if allowReserved, exists := value["allowReserved"].(bool); exists {
		fields = append(fields, fmt.Sprintf("allowReserved: %t", allowReserved))
	}
	headers, err := wire.multipartWireHeaders(document, value["headers"], direction)
	if err != nil {
		return "", err
	}
	if headers != "" {
		fields = append(fields, "headers: "+headers)
	}
	if nested, err := wire.nestedMultipartWireEncodings(document, value["encoding"], direction); err != nil {
		return "", err
	} else if nested != "" {
		fields = append(fields, "encoding: "+nested)
	}
	if nested, err := wire.positionalMultipartWireEncodingsForDirection(document, value["prefixEncoding"], direction); err != nil {
		return "", err
	} else if nested != "" {
		fields = append(fields, "prefixEncoding: "+nested)
	}
	if nested, err := wire.positionalMultipartWireEncodingForDirection(document, value["itemEncoding"], direction); err != nil {
		return "", err
	} else if nested != "" {
		fields = append(fields, "itemEncoding: "+nested)
	}
	return "{ " + strings.Join(fields, ", ") + " }", nil
}

func (wire *wireRenderContext) nestedMultipartWireEncodings(document *ir.Document, value any, direction projection) (string, error) {
	values, _ := value.(map[string]any)
	if len(values) == 0 {
		return "", nil
	}
	names := sortedAnyKeys(values)
	entries := make([]string, 0, len(names))
	for _, name := range names {
		encoding, _ := values[name].(map[string]any)
		entry, err := wire.multipartWireEncoding(document, encoding, name, direction)
		if err != nil {
			return "", err
		}
		entries = append(entries, entry)
	}
	return "[" + strings.Join(entries, ", ") + "]", nil
}

func (wire *wireRenderContext) multipartWireHeaders(document *ir.Document, value any, direction projection) (string, error) {
	headers, _ := value.(map[string]any)
	if len(headers) == 0 {
		return "", nil
	}
	names := sortedAnyKeys(headers)
	entries := make([]string, 0, len(names))
	for _, name := range names {
		header, _ := headers[name].(map[string]any)
		header, err := resolveComponentObject(document, header, "headers")
		if err != nil {
			return "", err
		}
		schema, contentType, err := responseHeaderSchema(document, header)
		if err != nil {
			return "", err
		}
		descriptor, err := wire.wireSchemaDescriptorForDocument(document, schema, direction)
		if err != nil {
			return "", err
		}
		style, _ := header["style"].(string)
		if style == "" {
			style = "simple"
		}
		explode, hasExplode := header["explode"].(bool)
		if !hasExplode {
			explode = style == "form"
		}
		fields := []string{"name: " + quoteTS(name), "style: " + quoteTS(style), fmt.Sprintf("explode: %t", explode), "schema: " + descriptor}
		if boolValue(header, "required") {
			fields = append(fields, "required: true")
		}
		if contentType != "" {
			wire.recordRuntimeMedia("part.header", contentType, ir.StreamFramingNone, false)
			fields = append(fields, "contentType: "+quoteTS(contentType))
		}
		entries = append(entries, "{ "+strings.Join(fields, ", ")+" }")
	}
	return "[" + strings.Join(entries, ", ") + "]", nil
}

func (wire *wireRenderContext) operationResponseWireBodies(document *ir.Document, operation ir.Operation) (string, bool, error) {
	responses, err := operationResponses(document, operation)
	if err != nil {
		return "", false, err
	}
	var entries []string
	for _, response := range responses {
		headers, err := wire.responseWireHeaders(document, response.Raw)
		if err != nil {
			return "", false, err
		}
		if len(response.Content) == 0 {
			descriptor, err := wire.wireSchemaDescriptorForDocument(document, nil, projectionOutput)
			if err != nil {
				return "", false, err
			}
			entry := "{ status: " + quoteTS(response.Status) + ", contentType: \"\", schema: " + descriptor
			if headers != "" {
				entry += ", headers: " + headers
			}
			entries = append(entries, entry+" }")
		}
		for _, media := range response.Content {
			wire.recordRuntimeMedia("response", media.ContentType, media.Stream.Framing, media.ItemSchema != nil)
			schemaObject, _ := media.Schema.(map[string]any)
			booleanSchema, isBooleanSchema := media.Schema.(bool)
			schemaIsFalse := isBooleanSchema && !booleanSchema
			descriptor := "{}"
			if !schemaIsFalse && isBinaryMediaForDocument(document, media.ContentType, schemaObject) {
				descriptor, err = wire.wireSchemaDescriptorForDocument(document, nil, projectionOutput)
				if err != nil {
					return "", false, err
				}
			}
			if schemaIsFalse || !isBinaryMediaForDocument(document, media.ContentType, schemaObject) {
				descriptor, err = wire.wireMediaSchemaDescriptorForDocument(document, media.Schema, projectionOutput, media.ContentType)
				if err != nil {
					return "", false, err
				}
			}
			entry := "{ status: " + quoteTS(response.Status) + ", contentType: " + quoteTS(media.ContentType) + ", schema: " + descriptor
			if !schemaIsFalse && !media.Stream.IsStreaming() && isBinaryMediaForDocument(document, media.ContentType, schemaObject) {
				entry += ", binary: true"
			}
			if _, exists := media.Raw["schema"]; exists {
				entry += ", schemaDeclared: true"
			}
			if media.Stream.IsStreaming() {
				entry += ", streamFraming: " + quoteTS(string(media.Stream.Framing))
			}
			if _, exists := media.Raw["itemSchema"]; exists {
				itemDescriptor, err := wire.wireSchemaDescriptorForDocument(document, media.ItemSchema, projectionOutput)
				if err != nil {
					return "", false, err
				}
				entry += ", itemSchema: " + itemDescriptor
			}
			if prefixEncoding, err := wire.positionalMultipartWireEncodingsForDirection(document, media.Raw["prefixEncoding"], projectionOutput); err != nil {
				return "", false, err
			} else if prefixEncoding != "" {
				entry += ", prefixEncoding: " + prefixEncoding
			}
			if itemEncoding, err := wire.positionalMultipartWireEncodingForDirection(document, media.Raw["itemEncoding"], projectionOutput); err != nil {
				return "", false, err
			} else if itemEncoding != "" {
				entry += ", itemEncoding: " + itemEncoding
			}
			if headers != "" {
				entry += ", headers: " + headers
			}
			entries = append(entries, entry+" }")
		}
	}
	return "[" + strings.Join(entries, ", ") + "]", len(entries) > 0, nil
}

func (wire *wireRenderContext) positionalMultipartWireEncodingForDirection(document *ir.Document, value any, direction projection) (string, error) {
	encoding, _ := value.(map[string]any)
	if len(encoding) == 0 {
		return "", nil
	}
	return wire.multipartWireEncoding(document, encoding, "", direction)
}

func (wire *wireRenderContext) positionalMultipartWireEncodingsForDirection(document *ir.Document, value any, direction projection) (string, error) {
	values, _ := value.([]any)
	if len(values) == 0 {
		return "", nil
	}
	entries := make([]string, 0, len(values))
	for _, item := range values {
		encoding, _ := item.(map[string]any)
		entry, err := wire.multipartWireEncoding(document, encoding, "", direction)
		if err != nil {
			return "", err
		}
		entries = append(entries, entry)
	}
	return "[" + strings.Join(entries, ", ") + "]", nil
}

func (wire *wireRenderContext) responseWireHeaders(document *ir.Document, response map[string]any) (string, error) {
	headers, _ := response["headers"].(map[string]any)
	if len(headers) == 0 {
		return "", nil
	}
	names := sortedAnyKeys(headers)
	entries := make([]string, 0, len(names))
	for _, name := range names {
		header, _ := headers[name].(map[string]any)
		header, err := resolveComponentObject(document, header, "headers")
		if err != nil {
			return "", err
		}
		schema, contentType, err := responseHeaderSchema(document, header)
		if err != nil {
			return "", err
		}
		descriptor, err := wire.wireSchemaDescriptorForDocument(document, schema, projectionOutput)
		if err != nil {
			return "", err
		}
		style, _ := header["style"].(string)
		if style == "" {
			style = "simple"
		}
		explode, hasExplode := header["explode"].(bool)
		if !hasExplode {
			explode = style == "form"
		}
		entry := "{ name: " + quoteTS(name) + ", property: " + quoteTS(name) + ", style: " + quoteTS(style) + ", explode: " + fmt.Sprint(explode) + ", schema: " + descriptor
		if contentType != "" {
			wire.recordRuntimeMedia("response.header", contentType, ir.StreamFramingNone, false)
			entry += ", contentType: " + quoteTS(contentType)
		}
		if boolValue(header, "required") {
			entry += ", required: true"
		}
		entries = append(entries, entry+" }")
	}
	return "[" + strings.Join(entries, ", ") + "]", nil
}
