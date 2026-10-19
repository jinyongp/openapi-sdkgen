package golang

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

// Compatibility concerns the accepted Go representation. Each operation still
// validates its own constraints when the call executes.
func goTypeSignature(document *ir.Document, value any, input bool) string {
	var shape func(any, map[string]int) any
	shape = func(value any, stack map[string]int) any {
		source, ok := value.(map[string]any)
		if !ok {
			return value
		}
		if reference, _ := source["$ref"].(string); reference != "" {
			if index, seen := stack[reference]; seen {
				return map[string]any{"recursive": index}
			}
			const prefix = "#/components/schemas/"
			if strings.HasPrefix(reference, prefix) {
				name := strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(reference, prefix), "~1", "/"), "~0", "~")
				if target, exists := document.ComponentSchemas[name]; exists {
					next := make(map[string]int, len(stack)+1)
					for key, index := range stack {
						next[key] = index
					}
					next[reference] = len(stack)
					resolved := make(map[string]any, len(target)+len(source))
					for key, item := range target {
						resolved[key] = item
					}
					for key, item := range source {
						if key != "$ref" {
							resolved[key] = item
						}
					}
					return shape(resolved, next)
				}
			}
			return map[string]any{"reference": reference}
		}
		result := make(map[string]any)
		if input {
			// A tuple's required prefix changes its Go fields. Sealed composition
			// constructors also enforce constraints before a selector can snapshot
			// their value, so differing contracts cannot share that argument type.
			if prefix, ok := source["prefixItems"].([]any); ok && len(prefix) != 0 {
				encoded, _ := json.Marshal(source["minItems"])
				minimum, _ := strconv.ParseFloat(string(encoded), 64)
				required := 0
				for index := range prefix {
					if minimum >= float64(index+1) {
						required++
					}
				}
				result["requiredPrefix"] = required
			}
			sealed := false
			for _, keyword := range []string{"oneOf", "anyOf", "allOf", "not", "if", "dependentSchemas", "$dynamicRef", "x-sdkgen-dynamic-reference"} {
				if _, present := source[keyword]; present {
					sealed = true
				}
			}
			if kinds, ok := source["type"].([]any); ok {
				nonNull := 0
				for _, kind := range kinds {
					if kind != "null" {
						nonNull++
					}
				}
				sealed = sealed || nonNull > 1
			}
			if _, typed := source["type"]; !typed {
				for _, keyword := range []string{"properties", "items", "prefixItems", "minimum", "maximum", "minLength", "maxLength", "pattern"} {
					if _, present := source[keyword]; present {
						sealed = true
					}
				}
			}
			if sealed {
				encoded, _ := json.Marshal(source)
				result["sealedContract"] = string(encoded)
			}
		}
		for _, key := range []string{"type", "enum", "const", "nullable", "discriminator"} {
			if item, present := source[key]; present {
				result[key] = item
			}
		}
		if result["type"] == "integer" {
			result["type"] = "number"
		}
		if kinds, ok := result["type"].([]any); ok {
			normalized := make([]string, 0, len(kinds))
			for _, kind := range kinds {
				if name, ok := kind.(string); ok {
					if name == "integer" {
						name = "number"
					}
					normalized = append(normalized, name)
				}
			}
			sort.Strings(normalized)
			result["type"] = normalized
		}
		properties, _ := source["properties"].(map[string]any)
		kept := make(map[string]any, len(properties))
		for name, item := range properties {
			child, _ := item.(map[string]any)
			omit := "writeOnly"
			if input {
				omit = "readOnly"
			}
			if hidden, _ := child[omit].(bool); hidden {
				continue
			}
			kept[name] = shape(item, stack)
		}
		if len(kept) != 0 {
			result["properties"] = kept
		}
		if required, ok := source["required"].([]any); ok {
			names := make([]string, 0, len(required))
			for _, item := range required {
				if name, ok := item.(string); ok {
					if _, present := kept[name]; present {
						names = append(names, name)
					}
				}
			}
			sort.Strings(names)
			if len(names) != 0 {
				result["required"] = names
			}
		}
		for _, key := range []string{"items", "additionalProperties", "unevaluatedProperties"} {
			if child, present := source[key]; present {
				result[key] = shape(child, stack)
			}
		}
		for _, key := range []string{"oneOf", "anyOf", "allOf", "prefixItems"} {
			if children, ok := source[key].([]any); ok {
				items := make([]any, len(children))
				for index, child := range children {
					items[index] = shape(child, stack)
				}
				result[key] = items
			}
		}
		// Pattern properties force an open Value map in the generated object DTO.
		if children, ok := source["patternProperties"].(map[string]any); ok && len(children) != 0 {
			result["patternProperties"] = true
		}
		return result
	}
	encoded, _ := json.Marshal(shape(value, make(map[string]int)))
	return string(encoded)
}
