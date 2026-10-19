package golang

import (
	"encoding/json"
	"testing"

	"openapi-sdkgen/internal/generator"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func TestGeneratedInboundParameterGraph(t *testing.T) {
	var source any
	if err := json.Unmarshal([]byte(`{"type":"object","properties":{"count":{"type":"integer"},"tuple":{"type":"array","prefixItems":[{"type":"integer"},{"type":"boolean"}],"items":{"type":"string"}},"choice":{"oneOf":[{"type":"object","properties":{"a":{"type":"integer"},"b":{"type":"string"}},"required":["a","b"]},{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"integer"}},"required":["a","b"]}]},"conditional":{"type":"array","if":{"minItems":2},"then":{"items":{"type":"integer"}},"else":{"items":{"type":"boolean"}}}},"patternProperties":{"^n_":{"type":"number"}},"additionalProperties":false}`), &source); err != nil {
		t.Fatal(err)
	}
	schemas := new(schemaPlanner)
	if _, err := schemas.prepare("ValidateInput", source, schemaplan.Input, false); err != nil {
		t.Fatal(err)
	}
	graph := prepareParameterGraph("parameterGraph", map[string]*schemaplan.Node{"root": schemas.nodes["ValidateInput"]}, schemas)
	for index := range schemas.programs {
		schemas.programs[index].Public = true
	}
	config := generator.GoOptions{ImportPath: "example.test/consumer/api", Package: "api"}
	roots := append(sortedKeys(schemas.helperRoots), "server-parameters")
	files, err := planFiles(config, nil, schemas.needsRegex, false, roots...)
	if err != nil {
		t.Fatal(err)
	}
	files, err = completeFiles(config, files, schemas.programs, nil)
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, filePlan{Path: "server/testhook/parameters.go", Package: "testhook", Template: "templates/prepared.go.tmpl", Prepared: graph, Imports: []importPlan{{Path: "context"}, {Path: config.ImportPath + "/types"}, {Path: config.ImportPath + "/internal/schema"}, {Path: config.ImportPath + "/server/runtime"}}})
	testGeneratedModule(t, generator.NewPlan("golang", sourcePlan{Files: files}), map[string]string{"api/server/testhook/parameters_test.go": `package testhook
import("context";"testing";"example.test/consumer/api/types";"example.test/consumer/api/server/runtime")
func TestPreparedGraph(t *testing.T){shapes:=parameterGraph();for _,test:=range []struct{name,raw,want string}{ {"count","\"0x10\"","16"},{"tuple","[\"2\",\"true\",\"tail\"]","[2,true,\"tail\"]"},{"choice","{\"a\":\"x\",\"b\":\"2\"}","{\"a\":\"x\",\"b\":2}"},{"conditional","[\"1\",\"2\"]","[1,2]"},{"conditional","[\"true\"]","[true]"} }{t.Run(test.name+test.raw,func(t *testing.T){raw,_:=types.ParseValue([]byte(test.raw));value,err:=runtime.CoerceParameter(context.Background(),shapes["root"].Properties[test.name],raw,false);encoded,_:=value.MarshalJSON();if err!=nil||string(encoded)!=test.want{t.Fatal(string(encoded),err)}})};raw,_:=types.ParseValue([]byte("{\"n_value\":\"2\"}"));value,err:=runtime.CoerceParameter(context.Background(),shapes["root"],raw,false);encoded,_:=value.MarshalJSON();if err!=nil||string(encoded)!="{\"n_value\":2}"{t.Fatal(string(encoded),err)}}
`})
}
