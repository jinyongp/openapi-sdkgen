package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedPunctuationResourcePath(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Literal resource","version":"1"},"paths":{"/-":{"get":{"operationId":"readDash","responses":{"204":{"description":"OK"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"literal_test.go": `package consumer_test
import("context";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api")
func TestLiteralPath(t *testing.T){s:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.URL.Path!="/-"{t.Error(r.URL.Path)};w.WriteHeader(204)}));defer s.Close();c,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:s.URL});if err!=nil{t.Fatal(err)};if _,err:=c.Operations.ReadDash().Do(context.Background());err!=nil{t.Fatal(err)}}
`})
}
