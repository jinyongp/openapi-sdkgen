package plan

// Documentation is annotation data, independent of validation identity.
type Documentation struct {
	Title, Description string
	Deprecated         bool
}

type AnnotatedMember struct {
	Value         any
	Documentation Documentation
}

// ConstraintKeywords fixes the annotation order across language renderers.
func ConstraintKeywords() []string {
	return []string{"multipleOf", "maximum", "exclusiveMaximum", "minimum", "exclusiveMinimum", "maxLength", "minLength", "pattern", "maxItems", "minItems", "uniqueItems", "contains", "minContains", "maxContains", "maxProperties", "minProperties", "dependentRequired", "propertyNames", "unevaluatedItems", "unevaluatedProperties", "contentEncoding", "contentMediaType", "contentSchema"}
}

func SchemaDocumentation(schema map[string]any) Documentation {
	title, _ := schema["title"].(string)
	description, _ := schema["description"].(string)
	deprecated, _ := schema["deprecated"].(bool)
	return Documentation{Title: title, Description: description, Deprecated: deprecated}
}

// AnnotatedEnum recognizes only distinct const branches without assertions.
// The caller supplies its literal identity so existing target behavior remains stable.
func AnnotatedEnum(version string, schema map[string]any, identity func(any) (string, error)) ([]AnnotatedMember, bool, error) {
	if version != "3.1" && version != "3.2" {
		return nil, false, nil
	}
	var branches []any
	compositions := 0
	for _, keyword := range []string{"oneOf", "anyOf"} {
		if value, exists := schema[keyword]; exists {
			compositions++
			var ok bool
			branches, ok = value.([]any)
			if !ok || len(branches) == 0 {
				return nil, false, nil
			}
		}
	}
	if compositions != 1 {
		return nil, false, nil
	}
	seen := make(map[string]bool, len(branches))
	members := make([]AnnotatedMember, 0, len(branches))
	for _, item := range branches {
		branch, ok := item.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		value, exists := branch["const"]
		if !exists {
			return nil, false, nil
		}
		for key := range branch {
			switch key {
			case "const", "title", "description", "default", "deprecated", "readOnly", "writeOnly", "examples", "example", "externalDocs":
			default:
				return nil, false, nil
			}
		}
		key, err := identity(value)
		if err != nil {
			return nil, false, err
		}
		if seen[key] {
			return nil, false, nil
		}
		seen[key] = true
		members = append(members, AnnotatedMember{Value: value, Documentation: SchemaDocumentation(branch)})
	}
	return members, true, nil
}

// AlwaysDeprecated follows same-instance composition and guards reference cycles.
func AlwaysDeprecated(value any, resolve func(string) any) bool {
	seen := make(map[string]bool)
	var visit func(any) bool
	visit = func(value any) bool {
		schema, ok := value.(map[string]any)
		if !ok {
			return false
		}
		if SchemaDocumentation(schema).Deprecated {
			return true
		}
		if ref, _ := schema["$ref"].(string); ref != "" && resolve != nil && !seen[ref] {
			seen[ref] = true
			deprecated := visit(resolve(ref))
			delete(seen, ref)
			if deprecated {
				return true
			}
		}
		if branches, ok := schema["allOf"].([]any); ok {
			for _, branch := range branches {
				if visit(branch) {
					return true
				}
			}
		}
		for _, keyword := range []string{"oneOf", "anyOf"} {
			branches, ok := schema[keyword].([]any)
			if !ok || len(branches) == 0 {
				continue
			}
			all := true
			for _, branch := range branches {
				if !visit(branch) {
					all = false
					break
				}
			}
			if all {
				return true
			}
		}
		return false
	}
	return visit(value)
}
