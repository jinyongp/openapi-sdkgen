package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedMediaSelectionPreservesParameters(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Selected media","version":"1"},"paths":{"/source":{"get":{"operationId":"source","responses":{"204":{"description":"OK","links":{"Next":{"operationId":"target","requestBody":{"contentType":"application/json","value":{"id":"x"}}}}}}}},"/target":{"post":{"operationId":"target","requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}},"text/plain":{"schema":{"type":"string"}}}},"responses":{"204":{"description":"OK"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"media_test.go": `package consumer_test
import("context";"net/http";"net/http/httptest";"reflect";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/selective";"example.test/consumer/api/types")
func TestSelectedContentType(t *testing.T){var seen []string;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.URL.Path=="/target"{seen=append(seen,r.Header.Get("Content-Type"))};w.WriteHeader(204)}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};ctx:=context.Background();raw,err:=client.Operations.Source().Raw(ctx);if err!=nil{t.Fatal(err)};override:=(sdk.TargetLinkOverride{}).WithBodyContentType("application/json; charset=utf-8");if _,err:=client.Links.Source.Next.Follow(raw,sdk.SourceNextLinkInvocation{Input:types.Some(override)}).Do(ctx);err!=nil{t.Fatal(err)}
 ref,ok:=selective.LookupOperation("target");if !ok{t.Fatal("missing target")};prepared,err:=selective.Prepare(ref);if err!=nil{t.Fatal(err)};selected,err:=selective.NewClient(selective.ClientOptions{BaseURL:server.URL},prepared);if err!=nil{t.Fatal(err)};target,err:=selective.Bind(selected,ref);if err!=nil{t.Fatal(err)};input,err:=types.ValueFrom(map[string]any{"body":map[string]any{"contentType":"application/json; charset=utf-8","value":map[string]any{"id":"x"}}});if err!=nil{t.Fatal(err)};if _,err:=target.Call(input).Do(ctx);err!=nil{t.Fatal(err)}
 if !reflect.DeepEqual(seen,[]string{"application/json; charset=utf-8","application/json; charset=utf-8"}){t.Fatal("selected media parameters lost",seen)}
}
`})
}
