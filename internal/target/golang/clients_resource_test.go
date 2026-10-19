package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"strings"
	"testing"
)

const independentResourceFixture = `{"openapi":"3.2.0","info":{"title":"Independent resources","version":"1"},"paths":{"/items/{id}":{"get":{"operationId":"read","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"204":{"description":"OK"}}},"delete":{"operationId":"remove","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"number"}}],"responses":{"204":{"description":"OK"}}}}}}`

func TestGeneratedIndependentResourceContracts(t *testing.T) {
	document, err := compiler.Compile([]byte(independentResourceFixture))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}, Selection: &generator.Selection{Operations: []string{"read"}}, Clients: map[string]generator.Client{"numbers": {Selection: &generator.Selection{Operations: []string{"remove"}}}}})
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatal(err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"contracts_test.go": `package consumer_test
import("context";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/clients/numbers")
func TestIndependentResourcePaths(t *testing.T){server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/items/text"&&r.URL.Path!="/items/3"{t.Error(r.URL.Path)};w.WriteHeader(204)}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};other,err:=numbers.NewClient(numbers.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};if _,err:=client.Items.ByID("text").Get().Do(context.Background());err!=nil{t.Fatal(err)};if _,err:=other.Items.ByID(3).Delete().Do(context.Background());err!=nil{t.Fatal(err)}}
`})
}

func TestGeneratedIndependentComplexResourceContracts(t *testing.T) {
	fixture := strings.NewReplacer(`"schema":{"type":"string"}`, `"schema":{"type":"object","required":["value"],"properties":{"value":{"type":"string"}},"additionalProperties":false}`, `"schema":{"type":"number"}`, `"schema":{"type":"object","required":["value"],"properties":{"value":{"type":"number"}},"additionalProperties":false}`).Replace(independentResourceFixture)
	document, err := compiler.Compile([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}, Selection: &generator.Selection{Operations: []string{"read"}}, Clients: map[string]generator.Client{"numbers": {Selection: &generator.Selection{Operations: []string{"remove"}}}}, FailOnResourceOmission: true})
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"complex_test.go": `package consumer_test
import("context";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/clients/numbers";"example.test/consumer/api/models")
func TestIndependentComplexPaths(t *testing.T){server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/items/value,text"&&r.URL.Path!="/items/value,3"{t.Error(r.URL.Path)};w.WriteHeader(204)}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};other,err:=numbers.NewClient(numbers.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};if _,err:=client.Items.ByID(models.ResourceItemsByIDParameter{Value:"text"}).Get().Do(context.Background());err!=nil{t.Fatal(err)};if _,err:=other.Items.ByID(models.ClientNumbersResourceItemsByIDParameter{Value:3}).Delete().Do(context.Background());err!=nil{t.Fatal(err)}}
`})
}
