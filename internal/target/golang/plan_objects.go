package golang

import (
	"reflect"
	"sort"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

// flattenObject changes only DTO representation. The original complete schema
// program remains the codec's validator, including every allOf assertion.
func (planner *modelPlanner) flattenObject(root *schemaplan.Node) (*schemaplan.Node, bool) {
	properties := make(map[string]*schemaplan.Node)
	required := make(map[string]bool)
	seen := make(map[*schemaplan.Node]bool)
	object, closed, patterns := false, false, false
	var additional *schemaplan.Node
	var collect func(*schemaplan.Node) bool
	collect = func(node *schemaplan.Node) bool {
		if seen[node] {
			return false
		}
		seen[node] = true
		defer delete(seen, node)
		for _, keyword := range []string{"oneOf", "anyOf", "not", "if", "dynamicReference"} {
			if node.Get(keyword) != nil {
				return false
			}
		}
		if literal, ok := node.Get("boolean").(schemaplan.Literal); ok && literal.Data == false {
			return false
		}
		if literal, ok := node.Get("types").(schemaplan.Literal); ok {
			types, _ := literal.Data.([]string)
			if len(types) != 1 || types[0] != "object" {
				return false
			}
			object = true
		}
		if reference, ok := node.Get("reference").(schemaplan.Literal); ok {
			target, ok := planner.referenceNodes[reference.Data.(string)]
			if !ok || !collect(target) {
				return false
			}
		}
		if children, ok := node.Get("allOf").(schemaplan.Children); ok {
			for _, child := range children.Nodes {
				if !collect(child) {
					return false
				}
			}
		}
		if fields, ok := node.Get("properties").(schemaplan.Properties); ok {
			for _, property := range fields.Entries {
				if previous, exists := properties[property.Name]; exists {
					if !reflect.DeepEqual(previous, property.Schema) {
						return false
					}
					merged := &schemaplan.Node{Fields: append([]schemaplan.Field(nil), property.Schema.Fields...)}
					annotation := make(map[string]any)
					if current, ok := planner.annotations[property.Schema].(map[string]any); ok {
						for key, value := range current {
							annotation[key] = value
						}
					}
					annotation["allOf"] = []any{planner.annotations[previous], planner.annotations[property.Schema]}
					planner.annotations[merged] = annotation
					planner.validators[merged] = planner.validators[property.Schema]
					planner.sources[merged] = planner.sources[property.Schema]
					property.Schema = merged
				}
				properties[property.Name] = property.Schema
			}
		}
		if names, ok := node.Get("required").(schemaplan.Literal); ok {
			for _, name := range names.Data.([]string) {
				required[name] = true
			}
		}
		if value, ok := node.Get("additionalProperties").(schemaplan.Literal); ok && value.Data == false {
			closed = true
		}
		if value, ok := node.Get("additionalProperties").(schemaplan.Child); ok {
			if additional == nil {
				additional = value.Node
			} else if !reflect.DeepEqual(additional, value.Node) {
				additional = &schemaplan.Node{}
			}
		}
		patterns = patterns || node.Get("patternProperties") != nil
		return true
	}
	if !collect(root) || !object {
		return nil, false
	}
	entries := make([]schemaplan.Property, 0, len(properties))
	for _, name := range sortedKeys(properties) {
		entries = append(entries, schemaplan.Property{Name: name, Schema: properties[name]})
	}
	names := make([]string, 0, len(required))
	for name := range required {
		names = append(names, name)
	}
	sort.Strings(names)
	result := &schemaplan.Node{Fields: []schemaplan.Field{{Name: "types", Value: schemaplan.Literal{Data: []string{"object"}}}, {Name: "properties", Value: schemaplan.Properties{Entries: entries}}, {Name: "required", Value: schemaplan.Literal{Data: names}}}}
	if closed {
		result.Fields = append(result.Fields, schemaplan.Field{Name: "additionalProperties", Value: schemaplan.Literal{Data: false}})
	} else if additional != nil {
		result.Fields = append(result.Fields, schemaplan.Field{Name: "additionalProperties", Value: schemaplan.Child{Node: additional}})
	}
	if patterns {
		result.Fields = append(result.Fields, schemaplan.Field{Name: "patternProperties", Value: schemaplan.SchemaMap{}})
	}
	return result, true
}
