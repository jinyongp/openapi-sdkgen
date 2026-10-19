package golang

import (
	"fmt"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	httpplan "openapi-sdkgen/internal/http/plan"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func prepareInboundResponseEncodings(planned *operationPlan, operation ir.Operation, document *ir.Document, schemas *schemaPlanner) (map[string]string, error) {
	result := make(map[string]string)
	for _, response := range operation.Responses {
		for _, media := range response.Content {
			multipart := mediaMayContainMultipart(media.ContentType)
			if !multipart && bareMediaType(media.ContentType) != "application/x-www-form-urlencoded" {
				continue
			}
			identity := response.Pointer + "/content/" + pointerToken(media.ContentType)
			definitions, err := prepareEncodingFields(planned, media.Raw, media.Schema, document, schemas, identity, multipart, multipart)
			if err != nil {
				return nil, err
			}
			for _, branch := range planned.Responses {
				if branch.Status == response.Status && branch.MediaType == media.ContentType {
					name := "responseEncodingFor" + planned.Name + branch.Name
					planned.BodyEncodings += fmt.Sprintf("func %s(execution *core.Execution)[]core.FormField{return %s}\n", name, definitions)
					result[response.Status+"\x00"+media.ContentType] = name
				}
			}
		}
	}
	return result, nil
}

func prepareInboundRequestParts(planned *operationPlan, operation ir.Operation, document *ir.Document, schemas *schemaPlanner) error {
	if operation.RequestBody == nil {
		return nil
	}
	parts := newResponsePartPlanner(planned, document, schemas)
	parts.projection = schemaplan.Input
	parts.prefix = "requestPartsFor"
	var named strings.Builder
	fmt.Fprintf(&named, "func namedPartFor%s(execution *core.Execution,media,name string)core.StreamPartDefinition{switch media{\n", planned.Name)
	for _, media := range operation.RequestBody.Content {
		if !mediaMayContainMultipart(media.ContentType) || media.Stream.IsStreaming() {
			continue
		}
		fmt.Fprintf(&named, "case %q:switch name{\n", media.ContentType)
		encoding, _ := media.Raw["encoding"].(map[string]any)
		for _, name := range inboundPropertyNames(document, media.Schema) {
			part, err := httpplan.MultipartPartSchema(document, media.Schema, name, false)
			if err != nil {
				return err
			}
			identity := operation.RequestBody.Pointer + "/content/" + pointerToken(media.ContentType) + "/inbound-part/" + pointerToken(name)
			validation, err := schemas.prepare(headerValidationName(planned.Name+"InboundPart", identity), part, schemaplan.Input, document.OpenAPIVersionLine == "3.0", schemaOccurrence{Source: identity})
			if err != nil {
				return err
			}
			planned.MultipartResponses = append(planned.MultipartResponses, validation)
			field, _ := encoding[name].(map[string]any)
			nested, err := parts.prepare(part, field)
			if err != nil {
				return err
			}
			fmt.Fprintf(&named, "case %q:return core.StreamPartDefinition{XML:schema.DecodeXMLHTTP__%s__,Nested:func(index int)core.StreamPartDefinition{return %s(execution,index)}}\n", name, validation, nested)
		}
		named.WriteString("}\n")
	}
	named.WriteString("};return core.StreamPartDefinition{}}\n")
	for _, stream := range planned.RequestStreams {
		if !stream.Multipart {
			continue
		}
		for _, media := range operation.RequestBody.Content {
			if media.ContentType != stream.MediaType {
				continue
			}
			root, raw := media.Schema, media.Raw
			if stream.ItemValidation != "" {
				root = map[string]any{"items": media.ItemSchema}
				raw = map[string]any{"itemEncoding": media.Raw["itemEncoding"]}
			}
			var err error
			stream.Parts, err = parts.prepare(root, raw)
			if err != nil {
				return err
			}
		}
	}
	planned.BodyEncodings += named.String() + parts.source()
	return nil
}

func inboundPropertyNames(document *ir.Document, source any) []string {
	names, active := make(map[string]bool), make(map[string]bool)
	var visit func(any)
	visit = func(value any) {
		object, _ := value.(map[string]any)
		if reference, _ := object["$ref"].(string); reference != "" && !active[reference] {
			if name, err := schemaplan.ComponentReferenceName(reference); err == nil {
				active[reference] = true
				visit(document.ComponentSchemas[name])
				delete(active, reference)
			}
		}
		properties, _ := object["properties"].(map[string]any)
		for name := range properties {
			names[name] = true
		}
		for _, keyword := range []string{"allOf", "oneOf", "anyOf"} {
			children, _ := object[keyword].([]any)
			for _, child := range children {
				visit(child)
			}
		}
		visitBranch := func(key string) {
			if child, present := object[key]; present {
				visit(child)
			}
		}
		visitBranch("then")
		visitBranch("else")
	}
	visit(source)
	return sortedKeys(names)
}
