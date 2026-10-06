package plan

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const FormatAssertionVocabulary = "https://json-schema.org/draft/2020-12/vocab/format-assertion"

func RequiresFormatAssertion(value any) bool {
	schema, _ := value.(map[string]any)
	vocabularies, _ := schema["$vocabulary"].(map[string]any)
	required, _ := vocabularies[FormatAssertionVocabulary].(bool)
	return required
}

// Lower performs projection, dialect and reference decisions exactly once.
func Lower(value any, options Options) (*Node, error) {
	if input, ok := value.(BinaryInput); ok {
		schema := binaryInputAssertions(input.Schema)
		node, err := Lower(schema, options)
		if err != nil {
			return nil, err
		}
		node.Fields = append(node.Fields, Field{Name: "binaryInput", Value: Literal{true}}, Field{Name: "binaryContentType", Value: Literal{input.ContentType}})
		if input.Nullable {
			node.Fields = append(node.Fields, Field{Name: "binaryNullable", Value: Literal{true}})
		}
		return node, nil
	}
	node := &Node{}
	add := func(name string, value Value, feature string) {
		node.Fields = append(node.Fields, Field{Name: name, Value: value})
		if feature != "" && options.Observer != nil {
			options.Observer.Field(feature)
		}
	}
	if boolean, ok := value.(bool); ok {
		add("boolean", Literal{boolean}, "boolean")
		return node, nil
	}
	schema, ok := value.(map[string]any)
	if !ok || len(schema) == 0 {
		return node, nil
	}
	node.Fields = make([]Field, 0, len(schema))
	child := func(value any) (*Node, error) { return Lower(value, options) }
	if dynamic, ok := schema["x-sdkgen-dynamic-reference"].(map[string]any); ok {
		anchor, _ := dynamic["anchor"].(string)
		reference, _ := dynamic["reference"].(string)
		name, err := options.ReferenceName(reference)
		if err != nil {
			return nil, err
		}
		if anchor == "" {
			return nil, fmt.Errorf("dynamic reference has no anchor")
		}
		if options.Observer != nil {
			options.Observer.Reference(name, options.Projection)
			options.Observer.Dynamic()
		}
		fallback := &Node{Fields: []Field{{Name: "reference", Value: Literal{name}}}}
		add("dynamicReference", DynamicReference{Anchor: anchor, Fallback: fallback}, "dynamic")
		siblings := without(schema, "x-sdkgen-dynamic-reference")
		other, err := child(siblings)
		if err != nil {
			return nil, err
		}
		node.Fields = append(node.Fields, other.Fields...)
		return node, nil
	}
	if reference, _ := schema["$ref"].(string); reference != "" {
		name, err := options.ReferenceName(reference)
		if err != nil {
			return nil, err
		}
		if options.Observer != nil {
			options.Observer.Reference(name, options.Projection)
		}
		add("reference", Literal{name}, "reference")
		other, err := child(without(schema, "$ref"))
		if err != nil {
			return nil, err
		}
		node.Fields = append(node.Fields, other.Fields...)
		return node, nil
	}
	if anchor, _ := schema["x-sdkgen-dynamic-anchor"].(string); anchor != "" {
		add("dynamicAnchor", Literal{anchor}, "dynamic")
	}
	var types []string
	switch typed := schema["type"].(type) {
	case string:
		types = []string{typed}
	case []any:
		for _, value := range typed {
			if name, ok := value.(string); ok {
				types = append(types, name)
			}
		}
	}
	if nullable, _ := schema["nullable"].(bool); options.LegacyNullable && nullable && len(types) > 0 {
		found := false
		for _, name := range types {
			found = found || name == "null"
		}
		if !found {
			types = append(types, "null")
		}
	}
	if len(types) > 0 {
		if options.Observer != nil {
			for _, name := range types {
				options.Observer.Value("type", name)
			}
		}
		add("types", Literal{types}, "")
	}
	if value, exists := schema["const"]; exists {
		if _, err := json.Marshal(value); err != nil {
			return nil, fmt.Errorf("encode const: %w", err)
		}
		add("constValue", Literal{value}, "const")
	}
	if values, ok := schema["enum"].([]any); ok && len(values) > 0 {
		if _, err := json.Marshal(values); err != nil {
			return nil, fmt.Errorf("encode enum: %w", err)
		}
		add("enumValues", Literal{values}, "enum")
	}
	exclusiveMaximum, maximumBoolean := schema["exclusiveMaximum"].(bool)
	exclusiveMinimum, minimumBoolean := schema["exclusiveMinimum"].(bool)
	for _, bound := range []struct {
		name   string
		active bool
	}{{"Maximum", maximumBoolean && exclusiveMaximum}, {"Minimum", minimumBoolean && exclusiveMinimum}} {
		if !bound.active {
			continue
		}
		if value, exists := schema[strings.ToLower(bound.name)]; exists {
			encoded, err := json.Marshal(value)
			if err != nil {
				return nil, fmt.Errorf("encode %s: %w", strings.ToLower(bound.name), err)
			}
			if len(encoded) > 0 && encoded[0] != '"' {
				add("exclusive"+bound.name, Literal{json.RawMessage(encoded)}, "exclusive"+bound.name)
			}
		}
	}
	for _, keyword := range []string{"multipleOf", "maximum", "exclusiveMaximum", "minimum", "exclusiveMinimum", "minLength", "maxLength", "minItems", "maxItems", "minProperties", "maxProperties"} {
		if (keyword == "maximum" && maximumBoolean && exclusiveMaximum) || (keyword == "minimum" && minimumBoolean && exclusiveMinimum) || (keyword == "exclusiveMaximum" && maximumBoolean) || (keyword == "exclusiveMinimum" && minimumBoolean) {
			continue
		}
		value, exists := schema[keyword]
		if !exists {
			continue
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", keyword, err)
		}
		if string(encoded) != "true" && string(encoded) != "false" && string(encoded) != "null" && len(encoded) > 0 && encoded[0] != '"' {
			add(keyword, Literal{json.RawMessage(encoded)}, keyword)
		}
	}
	if value, ok := schema["pattern"].(string); ok {
		add("pattern", Literal{value}, "pattern")
	}
	if value, ok := schema["format"].(string); ok && value != "" {
		add("format", Literal{value}, "")
		if options.FormatAssertion || RequiresFormatAssertion(schema) {
			if options.Observer != nil {
				options.Observer.Value("format", value)
			}
			add("formatAssertion", Literal{true}, "")
		}
	}
	if value, ok := schema["uniqueItems"].(bool); ok && value {
		add("uniqueItems", Literal{true}, "uniqueItems")
	}
	for _, keyword := range []string{"contentEncoding", "contentMediaType"} {
		if value, ok := schema[keyword].(string); ok && value != "" {
			add(keyword, Literal{value}, keyword)
			if keyword == "contentMediaType" && options.Observer != nil {
				options.Observer.ContentMedia(value)
			}
		}
	}
	if value, exists := schema["contentSchema"]; exists {
		nested, err := child(value)
		if err != nil {
			return nil, err
		}
		add("contentSchema", Child{nested}, "contentSchema")
	}
	if xml, ok := schema["xml"].(map[string]any); ok && len(xml) > 0 {
		if _, err := json.Marshal(xml); err != nil {
			return nil, fmt.Errorf("encode XML Object: %w", err)
		}
		add("xml", Literal{xml}, "xml")
	}
	properties, _ := schema["properties"].(map[string]any)
	entries := make([]Property, 0, len(properties))
	for _, name := range keys(properties) {
		if projected(properties[name], options.Projection) {
			continue
		}
		nested, err := child(properties[name])
		if err != nil {
			return nil, err
		}
		entries = append(entries, Property{Name: name, Schema: nested})
	}
	if len(entries) > 0 {
		add("properties", Properties{entries}, "properties")
	}
	if patterns, ok := schema["patternProperties"].(map[string]any); ok && len(patterns) > 0 {
		entries := make([]Property, 0, len(patterns))
		for _, name := range keys(patterns) {
			nested, err := child(patterns[name])
			if err != nil {
				return nil, err
			}
			entries = append(entries, Property{Name: name, Schema: nested})
		}
		add("patternProperties", SchemaMap{entries}, "patternProperties")
	}
	if value, exists := schema["propertyNames"]; exists {
		nested, err := child(value)
		if err != nil {
			return nil, err
		}
		add("propertyNames", Child{nested}, "propertyNames")
	}
	if dependencies, ok := schema["dependentRequired"].(map[string]any); ok && len(dependencies) > 0 {
		result := make(map[string][]string)
		for _, name := range keys(dependencies) {
			values, _ := dependencies[name].([]any)
			items := []string{}
			for _, value := range values {
				if dependency, ok := value.(string); ok {
					items = append(items, dependency)
				}
			}
			result[name] = items
		}
		add("dependentRequired", Literal{result}, "dependentRequired")
	}
	if dependencies, ok := schema["dependentSchemas"].(map[string]any); ok && len(dependencies) > 0 {
		entries := make([]Property, 0, len(dependencies))
		for _, name := range keys(dependencies) {
			nested, err := child(dependencies[name])
			if err != nil {
				return nil, err
			}
			entries = append(entries, Property{Name: name, Schema: nested})
		}
		add("dependentSchemas", SchemaMap{entries}, "dependentSchemas")
	}
	if values, ok := schema["required"].([]any); ok && len(values) > 0 {
		names := make([]string, 0, len(values))
		for _, value := range values {
			if name, ok := value.(string); ok && !projected(properties[name], options.Projection) {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		if len(names) > 0 {
			add("required", Literal{names}, "required")
		}
	}
	for _, keyword := range []string{"items", "contains"} {
		value, exists := schema[keyword]
		if !exists {
			continue
		}
		nested, err := child(value)
		if err != nil {
			return nil, err
		}
		add(keyword, Child{nested}, keyword)
		if keyword == "contains" {
			for _, bound := range []string{"minContains", "maxContains"} {
				if value, exists := schema[bound]; exists {
					if _, err := json.Marshal(value); err != nil {
						return nil, fmt.Errorf("encode %s: %w", bound, err)
					}
					add(bound, Literal{value}, "")
				}
			}
		}
	}
	if values, ok := schema["prefixItems"].([]any); ok && len(values) > 0 {
		nodes := make([]*Node, 0, len(values))
		for _, value := range values {
			nested, err := child(value)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, nested)
		}
		add("prefixItems", Children{nodes}, "prefixItems")
	}
	for _, keyword := range []string{"additionalProperties", "unevaluatedProperties", "unevaluatedItems"} {
		value, exists := schema[keyword]
		if !exists {
			continue
		}
		if boolean, ok := value.(bool); ok && !boolean {
			add(keyword, Literal{false}, keyword)
			continue
		}
		nested, err := child(value)
		if err != nil {
			return nil, err
		}
		add(keyword, Child{nested}, keyword)
	}
	for _, keyword := range []string{"allOf", "oneOf", "anyOf"} {
		values, _ := schema[keyword].([]any)
		if len(values) == 0 {
			continue
		}
		nodes := make([]*Node, 0, len(values))
		for _, value := range values {
			nested, err := child(value)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, nested)
		}
		add(keyword, Children{nodes}, keyword)
	}
	for _, keyword := range []string{"not", "if", "then", "else"} {
		value, exists := schema[keyword]
		if !exists {
			continue
		}
		nested, err := child(value)
		if err != nil {
			return nil, err
		}
		add(keyword, Child{nested}, keyword)
	}
	if discriminator, ok := schema["discriminator"].(map[string]any); ok {
		if property, ok := discriminator["propertyName"].(string); ok && property != "" {
			mapping := make(map[string]any)
			if explicit, ok := discriminator["mapping"].(map[string]any); ok {
				for name, value := range explicit {
					if reference, ok := value.(string); ok {
						mapping[name] = map[string]any{"$ref": normalizeDiscriminatorReference(reference)}
					}
				}
			}
			variants, _ := schema["oneOf"].([]any)
			for _, variant := range variants {
				object, _ := variant.(map[string]any)
				reference, _ := object["$ref"].(string)
				name, err := options.ReferenceName(reference)
				if err != nil || name == "" {
					continue
				}
				if _, exists := mapping[name]; !exists {
					mapping[name] = map[string]any{"$ref": reference}
				}
			}
			var entries []Property
			for _, name := range keys(mapping) {
				nested, err := child(mapping[name])
				if err != nil {
					return nil, err
				}
				entries = append(entries, Property{Name: name, Schema: nested})
			}
			result := Discriminator{Property: property, Mapping: entries}
			if reference, ok := discriminator["defaultMapping"].(string); ok && reference != "" {
				defaultOptions := options
				defaultOptions.FormatAssertion = false
				nested, err := Lower(map[string]any{"$ref": normalizeDiscriminatorReference(reference)}, defaultOptions)
				if err != nil {
					return nil, err
				}
				result.Default = nested
			}
			add("discriminator", result, "discriminator")
		}
	}
	return node, nil
}

func binaryInputAssertions(schema map[string]any) map[string]any {
	result := schema
	for _, name := range []string{"nullable", "contentMediaType"} {
		result = without(result, name)
	}
	if format, _ := schema["format"].(string); format == "binary" {
		result = without(result, "format")
	}
	if encoding, _ := schema["contentEncoding"].(string); encoding == "binary" {
		result = without(result, "contentEncoding")
	}
	if kind, _ := schema["type"].(string); kind == "string" {
		result = without(result, "type")
	}
	if kinds, ok := schema["type"].([]any); ok {
		stringsOnly := true
		for _, kind := range kinds {
			stringsOnly = stringsOnly && (kind == "string" || kind == "null")
		}
		if stringsOnly {
			result = without(result, "type")
		}
	}
	if branches, ok := schema["allOf"].([]any); ok {
		projected := append([]any(nil), branches...)
		for index, branch := range branches {
			if child, ok := branch.(map[string]any); ok {
				projected[index] = binaryInputAssertions(child)
			}
		}
		result["allOf"] = projected
	}
	return result
}

func without(schema map[string]any, omitted string) map[string]any {
	result := make(map[string]any, len(schema))
	for key, value := range schema {
		if key != omitted {
			result[key] = value
		}
	}
	return result
}
func keys(values map[string]any) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
func projected(value any, direction Projection) bool {
	schema, _ := value.(map[string]any)
	input, _ := schema["readOnly"].(bool)
	output, _ := schema["writeOnly"].(bool)
	return direction == Input && input || direction == Output && output
}
func normalizeDiscriminatorReference(reference string) string {
	if strings.HasPrefix(reference, "#") || strings.Contains(reference, ":") || strings.HasPrefix(reference, "./") || strings.HasPrefix(reference, "../") {
		return reference
	}
	return "#/components/schemas/" + strings.ReplaceAll(strings.ReplaceAll(reference, "~", "~0"), "/", "~1")
}
