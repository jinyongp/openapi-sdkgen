package golang

import (
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedQueryBoundaries(t *testing.T) {
	files, err := planFiles(generator.GoOptions{ImportPath: "example.test/consumer/api", Package: "api"}, nil, false, false, "core", "core-parameters", "server-parameters")
	if err != nil {
		t.Fatal(err)
	}
	testGeneratedModule(t, generator.NewPlan("golang", sourcePlan{Files: files}), map[string]string{
		"api/internal/core/query_test.go": `package core
import("context";"testing";"example.test/consumer/api/types")
func TestQueryTokens(t *testing.T){for _,c:=range []struct{input,want string}{ {"[\"a,b\",\"c\"]","p=a%2Cb,c"},{"[\"a\",\"b,c\"]","p=a,b%2Cc"},{"{\"a,b\":\"c,d\"}","p=a%2Cb,c%2Cd"} }{value,_:=types.ParseValue([]byte(c.input));data:=RequestData{};execution:=Execution{Context:context.Background()};if err:=execution.AddParameter(&data,value,Parameter{Name:"p",Location:"query",Style:"form"});err!=nil||data.Query!=c.want{t.Fatal(data.Query,err)}}}
`,
		"api/server/runtime/query_test.go": `package runtime
import("context";"net/http/httptest";"testing";"example.test/consumer/api/types";"example.test/consumer/api/internal/schema")
func TestQueryTokens(t *testing.T){shape:=&ParameterSchema{Kinds:ParameterArray,Items:&ParameterSchema{Kinds:ParameterString},Validate:func(_ context.Context,value types.Value)error{if value.Kind()!=types.ValueArray{return schema.Failure("type","")};return nil}};for _,c:=range []struct{style,query,want string;explode bool}{ {"form","p=a%2Cb&p=c","[\"a,b\",\"c\"]",true},{"form","p=a%2Cb,c","[\"a,b\",\"c\"]",false},{"pipeDelimited","p=a%7Cb","[\"a\",\"b\"]",false},{"spaceDelimited","p=a%20b","[\"a\",\"b\"]",false} }{execution:=&Execution{Context:context.Background(),Request:httptest.NewRequest("POST","https://example.test/?"+c.query,nil)};value,err:=DecodeParameters(execution,nil,[]Parameter{{Name:"p",Location:"query",Style:c.style,Explode:c.explode,Schema:shape}});query,_:=value.Get("query");p,_:=query.Get("p");encoded,_:=p.MarshalJSON();if err!=nil||string(encoded)!=c.want{t.Fatal(c.query,string(encoded),err)}}}
`,
	})
}
