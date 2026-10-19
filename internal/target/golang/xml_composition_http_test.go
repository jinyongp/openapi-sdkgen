package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedXMLAnyOfConditionalAndDynamicHTTP(t *testing.T) {
	document, err := compiler.Compile([]byte(`openapi: 3.2.0
info: {title: XML composition, version: '1'}
paths:
  /any:
    post:
      operationId: sendAnyXML
      requestBody: {required: true, content: {application/xml: {schema: {$ref: '#/components/schemas/Any'}}}}
      responses: {'200': {description: Done, content: {application/xml: {schema: {$ref: '#/components/schemas/Any'}}}}}
  /conditional:
    post:
      operationId: sendConditionalXML
      requestBody: {required: true, content: {application/xml: {schema: {$ref: '#/components/schemas/Conditional'}}}}
      responses: {'200': {description: Done, content: {application/xml: {schema: {$ref: '#/components/schemas/Conditional'}}}}}
  /dynamic:
    post:
      operationId: sendDynamicXML
      requestBody: {required: true, content: {application/xml: {schema: {$ref: '#/components/schemas/Strict'}}}}
      responses: {'200': {description: Done, content: {application/xml: {schema: {$ref: '#/components/schemas/Strict'}}}}}
components:
  schemas:
    Any:
      type: object
      xml: {name: payload}
      anyOf:
        - {type: object, required: [label], properties: {label: {type: string}}}
        - {type: object, required: [count], properties: {count: {type: integer, xml: {attribute: true}}}}
    Conditional:
      type: object
      xml: {name: payload}
      required: [flag]
      properties: {flag: {type: boolean}}
      if: {properties: {flag: {const: true}}}
      then: {required: [note], properties: {note: {type: string, xml: {name: message}}}}
      else: {required: [count], properties: {count: {type: integer, xml: {attribute: true}}}}
    Base:
      $id: https://schemas.example.test/native/base
      $dynamicAnchor: node
      type: object
      required: [value]
      properties:
        value: {type: integer}
        children:
          type: array
          xml: {wrapped: true}
          items: {$dynamicRef: '#node', xml: {name: child}}
    Strict:
      $id: https://schemas.example.test/native/strict
      $dynamicAnchor: node
      $ref: '#/components/schemas/Base'
      xml: {name: root}
      required: [extra]
      properties: {extra: {type: boolean}}
`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"xml_composition_test.go": xmlCompositionHTTPConsumer})
}

const xmlCompositionHTTPConsumer = `package consumer_test
import("context";"encoding/json";"errors";"io";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestXMLCompositionHTTP(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){data,err:=io.ReadAll(request.Body);if err!=nil{t.Error(err)};switch request.URL.Path{case "/any":if string(data)!="<payload count=\"2\"><label>Hello</label></payload>"{t.Error("anyOf mapping",string(data))};case "/conditional":if string(data)!="<payload><flag>true</flag><message>Hello</message></payload>"&&string(data)!="<payload count=\"2\"><flag>false</flag></payload>"{t.Error("conditional mapping",string(data))};case "/dynamic":if string(data)!="<root><children><child><children></children><extra>false</extra><value>2</value></child></children><extra>true</extra><value>1</value></root>"{t.Error("dynamic mapping",string(data))}}
 writer.Header().Set("Content-Type","application/xml");if request.Header.Get("X-Case")=="invalid"{io.WriteString(writer,"<root><value>1</value><extra>true</extra><children><child><value>2</value><children/></child></children></root>");return};writer.Write(data)
 }));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 parse:=func(text string)types.Value{value,err:=types.ParseValue([]byte(text));if err!=nil{t.Fatal(err)};return value}
 compare:=func(expected types.Value,value any){data,err:=json.Marshal(value);if err!=nil{t.Fatal(err)};decoded,err:=types.ParseValue(data);if err!=nil||!decoded.Equal(expected){t.Fatal("XML roundtrip",expected,string(data),err)}}
 value:=parse("{\"label\":\"Hello\",\"count\":2}");body,err:=models.DecodeSendAnyXMLBody(context.Background(),value);if err!=nil{t.Fatal(err)};result,err:=client.Operations.SendAnyXML(sdk.SendAnyXMLInput{Body:body}).Do(context.Background());if err!=nil{t.Fatal(err)};compare(value,result)
 for _,text:=range []string{"{\"flag\":true,\"note\":\"Hello\"}","{\"flag\":false,\"count\":2}"}{value:=parse(text);body,err:=models.DecodeSendConditionalXMLBody(context.Background(),value);if err!=nil{t.Fatal(err)};result,err:=client.Operations.SendConditionalXML(sdk.SendConditionalXMLInput{Body:body}).Do(context.Background());if err!=nil{t.Fatal(err)};compare(value,result)}
 value=parse("{\"value\":1,\"extra\":true,\"children\":[{\"value\":2,\"extra\":false,\"children\":[]}]}");dynamic,err:=models.DecodeSendDynamicXMLBody(context.Background(),value);if err!=nil{t.Fatal(err)};resultDynamic,err:=client.Operations.SendDynamicXML(sdk.SendDynamicXMLInput{Body:dynamic}).Do(context.Background());if err!=nil{t.Fatal(err)};compare(value,resultDynamic)
 _,err=client.Operations.SendDynamicXML(sdk.SendDynamicXMLInput{Body:dynamic}).WithOptions(types.RequestOptions{Headers:http.Header{"X-Case":{"invalid"}}}).Do(context.Background());var apiError *types.APIError;if !errors.As(err,&apiError)||apiError.Code!=types.ResponseDecodeFailed{t.Fatal("dynamic child assertion lost",err)}
}
`
