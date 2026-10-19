package plan

import (
	"context"
	"maps"
	"slices"
	"strconv"

	"openapi-sdkgen/internal/goruntime/ecmaregexp"

	"openapi-sdkgen/internal/compiler/ir"
)

// MultipartPartSchema selects the schema assertions for one prepared named or
// positional occurrence. The returned value is a build-time schema projection.
func MultipartPartSchema(document *ir.Document, value any, name string, positional bool) (any, error) {
	return multipartPartSchema(document, value, name, positional, make(map[string]bool))
}

func multipartPartSchema(document *ir.Document, value any, name string, positional bool, active map[string]bool) (any, error) {
	source, _ := value.(map[string]any)
	reference, _ := source["$ref"].(string)
	if reference != "" {
		if active[reference] {
			return true, nil
		}
		active[reference] = true
		defer delete(active, reference)
	}
	schema, ok := multipartResolvedSchema(document, value, make(map[string]bool))
	if !ok {
		return true, nil
	}
	var assertions []any
	matched := false
	if properties, ok := schema["properties"].(map[string]any); ok && !positional {
		if property, present := properties[name]; present {
			assertions = append(assertions, property)
			matched = true
		}
	}
	if !positional {
		patterns, _ := schema["patternProperties"].(map[string]any)
		for _, pattern := range slices.Sorted(maps.Keys(patterns)) {
			expression, err := ecmaregexp.Compile(pattern)
			if err != nil {
				return nil, err
			}
			accepts, err := expression.Match(context.Background(), name)
			if err != nil {
				return nil, err
			}
			if accepts {
				assertions = append(assertions, patterns[pattern])
				matched = true
			}
		}
		if !matched {
			if additional, exists := schema["additionalProperties"]; exists && additional != false {
				assertions = append(assertions, additional)
			}
		}
	}
	if index, err := strconv.Atoi(name); positional && (err == nil && index >= 0 || name == "*") {
		prefix, _ := schema["prefixItems"].([]any)
		if name != "*" && index < len(prefix) {
			assertions = append(assertions, prefix[index])
		} else if item, present := schema["items"]; present {
			assertions = append(assertions, item)
		}
	}
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		branches, _ := schema[keyword].([]any)
		if len(branches) == 0 {
			continue
		}
		parts := make([]any, len(branches))
		for index, branch := range branches {
			part, err := multipartPartSchema(document, branch, name, positional, active)
			if err != nil {
				return nil, err
			}
			parts[index] = part
		}
		// Exclusivity is checked on the complete body, not on one field.
		if keyword == "oneOf" {
			keyword = "anyOf"
		}
		assertions = append(assertions, map[string]any{keyword: parts})
	}
	if dynamic, ok := schema["x-sdkgen-dynamic-reference"].(map[string]any); ok {
		if reference, _ := dynamic["reference"].(string); reference != "" {
			part, err := multipartPartSchema(document, map[string]any{"$ref": reference}, name, positional, active)
			if err != nil {
				return nil, err
			}
			assertions = append(assertions, part)
		}
	}
	if _, conditional := schema["if"]; conditional {
		parts := make([]any, 2)
		for index, keyword := range []string{"then", "else"} {
			part, err := multipartPartSchema(document, schema[keyword], name, positional, active)
			if err != nil {
				return nil, err
			}
			parts[index] = part
		}
		assertions = append(assertions, map[string]any{"anyOf": parts})
	}
	if len(assertions) == 0 {
		return true, nil
	}
	if len(assertions) == 1 {
		return assertions[0], nil
	}
	return map[string]any{"allOf": assertions}, nil
}

// MultipartSchemaNames discovers finite field identities for generated codec
// dispatch. Runtime values are still validated by their complete body program.
func MultipartSchemaNames(document *ir.Document, value any) ([]string, int) {
	names := make(map[string]bool)
	prefixCount := 0
	active := make(map[string]bool)
	var visit func(any)
	visit = func(value any) {
		source, _ := value.(map[string]any)
		reference, _ := source["$ref"].(string)
		if reference != "" {
			if active[reference] {
				return
			}
			active[reference] = true
			defer delete(active, reference)
		}
		schema, ok := multipartResolvedSchema(document, value, make(map[string]bool))
		if !ok {
			return
		}
		if properties, ok := schema["properties"].(map[string]any); ok {
			for name := range properties {
				names[name] = true
			}
		}
		if prefix, ok := schema["prefixItems"].([]any); ok && len(prefix) > prefixCount {
			prefixCount = len(prefix)
		}
		for _, keyword := range []string{"allOf", "oneOf", "anyOf"} {
			branches, _ := schema[keyword].([]any)
			for _, branch := range branches {
				visit(branch)
			}
		}
		for _, keyword := range []string{"if", "then", "else"} {
			if branch, exists := schema[keyword]; exists {
				visit(branch)
			}
		}
		if dynamic, ok := schema["x-sdkgen-dynamic-reference"].(map[string]any); ok {
			if reference, _ := dynamic["reference"].(string); reference != "" {
				visit(map[string]any{"$ref": reference})
			}
		}
	}
	visit(value)
	return slices.Sorted(maps.Keys(names)), prefixCount
}
