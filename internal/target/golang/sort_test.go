package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedStructuredSort(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Sort","version":"1"},"paths":{"/things":{"get":{"operationId":"listThings","parameters":[{"name":"sort","in":"query","x-sort":{"format":"field-direction"},"schema":{"type":"array","items":{"type":"string","enum":["name:asc","createdAt:desc"]}}},{"name":"plain","in":"query","schema":{"type":"array","items":{"type":"string","enum":["name:asc","createdAt:desc"]}}}],"responses":{"204":{"description":"OK"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"sort_test.go": sortConsumer})
}

const sortConsumer = `package consumer_test
import("context";"errors";"net/http";"net/http/httptest";"reflect";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestSortWireProjection(t *testing.T){var requests atomic.Int32;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1);if !reflect.DeepEqual(r.URL.Query()["sort"],[]string{"name:asc","createdAt:desc"})||!reflect.DeepEqual(r.URL.Query()["plain"],[]string{"name:asc"}){t.Error("wire values",r.URL.RawQuery)};w.WriteHeader(204)}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 input:=sdk.ListThingsInput{Query:types.Some(sdk.ListThingsQuery{Sort:types.Some(models.ListThingsQuerySort{{Field:"name",Direction:"asc"},{Field:"createdAt",Direction:"desc"}}),Plain:types.Some(models.ListThingsQueryPlain{"name:asc"})})}
 if _,err:=client.Things.List(input).Do(context.Background());err!=nil||requests.Load()!=1{t.Fatal(err,requests.Load())}
 bad:=input;query,_:=bad.Query.Get();query.Sort=types.Some(models.ListThingsQuerySort{{Field:"name",Direction:"desc"}});bad.Query=types.Some(query);_,err=client.Things.List(bad).Do(context.Background());var failure *sdk.APIError;if !errors.As(err,&failure)||failure.Code!=types.RequestEncodeFailed||requests.Load()!=1{t.Fatal("undeclared pair sent",err,requests.Load())}
}
`
