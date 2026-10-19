package golang

import (
	"bytes"
	"encoding/json"
	"fmt"
	schemaplan "openapi-sdkgen/internal/schema/plan"
	"strings"
)

type headerDecoderPlanner struct {
	schemas    *schemaPlanner
	bindings   map[*schemaplan.Node]string
	references map[string]*schemaplan.Node
	programs   []string
	patterns   map[string]string
	err        error
}

func prepareHeaderDecoders(operations []operationPlan, schemas *schemaPlanner) (string, error) {
	planner := headerDecoderPlanner{schemas: schemas, bindings: make(map[*schemaplan.Node]string), references: make(map[string]*schemaplan.Node), patterns: make(map[string]string)}
	for _, program := range schemas.programs {
		for _, pattern := range program.Patterns {
			if planner.patterns[pattern.Pattern] == "" {
				planner.patterns[pattern.Pattern] = pattern.Name
			}
		}
	}
	for name, node := range schemas.nodes {
		planner.references["check"+name] = node
	}
	for operationIndex := range operations {
		for fieldIndex := range operations[operationIndex].MultipartHeaders {
			field := &operations[operationIndex].MultipartHeaders[fieldIndex]
			field.Decoder = planner.function(schemas.nodes[field.Validation])
		}
		for groupIndex := range operations[operationIndex].Headers {
			for fieldIndex := range operations[operationIndex].Headers[groupIndex].Fields {
				field := &operations[operationIndex].Headers[groupIndex].Fields[fieldIndex]
				field.Decoder = planner.function(schemas.nodes[field.Validation])
			}
		}
	}
	return strings.Join(planner.programs, "\n"), planner.err
}

func (planner *headerDecoderPlanner) shapes(node *schemaplan.Node, active map[*schemaplan.Node]bool) []*schemaplan.Node {
	if node == nil || active[node] {
		return nil
	}
	active[node] = true
	defer delete(active, node)
	result := []*schemaplan.Node{node}
	if reference, ok := node.Get("reference").(schemaplan.Literal); ok {
		result = append(result, planner.shapes(planner.references[reference.Data.(string)], active)...)
	}
	if children, ok := node.Get("allOf").(schemaplan.Children); ok {
		for _, child := range children.Nodes {
			result = append(result, planner.shapes(child, active)...)
		}
	}
	return result
}

func (planner *headerDecoderPlanner) types(node *schemaplan.Node, active map[*schemaplan.Node]bool) map[string]bool {
	result := make(map[string]bool)
	if node == nil || active[node] {
		return result
	}
	active[node] = true
	defer delete(active, node)
	if literal, ok := node.Get("types").(schemaplan.Literal); ok {
		for _, value := range literal.Data.([]string) {
			result[value] = true
		}
	}
	addValue := func(value any) {
		switch value.(type) {
		case string:
			result["string"] = true
		case bool:
			result["boolean"] = true
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json.Number:
			result["number"] = true
		case []any:
			result["array"] = true
		case map[string]any:
			result["object"] = true
		}
	}
	if literal, ok := node.Get("constValue").(schemaplan.Literal); ok {
		addValue(literal.Data)
	}
	if literal, ok := node.Get("enumValues").(schemaplan.Literal); ok {
		for _, value := range literal.Data.([]any) {
			addValue(value)
		}
	}
	for _, key := range []string{"prefixItems", "items", "contains", "minItems", "maxItems", "uniqueItems"} {
		if node.Get(key) != nil {
			result["array"] = true
		}
	}
	for _, key := range []string{"properties", "patternProperties", "additionalProperties", "required", "minProperties", "maxProperties"} {
		if node.Get(key) != nil {
			result["object"] = true
		}
	}
	merge := func(child *schemaplan.Node) {
		for value := range planner.types(child, active) {
			result[value] = true
		}
	}
	if reference, ok := node.Get("reference").(schemaplan.Literal); ok {
		merge(planner.references[reference.Data.(string)])
	}
	if dynamic, ok := node.Get("dynamicReference").(schemaplan.DynamicReference); ok {
		merge(dynamic.Fallback)
	}
	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		if children, ok := node.Get(key).(schemaplan.Children); ok {
			for _, child := range children.Nodes {
				merge(child)
			}
		}
	}
	for _, key := range []string{"then", "else"} {
		if child, ok := node.Get(key).(schemaplan.Child); ok {
			merge(child.Node)
		}
	}
	return result
}

func (planner *headerDecoderPlanner) rule(node *schemaplan.Node) string {
	flags := 0
	types := planner.types(node, make(map[*schemaplan.Node]bool))
	if types["number"] || types["integer"] {
		flags |= 1
	}
	if types["boolean"] {
		flags |= 2
	}
	return fmt.Sprintf("HeaderScalarRule{Validate:check%s,Types:%d,MediaType:%q}", planner.schemas.nodeNames[node], flags, planner.schemas.programMedia[planner.schemas.nodeNames[node]])
}

func (planner *headerDecoderPlanner) function(node *schemaplan.Node) string {
	if name := planner.bindings[node]; name != "" {
		return name
	}
	name := "DecodeHeader" + planner.schemas.nodeNames[node]
	planner.bindings[node] = name
	validator := "check" + planner.schemas.nodeNames[node]
	var body bytes.Buffer
	fmt.Fprintf(&body, "func %s(ctx context.Context,text string,explode bool)(wire.Value,error){return decode%s(ctx,text,explode,make(map[string]bool))}\n", name, name)
	fmt.Fprintf(&body, "func decode%s(ctx context.Context,text string,explode bool,active map[string]bool)(wire.Value,error){\nif err:=ctx.Err();err!=nil{return wire.Value{},err};if active[%q]{return wire.Value{},Failure(\"header\",\"\")};active[%q]=true;defer delete(active,%q)\n", name, name, name, name)
	types := planner.types(node, make(map[*schemaplan.Node]bool))
	shapes := planner.shapes(node, make(map[*schemaplan.Node]bool))
	attempt := func(code string) {
		fmt.Fprintf(&body, "{candidate,err:=func()(wire.Value,error){%s}();if err==nil{if ok,err:=HeaderAccept(ctx,candidate,%s,%q);err!=nil{return wire.Value{},err}else if ok{return candidate,nil}}else if !IsValidationError(err){return wire.Value{},err}}\n", code, validator, planner.schemas.programMedia[planner.schemas.nodeNames[node]])
	}
	if types["array"] {
		var code bytes.Buffer
		fmt.Fprintln(&code, "tokens:=strings.Split(text,\",\");items:=make([]wire.Value,len(tokens));for index,token:=range tokens{_=index;rules:=[]HeaderScalarRule{}")
		for _, shape := range shapes {
			prefix, _ := shape.Get("prefixItems").(schemaplan.Children)
			for index, child := range prefix.Nodes {
				fmt.Fprintf(&code, "if index==%d{rules=append(rules,%s)}\n", index, planner.rule(child))
			}
			if child, ok := shape.Get("items").(schemaplan.Child); ok {
				fmt.Fprintf(&code, "if index>=%d{rules=append(rules,%s)}\n", len(prefix.Nodes), planner.rule(child.Node))
			}
		}
		fmt.Fprintln(&code, "value,err:=HeaderScalar(ctx,token,rules);if err!=nil{return wire.Value{},err};items[index]=value};return wire.List(items),nil")
		attempt(code.String())
	}
	if types["object"] {
		var code bytes.Buffer
		fmt.Fprintln(&code, "tokens,err:=HeaderObjectTokens(text,explode);if err!=nil{return wire.Value{},err};fields:=make(map[string]wire.Value,len(tokens));for key,token:=range tokens{rules:=[]HeaderScalarRule{}")
		for _, shape := range shapes {
			fmt.Fprintln(&code, "{matched:=false;_=matched")
			if properties, ok := shape.Get("properties").(schemaplan.Properties); ok {
				for _, property := range properties.Entries {
					fmt.Fprintf(&code, "if key==%q{matched=true;rules=append(rules,%s)}\n", property.Name, planner.rule(property.Schema))
				}
			}
			if patterns, ok := shape.Get("patternProperties").(schemaplan.SchemaMap); ok {
				for _, property := range patterns.Entries {
					// Header coercion uses the same immutable compiled pattern
					// as the schema validator; it never recompiles source.
					pattern := planner.patterns[property.Name]
					if pattern == "" {
						planner.err = fmt.Errorf("missing prepared header pattern %q", property.Name)
						return ""
					}
					fmt.Fprintf(&code, "if ok,err:=%s.Match(ctx,key);err!=nil{return wire.Value{},err}else if ok{matched=true;rules=append(rules,%s)}\n", pattern, planner.rule(property.Schema))
				}
			}
			if additional, ok := shape.Get("additionalProperties").(schemaplan.Child); ok {
				fmt.Fprintf(&code, "if !matched{rules=append(rules,%s)}\n", planner.rule(additional.Node))
			}
			fmt.Fprintln(&code, "}")
		}
		fmt.Fprintln(&code, "value,err:=HeaderScalar(ctx,token,rules);if err!=nil{return wire.Value{},err};fields[key]=value};return wire.CheckedRecord(fields)")
		attempt(code.String())
	}
	children := make([]*schemaplan.Node, 0)
	if reference, ok := node.Get("reference").(schemaplan.Literal); ok {
		if target := planner.references[reference.Data.(string)]; target != nil {
			children = append(children, target)
		}
	}
	if dynamic, ok := node.Get("dynamicReference").(schemaplan.DynamicReference); ok {
		children = append(children, dynamic.Fallback)
	}
	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		if nested, ok := node.Get(key).(schemaplan.Children); ok {
			children = append(children, nested.Nodes...)
		}
	}
	for _, key := range []string{"then", "else"} {
		if child, ok := node.Get(key).(schemaplan.Child); ok {
			children = append(children, child.Node)
		}
	}
	for _, child := range children {
		function := planner.function(child)
		attempt(fmt.Sprintf("return decode%s(ctx,text,explode,active)", function))
	}
	fmt.Fprintf(&body, "return HeaderScalar(ctx,text,[]HeaderScalarRule{%s})\n}\n", planner.rule(node))
	planner.programs = append(planner.programs, body.String())
	return name
}
