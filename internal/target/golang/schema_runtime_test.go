package golang

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedSchemaAssertionsAndAnnotations(t *testing.T) {
	cases := []struct {
		schema, input string
		valid         bool
	}{
		{`false`, `null`, false},
		{`true`, `{"anything":1}`, true},
		{`{"type":"integer"}`, `1.5`, false},
		{`{"type":["string","null"]}`, `null`, true},
		{`{"type":"number","multipleOf":0.1}`, `0.3`, true},
		{`{"type":"number","multipleOf":0.1}`, `0.31`, false},
		{`{"minimum":1,"exclusiveMaximum":3}`, `3`, false},
		{`{"minimum":1,"exclusiveMaximum":3}`, `"x"`, true},
		{`{"minLength":1,"maxLength":1}`, `"\ud800"`, true},
		{`{"minLength":1,"maxLength":1}`, `"😀"`, true},
		{`{"minLength":1,"maxLength":1}`, `"ab"`, false},
		{`{"const":{"a":1,"b":[false,null]}}`, `{"b":[false,null],"a":1}`, true},
		{`{"const":{"a":1}}`, `{"a":"1"}`, false},
		{`{"enum":[null,"x",1,{"a":true},[1,"x"]]}`, `[1,"x"]`, true},
		{`{"enum":[null,"x",1,{"a":true},[1,"x"]]}`, `["x",1]`, false},
		{`{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}`, `{"ID":"x"}`, false},
		{`{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}`, `{"id":"x"}`, true},
		{`{"properties":{"x":{"type":"boolean"}},"additionalProperties":false}`, `{"x":false}`, true},
		{`{"properties":{"x":{"type":"boolean"}},"additionalProperties":false}`, `{"X":false}`, false},
		{`{"additionalProperties":{"type":"integer"}}`, `{"a":1,"b":2}`, true},
		{`{"additionalProperties":{"type":"integer"}}`, `{"a":1,"b":false}`, false},
		{`{"prefixItems":[{"type":"string"}],"items":{"type":"integer"}}`, `["x",1,2]`, true},
		{`{"prefixItems":[{"type":"string"}],"items":false}`, `["x",1]`, false},
		{`{"items":{"type":"integer"},"minItems":1,"maxItems":2}`, `[1,2,3]`, false},
		{`{"uniqueItems":true}`, `[{"a":1,"b":2},{"b":2,"a":1}]`, false},
		{`{"uniqueItems":true}`, `[1,"1",null,false]`, true},
		{`{"allOf":[{"minimum":1},{"maximum":3}]}`, `2`, true},
		{`{"allOf":[{"minimum":1},{"maximum":3}]}`, `4`, false},
		{`{"anyOf":[{"type":"string"},{"type":"number"}]}`, `false`, false},
		{`{"oneOf":[{"type":"number"},{"type":"integer"}]}`, `1`, false},
		{`{"oneOf":[{"type":"number"},{"type":"integer"}]}`, `1.5`, true},
		{`{"not":{"type":"string"}}`, `"x"`, false},
		{`{"if":{"required":["a"]},"then":{"required":["b"]},"else":{"required":["c"]}}`, `{"a":1,"b":2}`, true},
		{`{"if":{"required":["a"]},"then":{"required":["b"]},"else":{"required":["c"]}}`, `{}`, false},
		{`{"dependentRequired":{"a":["b"]}}`, `{"a":true}`, false},
		{`{"dependentSchemas":{"a":{"properties":{"b":{"type":"integer"}}}}}`, `{"a":true,"b":false}`, false},
		{`{"contains":{"type":"integer"},"minContains":2,"maxContains":2}`, `[1,"x",2]`, true},
		{`{"contains":{"type":"integer"},"minContains":0,"maxContains":0}`, `["x"]`, true},
		{`{"contains":{"type":"integer"},"minContains":0,"maxContains":0}`, `[1]`, false},
		{`{"allOf":[{"properties":{"a":true}},{"properties":{"b":true}}],"unevaluatedProperties":false}`, `{"a":1,"b":2}`, true},
		{`{"allOf":[{"properties":{"a":true}},{"properties":{"b":true}}],"unevaluatedProperties":false}`, `{"a":1,"c":2}`, false},
		{`{"anyOf":[{"properties":{"a":true}},{"properties":{"b":true}}],"unevaluatedProperties":false}`, `{"a":1,"b":2}`, true},
		{`{"anyOf":[{"required":["a"],"properties":{"a":true}},{"required":["b"],"properties":{"b":true}}],"unevaluatedProperties":false}`, `{"a":1,"c":2}`, false},
		{`{"prefixItems":[true],"unevaluatedItems":false}`, `[1,2]`, false},
		{`{"contains":{"type":"integer"},"unevaluatedItems":false}`, `[1,2]`, true},
		{`{"contains":{"type":"integer"},"unevaluatedItems":false}`, `[1,"x"]`, false},
		{`{"pattern":"^(?<word>\\p{Letter}+)-\\k<word>$"}`, `"한글-한글"`, true},
		{`{"pattern":"^(?<word>\\p{Letter}+)-\\k<word>$"}`, `"한글-other"`, false},
		{`{"pattern":"^.$"}`, `"\ud800"`, true},
		{`{"patternProperties":{"^x":{"type":"integer"}},"additionalProperties":false}`, `{"x1":1}`, true},
		{`{"patternProperties":{"^x":{"type":"integer"}},"additionalProperties":false}`, `{"y1":1}`, false},
		{`{"patternProperties":{"^x":{"type":"integer"}},"additionalProperties":false}`, `{"x1":false}`, false},
		{`{"propertyNames":{"minLength":2}}`, `{"a":1}`, false},
		{`{"propertyNames":{"minLength":2},"unevaluatedProperties":false}`, `{"ab":1}`, false},
		{`{"minProperties":2,"maxProperties":3}`, `{"a":1}`, false},
		{`{"minProperties":2,"maxProperties":3}`, `{"a":1,"b":2}`, true},
		{`{"minProperties":2,"maxProperties":3}`, `{"a":1,"b":2,"c":3,"d":4}`, false},
		{`{"minItems":1}`, `[]`, false},
		{`{"minItems":0}`, `[]`, true},
		{`{"maximum":2,"minimum":-2}`, `-2`, true},
		{`{"exclusiveMinimum":0}`, `0`, false},
		{`{"exclusiveMinimum":0}`, `0.1`, true},
		{`{"format":"date"}`, `"not a date"`, true},
	}
	document := &ir.Document{OpenAPIVersionLine: "3.1"}
	var source strings.Builder
	source.WriteString("package schema\nimport(\"context\";\"testing\";\"example.test/consumer/api/internal/wire\")\nfunc TestAssertions(t *testing.T){ctx:=context.Background()\n")
	for index, test := range cases {
		var schema any
		if err := json.Unmarshal([]byte(test.schema), &schema); err != nil {
			t.Fatal(err)
		}
		id := fmt.Sprintf("check%d", index)
		document.Operations = append(document.Operations, ir.Operation{RouteKey: fmt.Sprintf("GET /%d", index), OperationID: id, Method: "GET", Path: fmt.Sprintf("/%d", index), Responses: []ir.Response{{Status: "200", Content: []ir.MediaType{{ContentType: "application/json", Schema: schema}}}}})
		fmt.Fprintf(&source, "{value,err:=wire.Parse([]byte(%q));if err!=nil{t.Fatal(err)};_,err=ValidateHTTP__Check%dOutput__(ctx,value,\"\");if (err==nil)!=%t{t.Errorf(\"case %d: %%v\",err)}}\n", test.input, index, test.valid, index)
	}
	source.WriteString("}\n")
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"api/internal/schema/assertions_test.go": source.String()})
}

func TestGeneratedRecursiveAndDynamicReferences(t *testing.T) {
	const input = `{"openapi":"3.1.0","info":{"title":"Tree","version":"1"},"paths":{"/tree":{"get":{"operationId":"getTree","responses":{"200":{"description":"Tree","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Strict"}}}}}}}},"components":{"schemas":{"Node":{"$id":"https://schemas.example.test/node","$dynamicAnchor":"node","type":"object","properties":{"data":{"type":"integer"},"children":{"type":"array","items":{"$dynamicRef":"#node"}}}},"Strict":{"$id":"https://schemas.example.test/strict","$dynamicAnchor":"node","$ref":"#/components/schemas/Node","unevaluatedProperties":false}}}}`
	document, err := compiler.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	const source = `package schema
import("context";"testing";"example.test/consumer/api/internal/wire")
func TestDynamic(t *testing.T){
 for _,test:=range []struct{input string;valid bool}{
  {"{\"data\":1,\"children\":[{\"data\":2}]}",true},
  {"{\"data\":1,\"children\":[{\"extra\":2}]}",false},
  {"{\"data\":1,\"children\":[{\"data\":false}]}",false},
 }{
  value,err:=wire.Parse([]byte(test.input));if err!=nil{t.Fatal(err)}
  _,err=ValidateHTTP__GetTreeOutput__(context.Background(),value,"")
  if (err==nil)!=test.valid{t.Errorf("%s: %v",test.input,err)}
 }
}
`
	testGeneratedModule(t, plan, map[string]string{"api/internal/schema/dynamic_test.go": source})
}
