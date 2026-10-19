package golang

import (
	"encoding/json"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedNativeGoMethodsAndHeaders(t *testing.T) {
	response := map[string]any{"200": map[string]any{"description": "OK", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "boolean"}}}}}
	operation := func(id string) map[string]any { return map[string]any{"operationId": id, "responses": response} }
	query, trace, custom, lowerHead := operation("queryItems"), operation("traceItems"), operation("customItems"), operation("lowerHead")
	getBody := operation("getBody")
	getBody["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "boolean"}}}}
	header := operation("nativeHeaders")
	header["parameters"] = []any{map[string]any{"name": "Host", "in": "header", "required": true, "schema": map[string]any{"type": "string"}}, map[string]any{"name": "Accept-Encoding", "in": "header", "required": true, "schema": map[string]any{"type": "string"}}}
	input := map[string]any{"openapi": "3.2.0", "info": map[string]any{"title": "Methods", "version": "1"}, "paths": map[string]any{
		"/methods":  map[string]any{"query": query, "trace": trace, "additionalOperations": map[string]any{"do-it": custom, "head": lowerHead}},
		"/get-body": map[string]any{"get": getBody}, "/headers": map[string]any{"get": header},
	}}
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	document, err := compiler.Compile(encoded)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": nativeMethodsConsumer})
}

const nativeMethodsConsumer = `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api")
func TestNativeMethodsAndHeaders(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  switch r.URL.Path{case "/methods":if r.Method!="QUERY"&&r.Method!="TRACE"&&r.Method!="do-it"&&r.Method!="head"{t.Error("method was changed",r.Method)}
  case "/get-body":data,err:=io.ReadAll(r.Body);if err!=nil||r.Method!="GET"||string(data)!="true"{t.Error("GET body inherited Fetch restriction",r.Method,string(data),err)}
  case "/headers":if r.Host!="contract.example.test"||r.Header.Get("Accept-Encoding")!="identity"{t.Error("native header mapping",r.Host,r.Header)}
  };w.Header().Set("Content-Type","application/json");w.Write([]byte("true"))
 }));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 ctx:=context.Background()
 if result,err:=client.Operations.QueryItems().Do(ctx);err!=nil||!bool(result){t.Fatal("QUERY",result,err)}
 if result,err:=client.Operations.TraceItems().Do(ctx);err!=nil||!bool(result){t.Fatal("TRACE",result,err)}
 if result,err:=client.Operations.CustomItems().Do(ctx);err!=nil||!bool(result){t.Fatal("custom method",result,err)}
 if result,err:=client.Operations.LowerHead().Do(ctx);err!=nil||!bool(result){t.Fatal("lower-case custom head lost body",result,err)}
 if result,err:=client.Operations.GetBody(sdk.GetBodyInput{Body:true}).Do(ctx);err!=nil||!bool(result){t.Fatal("GET body",result,err)}
 if result,err:=client.Operations.NativeHeaders(sdk.NativeHeadersInput{Header:sdk.NativeHeadersHeader{Host:"contract.example.test",AcceptEncoding:"identity"}}).Do(ctx);err!=nil||!bool(result){t.Fatal("native headers",result,err)}
}
`
