package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedPaginationItemVariants(t *testing.T) {
	document, err := compiler.CompileFile("../../../test/fixtures/pagination-item-variants.json")
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"pagination_variants_test.go": paginationVariantsConsumer})
}

const paginationVariantsConsumer = `package consumer_test
import("context";"encoding/json";"net/http";"net/http/httptest";"reflect";"testing";sdk "example.test/consumer/api")
func TestEachResponseItemContractAndEnvelope(t *testing.T){server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");second:=r.URL.Query().Get("cursor")!="";var items any;var next any="next";if second{w.WriteHeader(206);next=nil};if r.URL.Path=="/names"{name:="long";if second{name="x"};items=[]string{name}}else{tuple:=[]string{"first"};if second{tuple=[]string{}};items=[][]string{tuple}};json.NewEncoder(w).Encode(map[string]any{"data":items,"meta":map[string]any{"pagination":map[string]any{"nextCursor":next}}})}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 names:=client.Names.List().Paginate(context.Background());defer names.Close();var values []string;for names.Next(){item:=names.Item();if first,ok:=item.AsAnyOf1();ok{values=append(values,string(first))}else if second,ok:=item.AsAnyOf2();ok{values=append(values,string(second))}else{t.Fatal("unreadable item")};if _,err:=json.Marshal(item);err!=nil{t.Fatal("first response constraints applied to later item",err)}};if names.Err()!=nil||!reflect.DeepEqual(values,[]string{"long","x"}){t.Fatal(values,names.Err())}
 tuples:=client.Tuples.List().Paginate(context.Background());defer tuples.Close();count:=0;for tuples.Next(){item:=tuples.Item();if count==0{first,ok:=item.AsAnyOf1();if !ok||first.Item0!="first"{t.Fatal(first,ok)}}else{second,ok:=item.AsAnyOf2();if !ok||second.Item0.IsSet(){t.Fatal("empty tuple was narrowed to required prefix",second,ok)}};if _,err:=json.Marshal(item);err!=nil{t.Fatal(err)};count++};if tuples.Err()!=nil||count!=2{t.Fatal(count,tuples.Err())}
}
`
