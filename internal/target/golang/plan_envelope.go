package golang

import (
	"openapi-sdkgen/internal/compiler/ir"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

// Optional data preserves the native envelope fallback as a typed result union.
// Requiredness can be proven conjunctively; alternatives must all require it.
func envelopeDataRequired(document *ir.Document, schema map[string]any, seen map[string]bool) bool {
	if properties, ok := schema["properties"].(map[string]any); ok {
		if data, ok := properties["data"].(map[string]any); ok && data["writeOnly"] == true {
			return false
		}
	}
	required, _ := schema["required"].([]any)
	for _, value := range required {
		if value == "data" {
			return true
		}
	}
	if reference, _ := schema["$ref"].(string); reference != "" && !seen[reference] {
		seen[reference] = true
		name, err := schemaplan.ComponentReferenceName(reference)
		if err == nil && envelopeDataRequired(document, document.ComponentSchemas[name], seen) {
			return true
		}
		delete(seen, reference)
	}
	conjuncts, _ := schema["allOf"].([]any)
	for _, value := range conjuncts {
		child, _ := value.(map[string]any)
		if envelopeDataRequired(document, child, seen) {
			return true
		}
	}
	for _, keyword := range []string{"anyOf", "oneOf"} {
		branches, _ := schema[keyword].([]any)
		all := len(branches) > 0
		for _, value := range branches {
			child, _ := value.(map[string]any)
			all = all && envelopeDataRequired(document, child, seen)
		}
		if all {
			return true
		}
	}
	return false
}
