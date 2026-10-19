package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedLinkStatusGroups(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Link status","version":"1"},"paths":{"/exact/{id}":{"get":{"operationId":"readExact","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}},{"name":"status","in":"query","required":true,"schema":{"type":"integer"}}],"responses":{"200":{"description":"Done","content":{"application/json":{"schema":{"type":"object","properties":{"path":{"type":"string"},"status":{"type":"string"}},"required":["path","status"],"additionalProperties":false}}}}}}},"/range/{id}":{"get":{"operationId":"readRange","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}},{"name":"status","in":"query","required":true,"schema":{"type":"integer"}}],"responses":{"200":{"description":"Done","content":{"application/json":{"schema":{"type":"object","properties":{"path":{"type":"string"},"status":{"type":"string"}},"required":["path","status"],"additionalProperties":false}}}}}}},"/conflict/{id}":{"get":{"operationId":"readConflict","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}},{"name":"status","in":"query","required":true,"schema":{"type":"integer"}}],"responses":{"200":{"description":"Done","content":{"application/json":{"schema":{"type":"object","properties":{"path":{"type":"string"},"status":{"type":"string"}},"required":["path","status"],"additionalProperties":false}}}}},"security":[{"key":[]},{}]}},"/default/{id}":{"get":{"operationId":"readDefault","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}},{"name":"status","in":"query","required":true,"schema":{"type":"integer"}}],"responses":{"200":{"description":"Done","content":{"application/json":{"schema":{"type":"object","properties":{"path":{"type":"string"},"status":{"type":"string"}},"required":["path","status"],"additionalProperties":false}}}}}}},"/source":{"get":{"operationId":"readSource","responses":{"201":{"description":"Done","content":{"application/json":{"schema":{"type":"object","properties":{"id":{"type":"string"}},"required":["id"],"additionalProperties":false}}},"links":{"Next":{"operationId":"readExact","parameters":{"id":"$response.body#/id","status":"$statusCode"}}}},"2XX":{"description":"Done","content":{"application/json":{"schema":{"type":"object","properties":{"id":{"type":"string"}},"required":["id"],"additionalProperties":false}}},"links":{"Next":{"operationId":"readRange","parameters":{"id":"$response.body#/id","status":"$statusCode"}}}},"409":{"description":"Done","content":{"application/json":{"schema":{"type":"object","properties":{"id":{"type":"string"}},"required":["id"],"additionalProperties":false}}},"links":{"Next":{"operationId":"readConflict","parameters":{"id":"$response.body#/id","status":"$statusCode"}}}},"default":{"description":"Done","content":{"application/json":{"schema":{"type":"object","properties":{"id":{"type":"string"}},"required":["id"],"additionalProperties":false}}},"links":{"Next":{"operationId":"readDefault","parameters":{"id":"$response.body#/id","status":"$statusCode"}}}}}}}},"components":{"securitySchemes":{"key":{"type":"apiKey","in":"header","name":"X-Key"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"link_status_test.go": linkStatusConsumer})
}

const linkStatusConsumer = `package consumer_test
import("context";"encoding/json";"net/http";"net/http/httptest";"strconv";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestStatusDispatchTagsAndRequiredOptions(t *testing.T){
 var requests atomic.Int32;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1);w.Header().Set("Content-Type","application/json");if r.URL.Path=="/source"{code,_:=strconv.Atoi(r.Header.Get("X-Status"));w.WriteHeader(code);json.NewEncoder(w).Encode(map[string]any{"id":"source-id"});return};if r.Header.Get("X-Status")!=""{t.Error("source request options inherited")};json.NewEncoder(w).Encode(map[string]any{"path":r.URL.Path,"status":r.URL.Query().Get("status")})}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 for _,code:=range []int{201,206,409,503}{source,err:=client.Operations.ReadSource().WithOptions(sdk.RequestOptions{Headers:http.Header{"X-Status":[]string{strconv.Itoa(code)}}}).Raw(context.Background());if source==nil{t.Fatal(err)}
  invocation:=sdk.ReadSourceNextLinkInvocation{};if code==409{invocation.Options=sdk.ReadSourceNextLinkOptionsFor409(sdk.ReadConflictLinkOptions{SecurityRequirement:sdk.ReadConflictSecurityAnonymous})}
  result,err:=client.Links.ReadSource.Next.Follow(source,invocation).Do(context.Background());if err!=nil{t.Fatal(code,err)}
  switch code{case 201:value,ok:=result.As201();if !ok||value.Path!="/exact/source-id"||value.Status!="201"{t.Fatal(code,result)};case 206:value,ok:=result.As2XX();if !ok||value.Path!="/range/source-id"||value.Status!="206"{t.Fatal(code,result)};case 409:value,ok:=result.As409();if !ok||value.Path!="/conflict/source-id"||value.Status!="409"{t.Fatal(code,result)};case 503:value,ok:=result.AsDefault();if !ok||value.Path!="/default/source-id"||value.Status!="503"{t.Fatal(code,result)}}
  before:=requests.Load();wrong:=sdk.ReadSourceNextLinkInvocation{Input:types.Some(sdk.ReadSourceNextLinkInputFor409(sdk.ReadConflictLinkOverride{}))};if code==409{wrong.Input=types.Some(sdk.ReadSourceNextLinkInputFor201(sdk.ReadExactLinkOverride{}))};if _,err:=client.Links.ReadSource.Next.Follow(source,wrong).Do(context.Background());err==nil||requests.Load()!=before{t.Fatal("mismatched invocation sent",code,err)}
  if code==201{forced,err:=client.Links.ReadSource.Next.ByStatus409.Follow(source,sdk.ReadSourceNextLinkInvocationFor409{Options:sdk.ReadConflictLinkOptions{SecurityRequirement:sdk.ReadConflictSecurityAnonymous}}).Do(context.Background());if err!=nil||forced.Path!="/conflict/source-id"||forced.Status!="201"{t.Fatal("forced definition changed source status",forced,err)}}
  if code==409{before=requests.Load();if _,err:=client.Links.ReadSource.Next.Follow(source,sdk.ReadSourceNextLinkInvocation{}).Do(context.Background());err==nil||requests.Load()!=before{t.Fatal("missing required branch options sent",err)}}
 }
}
`
