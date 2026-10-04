package typescript

// These assertions are emitted directly from the typed node, so their generic
// assertion handlers are not part of a generated program composition.
func programHandlerFeatures(features []runtimeFeature) []runtimeFeature {
	result := make([]runtimeFeature, 0, len(features))
	for _, feature := range features {
		switch string(feature) {
		case "schema.maximum", "schema.exclusiveMaximum", "schema.minimum", "schema.exclusiveMinimum", "schema.minLength", "schema.maxLength", "schema.minItems", "schema.maxItems", "schema.minProperties", "schema.maxProperties":
			continue
		}
		result = append(result, feature)
	}
	return result
}
