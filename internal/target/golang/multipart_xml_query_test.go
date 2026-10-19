package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedMultipartXMLQueriesForUnknownAndRepeatedParts(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Multipart XML field queries","version":"1"},"paths":{
"/properties":{"post":{"operationId":"sendProperties","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","patternProperties":{"^record":{"type":"object","xml":{"name":"record"},"required":["id"],"properties":{"id":{"type":"integer","xml":{"attribute":true}}}}},"additionalProperties":{"type":"object","xml":{"name":"extra"},"required":["id"],"properties":{"id":{"type":"integer","xml":{"attribute":true}}}}}}}},"responses":{"204":{"description":"Done"}}}},
"/arrays":{"post":{"operationId":"sendArrays","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["records"],"properties":{"records":{"type":"array","items":{"type":"object","xml":{"name":"record"},"required":["id"],"properties":{"id":{"type":"integer","xml":{"attribute":true}}}}}}}}}},"responses":{"204":{"description":"Done"}}}}
}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"multipart_xml_query_test.go": multipartXMLQueryConsumer})
}

const multipartXMLQueryConsumer = `package consumer_test
import("context";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"strings";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestUnknownAndExplodedXMLParts(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){_,parameters,err:=mime.ParseMediaType(request.Header.Get("Content-Type"));if err!=nil{t.Error(err);writer.WriteHeader(500);return};reader:=multipart.NewReader(request.Body,parameters["boundary"]);names:=[]string{};values:=[]string{};for{part,err:=reader.NextPart();if err==io.EOF{break};if err!=nil{t.Error(err);break};data,err:=io.ReadAll(part);if err!=nil||part.Header.Get("Content-Type")!="application/xml"{t.Error("XML part",part.Header,err)};names=append(names,part.FormName());values=append(values,string(data))};if request.URL.Path=="/properties"{if strings.Join(names,",")!="other,recordA"||strings.Join(values,"|")!="<extra id=\"2\"></extra>|<record id=\"1\"></record>"{t.Error("unknown part schema query",names,values)}}else if strings.Join(names,",")!="records,records"||strings.Join(values,"|")!="<record id=\"1\"></record>|<record id=\"2\"></record>"{t.Error("exploded part schema query",names,values)};writer.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 value,_:=types.ParseValue([]byte("{\"recordA\":{\"id\":1},\"other\":{\"id\":2}}"));body,err:=models.DecodeSendPropertiesBody(context.Background(),value);if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendProperties(sdk.SendPropertiesInput{Body:body}).WithOptions(types.RequestOptions{MultipartContentTypes:map[string]string{"recordA":"application/xml","other":"application/xml"}}).Do(context.Background());err!=nil{t.Fatal(err)}
 value,_=types.ParseValue([]byte("{\"records\":[{\"id\":1},{\"id\":2}]}"));array,err:=models.DecodeSendArraysBody(context.Background(),value);if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendArrays(sdk.SendArraysInput{Body:array}).WithOptions(types.RequestOptions{MultipartContentTypes:map[string]string{"records":"application/xml"}}).Do(context.Background());err!=nil{t.Fatal(err)}
}
`
