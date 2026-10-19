package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedStaticSelection(t *testing.T) {
	document, err := compiler.Compile([]byte(selectionFixture))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"static_test.go": `package consumer_test
import("context";"errors";"io";"net/http";"net/http/httptest";"reflect";"sync/atomic";"testing";"example.test/consumer/api/operations";"example.test/consumer/api/selective";"example.test/consumer/api/types")
type emptyReference struct{operations.Reference}
func TestPreparedTypedCalls(t *testing.T){var requests atomic.Int32;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1);if r.URL.Path=="/source"{w.Header().Set("Content-Type","application/json");io.WriteString(w,"{\"id\":\"item\"}")}else{w.Header().Set("Content-Type","text/plain");io.WriteString(w,"linked")}}));defer server.Close()
 ref:=operations.Source();prepared,err:=selective.Prepare(ref,ref);if err!=nil||!reflect.DeepEqual(prepared.Routes(),[]string{"GET /source"}){t.Fatal(err,prepared.Routes())};routes:=prepared.Routes();routes[0]="mutated";client,err:=selective.NewClient(selective.ClientOptions{BaseURL:server.URL},prepared);if err!=nil{t.Fatal(err)};source,err:=selective.Bind(client,ref);if err!=nil||requests.Load()!=0{t.Fatal("eager preparation",err)}
 if _,err:=selective.Bind(client,operations.Unused());err==nil{t.Fatal("unprepared operation accepted")}else{var failure *types.SDKError;if !errors.As(err,&failure)||failure.Code!=types.OperationNotPrepared{t.Fatal(err)}}
 raw,err:=source.Call().Raw(context.Background());if err!=nil{t.Fatal(err)};linked,err:=source.Links.Next.Follow(raw).Do(context.Background());if err!=nil||linked!="linked"||requests.Load()!=2{t.Fatal(linked,err,requests.Load())}
 if _,err:=selective.Prepare();err==nil{t.Fatal("empty preparation accepted")};var zero operations.Ref[operations.SourceOperation];if _,err:=selective.Prepare(zero);err==nil{t.Fatal("zero reference accepted")};var nilRef *operations.Ref[operations.SourceOperation];if _,err:=selective.Prepare(nilRef);err==nil{t.Fatal("nil reference accepted")}
 if _,err:=selective.Prepare(emptyReference{});err==nil{t.Fatal("empty embedded reference accepted")}
}
`})
}
