package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedResourceArgumentFallback(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Resource typed fallback","version":"1"},"paths":{"/tuples/{id}":{"get":{"operationId":"getTuples","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"array","prefixItems":[{"type":"string"}],"items":false,"minItems":1}}],"responses":{"200":{"description":"Done","content":{"text/plain":{"schema":{"type":"string"}}}}}}},"/tuples/{key}":{"post":{"operationId":"createTuples","parameters":[{"name":"key","in":"path","required":true,"schema":{"type":"array","prefixItems":[{"type":"string"}],"items":false,"minItems":0}}],"responses":{"200":{"description":"Done","content":{"text/plain":{"schema":{"type":"string"}}}}}}},"/choices/{id}":{"get":{"operationId":"getChoices","parameters":[{"name":"id","in":"path","required":true,"schema":{"anyOf":[{"type":"string","minLength":3},{"type":"integer"}]}}],"responses":{"200":{"description":"Done","content":{"text/plain":{"schema":{"type":"string"}}}}}}},"/choices/{key}":{"post":{"operationId":"createChoices","parameters":[{"name":"key","in":"path","required":true,"schema":{"anyOf":[{"type":"string","minLength":1},{"type":"integer"}]}}],"responses":{"200":{"description":"Done","content":{"text/plain":{"schema":{"type":"string"}}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	requireResourceWarnings(t, diagnostics, 4)
	testGeneratedModule(t, plan, map[string]string{"resource_fallback_test.go": resourceFallbackConsumer})
}

const resourceFallbackConsumer = `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"reflect";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models")
func TestTypedRoutesForIncompatibleArgumentContracts(t *testing.T){server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","text/plain");io.WriteString(w,r.URL.Path)}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 for _,namespace:=range []string{"Tuples","Choices"}{if _,present:=reflect.TypeOf(*client).FieldByName(namespace);present{t.Fatal("incompatible selector exposed",namespace)}}
 tuple,err:=client.Routes.PostTuplesByKey(sdk.CreateTuplesInput{Path:sdk.CreateTuplesPath{Key:models.CreateTuplesPathKey{}}}).Do(context.Background());if err!=nil||tuple!="/tuples/"{t.Fatal("empty tuple route fallback",tuple,err)}
 key,err:=models.CreateChoicesPathKeyFromAnyOf1("x");if err!=nil{t.Fatal(err)};choice,err:=client.Routes.PostChoicesByKey(sdk.CreateChoicesInput{Path:sdk.CreateChoicesPath{Key:key}}).Do(context.Background());if err!=nil||choice!="/choices/x"{t.Fatal("broader sealed value route fallback",choice,err)}
}
`
