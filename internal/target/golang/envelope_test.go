package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedEnvelopeViews(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Envelope views","version":"1"},"paths":{"/required":{"get":{"operationId":"requiredData","responses":{"200":{"description":"Done","content":{"application/json":{"schema":{"type":"object","required":["data","meta"],"properties":{"data":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}},"meta":{"type":"string"}}}}}}},"x-envelope":"data"}},"/optional":{"get":{"operationId":"optionalData","responses":{"200":{"description":"Done","content":{"application/json":{"schema":{"type":"object","required":["meta"],"properties":{"data":{"type":"boolean"},"meta":{"type":"string"}}}}}}},"x-envelope":"data"}},"/internal":{"get":{"operationId":"internalCall","responses":{"204":{"description":"Done"}},"x-sdk-visibility":"internal"}},"/hidden":{"get":{"operationId":"hiddenCall","responses":{"204":{"description":"Done"}},"x-sdk-visibility":"hidden"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"envelope_test.go": envelopeConsumer})
}

const envelopeConsumer = `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"reflect";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestEnvelopeProjectionAndTypedFallback(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");if r.URL.Path=="/required"{io.WriteString(w,"{\"data\":{\"id\":7},\"meta\":\"whole\"}");return};if r.Header.Get("Mode")=="missing"{io.WriteString(w,"{\"meta\":\"missing\"}");return};io.WriteString(w,"{\"data\":false,\"meta\":\"whole\"}")}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 output,err:=client.Operations.RequiredData().Do(context.Background());if err!=nil||output.ID!=7{t.Fatal("ordinary data",output,err)};raw,err:=client.Operations.RequiredData().Raw(context.Background());if err!=nil{t.Fatal(err)};whole,present:=raw.As200JSON();if !present||whole.Meta!="whole"||whole.Data.ID!=7{t.Fatal("whole Raw body",whole,present)};if _,present:=raw.Data.Get("meta");!present{t.Fatal("raw value was projected")}
 optional,err:=client.Operations.OptionalData().Do(context.Background());if err!=nil{t.Fatal(err)};value,present:=optional.As200JSON();if !present||bool(value){t.Fatal("false data was treated as absent",value,present)};if _,present:=optional.As200JSONEnvelope();present{t.Fatal("present data selected fallback")}
 fallback,err:=client.Operations.OptionalData().WithOptions(types.RequestOptions{Headers:http.Header{"Mode":{"missing"}}}).Do(context.Background());if err!=nil{t.Fatal(err)};if _,present:=fallback.As200JSON();present{t.Fatal("missing data selected projected branch")};wholeFallback,present:=fallback.As200JSONEnvelope();if !present||wholeFallback.Meta!="missing"||wholeFallback.Data.IsSet(){t.Fatal("typed complete fallback",wholeFallback,present)}
 if _,present:=reflect.TypeOf(client.Operations).MethodByName("HiddenCall");present{t.Fatal("hidden operation leaked")};if _,present:=reflect.TypeOf(client.Operations).MethodByName("InternalCall");!present{t.Fatal("internal direct call disappeared")}
}
`
