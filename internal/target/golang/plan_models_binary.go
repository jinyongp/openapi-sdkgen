package golang

import (
	"fmt"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func (planner *modelPlanner) binary(name string, node *schemaplan.Node, root bool) (modelType, error) {
	planner.needsFmt = true
	nullable := node.Get("binaryNullable") != nil
	typeName := "types.Binary"
	encode := "return state.binary(types.Binary(value))"
	wrap := func(value string) string { return name + "(" + value + ")" }
	if nullable {
		typeName = "types.Nullable[types.Binary]"
		encode = "if item,present:=value.Get();present{return state.binary(item)};return types.Value{},nil"
		wrap = func(value string) string { return name + "(types.NonNull(" + value + "))" }
	}
	decode := fmt.Sprintf("if binary,present:=state.binaries[state.path];present{state.consumeBinary(state.path);return %s,nil};", wrap("binary"))
	if nullable {
		decode += "if value.Kind()==types.ValueNull{return " + name + "(types.Null[types.Binary]()),nil};"
	}
	decode += fmt.Sprintf("if text,ok:=value.String();ok{return %s,nil};return zero,fmt.Errorf(\"binary media value is missing its source\")", wrap("types.BinaryFromString(text)"))
	planner.plans = append(planner.plans, modelPlan{Name: name, Type: typeName, Encode: encode, Decode: decode, Named: root, NullableRoot: root && nullable, NullableInner: "types.Binary"})
	return modelType{Type: chooseModelName(root, name, typeName), Codec: name}, nil
}
