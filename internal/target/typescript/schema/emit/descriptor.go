// Package emit writes TypeScript from the immutable semantic schema plan.
package emit

import (
	"fmt"
	"openapi-sdkgen/internal/target/typescript/schema/plan"
	"strings"
)

type PropertyExpression struct {
	Name       string
	Expression string
}
type DescriptorOptions struct {
	Literal    func(any) (string, error)
	Properties func([]PropertyExpression) (string, error)
	Program    func(*plan.Node) (string, error)
}

func Descriptor(node *plan.Node, options DescriptorOptions) (string, error) {
	fields := make([]string, 0, len(node.Fields)+1)
	for _, field := range node.Fields {
		value, err := descriptorValue(field.Value, options)
		if err != nil {
			return "", err
		}
		fields = append(fields, field.Name+": "+value)
	}
	if options.Program != nil {
		program, err := options.Program(node)
		if err != nil {
			return "", err
		}
		fields = append(fields, "program: "+program)
	}
	if len(fields) == 0 {
		return "{}", nil
	}
	return "{ " + strings.Join(fields, ", ") + " }", nil
}
func descriptorValue(value plan.Value, options DescriptorOptions) (string, error) {
	switch typed := value.(type) {
	case plan.Literal:
		return options.Literal(typed.Data)
	case plan.Child:
		return Descriptor(typed.Node, options)
	case plan.Children:
		var items []string
		for _, node := range typed.Nodes {
			item, err := Descriptor(node, options)
			if err != nil {
				return "", err
			}
			items = append(items, item)
		}
		return "[" + strings.Join(items, ", ") + "]", nil
	case plan.Properties:
		var entries []PropertyExpression
		for _, property := range typed.Entries {
			child, err := Descriptor(property.Schema, options)
			if err != nil {
				return "", err
			}
			entries = append(entries, PropertyExpression{Name: property.Name, Expression: child})
		}
		return options.Properties(entries)
	case plan.SchemaMap:
		return descriptorMap(typed.Entries, options)
	case plan.DynamicReference:
		anchor, err := options.Literal(typed.Anchor)
		if err != nil {
			return "", err
		}
		fallback, err := Descriptor(typed.Fallback, options)
		if err != nil {
			return "", err
		}
		return "{ anchor: " + anchor + ", fallback: " + fallback + " }", nil
	case plan.Discriminator:
		property, err := options.Literal(typed.Property)
		if err != nil {
			return "", err
		}
		result := "{ property: " + property
		if len(typed.Mapping) > 0 {
			mapping, err := descriptorMap(typed.Mapping, options)
			if err != nil {
				return "", err
			}
			result += ", mapping: " + mapping
		}
		if typed.Default != nil {
			child, err := Descriptor(typed.Default, options)
			if err != nil {
				return "", err
			}
			result += ", defaultMapping: " + child
		}
		return result + " }", nil
	default:
		return "", fmt.Errorf("unknown schema plan value %T", value)
	}
}
func descriptorMap(entries []plan.Property, options DescriptorOptions) (string, error) {
	var items []string
	for _, entry := range entries {
		key, err := options.Literal(entry.Name)
		if err != nil {
			return "", err
		}
		value, err := Descriptor(entry.Schema, options)
		if err != nil {
			return "", err
		}
		items = append(items, "["+key+", "+value+"]")
	}
	return "/* @__PURE__ */ Object.fromEntries([" + strings.Join(items, ", ") + "])", nil
}
