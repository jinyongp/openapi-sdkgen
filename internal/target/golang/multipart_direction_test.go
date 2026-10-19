package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedMultipartWithinOpenRequestMedia(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Open multipart","version":"1"},"paths":{"/open":{"post":{"operationId":"sendOpenMedia","requestBody":{"required":true,"content":{"*/*":{"schema":{"type":"object","required":["title"],"properties":{"title":{"type":"string"}}}}}},"responses":{"204":{"description":"Done"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"multipart_open_direction_test.go": multipartOpenDirectionConsumer})
}

func TestGeneratedFiniteMultipartResponseUsesJSONCodecContract(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Multipart response codec","version":"1"},"paths":{"/result":{"get":{"operationId":"readMultipartResult","responses":{"200":{"description":"Done","content":{"multipart/form-data":{"schema":{"type":"object","required":["count"],"properties":{"count":{"type":"integer"}}}}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"multipart_response_direction_test.go": multipartResponseDirectionConsumer})
}

const multipartOpenDirectionConsumer = `package consumer_test
import("context";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestOpenRangeChoosesNativeMultipart(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){media,parameters,err:=mime.ParseMediaType(request.Header.Get("Content-Type"));if err!=nil||media!="multipart/form-data"{t.Error("multipart range media",request.Header,err);writer.WriteHeader(500);return};reader:=multipart.NewReader(request.Body,parameters["boundary"]);part,err:=reader.NextPart();if err!=nil{t.Error(err);writer.WriteHeader(500);return};body,err:=io.ReadAll(part);if err!=nil||part.FormName()!="title"||string(body)!="Hello"||part.Header.Get("Content-Type")!=""||part.FileName()!=""{t.Error("open multipart part",part.Header,string(body),err)};if _,err:=reader.NextPart();err!=io.EOF{t.Error("extra part",err)};writer.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 body:=sdk.NewSendOpenMediaMediaBody("multipart/form-data",sdk.SendOpenMediaMediaBody{Title:"Hello"});if _,err:=client.Operations.SendOpenMedia(sdk.SendOpenMediaInput{Body:body}).Do(context.Background());err!=nil{t.Fatal(err)}
 value,_:=types.ValueFrom("public codec type");var prepared types.MultipartData=types.MultipartData{Value:value};var copied types.MultipartData;copied,err=types.MultipartFromWireValue(prepared).WireValue();if err!=nil||!copied.Value.Equal(value){t.Fatal("public media data contract",err)}
}
`

const multipartResponseDirectionConsumer = `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestFiniteResponseKeepsStrictJSONOutput(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){writer.Header().Set("Content-Type","multipart/form-data; boundary=unused");io.WriteString(writer,"host-owned")}));defer server.Close()
 codec:=types.MediaCodec{Decode:func(_ context.Context,response *http.Response,metadata types.MediaContext)(types.Value,error){body,err:=io.ReadAll(response.Body);if err!=nil{return types.Value{},err};if string(body)!="host-owned"||metadata.ContentType!="multipart/form-data; boundary=unused"{t.Error("finite codec metadata",string(body),metadata)};return types.ParseValue([]byte("{\"count\":2,\"extra\":true}"))}}
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Codecs:map[string]types.MediaCodec{"multipart/form-data":codec}});if err!=nil{t.Fatal(err)}
 result,err:=client.Operations.ReadMultipartResult().Do(context.Background());if err!=nil||result.Count!=2{t.Fatal("finite multipart response",result,err)};var additional map[string]types.Value=result.AdditionalProperties;extra,_:=additional["extra"].Bool();if !extra{t.Fatal("ordinary JSON output changed",additional)}
}
`
