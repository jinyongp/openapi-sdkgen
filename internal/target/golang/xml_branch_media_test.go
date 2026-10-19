package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedXMLHTTPCompositionAndReferences(t *testing.T) {
	document, err := compiler.Compile([]byte(`{
 "openapi":"3.1.1","info":{"title":"XML branch contract","version":"1"},
 "components":{"schemas":{
 "Count":{"type":"integer","minimum":2},
 "Payload":{"type":"object","xml":{"name":"payload"},"oneOf":[
 {"required":["kind","wire_count"],"properties":{"kind":{"const":"count"},"wire_count":{"$ref":"#/components/schemas/Count","xml":{"name":"value"}}}},
 {"required":["kind","wire_label"],"properties":{"kind":{"const":"label"},"wire_label":{"type":"string","xml":{"name":"value"}}}}
 ]}}},
 "paths":{"/payload":{"post":{"operationId":"savePayload","requestBody":{"required":true,"content":{"application/xml":{"schema":{"$ref":"#/components/schemas/Payload"}}}},"responses":{"200":{"description":"XML","content":{"application/xml":{"schema":{"$ref":"#/components/schemas/Payload"}}}}}}}}
 }`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"xml_branch_test.go": xmlBranchHTTPConsumer})
}

const xmlBranchHTTPConsumer = `package consumer_test
import("context";"encoding/json";"errors";"io";"net/http";"net/http/httptest";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestXMLBranchesAndReferenceConstraints(t *testing.T){
 var payload atomic.Value;payload.Store("<payload><kind>label</kind><value>hello</value></payload>")
 var requests atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){requests.Add(1);body,err:=io.ReadAll(request.Body);if err!=nil||string(body)!="<payload><kind>count</kind><value>2</value></payload>"{t.Error("XML request branch",string(body),err)};writer.Header().Set("Content-Type","application/xml");io.WriteString(writer,payload.Load().(string))}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 value,err:=types.ParseValue([]byte("{\"kind\":\"count\",\"wire_count\":2}"));if err!=nil{t.Fatal(err)}
 input,err:=models.DecodeSavePayloadBody(context.Background(),value);if err!=nil{t.Fatal(err)}
 result,err:=client.Operations.SavePayload(sdk.SavePayloadInput{Body:input}).Do(context.Background());if err!=nil{t.Fatal(err)}
 encoded,err:=json.Marshal(result);if err!=nil||string(encoded)!="{\"kind\":\"label\",\"wire_label\":\"hello\"}"{t.Fatal("XML response branch",string(encoded),err)}
 _,err=client.Operations.SavePayload(sdk.SavePayloadInput{}).Do(context.Background());var failure *types.APIError;if !errors.As(err,&failure)||failure.Code!=types.RequestEncodeFailed||requests.Load()!=1{t.Fatal("invalid XML input reached HTTP",err,requests.Load())}
 payload.Store("<payload><kind>count</kind><value>1</value></payload>")
 _,err=client.Operations.SavePayload(sdk.SavePayloadInput{Body:input}).Do(context.Background());if !errors.As(err,&failure)||failure.Code!=types.ResponseDecodeFailed{t.Fatal("referenced XML minimum ignored",err)}
}
`
