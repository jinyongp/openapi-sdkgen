package golang

import (
	"bytes"
	"fmt"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

type xmlPartQueryKey struct {
	Node       *schemaplan.Node
	Positional bool
}
type multipartQueryPlanner struct {
	schemas        *schemaPlanner
	references     map[string]*schemaplan.Node
	xml            *xmlEncodePlanner
	partBindings   map[xmlPartQueryKey]string
	partQueries    []string
	binaryBindings map[*schemaplan.Node]string
	binaryPrograms []string
}

// Part queries compile the existing field-query rules into direct functions.
// They never retain or interpret a schema document in the generated SDK.
func (planner *multipartQueryPlanner) partQuery(node *schemaplan.Node, positional bool) (string, error) {
	if planner.partBindings == nil {
		planner.partBindings = make(map[xmlPartQueryKey]string)
	}
	key := xmlPartQueryKey{Node: node, Positional: positional}
	if name := planner.partBindings[key]; name != "" {
		return name, nil
	}
	name := fmt.Sprintf("queryXMLPart%d", len(planner.partBindings))
	planner.partBindings[key] = name
	anyMapping, err := planner.xml.function(planner.xml.any)
	if err != nil {
		return "", err
	}
	var body bytes.Buffer
	fmt.Fprintf(&body, "func %s(ctx context.Context,key,itemKey string,value wire.Value,state *xmlEncodeState)(xmlPartMapping,error){if err:=ctx.Err();err!=nil{return xmlPartMapping{},err};var mapping *wire.XMLMapping;if !state.mediaOnly{var err error;mapping,err=%s(ctx,value,\"\",state);if err!=nil{return xmlPartMapping{},err}};result:=xmlPartMapping{Mapping:mapping,Fallback:\"root\"};if state.queries==nil{state.queries=make(map[string]bool)};if state.queries[%q]{return result,nil};state.queries[%q]=true;defer delete(state.queries,%q)\n", name, anyMapping, name, name, name)
	apply := func(child *schemaplan.Node) error {
		itemQuery, err := planner.partQuery(child, true)
		if err != nil {
			return err
		}
		fmt.Fprintf(&body, "if itemKey!=\"\"{key,remaining:=splitMultipartIndex(itemKey);candidate,err:=%s(ctx,key,remaining,value,state);if err!=nil{return xmlPartMapping{},err};if err:=mergeXMLPart(&result,candidate);err!=nil{return xmlPartMapping{},err}}else{", itemQuery)
		encode, err := planner.xml.function(child)
		if err != nil {
			return err
		}
		if validation := planner.schemas.nodeNames[child]; validation != "" {
			fmt.Fprintf(&body, "if _,err:=check%s(ctx,value,\"\",state.validation);err!=nil{return xmlPartMapping{},err};", validation)
		}
		binary, err := planner.binaryMapping(child)
		if err != nil {
			return err
		}
		fmt.Fprintf(&body, "if state.mediaOnly{contentType,binary,err:=%s(ctx,value,state);if err!=nil{return xmlPartMapping{},err};if !binary{contentType=%q};if err:=mergeXMLPart(&result,xmlPartMapping{Fallback:\"root\",Count:1,ContentType:contentType,Binary:binary});err!=nil{return xmlPartMapping{},err}}else{", binary, multipartDefaultContentType(planner.schemas.annotations[child]))
		fallback := "root"
		if source, ok := planner.schemas.annotations[child].(map[string]any); ok {
			if reference, _ := source["$ref"].(string); reference != "" {
				if component, err := schemaplan.ComponentReferenceName(reference); err == nil {
					fallback = component
				}
			}
		}
		fmt.Fprintf(&body, "candidate,err:=%s(ctx,value,\"\",state);if err!=nil{return xmlPartMapping{},err};if err:=mergeXMLPart(&result,xmlPartMapping{Mapping:candidate,Fallback:%q,Count:1});err!=nil{return xmlPartMapping{},err}\n", encode, fallback)
		fmt.Fprintln(&body, "}}")
		return nil
	}
	if positional {
		prefix, _ := node.Get("prefixItems").(schemaplan.Children)
		fmt.Fprintln(&body, "matched:=false;_=matched")
		for index, child := range prefix.Nodes {
			fmt.Fprintf(&body, "if key==%q{matched=true;", fmt.Sprint(index))
			if err := apply(child); err != nil {
				return "", err
			}
			fmt.Fprintln(&body, "}")
		}
		if child, ok := node.Get("items").(schemaplan.Child); ok {
			fmt.Fprint(&body, "if !matched{")
			if err := apply(child.Node); err != nil {
				return "", err
			}
			fmt.Fprintln(&body, "}")
		} else if literal, ok := node.Get("items").(schemaplan.Literal); ok && literal.Data == false {
			fmt.Fprintln(&body, "if !matched{return xmlPartMapping{},Failure(\"items\",\"\")}")
		}
	} else {
		fmt.Fprintln(&body, "matched:=false;_=matched")
		properties, _ := node.Get("properties").(schemaplan.Properties)
		for _, property := range properties.Entries {
			fmt.Fprintf(&body, "if key==%q{matched=true;", property.Name)
			if err := apply(property.Schema); err != nil {
				return "", err
			}
			fmt.Fprintln(&body, "}")
		}
		patterns, _ := node.Get("patternProperties").(schemaplan.SchemaMap)
		for _, property := range patterns.Entries {
			pattern := ""
			for _, program := range planner.schemas.programs {
				if program.Name != planner.schemas.nodeNames[node] {
					continue
				}
				for _, candidate := range program.Patterns {
					if candidate.Pattern == property.Name {
						pattern = candidate.Name
						break
					}
				}
			}
			if pattern == "" {
				return "", fmt.Errorf("missing prepared XML part pattern %q", property.Name)
			}
			fmt.Fprintf(&body, "{accepts,err:=%s.Match(ctx,key);if err!=nil{return xmlPartMapping{},err};if accepts{matched=true;", pattern)
			if err := apply(property.Schema); err != nil {
				return "", err
			}
			fmt.Fprintln(&body, "}}")
		}
		if child, ok := node.Get("additionalProperties").(schemaplan.Child); ok {
			fmt.Fprint(&body, "if !matched{")
			if err := apply(child.Node); err != nil {
				return "", err
			}
			fmt.Fprintln(&body, "}")
		}
	}
	appendQuery := func(child *schemaplan.Node) error {
		query, err := planner.partQuery(child, positional)
		if err != nil {
			return err
		}
		fmt.Fprintf(&body, "{candidate,err:=%s(ctx,key,itemKey,value,state);if err!=nil{return xmlPartMapping{},err};if err:=mergeXMLPart(&result,candidate);err!=nil{return xmlPartMapping{},err}}\n", query)
		return nil
	}
	if reference, ok := node.Get("reference").(schemaplan.Literal); ok {
		target := planner.references[reference.Data.(string)]
		if target == nil {
			return "", fmt.Errorf("missing XML part query reference %q", reference.Data)
		}
		if err := appendQuery(target); err != nil {
			return "", err
		}
	}
	if dynamic, ok := node.Get("dynamicReference").(schemaplan.DynamicReference); ok {
		if err := appendQuery(dynamic.Fallback); err != nil {
			return "", err
		}
	}
	if children, ok := node.Get("allOf").(schemaplan.Children); ok {
		for _, child := range children.Nodes {
			if err := appendQuery(child); err != nil {
				return "", err
			}
		}
	}
	alternatives := func(children []*schemaplan.Node) error {
		fmt.Fprintln(&body, "{alternative:=xmlPartMapping{Fallback:\"root\",Count:1};matches:=0")
		for _, child := range children {
			query, err := planner.partQuery(child, positional)
			if err != nil {
				return err
			}
			fmt.Fprintf(&body, "{candidate,err:=%s(ctx,key,itemKey,value,state);if err!=nil{if !IsValidationError(err){return xmlPartMapping{},err}}else{matches++;if !alternative.Binary&&candidate.Binary{alternative.Binary=true;alternative.ContentType=candidate.ContentType};if !alternative.Binary&&!candidate.Binary&&(alternative.ContentType==\"\"||alternative.ContentType==\"application/octet-stream\"){alternative.ContentType=candidate.ContentType};alternative.Mapping,err=wire.MergeXMLMappings(alternative.Mapping,candidate.Mapping);if err!=nil{return xmlPartMapping{},err}}}\n", query)
		}
		fmt.Fprintln(&body, "if matches==0{return xmlPartMapping{},Failure(\"anyOf\",\"\")};if err:=mergeXMLPart(&result,alternative);err!=nil{return xmlPartMapping{},err}}")
		return nil
	}
	for _, keyword := range []string{"oneOf", "anyOf"} {
		if children, ok := node.Get(keyword).(schemaplan.Children); ok {
			if err := alternatives(children.Nodes); err != nil {
				return "", err
			}
		}
	}
	if node.Get("if") != nil {
		children := make([]*schemaplan.Node, 2)
		for index, keyword := range []string{"then", "else"} {
			children[index] = planner.xml.any
			if child, ok := node.Get(keyword).(schemaplan.Child); ok {
				children[index] = child.Node
			}
		}
		if err := alternatives(children); err != nil {
			return "", err
		}
	}
	fmt.Fprintln(&body, "return result,nil}")
	planner.partQueries = append(planner.partQueries, body.String())
	return name, nil
}
