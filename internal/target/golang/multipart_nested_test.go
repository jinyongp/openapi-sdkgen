package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedPositionalAndNestedMultipart(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Multipart positions","version":"1"},"paths":{
"/positions":{"post":{"operationId":"sendPositions","requestBody":{"required":true,"content":{"multipart/mixed":{"schema":{"type":"array","minItems":2,"maxItems":2,"prefixItems":[{"type":"string","format":"binary"},{"type":"string"}]},"prefixEncoding":[{"contentType":"application/octet-stream","headers":{"X-Part":{"required":true,"schema":{"type":"boolean"}},"Content-Disposition":{"required":true,"schema":{"type":"string"}}}}],"itemEncoding":{"contentType":"text/plain"}}}},"responses":{"204":{"description":"Done"}}}},
"/nested":{"post":{"operationId":"sendNested","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["bundle"],"properties":{"bundle":{"type":"array","minItems":2,"maxItems":2,"prefixItems":[{"type":"string","format":"binary"},{"type":"string"}]}}},"encoding":{"bundle":{"contentType":"multipart/mixed","prefixEncoding":[{"contentType":"application/octet-stream"}],"itemEncoding":{"contentType":"text/plain"}}}}}},"responses":{"204":{"description":"Done"}}}}
}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"multipart_nested_test.go": multipartNestedConsumer})
}

const multipartNestedConsumer = `package consumer_test
import("context";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"strings";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
type positionalFile struct{}
func(positionalFile)Metadata()types.BinaryMetadata{return types.BinaryMetadata{Filename:"source.bin"}}
func(positionalFile)Open(context.Context)(io.ReadCloser,error){return io.NopCloser(strings.NewReader("binary")),nil}
func TestPositionsAndNestedParts(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){
 _,parameters,err:=mime.ParseMediaType(request.Header.Get("Content-Type"));if err!=nil{t.Error(err);writer.WriteHeader(500);return};reader:=multipart.NewReader(request.Body,parameters["boundary"])
 if request.URL.Path=="/nested"{part,err:=reader.NextPart();if err!=nil{t.Error(err);writer.WriteHeader(500);return};media,parameters,err:=mime.ParseMediaType(part.Header.Get("Content-Type"));if err!=nil||media!="multipart/mixed"||part.FormName()!="bundle"{t.Error("nested part",part.Header,err)};reader=multipart.NewReader(part,parameters["boundary"])}
 for index,expected:=range []string{"binary","tail"}{part,err:=reader.NextPart();if err!=nil{t.Error(err);break};data,err:=io.ReadAll(part);if err!=nil||string(data)!=expected{t.Error("position body",index,string(data),err)};disposition:="";if index==0{disposition="attachment; filename=\"source.bin\"";if request.URL.Path=="/positions"{disposition="inline; filename=\"override.bin\""}};if part.Header.Get("Content-Disposition")!=disposition||part.FormName()!=""{t.Error("positional disposition",part.Header)};media:=part.Header.Get("Content-Type");if index==0&&media!="application/octet-stream"||index==1&&media!="text/plain"{t.Error("position media",index,media)};if request.URL.Path=="/positions"&&index==0&&part.Header.Get("X-Part")!="true"{t.Error("position header",part.Header)}}
 if _,err:=reader.NextPart();err!=io.EOF{t.Error("extra part",err)};writer.WriteHeader(204)
 }));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 binary:=types.BinaryFromSource(positionalFile{})
 if _,err:=client.Operations.SendPositions(sdk.SendPositionsInput{Body:sdk.SendPositionsBody{Item0:binary,Item1:"tail"}}).WithOptions(types.RequestOptions{MultipartHeaders:map[string]http.Header{"0":{"X-Part":{"true"},"Content-Disposition":{"inline; filename=\"override.bin\""}}}}).Do(context.Background());err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendNested(sdk.SendNestedInput{Body:sdk.SendNestedBody{Bundle:models.SendNestedBodyBundle{Item0:binary,Item1:"tail"}}}).Do(context.Background());err!=nil{t.Fatal(err)}
}
`
