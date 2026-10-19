package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedComplexResourceArguments(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.0","info":{"title":"Complex resource arguments","version":"1"},"paths":{"/items/{key}":{"get":{"operationId":"getLong","parameters":[{"name":"key","in":"path","required":true,"schema":{"$ref":"#/components/schemas/Long"}}],"responses":{"200":{"description":"Done","content":{"text/plain":{"schema":{"type":"string"}}}}}}},"/items/{id}":{"post":{"operationId":"createShort","parameters":[{"name":"id","in":"path","required":true,"schema":{"$ref":"#/components/schemas/Short"}}],"responses":{"200":{"description":"Done","content":{"text/plain":{"schema":{"type":"string"}}}}}}}},"components":{"schemas":{"Long":{"type":"object","required":["value"],"properties":{"value":{"type":"string","minLength":3}},"additionalProperties":false},"Short":{"type":"object","required":["value"],"properties":{"value":{"type":"string","minLength":1}},"additionalProperties":false}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"resource_complex_test.go": resourceComplexConsumer})
}

const resourceComplexConsumer = `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models")
func TestSharedRepresentationAndPerOperationConstraints(t *testing.T){var requests atomic.Int32;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1);w.Header().Set("Content-Type","text/plain");io.WriteString(w,r.Method+" "+r.URL.Path)}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 argument:=models.ResourceItemsByKeyParameter{Value:"x"};resource:=client.Items.ByKey(argument);argument.Value="changed";if requests.Load()!=0{t.Fatal("selector performed HTTP")};value,err:=resource.Create().Do(context.Background());if err!=nil||value!="POST /items/value,x"{t.Fatal("own parameter name, constraints and snapshot",value,err)};before:=requests.Load();if _,err:=resource.Get().Do(context.Background());err==nil||requests.Load()!=before{t.Fatal("GET used POST's weaker schema",err)}
 getValue,err:=client.Items.ByKey(models.ResourceItemsByKeyParameter{Value:"long"}).Get().Do(context.Background());if err!=nil||getValue!="GET /items/value,long"{t.Fatal("compatible references lost selector",getValue,err)}
}
`
