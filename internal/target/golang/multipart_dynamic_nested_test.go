package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedOptionSelectedNestedMultipartDefaults(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Nested selection","version":"1"},"paths":{"/nested":{"post":{"operationId":"sendDynamicNested","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["rows","batches","p_extra"],"properties":{"rows":{"type":"array","items":{"type":["object","null"],"properties":{"id":{"type":"integer"}}}},"batches":{"type":"array","items":{"type":"array","items":{"type":["object","null"]}}}},"patternProperties":{"^p_":{"type":"array","items":{"type":"array","items":{"type":["object","null"]}}}}},"encoding":{"rows":{"explode":false}}}}},"responses":{"204":{"description":"Done"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"dynamic_nested_test.go": dynamicNestedConsumer})
}

const dynamicNestedConsumer = `package consumer_test
import("context";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestOptionSelectedNestedMultipartKeepsSchemaDefaults(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){_,parameters,err:=mime.ParseMediaType(request.Header.Get("Content-Type"));if err!=nil{t.Error(err);writer.WriteHeader(500);return};outer:=multipart.NewReader(request.Body,parameters["boundary"]);for count:=0;count<3;count++{part,err:=outer.NextPart();if err!=nil{t.Error(err);writer.WriteHeader(500);return};_,parameters,err=mime.ParseMediaType(part.Header.Get("Content-Type"));if err!=nil{t.Error(err);writer.WriteHeader(500);return};inner:=multipart.NewReader(part,parameters["boundary"]);child,err:=inner.NextPart();if err!=nil{t.Error(err);writer.WriteHeader(500);return};body,err:=io.ReadAll(child);if err!=nil||child.Header.Get("Content-Type")!="application/json"||string(body)!="null"{t.Error("nested nullable schema default",child.Header,string(body),err)};if _,err:=inner.NextPart();err!=io.EOF{t.Error("extra child",err)}};if _,err:=outer.NextPart();err!=io.EOF{t.Error("extra parent",err)};writer.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};value,_:=types.ParseValue([]byte("{\"rows\":[null],\"batches\":[[null]],\"p_extra\":[[null]]}"));body,err:=models.DecodeSendDynamicNestedBody(context.Background(),value);if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendDynamicNested(sdk.SendDynamicNestedInput{Body:body}).WithOptions(types.RequestOptions{MultipartContentTypes:map[string]string{"rows":"multipart/mixed","batches":"multipart/mixed","p_extra":"multipart/mixed"}}).Do(context.Background());err!=nil{t.Fatal(err)}
}
`
