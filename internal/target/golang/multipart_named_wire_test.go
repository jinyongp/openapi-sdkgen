package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedNamedMultipartFormDataWireAndModeErrors(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Named multipart wire","version":"1"},"paths":{
"/named":{"post":{"operationId":"sendNamed","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["file","json","plain","typed"],"properties":{"file":{"type":"string","format":"binary"},"json":{"type":"object","properties":{"id":{"type":"integer"}}},"plain":{"type":"string"},"typed":{"type":"string"}}},"encoding":{"typed":{"contentType":"text/plain"}}}}},"responses":{"204":{"description":"Done"}}}},
"/mixed":{"post":{"operationId":"rejectNamed","requestBody":{"required":true,"content":{"multipart/mixed":{"schema":true}}},"responses":{"204":{"description":"Done"}}}},
"/array":{"post":{"operationId":"rejectArray","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":true}}},"responses":{"204":{"description":"Done"}}}},
"/positions":{"post":{"operationId":"rejectPositions","requestBody":{"required":true,"content":{"multipart/mixed":{"schema":true,"prefixEncoding":[]}}},"responses":{"204":{"description":"Done"}}}}
}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"multipart_named_test.go": multipartNamedWireConsumer})
}

const multipartNamedWireConsumer = `package consumer_test
import("context";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestNamedFormDataHeadersAndModes(t *testing.T){
 var calls atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){calls.Add(1);if request.URL.Path!="/named"{t.Error("invalid multipart mode reached HTTP",request.URL.Path);writer.WriteHeader(500);return};_,parameters,err:=mime.ParseMediaType(request.Header.Get("Content-Type"));if err!=nil{t.Error(err);writer.WriteHeader(500);return};reader:=multipart.NewReader(request.Body,parameters["boundary"]);count:=0;for{part,err:=reader.NextPart();if err==io.EOF{break};if err!=nil{t.Error(err);break};count++;data,err:=io.ReadAll(part);if err!=nil{t.Error(err)};expectedType,expectedFile,expectedBody:="","","text";switch part.FormName(){case "file":expectedType,expectedBody="application/octet-stream","binary";case "json":expectedType,expectedBody="application/json","{\"id\":1}";case "typed":expectedType="text/plain";case "plain":default:t.Error("unexpected part",part.FormName())};if expectedType!=""&&request.Header.Get("X-Case")!="manual"{expectedFile="blob"};if part.Header.Get("Content-Type")!=expectedType||part.FileName()!=expectedFile||string(data)!=expectedBody{t.Error("FormData part semantics",part.FormName(),part.Header,string(data))}};if count!=4{t.Error("part count",count)};writer.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 parse:=func(text string)types.Value{value,err:=types.ParseValue([]byte(text));if err!=nil{t.Fatal(err)};return value}
 body,err:=models.DecodeSendNamedBody(context.Background(),parse("{\"file\":\"binary\",\"json\":{\"id\":1},\"plain\":\"text\",\"typed\":\"text\"}"));if err!=nil{t.Fatal(err)};body.File=types.BinaryFromBytes([]byte("binary"))
 if _,err:=client.Operations.SendNamed(sdk.SendNamedInput{Body:body}).Do(context.Background());err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendNamed(sdk.SendNamedInput{Body:body}).WithOptions(types.RequestOptions{Headers:http.Header{"X-Case":{"manual"}},MultipartHeaders:map[string]http.Header{}}).Do(context.Background());err!=nil{t.Fatal(err)}
 object:=types.MultipartFromValue(parse("{}"));array:=types.MultipartFromValue(parse("[]"))
 if _,err:=client.Operations.RejectNamed(sdk.RejectNamedInput{Body:sdk.RejectNamedBody(object)}).Do(context.Background());err==nil{t.Fatal("named multipart/mixed accepted")}
 if _,err:=client.Operations.RejectArray(sdk.RejectArrayInput{Body:sdk.RejectArrayBody(array)}).Do(context.Background());err==nil{t.Fatal("array without positional encoding accepted")}
 if _,err:=client.Operations.RejectPositions(sdk.RejectPositionsInput{Body:sdk.RejectPositionsBody(object)}).Do(context.Background());err==nil{t.Fatal("object accepted as positional multipart")}
 if calls.Load()!=2{t.Fatal("invalid mode sent request",calls.Load())}
}
`
