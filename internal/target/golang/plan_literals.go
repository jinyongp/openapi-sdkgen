package golang

import (
	"encoding/json"
	"fmt"
	"strings"

	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func (planner *modelPlanner) literalMembers(name string, node *schemaplan.Node) ([]modelLiteral, string, error) {
	var values []any
	if field, ok := node.Get("enumValues").(schemaplan.Literal); ok {
		values = field.Data.([]any)
	}
	if field, ok := node.Get("constValue").(schemaplan.Literal); ok {
		values = []any{field.Data}
	}
	if len(values) == 0 {
		return nil, "", nil
	}
	literals := make([]modelLiteral, 0, len(values))
	kind := ""
	homogeneous := true
	counts := make(map[string]int)
	for index, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, "", err
		}
		category := "Number"
		expression := string(encoded)
		switch encoded[0] {
		case 'n':
			category = "Null"
		case 't', 'f':
			category = "Boolean"
		case '"':
			category = "String"
		case '{':
			category = "Object"
		case '[':
			category = "Array"
		}
		if kind == "" {
			kind = category
		} else if kind != category {
			homogeneous = false
		}
		counts[category]++
		stem := category
		switch category {
		case "String":
			text, _ := value.(string)
			stem += exportedName(text)
		case "Number":
			stem += strings.TrimPrefix(exportedName(strings.ReplaceAll(expression, "-", "Minus")), "N")
		case "Boolean":
			stem = exportedName(expression)
		case "Object", "Array":
			stem += fmt.Sprint(counts[category])
		}
		identity := planner.sources[node] + "/enum/" + fmt.Sprint(index)
		if node.Get("constValue") != nil {
			identity = planner.sources[node] + "/const"
		}
		member, err := reservePublicName(planner.used, name+stem, identity, "")
		if err != nil {
			return nil, "", err
		}
		documentation := planner.memberDocs[node][expression]
		literals = append(literals, modelLiteral{Name: member, Expression: expression, JSON: expression, Documentation: goDocumentation(documentation)})
	}
	if !homogeneous {
		kind = ""
	}
	return literals, kind, nil
}

func (planner *modelPlanner) literal(name string, node *schemaplan.Node) (modelType, bool, error) {
	literals, kind, err := planner.literalMembers(name, node)
	if err != nil {
		return modelType{}, true, err
	}
	if len(literals) == 0 {
		return modelType{}, false, nil
	}
	scalar := kind == "String" || kind == "Number" || kind == "Boolean"
	planned := modelPlan{Name: name, Literals: literals, ScalarLiteral: scalar, Named: true}
	if scalar {
		planner.needsWire = true
		planner.needsFmt = true
		switch kind {
		case "String":
			planned.Type = "string"
			planned.Encode = "return wire.CheckedText(string(value))"
			planned.Decode = fmt.Sprintf("item,ok:=value.String();if !ok{return zero,fmt.Errorf(\"expected string\")};return %s(item),nil", name)
		case "Number":
			planned.Type = "float64"
			planned.Encode = "return wire.Numeric(float64(value))"
			planned.Decode = fmt.Sprintf("item,ok:=value.Number();if !ok{return zero,fmt.Errorf(\"expected number\")};return %s(item),nil", name)
		case "Boolean":
			planned.Type = "bool"
			planned.Encode = "return wire.Bool(bool(value)),nil"
			planned.Decode = fmt.Sprintf("item,ok:=value.Bool();if !ok{return zero,fmt.Errorf(\"expected boolean\")};return %s(item),nil", name)
		}
	} else {
		planner.needsWire = true
		planner.needsLiteral = true
		planned.Type = "struct{value types.Value}"
		planned.Wrapper = true
		planned.Encode = "return value.value,nil"
		planned.Decode = "return " + name + "{value:value},nil"
	}
	planner.plans = append(planner.plans, planned)
	return modelType{Type: name, Codec: name}, true, nil
}
