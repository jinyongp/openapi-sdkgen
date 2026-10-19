package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedRuntimeSelection(t *testing.T) {
	document, err := compiler.Compile([]byte(selectionFixture))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"dynamic_test.go": `package consumer_test
import("context";"errors";"io";"net/http";"net/http/httptest";"sync";"sync/atomic";"testing";"example.test/consumer/api/operations";"example.test/consumer/api/selective";"example.test/consumer/api/types")
func TestComputedOperationSelection(t *testing.T){var requests atomic.Int32;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1);if r.URL.Path=="/source"{w.Header().Set("Content-Type","application/json");io.WriteString(w,"{\"id\":\"item\",\"extra\":\"preserved\"}")}else{if r.URL.Path!="/target/item"{t.Error(r.URL.Path)};w.Header().Set("Content-Type","text/plain");io.WriteString(w,"linked")}}));defer server.Close()
 var wait sync.WaitGroup;for range 8{wait.Go(func(){if _,ok:=selective.LookupOperation("source");!ok{t.Error("concurrent registry")}})};wait.Wait()
 sourceRef,ok:=selective.LookupOperation("source");if !ok{t.Fatal("missing operation")};targetRef,ok:=selective.LookupRoute("GET /target/{id}");if !ok{t.Fatal("exact route missing")};if _,ok:=selective.LookupRoute("get /target/{id}");ok{t.Fatal("route spelling changed")};if _,ok:=selective.LookupOperation("hidden");ok{t.Fatal("hidden operation leaked")};if _,ok:=selective.LookupOperation("missing");ok{t.Fatal("unknown operation invented")}
 prepared,err:=selective.Prepare(sourceRef,targetRef);if err!=nil{t.Fatal(err)};client,err:=selective.NewClient(selective.ClientOptions{BaseURL:server.URL},prepared);if err!=nil{t.Fatal(err)};source,err:=selective.Bind(client,sourceRef);if err!=nil||!source.CanDo()||source.CanStream()||requests.Load()!=0{t.Fatal("eager computed bind",err)};value,err:=source.Call(types.Value{}).Do(context.Background());if err!=nil{t.Fatal(err)};extra,_:=value.Get("extra");if text,_:=extra.String();text!="preserved"{t.Fatal("dynamic response property lost")}
 target,err:=selective.Bind(client,targetRef);if err!=nil{t.Fatal(err)};input,err:=types.ValueFrom(map[string]any{"path":map[string]any{"id":"item"}});if err!=nil{t.Fatal(err)};value,err=target.Call(input).Do(context.Background());if text,_:=value.String();err!=nil||text!="linked"{t.Fatal(text,err)}
 before:=requests.Load();if _,err:=target.Call(types.Value{}).Do(context.Background());err==nil||requests.Load()!=before{t.Fatal("missing required input sent",err)}else{var failure *types.APIError;if !errors.As(err,&failure)||failure.Code!=types.RequestEncodeFailed{t.Fatal("wrong error boundary",err)}}
 raw,err:=source.Call(types.Value{}).Raw(context.Background());if err!=nil||raw.Status!=200||raw.Body!=http.NoBody{t.Fatal(raw,err)};raw.Close();copy:=raw.Bytes();copy[0]='!';if string(raw.Bytes())==string(copy){t.Fatal("mutable raw bytes")}
 raw.Status=500;raw.Data=types.Value{};before=requests.Load();follow:=source.Links.Follow("Next",raw);if requests.Load()!=before{t.Fatal("Follow performed HTTP")};value,err=follow.Do(context.Background());if text,_:=value.String();err!=nil||text!="linked"{t.Fatal("computed Link original snapshot",text,err)}
 value,err=source.Links.ByStatus("200").Follow("Next",raw).Do(context.Background());if text,_:=value.String();err!=nil||text!="linked"{t.Fatal("forced computed Link",text,err)}
 before=requests.Load();if _,err:=source.Links.Follow("missing",raw).Do(context.Background());err==nil||requests.Load()!=before{t.Fatal("unknown Link sent HTTP")};if _,err:=source.Links.Follow("Next",&operations.DynamicRawResponse{}).Do(context.Background());err==nil||requests.Load()!=before{t.Fatal("fabricated dynamic source accepted")}
}
`})
}
