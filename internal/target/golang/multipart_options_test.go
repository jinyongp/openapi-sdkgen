package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedMultipartDefaultEncodingWithXMLOptions(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Default multipart encoding","version":"1"},"paths":{"/select":{"post":{"operationId":"selectPart","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["0"],"properties":{"0":{"type":"object","xml":{"name":"record"},"required":["id"],"properties":{"id":{"type":"integer","xml":{"attribute":true}}}}},"prefixItems":[{"type":"boolean"}]}}}},"responses":{"204":{"description":"Done"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"multipart_options_test.go": multipartOptionsConsumer})
}

const multipartOptionsConsumer = `package consumer_test
import("context";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestDefaultPartEncodingAndNumericName(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){_,parameters,err:=mime.ParseMediaType(request.Header.Get("Content-Type"));if err!=nil{t.Error(err);writer.WriteHeader(500);return};reader:=multipart.NewReader(request.Body,parameters["boundary"]);part,err:=reader.NextPart();if err!=nil{t.Error(err);writer.WriteHeader(500);return};body,err:=io.ReadAll(part);if err!=nil||part.FormName()!="0"||part.Header.Get("Content-Type")!="application/xml"||string(body)!="<record id=\"5\"></record>"{t.Error("default XML part",part.Header,string(body),err)};if _,err:=reader.NextPart();err!=io.EOF{t.Error("extra part",err)};writer.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 value,_:=types.ParseValue([]byte("{\"0\":{\"id\":5}}"));body,err:=models.DecodeSelectPartBody(context.Background(),value);if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SelectPart(sdk.SelectPartInput{Body:body}).WithOptions(types.RequestOptions{MultipartContentTypes:map[string]string{"0":"application/xml"}}).Do(context.Background());err!=nil{t.Fatal(err)}
}
`
