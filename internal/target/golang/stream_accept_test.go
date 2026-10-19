package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedStreamAcceptDefaultsAndOverrides(t *testing.T) {
	const input = `{"openapi":"3.2.0","info":{"title":"Stream Accept","version":"1"},"paths":{"/events":{"get":{"operationId":"events","responses":{"200":{"description":"JSON lines","content":{"application/x-ndjson":{"itemSchema":{"type":"integer"}}}},"201":{"description":"Events","content":{"text/event-stream":{"itemSchema":{"type":"object","required":["data"],"properties":{"data":{"type":"string"}}}}}}}}}}}`
	document, err := compiler.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	plan, findings, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(findings) != 0 {
		t.Fatal(err, findings)
	}
	testGeneratedModule(t, plan, map[string]string{"stream_accept_test.go": streamAcceptConsumer})
}

const streamAcceptConsumer = `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"testing";api "example.test/consumer/api";"example.test/consumer/api/types";"example.test/consumer/api/selective")
func TestAcceptFallbackAfterDefaults(t *testing.T){server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.Header.Get("Accept")=="text/event-stream"{w.Header().Set("Content-Type","text/event-stream");w.WriteHeader(201);io.WriteString(w,"data: selected\n\n")}else{w.Header().Set("Content-Type","application/x-ndjson");io.WriteString(w,"7\n")}}));defer server.Close()
 for _,test:=range []struct{name string;client,request types.Optional[string];status int}{
 {name:"automatic",status:200},
 {name:"client",client:types.Some("text/event-stream"),status:201},
 {name:"request",client:types.Some("application/x-ndjson"),request:types.Some("text/event-stream"),status:201},
 {name:"explicit-empty",client:types.Some("text/event-stream"),request:types.Some(""),status:200},
 }{t.Run(test.name,func(t *testing.T){ctx:=context.Background();options:=api.ClientOptions{BaseURL:server.URL,Accept:test.client};client,err:=api.NewClient(options);if err!=nil{t.Fatal(err)};stream:=client.Operations.Events().WithOptions(types.RequestOptions{Accept:test.request}).Stream(ctx);defer stream.Close();response,err:=stream.Response();if err!=nil||response.StatusCode!=test.status{t.Fatalf("typed status %v %v; want%d",response,err,test.status)};if !stream.Next(){t.Fatal(stream.Err())};if stream.Next()||stream.Err()!=nil{t.Fatal(stream.Err())}
 ref,ok:=selective.LookupOperation("events");if !ok{t.Fatal("missing operation")};prepared,err:=selective.Prepare(ref);if err!=nil{t.Fatal(err)};selected,err:=selective.NewClient(options,prepared);if err!=nil{t.Fatal(err)};operation,err:=selective.Bind(selected,ref);if err!=nil{t.Fatal(err)};dynamic:=operation.Call(types.Value{}).WithOptions(types.RequestOptions{Accept:test.request}).Stream(ctx);defer dynamic.Close();response,err=dynamic.Response();if err!=nil||response.StatusCode!=test.status{t.Fatalf("dynamic status %v %v; want%d",response,err,test.status)};if !dynamic.Next(){t.Fatal(dynamic.Err())};if dynamic.Next()||dynamic.Err()!=nil{t.Fatal(dynamic.Err())}
 })}}
`
