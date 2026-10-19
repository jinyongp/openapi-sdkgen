package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedXMLWithinGeneralMediaRanges(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"XML media range","version":"1"},"paths":{"/wide":{
 "get":{"operationId":"getWide","responses":{"200":{"description":"Media","content":{"*/*":{"schema":{"type":"object","required":["title"],"properties":{"title":{"type":"string"}}}}}}}},
 "post":{"operationId":"sendWide","requestBody":{"required":true,"content":{"*/*":{"schema":{"type":"object","required":["title"],"properties":{"title":{"type":"string"}}}}}},"responses":{"204":{"description":"Done"}}}
 }}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"xml_open_test.go": xmlOpenConsumer})
}

const xmlOpenConsumer = `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api")
func TestXMLMediaRangeInputAndOutput(t *testing.T){
 const body="<root><title>Hello</title></root>"
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){if request.Method=="POST"{data,err:=io.ReadAll(request.Body);if err!=nil||string(data)!=body||request.Header.Get("Content-Type")!="application/xml"{t.Error("XML range request",string(data),err,request.Header)};writer.WriteHeader(204);return};writer.Header().Set("Content-Type","application/xml");io.WriteString(writer,body)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 input:=sdk.SendWideInput{Body:sdk.NewSendWideMediaBody("application/xml",sdk.SendWideMediaBody{Title:"Hello"})}
 if _,err:=client.Operations.SendWide(input).Do(context.Background());err!=nil{t.Fatal(err)}
 result,err:=client.Operations.GetWide().Do(context.Background());if err!=nil{t.Fatal(err)}
 value,ok:=result.AsValue();if !ok||value.Title!="Hello"||result.ContentType()!="application/xml"{t.Fatal("XML range response",ok,value,result.ContentType())}
 if _,ok:=result.AsBinary();ok{t.Fatal("XML range became raw binary")}
}
`
