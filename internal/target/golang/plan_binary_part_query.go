package golang

import (
	"bytes"
	"fmt"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func (planner *multipartQueryPlanner) binaryMapping(node *schemaplan.Node) (string, error) {
	if planner.binaryBindings == nil {
		planner.binaryBindings = make(map[*schemaplan.Node]string)
	}
	if name := planner.binaryBindings[node]; name != "" {
		return name, nil
	}
	name := fmt.Sprintf("binaryPartMedia%d", len(planner.binaryBindings))
	planner.binaryBindings[node] = name
	var body bytes.Buffer
	fmt.Fprintf(&body, "func %s(ctx context.Context,value wire.Value,state *xmlEncodeState)(string,bool,error){if err:=ctx.Err();err!=nil{return \"\",false,err};if state.queries==nil{state.queries=make(map[string]bool)};if state.queries[%q]{return \"\",false,nil};state.queries[%q]=true;defer delete(state.queries,%q)\n", name, name, name, name)
	if node.Get("binaryInput") != nil {
		contentType := "application/octet-stream"
		if literal, ok := node.Get("binaryContentType").(schemaplan.Literal); ok {
			contentType, _ = literal.Data.(string)
		}
		fmt.Fprintf(&body, "return %q,true,nil}\n", contentType)
		planner.binaryPrograms = append(planner.binaryPrograms, body.String())
		return name, nil
	}
	var branches []*schemaplan.Node
	for _, keyword := range []string{"allOf", "anyOf", "oneOf"} {
		if children, ok := node.Get(keyword).(schemaplan.Children); ok {
			branches = append(branches, children.Nodes...)
		}
	}
	if reference, ok := node.Get("reference").(schemaplan.Literal); ok {
		target := planner.references[reference.Data.(string)]
		if target == nil {
			return "", fmt.Errorf("missing binary part reference %q", reference.Data)
		}
		branches = append(branches, target)
	}
	for _, branch := range branches {
		child, err := planner.binaryMapping(branch)
		if err != nil {
			return "", err
		}
		fmt.Fprint(&body, "{matched:=true;")
		if validator := planner.schemas.nodeNames[branch]; validator != "" {
			fmt.Fprintf(&body, "if _,err:=check%s(ctx,value,\"\",state.validation);err!=nil{if !IsValidationError(err){return \"\",false,err};matched=false};", validator)
		}
		fmt.Fprintf(&body, "if matched{contentType,binary,err:=%s(ctx,value,state);if err!=nil{return \"\",false,err};if binary{return contentType,true,nil}}}\n", child)
	}
	fmt.Fprintln(&body, "return \"\",false,nil}")
	planner.binaryPrograms = append(planner.binaryPrograms, body.String())
	return name, nil
}
