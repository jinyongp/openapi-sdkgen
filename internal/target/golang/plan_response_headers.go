package golang

import (
	"openapi-sdkgen/internal/compiler/ir"
	httpplan "openapi-sdkgen/internal/http/plan"
	schemaplan "openapi-sdkgen/internal/schema/plan"
	"strings"
)

func prepareResponseHeaders(planned *operationPlan, operation ir.Operation, document *ir.Document, schemas *schemaPlanner) error {
	for _, response := range operation.Responses {
		raw, _ := response.Raw["headers"].(map[string]any)
		status := strings.ToUpper(response.Status)
		if response.Status == "default" {
			status = "Default"
		}
		group := responseHeaderPlan{Status: response.Status, Name: status, Model: planned.Name + status + "Headers", Index: len(planned.Headers) + 1}
		if len(raw) == 0 {
			group.Model = ""
			planned.Headers = append(planned.Headers, group)
			continue
		}
		properties := make(map[string]any)
		members := make(map[string]schemaOccurrence)
		required := make([]any, 0)
		for _, name := range sortedKeys(raw) {
			if strings.EqualFold(name, "Content-Type") {
				continue
			}
			header, _ := raw[name].(map[string]any)
			header, err := httpplan.ResolveComponentObject(document, header, "headers")
			if err != nil {
				return err
			}
			value, media, err := httpplan.HeaderSchema(document, header)
			if err != nil {
				return err
			}
			identity := response.Pointer + "/headers/" + pointerToken(name) + "/schema"
			validation, err := schemas.prepare(headerValidationName(group.Model, identity), value, schemaplan.Output, document.OpenAPIVersionLine == "3.0", schemaOccurrence{Source: identity, MediaType: media})
			if err != nil {
				return err
			}
			field := headerFieldPlan{Name: name, Validation: validation, ContentType: media, Text: strings.HasPrefix(bareMediaType(media), "text/")}
			field.Required, _ = header["required"].(bool)
			field.Explode, _ = header["explode"].(bool)
			properties[name] = httpValueSchema(value)
			if schema, ok := properties[name].(map[string]any); ok {
				annotated := make(map[string]any, len(schema)+2)
				for key, value := range schema {
					annotated[key] = value
				}
				if description, ok := header["description"].(string); ok && description != "" {
					annotated["description"] = description
				}
				if deprecated, _ := header["deprecated"].(bool); deprecated {
					annotated["deprecated"] = true
				}
				properties[name] = annotated
			}
			members[name] = schemaOccurrence{Source: identity, MediaType: media}
			if field.Required {
				required = append(required, name)
				if strings.EqualFold(name, "Set-Cookie") {
					planned.NeedsCookieHeaders = true
				}
			}
			group.Fields = append(group.Fields, field)
		}
		if len(group.Fields) == 0 {
			group.Model = ""
			planned.Headers = append(planned.Headers, group)
			continue
		}
		planned.HasHeaders = true
		validation, err := schemas.prepare(httpValidationName(group.Model), map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}, schemaplan.Output, document.OpenAPIVersionLine == "3.0", schemaOccurrence{Source: response.Pointer + "/headers", Members: members})
		if err != nil {
			return err
		}
		group.Validation = validation

		planned.Headers = append(planned.Headers, group)
	}
	return nil
}
