package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedMultipartBinaryAlternativeMediaSelection(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Multipart binary alternatives","version":"1"},"paths":{"/media":{"post":{"operationId":"sendBinaryMedia","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["file"],"properties":{"file":{"anyOf":[{"type":"string","format":"binary","pattern":"^a","contentMediaType":"application/x-alpha"},{"type":"string","format":"binary","pattern":"^b","contentMediaType":"application/x-beta"}]}}}}}},"responses":{"204":{"description":"Done"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"multipart_binary_media_test.go": multipartBinaryMediaConsumer})
}

const multipartBinaryMediaConsumer = `package consumer_test
import("context";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestBinaryPartChoosesFirstMatchingContract(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){_,parameters,err:=mime.ParseMediaType(request.Header.Get("Content-Type"));if err!=nil{t.Error(err);writer.WriteHeader(500);return};part,err:=multipart.NewReader(request.Body,parameters["boundary"]).NextPart();if err!=nil{t.Error(err);writer.WriteHeader(500);return};data,err:=io.ReadAll(part);if err!=nil{t.Error(err)};expectedMedia,expectedBody:="application/x-beta","bbb";if request.Header.Get("X-Case")=="source"{expectedMedia,expectedBody="application/x-alpha","resource"};if part.Header.Get("Content-Type")!=expectedMedia||part.FileName()!="blob"||string(data)!=expectedBody{t.Error("binary branch media",part.Header,string(data))};writer.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 value,_:=types.ParseValue([]byte("{\"file\":\"bbb\"}"));body,err:=models.DecodeSendBinaryMediaBody(context.Background(),value);if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendBinaryMedia(sdk.SendBinaryMediaInput{Body:body}).Do(context.Background());err!=nil{t.Fatal(err)}
 file,err:=models.SendBinaryMediaBodyFileFromAnyOf1(types.BinaryFromBytes([]byte("resource")));if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendBinaryMedia(sdk.SendBinaryMediaInput{Body:sdk.SendBinaryMediaBody{File:file}}).WithOptions(types.RequestOptions{Headers:http.Header{"X-Case":{"source"}}}).Do(context.Background());err!=nil{t.Fatal(err)}
}
`
