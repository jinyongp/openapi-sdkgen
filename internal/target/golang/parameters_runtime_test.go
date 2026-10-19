package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedOperationParametersAndOptions(t *testing.T) {
	const source = `{"openapi":"3.1.0","info":{"title":"Parameters","version":"1"},"paths":{"/items/{id}":{"get":{"operationId":"getItem","parameters":[{"in":"path","name":"id","required":true,"description":"Original path ID","deprecated":true,"schema":{"type":"string","minLength":1,"readOnly":true}},{"in":"query","name":"tags","schema":{"type":"array","items":{"type":"string"}}},{"in":"query","name":"filter","style":"deepObject","explode":true,"schema":{"type":"object","additionalProperties":{"type":"string"}}},{"in":"header","name":"X-Count","schema":{"type":"integer","minimum":0}},{"in":"cookie","name":"session","schema":{"type":"string"}}],"responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"type":"boolean"}}}}}}},"/optional":{"get":{"operationId":"optional","parameters":[{"in":"query","name":"q","schema":{"type":"string"}}],"responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"type":"boolean"}}}}}}}}}`
	document, err := compiler.Compile([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": parameterConsumer})
}

const parameterConsumer = `package consumer_test
import("context";"errors";"net/http";"net/http/httptest";"strings";"sync";"sync/atomic";"testing";"time";sdk "example.test/consumer/api";"example.test/consumer/api/types";"example.test/consumer/api/models")
func TestParametersAndOptions(t *testing.T){
 var requests atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1)
  if r.URL.Path=="/optional" {w.Header().Set("Content-Type","application/json");w.Write([]byte("true"));return}
  if r.URL.EscapedPath()!="/items/a%2Fb%20%F0%9F%98%80"{t.Error("escaped path",r.URL.EscapedPath())}
  if r.URL.RawQuery!="tags=one&tags=two%20three&filter%5Ba%5D=x%26y"{t.Error("query",r.URL.RawQuery)}
  if r.Header.Get("X-Count")!="0"||r.Header.Get("Cookie")!="session=a%3Bb"{t.Error("parameters",r.Header)}
  if r.Header.Get("Authorization")!=""||r.Header.Get("X-Extra")!="copied"{t.Error("options snapshot",r.Header)}
  w.Header().Set("Content-Type","application/json");w.Write([]byte("true"))
 }));defer server.Close()
 defaults:=http.Header{"X-Extra":[]string{"initial"}}
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Headers:defaults,Authorization:types.Some("secret")});if err!=nil{t.Fatal(err)}
 defaults.Set("X-Extra","mutated")
 input:=sdk.GetItemInput{Path:sdk.GetItemPath{ID:"a/b 😀"},Query:types.Some(sdk.GetItemQuery{Tags:types.Some([]string{"one","two three"}),Filter:types.Some(models.GetItemQueryFilter{AdditionalProperties:map[string]string{"a":"x&y"}})}),Header:types.Some(sdk.GetItemHeader{XCount:types.Some(float64(0))}),Cookie:types.Some(sdk.GetItemCookie{Session:types.Some("a;b")})}
 headers:=http.Header{"X-Extra":[]string{"copied"}}
 call:=client.Operations.GetItem(input).WithOptions(sdk.RequestOptions{Headers:headers,Authorization:types.Some("")})
 headers.Set("X-Extra","changed")
 if _,err:=call.WithOptions(sdk.RequestOptions{BaseURL:types.Some(server.URL+"/unused/%2e%2E/")}).Do(context.Background());err!=nil{t.Fatal("base path dot resolution",err)}
 for count:=0;count<2;count++{value,err:=call.Do(context.Background());if err!=nil||!bool(value){t.Fatal(value,err)}}
 var workers sync.WaitGroup;for count:=0;count<16;count++{workers.Go(func(){value,err:=call.Do(context.Background());if err!=nil||!bool(value){t.Error("concurrent immutable call",value,err)}})};workers.Wait()
 if _,err:=client.Operations.Optional().Do(context.Background());err!=nil{t.Fatal(err)}
 before:=requests.Load()
 for _,invalid:=range []sdk.GetItemInput{{Path:sdk.GetItemPath{ID:""}},{Path:sdk.GetItemPath{ID:".."}}}{if _,err:=client.Operations.GetItem(invalid).Do(context.Background());err==nil{t.Fatal("accepted invalid input")}}
 if _,err:=client.Operations.Optional(sdk.OptionalInput{},sdk.OptionalInput{}).Do(context.Background());err==nil{t.Fatal("accepted excess arguments")}
 if _,err:=call.WithOptions(sdk.RequestOptions{Timeout:types.Some(time.Duration(0))}).Do(context.Background());err==nil{t.Fatal("accepted zero timeout")}
 if _,err:=call.WithOptions(sdk.RequestOptions{Headers:http.Header{"x-count":[]string{"2"}}}).Do(context.Background());err==nil{t.Fatal("raw contract header override")}
 if before!=requests.Load(){t.Fatal("invalid calls reached HTTP")}
 ctx,cancel:=context.WithCancel(context.Background());cancel()
 _,err=call.Do(ctx);var apiError *sdk.APIError
 if !errors.Is(err,context.Canceled)||!errors.As(err,&apiError)||apiError.Code!=types.RequestAborted{t.Fatal("cancellation",err)}
 if strings.Contains(err.Error(),"secret"){t.Fatal("credential in error")}
}
`
