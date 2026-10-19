package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedXMLParametersAndHeaders(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"XML parameters","version":"1"},"components":{"schemas":{"Filter":{"type":"object","xml":{"name":"filter"},"required":["id"],"properties":{"id":{"type":"integer","minimum":1,"xml":{"attribute":true}}}}}},"paths":{"/filter":{"get":{"operationId":"getFilter","parameters":[{"name":"filter","in":"query","required":true,"content":{"application/xml":{"schema":{"$ref":"#/components/schemas/Filter"}}}},{"name":"X-Filter","in":"header","required":true,"content":{"application/xml":{"schema":{"$ref":"#/components/schemas/Filter"}}}}],"responses":{"204":{"description":"Done","headers":{"X-Result":{"required":true,"content":{"application/xml":{"schema":{"$ref":"#/components/schemas/Filter"}}}}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"xml_parameter_test.go": xmlParameterConsumer})
}

const xmlParameterConsumer = `package consumer_test
import("context";"net/http";"net/http/httptest";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestXMLParametersAndResponseHeaders(t *testing.T){
 var requests atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){requests.Add(1);if request.URL.Query().Get("filter")!="<filter id=\"2\"></filter>"||request.Header.Get("X-Filter")!="<filter id=\"3\"></filter>"{t.Error("XML parameter wire",request.URL,request.Header)};writer.Header().Set("X-Result","<filter id=\"4\"/>");writer.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Codecs:map[string]types.MediaCodec{"application/xml":{EncodeParameter:func(context.Context,types.Value,types.MediaContext)(string,error){t.Error("XML delegated to host");return "invalid",nil}}}});if err!=nil{t.Fatal(err)}
 value,_:=types.ParseValue([]byte("{\"filter\":{\"id\":2}}"));query,err:=models.DecodeGetFilterQuery(context.Background(),value);if err!=nil{t.Fatal(err)}
 value,_=types.ParseValue([]byte("{\"X-Filter\":{\"id\":3}}"));header,err:=models.DecodeGetFilterHeader(context.Background(),value);if err!=nil{t.Fatal(err)}
 response,err:=client.Operations.GetFilter(sdk.GetFilterInput{Query:query,Header:header}).Raw(context.Background());if err!=nil{t.Fatal(err)}
 headers,present:=response.As204Headers();if !present{t.Fatal("XML header projection missing")};encoded,err:=models.EncodeGetFilter204Headers(context.Background(),headers);if err!=nil{t.Fatal(err)};result,_:=encoded.Get("X-Result");id,_:=result.Get("id");number,_:=id.Number();if number!=4||requests.Load()!=1{t.Fatal("XML response header",encoded,requests.Load())}
}
`
