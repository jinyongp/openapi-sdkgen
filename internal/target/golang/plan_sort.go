package golang

import (
	"bytes"
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	schemaplan "openapi-sdkgen/internal/schema/plan"
	"strconv"
)

func (planner *modelPlanner) sort(name string, node *schemaplan.Node, plan ir.SortParameterPlan) (modelType, error) {
	planner.needsWire = true
	planner.needsFmt = true
	names := make(map[string]string)
	for _, suffix := range []string{"Item", "Field", "Direction"} {
		reserved, err := reserveDerivedName(planner.used, name+suffix, planner.sources[node]+"/sort/"+suffix, modelNames)
		if err != nil {
			return modelType{}, err
		}
		names[suffix] = reserved
	}
	for _, suffix := range []string{"Field", "Direction"} {
		values := make(map[string]bool)
		for _, value := range plan.Values {
			item := value.Field
			if suffix == "Direction" {
				item = value.Direction
			}
			values[item] = true
		}
		literals := make([]modelLiteral, 0, len(values))
		for _, value := range sortedKeys(values) {
			member, err := reservePublicName(planner.used, names[suffix]+exportedName(value), planner.sources[node]+"/sort/"+suffix+"/"+value, "")
			if err != nil {
				return modelType{}, err
			}
			literals = append(literals, modelLiteral{Name: member, Expression: strconv.Quote(value), JSON: strconv.Quote(value)})
		}
		typeName := names[suffix]
		allowed := make([]string, 0, len(values))
		for _, value := range sortedKeys(values) {
			allowed = append(allowed, strconv.Quote(value))
		}
		cases := joinComma(allowed)
		planner.plans = append(planner.plans, modelPlan{Name: typeName, Named: true, ScalarLiteral: true, Type: "string", Literals: literals, Encode: fmt.Sprintf("switch string(value){case %s:return wire.CheckedText(string(value))};return types.Value{},fmt.Errorf(\"sort value is not declared\")", cases), Decode: fmt.Sprintf("text,ok:=value.String();if ok{switch text{case %s:return %s(text),nil}};return zero,fmt.Errorf(\"sort value is not declared\")", cases, typeName)})
	}
	var encode, decode bytes.Buffer
	fmt.Fprintln(&encode, "switch { ")
	fmt.Fprintln(&decode, "text,ok:=value.String();if !ok{return zero,fmt.Errorf(\"expected sort wire string\")};switch text{")
	for _, value := range plan.Values {
		fmt.Fprintf(&encode, "case string(value.Field)==%q&&string(value.Direction)==%q:return wire.CheckedText(%q)\n", value.Field, value.Direction, value.Wire)
		fmt.Fprintf(&decode, "case %q:return %s{Field:%s(%q),Direction:%s(%q)},nil\n", value.Wire, names["Item"], names["Field"], value.Field, names["Direction"], value.Direction)
	}
	fmt.Fprintln(&encode, "};return types.Value{},fmt.Errorf(\"sort field/direction pair is not declared\")")
	fmt.Fprintln(&decode, "};return zero,fmt.Errorf(\"sort wire value is not declared\")")
	planner.plans = append(planner.plans, modelPlan{Name: names["Item"], Named: true, Type: fmt.Sprintf("struct{Field %s;Direction %s}", names["Field"], names["Direction"]), Encode: encode.String(), Decode: decode.String()})
	planner.plans = append(planner.plans, modelPlan{Name: name, Named: true, Type: "[]" + names["Item"], Encode: fmt.Sprintf("items:=make([]types.Value,len(value));for index,item:=range value{encoded,err:=encode%s(item,state);if err!=nil{return types.Value{},err};items[index]=encoded};return wire.List(items),nil", names["Item"]), Decode: fmt.Sprintf("if value.Kind()!=wire.Array{return zero,fmt.Errorf(\"expected sort array\")};result:=make(%s,value.Len());for index:=range result{item,_:=value.At(index);decoded,err:=decode%s(item,state);if err!=nil{return zero,err};result[index]=decoded};return result,nil", name, names["Item"])})
	return modelType{Type: name, Codec: name}, nil
}

func prepareModelSorts(models *modelPlanner, operations []operationPlan, sources []ir.Operation, schemas *schemaPlanner) {
	models.sorts = make(map[*schemaplan.Node]ir.SortParameterPlan)
	byRoute := make(map[string]ir.Operation, len(sources))
	for _, source := range sources {
		byRoute[source.RouteKey] = source
	}
	for _, operation := range operations {
		for _, field := range operation.InputFields {
			if field.Name != "Query" {
				continue
			}
			properties, _ := schemas.nodes[field.Validation].Get("properties").(schemaplan.Properties)
			for _, property := range properties.Entries {
				if plan, present := byRoute[operation.Route].SortParameters["query\x00"+property.Name]; present {
					models.sorts[property.Schema] = plan
				}
			}
		}
	}
}
