package golang

// A parameter or header is a complete schema occurrence, not an object
// property. Grouping occurrences into a Go DTO must not project the entire
// occurrence away because its root carries readOnly/writeOnly annotations.
func httpValueSchema(value any) any {
	schema, ok := value.(map[string]any)
	if !ok {
		return value
	}
	result := make(map[string]any, len(schema))
	for key, value := range schema {
		if key != "readOnly" && key != "writeOnly" {
			result[key] = value
		}
	}
	return result
}
