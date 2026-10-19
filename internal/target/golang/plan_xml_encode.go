package golang

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

type xmlEncodeProgram struct{ Name, Body string }
type xmlEncodePlanner struct {
	xml      xmlPlanner
	bindings map[*schemaplan.Node]string
	programs []xmlEncodeProgram
	any      *schemaplan.Node
}

func (planner *xmlEncodePlanner) function(node *schemaplan.Node) (string, error) {
	if binding := planner.bindings[node]; binding != "" {
		return binding, nil
	}
	binding := fmt.Sprintf("encodeXMLNode%d", len(planner.bindings))
	planner.bindings[node] = binding
	shape, err := planner.xml.representation(node)
	if err != nil {
		return "", err
	}
	var body bytes.Buffer
	if anchor, ok := shape.Get("dynamicAnchor").(schemaplan.Literal); ok {
		fmt.Fprintf(&body, "state.scope=append(state.scope,xmlEncodeBinding{Anchor:%s,Encode:%s});defer func(){state.scope=state.scope[:len(state.scope)-1]}()\n", strconv.Quote(anchor.Data.(string)), binding)
		if original := planner.xml.schemas.nodeNames[node]; original != "" {
			fmt.Fprintf(&body, "state.validation.scope=append(state.validation.scope,DynamicBinding{Anchor:%s,Validate:check%s});defer func(){state.validation.scope=state.validation.scope[:len(state.validation.scope)-1]}()\n", strconv.Quote(anchor.Data.(string)), original)
		}
	}
	if dynamic, ok := node.Get("dynamicReference").(schemaplan.DynamicReference); ok {
		fallback, err := planner.function(dynamic.Fallback)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&body, "target:=%s;for _,binding:=range state.scope{if binding.Anchor==%s{target=binding.Encode;break}};mapping,err:=target(ctx,value,path,state);if err!=nil{return nil,err};metadata:=%s\n", fallback, strconv.Quote(dynamic.Anchor), xmlOutputMetadata(shape))
		for _, field := range []string{"Name", "Prefix", "Namespace", "Attribute", "Wrapped", "NodeType"} {
			fmt.Fprintf(&body, "if metadata.Defined&wire.XMLDefined%s!=0{mapping.Metadata.%s=metadata.%s;mapping.Metadata.Defined|=wire.XMLDefined%s}\n", field, field, field, field)
		}
		fmt.Fprintln(&body, "return mapping,nil")
		planner.programs = append(planner.programs, xmlEncodeProgram{Name: binding, Body: body.String()})
		return binding, nil
	}
	for _, keyword := range []string{"oneOf", "anyOf"} {
		branches, ok := shape.Get(keyword).(schemaplan.Children)
		if !ok {
			continue
		}
		base := &schemaplan.Node{}
		for _, field := range shape.Fields {
			if field.Name != keyword {
				base.Fields = append(base.Fields, field)
			}
		}
		encodeBase, err := planner.function(base)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&body, "result,err:=%s(ctx,value,path,state);if err!=nil{return nil,err};matches:=0\n", encodeBase)
		for _, branch := range branches.Nodes {
			child, err := planner.function(branch)
			if err != nil {
				return "", err
			}
			fmt.Fprint(&body, "{matched:=true;")
			if validator := planner.xml.schemas.nodeNames[branch]; validator != "" {
				fmt.Fprintf(&body, "if _,err:=check%s(ctx,value,path,state.validation);err!=nil{if !IsValidationError(err){return nil,err};matched=false};", validator)
			}
			fmt.Fprintf(&body, "if matched{matches++;candidate,err:=%s(ctx,value,path,state);if err!=nil{return nil,err};result,err=wire.MergeXMLMappings(result,candidate);if err!=nil{return nil,err}}}\n", child)
		}
		condition := "matches==0"
		if keyword == "oneOf" {
			condition = "matches!=1"
		}
		fmt.Fprintf(&body, "if %s{return nil,Failure(%s,path)};return result,nil\n", condition, strconv.Quote(keyword))
		planner.programs = append(planner.programs, xmlEncodeProgram{Name: binding, Body: body.String()})
		return binding, nil
	}
	if condition, ok := shape.Get("if").(schemaplan.Child); ok {
		base := &schemaplan.Node{}
		for _, field := range shape.Fields {
			if field.Name != "if" && field.Name != "then" && field.Name != "else" {
				base.Fields = append(base.Fields, field)
			}
		}
		encodeBase, err := planner.function(base)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&body, "result,err:=%s(ctx,value,path,state);if err!=nil{return nil,err};matched:=true\n", encodeBase)
		if validator := planner.xml.schemas.nodeNames[condition.Node]; validator != "" {
			fmt.Fprintf(&body, "if _,err:=check%s(ctx,value,path,state.validation);err!=nil{if !IsValidationError(err){return nil,err};matched=false}\n", validator)
		}
		for index, keyword := range []string{"then", "else"} {
			branch, exists := shape.Get(keyword).(schemaplan.Child)
			if !exists {
				continue
			}
			child, err := planner.function(branch.Node)
			if err != nil {
				return "", err
			}
			test := "matched"
			if index == 1 {
				test = "!matched"
			}
			fmt.Fprintf(&body, "if %s{candidate,err:=%s(ctx,value,path,state);if err!=nil{return nil,err};result,err=wire.MergeXMLMappings(result,candidate);if err!=nil{return nil,err}}\n", test, child)
		}
		fmt.Fprintln(&body, "_=matched;return result,nil")
		planner.programs = append(planner.programs, xmlEncodeProgram{Name: binding, Body: body.String()})
		return binding, nil
	}
	fmt.Fprintf(&body, "result:=&wire.XMLMapping{Value:value,Path:path,Metadata:%s}\n", xmlOutputMetadata(shape))
	properties, _ := shape.Get("properties").(schemaplan.Properties)
	if len(properties.Entries) > 0 {
		fmt.Fprintln(&body, "if value.Kind()==wire.Object{")
		for _, property := range properties.Entries {
			child, err := planner.function(property.Schema)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&body, "if item,present:=value.Get(%s);present{child,err:=%s(ctx,item,ChildPath(path,%s),state);if err!=nil{return nil,err};result.Fields=append(result.Fields,child)}\n", strconv.Quote(property.Name), child, strconv.Quote(property.Name))
		}
		fmt.Fprintln(&body, "}")
	}
	item := planner.any
	if field, ok := shape.Get("items").(schemaplan.Child); ok {
		item = field.Node
	}
	child, err := planner.function(item)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&body, "if value.Kind()==wire.Array{for index,item:=range value.Items(){child,err:=%s(ctx,item,IndexPath(path,index),state);if err!=nil{return nil,err};result.Items=append(result.Items,child)}};return result,nil\n", child)
	planner.programs = append(planner.programs, xmlEncodeProgram{Name: binding, Body: body.String()})
	return binding, nil
}

func xmlOutputMetadata(node *schemaplan.Node) string {
	metadata := xmlMetadata(node)
	var flags uint8
	for index, key := range []string{"name", "prefix", "namespace", "attribute", "wrapped", "nodeType"} {
		if _, present := metadata[key]; present {
			flags |= 1 << index
		}
	}
	return fmt.Sprintf("wire.XMLOutputMetadata{Name:%s,Prefix:%s,Namespace:%s,NodeType:%s,Attribute:%t,Wrapped:%t,Defined:%d}", strconv.Quote(xmlString(metadata, "name")), strconv.Quote(xmlString(metadata, "prefix")), strconv.Quote(xmlString(metadata, "namespace")), strconv.Quote(xmlString(metadata, "nodeType")), xmlBool(metadata, "attribute"), xmlBool(metadata, "wrapped"), flags)
}

func prepareXMLBodyEncoders(operations []operationPlan, schemas *schemaPlanner) ([]xmlEncodeProgram, string, error) {
	planner := xmlEncodePlanner{xml: xmlPlanner{schemas: schemas, references: make(map[string]*schemaplan.Node)}, bindings: make(map[*schemaplan.Node]string), any: &schemaplan.Node{}}
	for name, node := range schemas.nodes {
		planner.xml.references["check"+name] = node
	}
	parts := multipartQueryPlanner{schemas: schemas, references: planner.xml.references, xml: &planner}
	var prepared strings.Builder
	prepare := func(model, validation string, nodes ...*schemaplan.Node) (string, error) {
		node := schemas.nodes[validation]
		if len(nodes) != 0 {
			node = nodes[0]
		}
		binding, err := planner.function(node)
		if err != nil {
			return "", err
		}
		name := "EncodeXMLHTTP__" + model + "__"
		fallback := "root"
		if source, ok := schemas.annotations[node].(map[string]any); ok {
			if reference, _ := source["$ref"].(string); reference != "" {
				if component, err := schemaplan.ComponentReferenceName(reference); err == nil {
					fallback = component
				}
			}
		}
		fmt.Fprintf(&prepared, "func %s(ctx context.Context,value wire.Value,contentType string)([]byte,error){state:=&xmlEncodeState{validation:&State{active:make(map[validationVisit]bool),MediaType:contentType}};if _,err:=check%s(ctx,value,\"\",state.validation);err!=nil{return nil,err};mapping,err:=%s(ctx,value,\"\",state);if err!=nil{return nil,err};return wire.RenderXML(ctx,mapping,%s)}\n", name, validation, binding, strconv.Quote(fallback))
		return name, nil
	}
	for operationIndex := range operations {
		operation := &operations[operationIndex]
		for _, part := range operation.MultipartParts {
			named, err := parts.partQuery(schemas.nodes[part.Validation], false)
			if err != nil {
				return nil, "", err
			}
			positional, err := parts.partQuery(schemas.nodes[part.Validation], true)
			if err != nil {
				return nil, "", err
			}
			fmt.Fprintf(&prepared, "func EncodeXMLPartQuery__%s__(ctx context.Context,value wire.Value,contentType,key,index string,positional bool)([]byte,error){state:=&xmlEncodeState{validation:&State{active:make(map[validationVisit]bool),MediaType:contentType}};query:=%s;if positional{query=%s};result,err:=query(ctx,key,index,value,state);if err!=nil{return nil,err};return wire.RenderXML(ctx,result.Mapping,result.Fallback)}\n", part.Validation, named, positional)
			fmt.Fprintf(&prepared, "func MultipartPartMedia__%s__(ctx context.Context,value wire.Value,key,index string,positional bool,binaries map[string]bool)(string,bool,error){state:=&xmlEncodeState{mediaOnly:true,validation:&State{active:make(map[validationVisit]bool),Binaries:binaries,OpaqueAncestors:opaqueMediaAncestors(binaries)}};query:=%s;if positional{query=%s};result,err:=query(ctx,key,index,value,state);return result.ContentType,result.Binary,err}\n", part.Validation, named, positional)
		}
		for index := range operation.Parameters {
			parameter := &operation.Parameters[index]
			if !isXMLMedia(parameter.ContentType) {
				continue
			}
			for _, section := range operation.InputFields {
				if section.Name != parameter.Section {
					continue
				}
				properties, _ := schemas.nodes[section.Validation].Get("properties").(schemaplan.Properties)
				for _, property := range properties.Entries {
					if property.Name != parameter.Name {
						continue
					}
					name, err := prepare(schemas.nodeNames[property.Schema], schemas.nodeNames[property.Schema], property.Schema)
					if err != nil {
						return nil, "", err
					}
					parameter.Encoder = name
				}
			}
		}
		for index := range operation.InputFields {
			field := &operation.InputFields[index]
			if field.Validation != "" && field.Stream == nil && isXMLMedia(field.MediaType) {
				name, err := prepare(field.Type, field.Validation)
				if err != nil {
					return nil, "", err
				}
				field.Encoder = name
			}
		}
		for index := range operation.RequestBodies {
			body := &operation.RequestBodies[index]
			if body.Validation != "" && body.Stream == nil && (isXMLMedia(body.MediaType) || body.Open) {
				name, err := prepare(body.Model, body.Validation)
				if err != nil {
					return nil, "", err
				}
				body.Encoder = name
			}
		}
	}
	if len(planner.programs) != 0 {
		schemas.requireHelper("wire-xml-output")
	}
	prepared.WriteString(strings.Join(parts.partQueries, "\n"))
	prepared.WriteString(strings.Join(parts.binaryPrograms, "\n"))
	return planner.programs, prepared.String(), nil
}
