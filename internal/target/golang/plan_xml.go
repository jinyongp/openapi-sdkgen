package golang

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"openapi-sdkgen/internal/diagnostic"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

type xmlProgram struct{ Name, Body string }
type xmlDecodeKey struct {
	Node       *schemaplan.Node
	Validation string
}
type xmlPlanner struct {
	schemas    *schemaPlanner
	programs   []xmlProgram
	bindings   map[xmlDecodeKey]string
	references map[string]*schemaplan.Node
}

func (planner *schemaPlanner) prepareXML() ([]xmlProgram, *diagnostic.Diagnostic) {
	if len(planner.xmlRequests) == 0 {
		return nil, nil
	}
	xml := xmlPlanner{schemas: planner, bindings: make(map[xmlDecodeKey]string), references: make(map[string]*schemaplan.Node)}
	for name, node := range planner.nodes {
		xml.references["check"+name] = node
	}
	roots := make([]*schemaplan.Node, 0, len(planner.xmlRequests))
	for node := range planner.xmlRequests {
		roots = append(roots, node)
	}
	sort.Slice(roots, func(i, j int) bool { return planner.xmlRequests[roots[i]] < planner.xmlRequests[roots[j]] })
	for _, node := range roots {
		if _, err := xml.function(node, planner.nodeNames[node], planner.xmlRequests[node]); err != nil {
			return nil, &diagnostic.Diagnostic{Severity: diagnostic.SeverityError, Phase: diagnostic.PhaseTarget, Target: "golang", Code: "GO_XML_REPRESENTATION", Location: diagnostic.Location{Pointer: planner.sources[node]}, Message: err.Error()}
		}
	}
	return xml.programs, nil
}

// representation resolves the current element only. Child schemas remain lazy,
// so recursive XML models retain the original resource and validation boundaries.
func (planner *xmlPlanner) representation(root *schemaplan.Node) (*schemaplan.Node, error) {
	active := make(map[*schemaplan.Node]bool)
	var visit func(*schemaplan.Node) (*schemaplan.Node, error)
	visit = func(node *schemaplan.Node) (*schemaplan.Node, error) {
		if active[node] {
			return &schemaplan.Node{}, nil
		}
		active[node] = true
		defer delete(active, node)
		parents := make([]*schemaplan.Node, 0)
		if ref, ok := node.Get("reference").(schemaplan.Literal); ok {
			target := planner.references[ref.Data.(string)]
			if target == nil {
				return nil, fmt.Errorf("missing XML schema reference %s", ref.Data)
			}
			parents = append(parents, target)
		}
		if all, ok := node.Get("allOf").(schemaplan.Children); ok {
			parents = append(parents, all.Nodes...)
		}
		fields := make(map[string]schemaplan.Value)
		properties := make(map[string]*schemaplan.Node)
		inheritedXML := make(map[string]any)
		merge := func(current *schemaplan.Node, parent bool) error {
			for _, field := range current.Fields {
				switch field.Name {
				case "reference", "allOf":
					continue
				case "properties":
					for _, property := range field.Value.(schemaplan.Properties).Entries {
						if previous := properties[property.Name]; previous != nil {
							properties[property.Name] = &schemaplan.Node{Fields: []schemaplan.Field{{Name: "allOf", Value: schemaplan.Children{Nodes: []*schemaplan.Node{previous, property.Schema}}}}}
						} else {
							properties[property.Name] = property.Schema
						}
					}
				case "xml":
					metadata, _ := field.Value.(schemaplan.Literal).Data.(map[string]any)
					if parent {
						for _, key := range []string{"name", "namespace", "prefix", "attribute", "nodeType", "wrapped"} {
							value, present := metadata[key]
							if !present {
								continue
							}
							if old, exists := inheritedXML[key]; exists && !reflect.DeepEqual(old, value) {
								return fmt.Errorf("ambiguous inherited XML %s", key)
							}
							inheritedXML[key] = value
						}
					}
					combined := make(map[string]any)
					if previous, ok := fields["xml"].(schemaplan.Literal); ok {
						for key, value := range previous.Data.(map[string]any) {
							combined[key] = value
						}
					}
					for _, key := range []string{"name", "namespace", "prefix", "attribute", "nodeType", "wrapped"} {
						if value, present := metadata[key]; present {
							combined[key] = value
						}
					}
					fields["xml"] = schemaplan.Literal{Data: combined}
				default:
					fields[field.Name] = field.Value
				}
			}
			return nil
		}
		for _, parent := range parents {
			current, err := visit(parent)
			if err != nil {
				return nil, err
			}
			if err := merge(current, true); err != nil {
				return nil, err
			}
		}
		if err := merge(node, false); err != nil {
			return nil, err
		}
		result := &schemaplan.Node{}
		for _, name := range sortedKeys(fields) {
			result.Fields = append(result.Fields, schemaplan.Field{Name: name, Value: fields[name]})
		}
		if len(properties) > 0 {
			entries := make([]schemaplan.Property, 0, len(properties))
			for _, name := range sortedKeys(properties) {
				entries = append(entries, schemaplan.Property{Name: name, Schema: properties[name]})
			}
			result.Fields = append(result.Fields, schemaplan.Field{Name: "properties", Value: schemaplan.Properties{Entries: entries}})
		}
		return result, nil
	}
	return visit(root)
}

func xmlMetadata(node *schemaplan.Node) map[string]any {
	metadata, _ := node.Get("xml").(schemaplan.Literal)
	value, _ := metadata.Data.(map[string]any)
	return value
}
func xmlString(metadata map[string]any, key string) string {
	value, _ := metadata[key].(string)
	return value
}
func xmlBool(metadata map[string]any, key string) bool {
	value, _ := metadata[key].(bool)
	return value
}
func xmlKinds(node *schemaplan.Node) []string {
	field, _ := node.Get("types").(schemaplan.Literal)
	kinds, _ := field.Data.([]string)
	return kinds
}
func xmlHasKind(node *schemaplan.Node, kind string) bool {
	for _, value := range xmlKinds(node) {
		if value == kind {
			return true
		}
	}
	return false
}

func xmlScalarKind(node *schemaplan.Node, kind string) bool {
	if xmlHasKind(node, kind) {
		return true
	}
	var values []any
	if field, ok := node.Get("constValue").(schemaplan.Literal); ok {
		values = append(values, field.Data)
	}
	if field, ok := node.Get("enumValues").(schemaplan.Literal); ok {
		values = append(values, field.Data.([]any)...)
	}
	for _, value := range values {
		switch value.(type) {
		case float64:
			if kind == "number" {
				return true
			}
		case bool:
			if kind == "boolean" {
				return true
			}
		}
	}
	return false
}
func xmlMatch(expression, context string, metadata map[string]any, fallback string, attribute bool) string {
	name := xmlString(metadata, "name")
	if name == "" {
		name = fallback
	}
	_, prefix := metadata["prefix"]
	_, namespace := metadata["namespace"]
	return fmt.Sprintf("wire.XMLMatches(%s,%s,%s,%s,%s,%t,%t,%t)", expression, context, strconv.Quote(name), strconv.Quote(xmlString(metadata, "prefix")), strconv.Quote(xmlString(metadata, "namespace")), prefix, namespace, attribute)
}

func (planner *xmlPlanner) function(node *schemaplan.Node, validation, requested string) (string, error) {
	key := xmlDecodeKey{Node: node, Validation: validation}
	if binding := planner.bindings[key]; binding != "" {
		return binding, nil
	}
	binding := requested
	if binding == "" {
		binding = fmt.Sprintf("decodeXMLNode%d", len(planner.bindings))
	}
	planner.bindings[key] = binding
	representation, err := planner.representation(node)
	if err != nil {
		return "", err
	}
	var body bytes.Buffer
	if anchor, ok := representation.Get("dynamicAnchor").(schemaplan.Literal); ok {
		fmt.Fprintf(&body, "state.scope=append(state.scope,xmlDynamicBinding{Anchor:%s,Decode:%s});defer func(){state.scope=state.scope[:len(state.scope)-1]}()\n", strconv.Quote(anchor.Data.(string)), binding)
		if original := planner.schemas.nodeNames[node]; original != "" {
			fmt.Fprintf(&body, "state.validation.scope=append(state.validation.scope,DynamicBinding{Anchor:%s,Validate:check%s});defer func(){state.validation.scope=state.validation.scope[:len(state.validation.scope)-1]}()\n", strconv.Quote(anchor.Data.(string)), original)
		}
	}
	if dynamic, ok := node.Get("dynamicReference").(schemaplan.DynamicReference); ok {
		fallback, err := planner.function(dynamic.Fallback, planner.schemas.nodeNames[dynamic.Fallback], "")
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&body, "target:= %s;for _,binding:=range state.scope{if binding.Anchor==%s{target=binding.Decode;break}};decoded,err:=target(ctx,node,path,state);if err!=nil{return wire.Value{},err};", fallback, strconv.Quote(dynamic.Anchor))
		if validation != "" {
			fmt.Fprintf(&body, "if _,err:=check%s(ctx,decoded,path,state.validation);err!=nil{return wire.Value{},err};", validation)
		}
		fmt.Fprintln(&body, "return decoded,nil")
		planner.programs = append(planner.programs, xmlProgram{Name: binding, Body: body.String()})
		return binding, nil
	}
	validate := func(value string) string {
		if validation == "" {
			return ""
		}
		return fmt.Sprintf("if _,err:=check%s(ctx,%s,path,state.validation);err!=nil{return wire.Value{},err};", validation, value)
	}
	for _, keyword := range []string{"oneOf", "anyOf"} {
		if branches, ok := representation.Get(keyword).(schemaplan.Children); ok {
			base := &schemaplan.Node{}
			for _, field := range representation.Fields {
				if field.Name != keyword {
					base.Fields = append(base.Fields, field)
				}
			}
			fmt.Fprintln(&body, "var decoded wire.Value;matched:=false;fields:=make(map[string]wire.Value)")
			for _, branch := range branches.Nodes {
				combined := &schemaplan.Node{Fields: []schemaplan.Field{{Name: "allOf", Value: schemaplan.Children{Nodes: []*schemaplan.Node{base, branch}}}}}
				child, err := planner.function(combined, validation, "")
				if err != nil {
					return "", err
				}
				branchValidation := planner.schemas.nodeNames[branch]
				fmt.Fprintf(&body, "{candidate,err:=%s(ctx,node,path,state);if err==nil{", child)
				if branchValidation != "" {
					fmt.Fprintf(&body, "_,err=check%s(ctx,candidate,path,state.validation);", branchValidation)
				}
				fmt.Fprintln(&body, "if err==nil{if !matched{decoded=candidate};matched=true;if decoded.Kind()==wire.Object&&candidate.Kind()==wire.Object{for name,value:=range candidate.Fields(){fields[name]=value}}}}else if !IsValidationError(err){return wire.Value{},err}}")
			}
			fmt.Fprintln(&body, "if !matched{return wire.Value{},Failure(\"XML composition\",path)};if decoded.Kind()==wire.Object{decoded=wire.Record(fields)}")
			body.WriteString(validate("decoded"))
			fmt.Fprintln(&body, "return decoded,nil")
			planner.programs = append(planner.programs, xmlProgram{Name: binding, Body: body.String()})
			return binding, nil
		}
	}
	if condition, ok := representation.Get("if").(schemaplan.Child); ok {
		base := &schemaplan.Node{}
		for _, field := range representation.Fields {
			if field.Name != "if" && field.Name != "then" && field.Name != "else" {
				base.Fields = append(base.Fields, field)
			}
		}
		probe, err := planner.function(&schemaplan.Node{Fields: []schemaplan.Field{{Name: "allOf", Value: schemaplan.Children{Nodes: []*schemaplan.Node{base, condition.Node}}}}}, planner.schemas.nodeNames[condition.Node], "")
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&body, "_,conditionErr:=%s(ctx,node,path,state);if conditionErr!=nil&&!IsValidationError(conditionErr){return wire.Value{},conditionErr}\n", probe)
		for index, keyword := range []string{"then", "else"} {
			target := base
			if branch, ok := representation.Get(keyword).(schemaplan.Child); ok {
				target = &schemaplan.Node{Fields: []schemaplan.Field{{Name: "allOf", Value: schemaplan.Children{Nodes: []*schemaplan.Node{base, branch.Node}}}}}
			}
			child, err := planner.function(target, validation, "")
			if err != nil {
				return "", err
			}
			if index == 0 {
				fmt.Fprintf(&body, "if conditionErr==nil{return %s(ctx,node,path,state)}\n", child)
			} else {
				fmt.Fprintf(&body, "return %s(ctx,node,path,state)\n", child)
			}
		}
	} else if xmlHasKind(representation, "array") {
		item := &schemaplan.Node{}
		if field, ok := representation.Get("items").(schemaplan.Child); ok {
			item = field.Node
		}
		child, err := planner.function(item, planner.schemas.nodeNames[item], "")
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&body, "items:=make([]wire.Value,0,len(node.Children));for index,element:=range node.Children{item,err:=%s(ctx,element,IndexPath(path,index),state);if err!=nil{return wire.Value{},err};items=append(items,item)};decoded:=wire.List(items);", child)
		body.WriteString(validate("decoded"))
		fmt.Fprintln(&body, "return decoded,nil")
	} else if xmlHasKind(representation, "object") || representation.Get("properties") != nil {
		fmt.Fprintln(&body, "fields:=make(map[string]wire.Value)")
		properties, _ := representation.Get("properties").(schemaplan.Properties)
		for _, property := range properties.Entries {
			shape, err := planner.representation(property.Schema)
			if err != nil {
				return "", err
			}
			metadata := xmlMetadata(shape)
			nodeType := xmlString(metadata, "nodeType")
			if nodeType == "none" {
				continue
			}
			child, err := planner.function(property.Schema, planner.schemas.nodeNames[property.Schema], "")
			if err != nil {
				return "", err
			}
			name := strconv.Quote(property.Name)
			path := "ChildPath(path," + name + ")"
			switch {
			case nodeType == "text" || nodeType == "cdata":
				fmt.Fprintf(&body, "{value,err:=%s(ctx,&wire.XMLNode{Text:node.Text},%s,state);if err!=nil{return wire.Value{},err};fields[%s]=value}\n", child, path, name)
			case xmlBool(metadata, "attribute") || nodeType == "attribute":
				fmt.Fprintf(&body, "for _,attribute:=range node.Attributes{if %s{value,err:=%s(ctx,&wire.XMLNode{Text:attribute.Value},%s,state);if err!=nil{return wire.Value{},err};fields[%s]=value;break}}\n", xmlMatch("attribute.Name", "node", metadata, property.Name, true), child, path, name)
			case xmlHasKind(shape, "array"):
				item := &schemaplan.Node{}
				if field, ok := shape.Get("items").(schemaplan.Child); ok {
					item = field.Node
				}
				itemShape, err := planner.representation(item)
				if err != nil {
					return "", err
				}
				itemMetadata := xmlMetadata(itemShape)
				itemName := xmlString(metadata, "name")
				if itemName == "" {
					itemName = property.Name
				}
				itemDecoder, err := planner.function(item, planner.schemas.nodeNames[item], "")
				if err != nil {
					return "", err
				}
				fmt.Fprintln(&body, "{container:=node")
				if xmlBool(metadata, "wrapped") || nodeType == "element" {
					fmt.Fprintf(&body, "container=nil;for _,element:=range node.Children{if %s{container=element;break}}\n", xmlMatch("element.Name", "element", metadata, property.Name, false))
				}
				fmt.Fprintf(&body, "if container!=nil{items:=make([]wire.Value,0);for _,element:=range container.Children{if %s{value,err:=%s(ctx,element,IndexPath(%s,len(items)),state);if err!=nil{return wire.Value{},err};items=append(items,value)}};fields[%s]=wire.List(items)}}\n", xmlMatch("element.Name", "element", itemMetadata, itemName, false), itemDecoder, path, name)
			default:
				fmt.Fprintf(&body, "for _,element:=range node.Children{if %s{value,err:=%s(ctx,element,%s,state);if err!=nil{return wire.Value{},err};fields[%s]=value;break}}\n", xmlMatch("element.Name", "element", metadata, property.Name, false), child, path, name)
			}
		}
		fmt.Fprint(&body, "decoded:=wire.Record(fields);")
		body.WriteString(validate("decoded"))
		fmt.Fprintln(&body, "return decoded,nil")
	} else {
		// Scalar coercion tries converted candidates before the original string,
		// and every candidate passes the original complete validation program.
		if xmlScalarKind(representation, "number") || xmlScalarKind(representation, "integer") {
			fmt.Fprint(&body, "if candidate,ok:=wire.ScalarNumber(node.Text);ok{")
			if validation != "" {
				fmt.Fprintf(&body, "if _,err:=check%s(ctx,candidate,path,state.validation);err==nil{return candidate,nil}else if !IsValidationError(err){return wire.Value{},err}", validation)
			} else {
				fmt.Fprint(&body, "return candidate,nil")
			}
			fmt.Fprintln(&body, "}")
		}
		if xmlScalarKind(representation, "boolean") {
			fmt.Fprint(&body, "if node.Text==\"true\"||node.Text==\"false\"{candidate:=wire.Bool(node.Text==\"true\");")
			if validation != "" {
				fmt.Fprintf(&body, "if _,err:=check%s(ctx,candidate,path,state.validation);err==nil{return candidate,nil}else if !IsValidationError(err){return wire.Value{},err}", validation)
			} else {
				fmt.Fprint(&body, "return candidate,nil")
			}
			fmt.Fprintln(&body, "}")
		}
		fmt.Fprint(&body, "decoded:=wire.Text(node.Text);")
		body.WriteString(validate("decoded"))
		fmt.Fprintln(&body, "return decoded,nil")
	}
	planner.programs = append(planner.programs, xmlProgram{Name: binding, Body: strings.TrimSpace(body.String())})
	return binding, nil
}
