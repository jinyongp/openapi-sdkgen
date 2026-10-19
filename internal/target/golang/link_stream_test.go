package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedLinkStreamTargets(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Link streams","version":"1"},"paths":{"/source":{"get":{"operationId":"readSource","responses":{"200":{"description":"Done","links":{"Next":{"operationId":"readStream"}}},"201":{"description":"Done","links":{"Next":{"operationId":"readFinite"}}}}}},"/stream":{"get":{"operationId":"readStream","responses":{"200":{"description":"Stream","links":{"Header":{"operationId":"done","parameters":{"id":"$response.header.X-ID"}},"Body":{"operationId":"done","parameters":{"id":"$response.body#/id"}}},"content":{"application/x-ndjson":{"itemSchema":{"type":"string"}}}}}}},"/finite":{"get":{"operationId":"readFinite","responses":{"204":{"description":"Done"}}}},"/done/{id}":{"get":{"operationId":"done","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"204":{"description":"Done"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"link_stream_test.go": linkStreamConsumer})
}

const linkStreamConsumer = `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"sync/atomic";"testing";sdk "example.test/consumer/api")
func TestLinkStreamsAndBorrowedSource(t *testing.T){
 var requests atomic.Int32;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1);switch r.URL.Path{case "/source":w.WriteHeader(200);case "/stream":w.Header().Set("Content-Type","application/x-ndjson");w.Header().Set("X-ID","header");io.WriteString(w,"\"one\"\n\"two\"\n");default:w.WriteHeader(204)}}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 source,err:=client.Operations.ReadSource().Raw(context.Background());if err!=nil{t.Fatal(err)};before:=requests.Load();request:=client.Links.ReadSource.Next.Follow(source);stream:=request.Stream(context.Background());defer stream.Close();if requests.Load()!=before{t.Fatal("stream Link started eagerly")};var items []string;for stream.Next(){item,ok:=stream.Item().As200();if !ok{t.Fatal("stream branch")};items=append(items,string(item))};if stream.Err()!=nil||len(items)!=2||items[0]!="one"||items[1]!="two"{t.Fatal(items,stream.Err())}
 borrowed,err:=client.Operations.ReadStream().Raw(context.Background());if err!=nil{t.Fatal(err)};defer borrowed.Close();before=requests.Load();if _,err:=client.Links.ReadStream.Header.Follow(borrowed).Do(context.Background());err!=nil||requests.Load()!=before+1{t.Fatal("header-only Link consumed source",err)}
 before=requests.Load();if _,err:=client.Links.ReadStream.Body.Follow(borrowed).Do(context.Background());err==nil||requests.Load()!=before{t.Fatal("unmaterialized body Link was sent",err)};bytes,err:=io.ReadAll(borrowed.Body);if err!=nil||string(bytes)!="\"one\"\n\"two\"\n"{t.Fatal("Link consumed borrowed body",string(bytes),err)}
}
`
