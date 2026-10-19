package extension

import (
	"openapi-sdkgen/internal/compiler/ir"
	"strings"
)

func (planner *Planner) EnvelopeSchema(document *ir.Document, schema map[string]any, seen map[string]bool) map[string]any {
	if reference, _ := schema["$ref"].(string); reference != "" {
		name, err := componentSchemaReferenceName(reference)
		if err != nil {
			return nil
		}
		if seen[name] {
			return nil
		}
		seen[name] = true
		return planner.EnvelopeSchema(document, document.ComponentSchemas[name], seen)
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
			data := planner.EnvelopeSchema(document, item, planner.copyStringBoolMap(seen))
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

func (planner *Planner) copyStringBoolMap(source map[string]bool) map[string]bool {
	result := make(map[string]bool, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func (planner *Planner) ResolveSchema(document *ir.Document, schema map[string]any, resolving map[string]bool) map[string]any {
	reference, _ := schema["$ref"].(string)
	if reference == "" {
		return schema
	}
	name, err := componentSchemaReferenceName(reference)
	if err != nil || resolving[name] {
		return schema
	}
	resolved, exists := document.ComponentSchemas[name]
	if !exists {
		return schema
	}
	resolving[name] = true
	defer delete(resolving, name)
	return planner.ResolveSchema(document, resolved, resolving)
}

func (planner *Planner) ReachableErrorSchemas(document *ir.Document) map[string]bool {
	result := make(map[string]bool)
	var visitSchema func(map[string]any)
	visitSchema = func(schema map[string]any) {
		for _, name := range planner.SchemaReferences(schema) {
			if result[name] {
				continue
			}
			result[name] = true
			visitSchema(document.ComponentSchemas[name])
		}
	}
	for _, operation := range document.Operations {
		if operation.Visibility == "hidden" {
			continue
		}
		responses, err := planner.responses(document, operation)
		if err != nil {
			continue
		}
		for _, response := range responses {
			if strings.HasPrefix(response.Status, "2") {
				continue
			}
			for _, media := range response.Content {
				schema, _ := media.Schema.(map[string]any)
				visitSchema(schema)
			}
		}
	}
	return result
}

func (planner *Planner) SchemaReferences(schema map[string]any) []string {
	var result []string
	if reference, _ := schema["$ref"].(string); reference != "" {
		name, err := componentSchemaReferenceName(reference)
		if err != nil {
			return nil
		}
		return []string{name}
	}
	for _, keyword := range []string{"oneOf", "anyOf", "allOf"} {
		variants, _ := schema[keyword].([]any)
		for _, value := range variants {
			variant, _ := value.(map[string]any)
			result = append(result, planner.SchemaReferences(variant)...)
		}
	}
	return result
}
