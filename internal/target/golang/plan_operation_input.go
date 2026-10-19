package golang

import (
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

// Input sections keep parameter names in one canonical object contract. The
// generated DTO encoder applies input projection before HTTP serialization.
func prepareOperationInput(planned *operationPlan, operation ir.Operation, schemas *schemaPlanner, openAPI30 bool, document *ir.Document, nativeInput ...bool) error {
	for _, location := range []string{"path", "query", "header", "cookie", "querystring"} {
		properties := make(map[string]any)
		members := make(map[string]schemaOccurrence)
		parameters := make(map[string]ir.Parameter)
		required := make([]any, 0)
		for _, parameter := range operation.Parameters {
			if parameter.Location != location {
				continue
			}
			if len(parameter.Content) > 1 {
				return fmt.Errorf("parameter content must select one media type at %s", parameter.Pointer)
			}
			parameters[parameter.Name] = parameter
			properties[parameter.Name] = httpValueSchema(parameter.Schema)
			pointer := parameter.Pointer
			if pointer == "" {
				pointer = operation.Pointer
				if pointer == "" {
					pointer = operation.RouteKey
				}
				pointer += "/parameters/" + location + "/" + pointerToken(parameter.Name)
			}
			identity := pointer + "/schema"
			if parameter.ContentType != "" {
				identity = pointer + "/content/" + pointerToken(parameter.ContentType) + "/schema"
			}
			members[parameter.Name] = schemaOccurrence{Source: identity, MediaType: parameter.ContentType}
			if parameter.Required {
				required = append(required, parameter.Name)
			}
			planned.Parameters = append(planned.Parameters, parameterPlan{Name: parameter.Name, Location: location, Section: exportedName(location), Style: parameter.Style, ContentType: parameter.ContentType, Explode: parameter.Explode, AllowReserved: parameter.AllowReserved, AllowEmptyValue: parameter.AllowEmptyValue})
		}
		if len(properties) == 0 {
			continue
		}
		name := planned.Name + exportedName(location)
		source := map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
		validation, err := schemas.prepare(httpValidationName(name), source, schemaplan.Input, openAPI30, schemaOccurrence{Source: operation.Pointer + "/parameters/" + location, Members: members})
		if err != nil {
			return err
		}
		if fields, ok := schemas.nodes[validation].Get("properties").(schemaplan.Properties); ok {
			for _, field := range fields.Entries {
				parameter := parameters[field.Name]
				annotation := make(map[string]any)
				if previous, ok := schemas.annotations[field.Schema].(map[string]any); ok {
					for key, value := range previous {
						annotation[key] = value
					}
				}
				if parameter.Description != "" {
					annotation["description"] = parameter.Description
				}
				if parameter.Deprecated {
					annotation["deprecated"] = true
				}
				schemas.annotations[field.Schema] = annotation
			}
		}
		field := inputFieldPlan{Name: exportedName(location), Type: name, Validation: validation, Required: len(required) > 0}
		planned.InputFields = append(planned.InputFields, field)
		planned.RequiredInput = planned.RequiredInput || field.Required
	}
	if err := prepareRequestBodyInput(planned, operation, schemas, openAPI30, document, nativeInput...); err != nil {
		return err
	}
	planned.HasInput = len(planned.InputFields) > 0
	return nil
}
