package golang

import (
	"encoding/json"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedPagination(t *testing.T) {
	parameter := func(name, kind string) any {
		schema := map[string]any{"type": kind}
		if name == "limit" {
			schema["minimum"] = 1
		}
		if name == "offset" {
			schema["minimum"] = 0
		}
		return map[string]any{"name": name, "in": "query", "schema": schema}
	}
	object := func(properties map[string]any) map[string]any {
		return map[string]any{"type": "object", "properties": properties}
	}
	page := object(map[string]any{"items": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "pagination": object(map[string]any{"nextCursor": map[string]any{"type": []any{"string", "null"}}, "offset": map[string]any{"type": "integer", "minimum": 0}, "limit": map[string]any{"type": "integer", "minimum": 1}, "total": map[string]any{"type": "integer", "minimum": 0}})})
	paths := make(map[string]any)
	for _, mode := range []string{"cursor", "offset", "both"} {
		parameters := []any{parameter("filter", "string"), parameter("limit", "integer")}
		if mode != "offset" {
			parameters = append(parameters, parameter("cursor", "string"))
		}
		if mode != "cursor" {
			parameters = append(parameters, parameter("offset", "integer"))
		}
		paths["/"+mode] = map[string]any{"get": map[string]any{"operationId": mode + "Pages", "parameters": parameters, "x-pagination": mode, "responses": map[string]any{"200": map[string]any{"description": "OK", "content": map[string]any{"application/json": map[string]any{"schema": page}}}}}}
	}
	paths["/start"] = map[string]any{"get": map[string]any{"operationId": "startPages", "responses": map[string]any{"204": map[string]any{"description": "OK", "links": map[string]any{"Next": map[string]any{"operationId": "cursorPages", "parameters": map[string]any{"filter": "keep"}}}}}}}
	encoded, err := json.Marshal(map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Pagination", "version": "1"}, "paths": paths})
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
	testGeneratedModule(t, plan, map[string]string{"pagination_test.go": paginationConsumer})
}

const paginationConsumer = `package consumer_test
import("context";"encoding/json";"errors";"net/http";"net/http/httptest";"reflect";"sync/atomic";"testing";"time";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestLazyCursorAndOffset(t *testing.T){
 var requests atomic.Int32;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1);w.Header().Set("Content-Type","application/json");if r.URL.Query().Get("filter")!="keep"{t.Error("filter lost",r.URL.RawQuery)};switch r.URL.Path{
 case "/cursor":switch r.URL.Query().Get("cursor"){case "":json.NewEncoder(w).Encode(map[string]any{"items":[]string{"a","b"},"pagination":map[string]any{"nextCursor":"empty"}});case "empty":json.NewEncoder(w).Encode(map[string]any{"items":[]string{},"pagination":map[string]any{"nextCursor":"last"}});case "last":json.NewEncoder(w).Encode(map[string]any{"items":[]string{"c"},"pagination":map[string]any{"nextCursor":"empty"}});default:t.Error("unexpected cursor")}
 case "/offset","/both":switch r.URL.Query().Get("offset"){case "","0":json.NewEncoder(w).Encode(map[string]any{"items":[]string{"a","b"},"pagination":map[string]any{"offset":0,"limit":2,"total":3}});case "2":json.NewEncoder(w).Encode(map[string]any{"items":[]string{"c"},"pagination":map[string]any{"offset":2,"limit":2,"total":3}});default:t.Error("unexpected offset")}
 }}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 input:=sdk.CursorPagesInput{Query:types.Some(sdk.CursorPagesQuery{Filter:types.Some("keep"),Limit:types.Some(float64(2))})};pager:=client.Cursor.Get(input).Paginate(context.Background());defer pager.Close();if requests.Load()!=0{t.Fatal("eager HTTP")};var items []string;for pager.Next(){items=append(items,string(pager.Item()))};if pager.Err()!=nil||!reflect.DeepEqual(items,[]string{"a","b","c"})||requests.Load()!=3{t.Fatal(items,pager.Err(),requests.Load())};original,_:=input.Query.Get();if original.Cursor.IsSet(){t.Fatal("input mutated")}
 offset:=client.Operations.OffsetPages(sdk.OffsetPagesInput{Query:types.Some(sdk.OffsetPagesQuery{Filter:types.Some("keep"),Limit:types.Some(float64(2))})}).Paginate(context.Background());defer offset.Close();items=nil;for offset.Next(){items=append(items,string(offset.Item()))};if offset.Err()!=nil||!reflect.DeepEqual(items,[]string{"a","b","c"})||requests.Load()!=5{t.Fatal(items,offset.Err(),requests.Load())}
 both:=client.Operations.BothPages(sdk.BothPagesInput{Query:types.Some(sdk.BothPagesQuery{Filter:types.Some("keep"),Limit:types.Some(float64(2))})}).Paginate(context.Background(),sdk.BothPagesPaginationOffset);defer both.Close();for both.Next(){};if both.Err()!=nil||requests.Load()!=7{t.Fatal("both mode",both.Err(),requests.Load())}
 before:=requests.Load();wrong:=client.Operations.BothPages(sdk.BothPagesInput{Query:types.Some(sdk.BothPagesQuery{Offset:types.Some(float64(0))})}).Paginate(context.Background(),sdk.BothPagesPaginationCursor);defer wrong.Close();if wrong.Next()||wrong.Err()==nil||requests.Load()!=before{t.Fatal("forbidden offset was sent")}
 unstarted:=client.Cursor.Get().Paginate(context.Background());if err:=unstarted.Close();err!=nil{t.Fatal(err)};if unstarted.Next()||unstarted.Err()!=nil||requests.Load()!=before{t.Fatal("closed unstarted pager",unstarted.Err())}
}
func TestPaginationCancellation(t *testing.T){
 started:=make(chan struct{});server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){close(started);<-r.Context().Done()}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 pager:=client.Cursor.Get().Paginate(context.Background());done:=make(chan bool,1);go func(){done<-pager.Next()}();select{case <-started:case <-time.After(time.Second*5):t.Fatal("request did not start")};if err:=pager.Close();err!=nil{t.Fatal(err)};select{case result:=<-done:if result{t.Fatal("item after close")};case <-time.After(time.Second*5):t.Fatal("close did not cancel HTTP")};if err:=pager.Err();err!=nil&&!errors.Is(err,context.Canceled){t.Fatal(err)}
}
func TestLinkPaginationPreparesOnce(t *testing.T){
 var pages atomic.Int32;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.URL.Path=="/start"{w.WriteHeader(204);return};pages.Add(1);w.Header().Set("Content-Type","application/json");if r.URL.Query().Get("filter")!="keep"{t.Error("derived filter lost")};if r.URL.Query().Get("cursor")=="next"{json.NewEncoder(w).Encode(map[string]any{"items":[]string{"b"},"pagination":map[string]any{}})}else{json.NewEncoder(w).Encode(map[string]any{"items":[]string{"a"},"pagination":map[string]any{"nextCursor":"next"}})}}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};source,err:=client.Operations.StartPages().Raw(context.Background());if err!=nil{t.Fatal(err)};pager:=client.Links.StartPages.Next.Follow(source).Paginate(context.Background());defer pager.Close();if pages.Load()!=0{t.Fatal("eager Link pagination")};var got []string;for pager.Next(){got=append(got,string(pager.Item()))};if pager.Err()!=nil||!reflect.DeepEqual(got,[]string{"a","b"})||pages.Load()!=2{t.Fatal("Link query reset per page",got,pages.Load(),pager.Err())}
}
`
