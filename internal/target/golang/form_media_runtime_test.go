package golang

import (
	"encoding/json"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedURLFormBodies(t *testing.T) {
	schema := map[string]any{"type": "object", "required": []string{"arrays", "json", "joined", "nullable", "object", "plain", "scalar"}, "properties": map[string]any{
		"arrays": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		"json":   map[string]any{"type": "object"}, "object": map[string]any{"type": "object"}, "plain": map[string]any{"type": "object"},
		"joined":   map[string]any{"type": "array", "items": map[string]any{"type": []string{"string", "null"}}},
		"nullable": map[string]any{"type": []string{"string", "null"}}, "scalar": map[string]any{"type": "string"},
	}}
	media := map[string]any{"schema": schema, "encoding": map[string]any{"json": map[string]any{"contentType": "application/json"}, "joined": map[string]any{"explode": false}, "plain": map[string]any{"explode": false}}}
	paths := map[string]any{
		"/form":  map[string]any{"post": map[string]any{"operationId": "sendForm", "requestBody": map[string]any{"required": true, "content": map[string]any{"application/x-www-form-urlencoded": media}}, "responses": map[string]any{"204": map[string]any{"description": "Done"}}}},
		"/multi": map[string]any{"post": map[string]any{"operationId": "sendMulti", "requestBody": map[string]any{"required": true, "content": map[string]any{"application/x-www-form-urlencoded": media, "application/json": map[string]any{"schema": schema}}}, "responses": map[string]any{"204": map[string]any{"description": "Done"}}}},
	}
	data, err := json.Marshal(map[string]any{"openapi": "3.2.0", "info": map[string]any{"title": "Form encoding", "version": "1"}, "paths": paths})
	if err != nil {
		t.Fatal(err)
	}
	document, err := compiler.Compile(data)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"form_test.go": formBodyConsumer, "api/internal/media/form_test.go": formBodyBoundaryConsumer})
}

const formBodyConsumer = `package consumer_test
import("context";"errors";"io";"net/http";"net/http/httptest";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestFormWireAndMediaChoice(t *testing.T){
 const input="{\"arrays\":[\"a\",\"b\"],\"json\":{\"x\":1},\"joined\":[\"a\",null,\"c\"],\"nullable\":null,\"object\":{\"a\":1,\"b\":2},\"plain\":{\"a\":1,\"b\":2},\"scalar\":\"a+b ~*\"}"
 const expected="arrays=a&arrays=b&joined=a,null,c&json=%7B%22x%22%3A1%7D&nullable=null&object=%7B%22a%22%3A1%2C%22b%22%3A2%7D&plain=a,1,b,2&scalar=a%2Bb+%7E*"
 var requests,hostCalls atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){requests.Add(1);body,err:=io.ReadAll(request.Body);if err!=nil{t.Error(err)};if request.Header.Get("Content-Type")!="application/x-www-form-urlencoded"||string(body)!=expected{t.Error("form wire",request.Header,string(body))};writer.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Codecs:map[string]types.MediaCodec{"application/x-www-form-urlencoded":{Encode:func(context.Context,types.Value,types.MediaContext)(types.BinarySource,error){hostCalls.Add(1);return nil,errors.New("builtin form must take priority")}}}});if err!=nil{t.Fatal(err)}
 value,err:=types.ParseValue([]byte(input));if err!=nil{t.Fatal(err)}
 body,err:=models.DecodeSendFormBody(context.Background(),value);if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendForm(sdk.SendFormInput{Body:body}).Do(context.Background());err!=nil{t.Fatal(err)}
 multi,err:=models.DecodeSendMultiApplicationXWwwFormUrlencodedBody(context.Background(),value);if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendMulti(sdk.SendMultiInput{Body:sdk.NewSendMultiApplicationXWwwFormUrlencodedBody(multi)}).Do(context.Background());err!=nil{t.Fatal(err)}
 if requests.Load()!=2||hostCalls.Load()!=0{t.Fatal("form execution/priority",requests.Load(),hostCalls.Load())}
}
`

const formBodyBoundaryConsumer = `package media
import("testing";"example.test/consumer/api/types")
func TestFormStyles(t *testing.T){for _,c:=range []struct{input,want string;field FormField}{
 {input:"{\"p\":[1,2]}",want:"p=1%7C2",field:FormField{Name:"p",Style:"pipeDelimited",StyleBased:true}},
 {input:"{\"p\":[1,2]}",want:"p=1%202",field:FormField{Name:"p",Style:"spaceDelimited",StyleBased:true}},
 {input:"{\"p\":{\"x\":1}}",want:"p%5Bx%5D=1",field:FormField{Name:"p",Style:"deepObject",StyleBased:true,Explode:true}},
 {input:"{\"p\":{\"x\":1}}",want:"p=x,1",field:FormField{Name:"p",Style:"form",StyleBased:true,ContentType:"application/json"}},
 {input:"{\"p\":\"a/b+c&d\"}",want:"p=a/b%2Bc%26d",field:FormField{Name:"p",Style:"form",StyleBased:true,AllowReserved:true}},
 }{value,_:=types.ParseValue([]byte(c.input));body,err:=EncodeFormBody(value,[]FormField{c.field});data,_:=body.Bytes();if err!=nil||string(data)!=c.want{t.Fatal(string(data),err)}}}
func TestFormRequiresObject(t *testing.T){for _,text:=range []string{"null","true","[]","\"text\"","1"}{value,err:=types.ParseValue([]byte(text));if err!=nil{t.Fatal(err)};if _,err:=EncodeFormBody(value,nil);err==nil{t.Fatal("accepted non-object form",text)}}}
`
