package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"strings"
	"testing"
)

func TestGeneratedEncodingIgnoresEntriesWithoutInstanceValues(t *testing.T) {
	document, err := compiler.Compile([]byte(`{
  "openapi":"3.2.0",
  "info":{"title":"Encoding applicability","version":"1"},
  "paths":{
    "/named":{"post":{
      "operationId":"namedEncoding",
      "requestBody":{"required":true,"content":{"multipart/form-data":{
        "schema":{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}},
        "encoding":{
          "name":{"contentType":"text/plain"},
          "unused":{"contentType":"application/vnd.missing","headers":{"X-Required":{"required":true,"schema":{"type":"string"}}}}
        }
      }}},
      "responses":{"204":{"description":"OK"}}
    }},
    "/positional":{"post":{
      "operationId":"positionalEncoding",
      "requestBody":{"required":true,"content":{"multipart/mixed":{
        "schema":{"type":"array","items":{"type":"string"}},
        "prefixEncoding":[
          {"contentType":"text/plain"},
          {"contentType":"application/vnd.missing"}
        ]
      }}},
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	metadata := string(document.SourceMetadataJSON)
	if !strings.Contains(metadata, `"unused"`) || !strings.Contains(metadata, `"application/vnd.missing"`) {
		t.Fatal("ignored encoding metadata lost")
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"encoding_applicability_test.go": encodingApplicabilityConsumer})
}

const encodingApplicabilityConsumer = `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"strings";"sync/atomic";"testing";sdk "example.test/consumer/api")
func TestOnlyExistingValuesUseTheirEncoding(t *testing.T){
 var calls atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){calls.Add(1);data,err:=io.ReadAll(request.Body);if err!=nil||!strings.Contains(string(data),"one")||strings.Contains(string(data),"X-Required")||strings.Contains(string(data),"application/vnd.missing"){t.Error("unused encoding affected body",string(data),err)};writer.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.NamedEncoding(sdk.NamedEncodingInput{Body:sdk.NamedEncodingBody{Name:"one"}}).Do(context.Background());err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.PositionalEncoding(sdk.PositionalEncodingInput{Body:sdk.PositionalEncodingBody([]string{"one"})}).Do(context.Background());err!=nil{t.Fatal(err)}
 if calls.Load()!=2{t.Fatal("request count",calls.Load())}
}
`
