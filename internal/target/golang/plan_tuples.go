package golang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func (planner *modelPlanner) tuple(name string, node *schemaplan.Node) (modelType, error) {
	planner.needsFmt = true
	planner.needsWire = true
	var declaration, encode, decode bytes.Buffer
	fmt.Fprintln(&declaration, "struct {")
	fmt.Fprintln(&encode, "items:=make([]types.Value,0);gap:=false")
	fmt.Fprintf(&decode, "if value.Kind()!=wire.Array{return zero,fmt.Errorf(\"expected tuple array\")};var result %s\n", name)
	minimum := float64(0)
	if field, ok := node.Get("minItems").(schemaplan.Literal); ok {
		encoded, _ := json.Marshal(field.Data)
		minimum, _ = strconv.ParseFloat(string(encoded), 64)
	}
	children := node.Get("prefixItems").(schemaplan.Children).Nodes
	for index, child := range children {
		field := fmt.Sprintf("Item%d", index)
		typed, err := planner.shape(name+field, child, false)
		if err != nil {
			return modelType{}, err
		}
		required := minimum >= float64(index+1)
		typeName := typed.Type
		if !required {
			typeName = "types.Optional[" + typeName + "]"
		}
		fmt.Fprintf(&declaration, "%s %s\n", field, typeName)
		if required {
			fmt.Fprintf(&encode, "{item:=value.%s;", field)
		} else {
			fmt.Fprintf(&encode, "if item,present:=value.%s.Get();present{", field)
		}
		fmt.Fprintf(&encode, "if gap{return types.Value{},fmt.Errorf(\"tuple positions must be contiguous\")};encoded,err:=encode%s(item,state.Index(len(items)));if err!=nil{return types.Value{},err};items=append(items,encoded)}", typed.Codec)
		if !required {
			fmt.Fprint(&encode, "else{gap=true}")
		}
		fmt.Fprintln(&encode)
		fmt.Fprintf(&decode, "if item,present:=value.At(%d);present{decoded,err:=decode%s(item,state.Index(%d));if err!=nil{return zero,err};", index, typed.Codec, index)
		if required {
			fmt.Fprintf(&decode, "result.%s=decoded}", field)
		} else {
			fmt.Fprintf(&decode, "result.%s=types.Some(decoded)}", field)
		}
		if required {
			fmt.Fprint(&decode, "else{return zero,fmt.Errorf(\"missing required tuple position\")}")
		}
		fmt.Fprintln(&decode)
	}
	restNode := &schemaplan.Node{}
	if field, ok := node.Get("items").(schemaplan.Child); ok {
		restNode = field.Node
	}
	closed := false
	if literal, ok := restNode.Get("boolean").(schemaplan.Literal); ok && literal.Data == false {
		closed = true
	}
	if !closed {
		rest, err := planner.shape(name+"RestItem", restNode, false)
		if err != nil {
			return modelType{}, err
		}
		fmt.Fprintf(&declaration, "Rest []%s\n", rest.Type)
		fmt.Fprintf(&encode, "if gap&&len(value.Rest)!=0{return types.Value{},fmt.Errorf(\"tuple rest requires every prefix position\")};for _,item:=range value.Rest{encoded,err:=encode%s(item,state.Index(len(items)));if err!=nil{return types.Value{},err};items=append(items,encoded)}\n", rest.Codec)
		fmt.Fprintf(&decode, "result.Rest=make([]%s,0);for index:=%d;index<value.Len();index++{item,_:=value.At(index);decoded,err:=decode%s(item,state.Index(index));if err!=nil{return zero,err};result.Rest=append(result.Rest,decoded)}\n", rest.Type, len(children), rest.Codec)
	}
	fmt.Fprintln(&declaration, "}")
	fmt.Fprintln(&encode, "return wire.List(items),nil")
	fmt.Fprintln(&decode, "return result,nil")
	planner.plans = append(planner.plans, modelPlan{Name: name, Type: declaration.String(), Encode: encode.String(), Decode: decode.String(), Named: true})
	return modelType{Type: name, Codec: name}, nil
}
