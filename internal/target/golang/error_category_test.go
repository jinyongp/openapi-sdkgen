package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedErrorCategoryDetails(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Errors","version":"1"},"paths":{"/thing":{"get":{"operationId":"readThing","responses":{"200":{"description":"Done"},"400":{"description":"Invalid","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Invalid"}}}},"404":{"description":"Missing","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Missing"}}}}}}}},"components":{"schemas":{"Invalid":{"x-error-category":"validation","type":"object","required":["error"],"properties":{"error":{"type":"object","required":["code","details"],"properties":{"code":{"type":"string","const":"BAD_INPUT"},"details":{"type":"object","required":["field"],"properties":{"field":{"type":"string"},"secret":{"type":"string","writeOnly":true}},"additionalProperties":false}}}}},"Missing":{"type":"object","required":["error"],"properties":{"error":{"type":"object","required":["code","category","details"],"properties":{"code":{"type":"string","const":"NOT_FOUND"},"category":{"type":"string","const":"missing"},"details":{"type":"string"}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"error_category_test.go": errorCategoryConsumer})
}

const errorCategoryConsumer = `package consumer_test
import("context";"encoding/json";"errors";"fmt";"net/http";"net/http/httptest";"strings";"testing";sdk "example.test/consumer/api")
func TestErrorCodesCategoriesAndDetails(t *testing.T){server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");w.Header().Set("X-Request-ID","response-id");if r.Header.Get("X-Missing")=="yes"{w.WriteHeader(404);json.NewEncoder(w).Encode(map[string]any{"error":map[string]any{"code":"NOT_FOUND","category":"missing","details":"gone"}});return};w.WriteHeader(400);json.NewEncoder(w).Encode(map[string]any{"error":map[string]any{"code":"BAD_INPUT","details":map[string]any{"field":"title","secret":"hidden"}}})}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 _,err=client.Thing.Get().Do(context.Background());wrapped:=fmt.Errorf("wrapped: %w",err);if !sdk.IsAPIError(wrapped)||!sdk.IsErrorCode(wrapped,sdk.ServerErrorBADINPUTCode)||!sdk.IsErrorCategory(wrapped,sdk.ServerErrorCategoryValidation)||sdk.IsErrorCategory(wrapped,sdk.ServerErrorCategoryMissing){t.Fatal("static category grouping",err)};code,present:=sdk.GetErrorCode(wrapped);if !present||code!="BAD_INPUT"{t.Fatal(code,present)};id,present:=sdk.GetRequestID(wrapped);if !present||id!="response-id"{t.Fatal(id,present)}
 details,ok:=sdk.AsServerErrorBADINPUTDetails(wrapped);if !ok||details.Field!="title"{t.Fatal(details,ok)};encoded,err:=json.Marshal(details);if err!=nil||strings.Contains(string(encoded),"secret"){t.Fatal("output projection",string(encoded),err)};var failure *sdk.APIError;if !errors.As(wrapped,&failure)||failure.Category!="http"{t.Fatal("HTTP phase category changed",wrapped)};if _,present:=failure.Data.Get("error");!present{t.Fatal("full error body missing")}
 _,err=client.Thing.Get().WithOptions(sdk.RequestOptions{Headers:http.Header{"X-Missing":[]string{"yes"}}}).Do(context.Background());if !sdk.IsErrorCategory(err,sdk.ServerErrorCategoryMissing){t.Fatal(err)};missing,ok:=sdk.AsServerErrorNOTFOUNDDetails(err);if !ok||missing!="gone"{t.Fatal(missing,ok)};if _,ok:=sdk.AsServerErrorBADINPUTDetails(err);ok{t.Fatal("details from wrong code")};if sdk.IsAPIError(errors.New("plain")){t.Fatal("plain error accepted")}
 codes:=sdk.ServerErrorCodes();codes[0]="mutated";if sdk.ServerErrorCodes()[0]=="mutated"{t.Fatal("code list shared mutable storage")}
}
`
