package golang

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

// The shape graph is created once for a handler. Its validation functions call
// the existing generated programs directly; no source schema is interpreted.
func prepareParameterGraph(name string, roots map[string]*schemaplan.Node, schemas *schemaPlanner) string {
	var nodes []*schemaplan.Node
	bindings := make(map[*schemaplan.Node]string)
	references := make(map[string]*schemaplan.Node)
	for binding, node := range schemas.nodes {
		references["check"+binding] = node
	}
	var visit func(*schemaplan.Node)
	visit = func(node *schemaplan.Node) {
		if node == nil || bindings[node] != "" {
			return
		}
		bindings[node] = fmt.Sprintf("p%d", len(nodes))
		nodes = append(nodes, node)
		schemaplan.Walk(node, func(child *schemaplan.Node) {
			if child != node {
				visit(child)
			}
			if reference, ok := child.Get("reference").(schemaplan.Literal); ok {
				visit(references[reference.Data.(string)])
			}
		})
	}
	for _, key := range sortedKeys(roots) {
		visit(roots[key])
	}
	var body bytes.Buffer
	fmt.Fprintf(&body, "func %s()map[string]*runtime.ParameterSchema{\n", name)
	for _, node := range nodes {
		fmt.Fprintf(&body, "%s:=new(runtime.ParameterSchema)\n", bindings[node])
	}
	types := headerDecoderPlanner{schemas: schemas, references: references}
	pointer := func(node *schemaplan.Node) string {
		if node == nil {
			return "nil"
		}
		return bindings[node]
	}
	for _, node := range nodes {
		binding := bindings[node]
		flags := 0
		kinds := types.types(node, make(map[*schemaplan.Node]bool))
		for index, kind := range []string{"string", "boolean", "number", "array", "object"} {
			if kinds[kind] || kind == "number" && kinds["integer"] {
				flags |= 1 << index
			}
		}
		fmt.Fprintf(&body, "%s.Kinds=%d\n%s.Validate=func(ctx context.Context,value types.Value)error{_,err:=schema.%s(ctx,value,\"\");return err}\n", binding, flags, binding, schemas.nodeNames[node])
		if reference, ok := node.Get("reference").(schemaplan.Literal); ok {
			fmt.Fprintf(&body, "%s.Reference=%s\n", binding, pointer(references[reference.Data.(string)]))
		}
		if dynamic, ok := node.Get("dynamicReference").(schemaplan.DynamicReference); ok {
			fmt.Fprintf(&body, "%s.DynamicFallback=%s\n", binding, pointer(dynamic.Fallback))
		}
		for _, field := range []struct{ keyword, member string }{{"properties", "Properties"}, {"patternProperties", "Patterns"}} {
			if properties, ok := node.Get(field.keyword).(schemaplan.Properties); ok {
				fmt.Fprintf(&body, "%s.%s=map[string]*runtime.ParameterSchema{", binding, field.member)
				for _, property := range properties.Entries {
					fmt.Fprintf(&body, "%q:%s,", property.Name, pointer(property.Schema))
				}
				fmt.Fprintln(&body, "}")
			}
			if patterns, ok := node.Get(field.keyword).(schemaplan.SchemaMap); ok {
				for _, property := range patterns.Entries {
					var matches []string
					for _, program := range schemas.programs {
						for _, pattern := range program.Patterns {
							if pattern.Pattern == property.Name {
								matches = append(matches, pattern.Name)
							}
						}
					}
					sort.Strings(matches)
					if len(matches) == 0 {
						panic("prepared schema pattern has no validator binding")
					}
					fmt.Fprintf(&body, "%s.Patterns=append(%s.Patterns,runtime.ParameterPattern{Match:schema.%s.Match,Schema:%s})\n", binding, binding, matches[0], pointer(property.Schema))
				}
			}
		}
		for _, field := range []struct{ keyword, member string }{{"additionalProperties", "Additional"}, {"items", "Items"}, {"then", "Then"}, {"else", "Else"}} {
			if child, ok := node.Get(field.keyword).(schemaplan.Child); ok {
				fmt.Fprintf(&body, "%s.%s=%s\n", binding, field.member, pointer(child.Node))
			}
		}
		if node.Get("if") != nil {
			fmt.Fprintf(&body, "%s.Conditional=true\n", binding)
		}
		for _, field := range []struct{ keyword, member string }{{"prefixItems", "PrefixItems"}, {"allOf", "AllOf"}, {"oneOf", "OneOf"}, {"anyOf", "AnyOf"}} {
			if children, ok := node.Get(field.keyword).(schemaplan.Children); ok {
				members := make([]string, len(children.Nodes))
				for index, child := range children.Nodes {
					members[index] = pointer(child)
				}
				fmt.Fprintf(&body, "%s.%s=[]*runtime.ParameterSchema{%s}\n", binding, field.member, strings.Join(members, ","))
			}
		}
	}
	fmt.Fprintln(&body, "return map[string]*runtime.ParameterSchema{")
	for _, key := range sortedKeys(roots) {
		fmt.Fprintf(&body, "%s:%s,\n", strconv.Quote(key), pointer(roots[key]))
	}
	fmt.Fprintln(&body, "}}")
	return body.String()
}
