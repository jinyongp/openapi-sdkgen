package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedComponentLinksAndNativeHeaders(t *testing.T) {
	document, err := compiler.Compile([]byte(`{
"openapi":"3.1.1","info":{"title":"Native header Links","version":"1"},
"components":{"links":{"ManagedOrigin":{"operationId":"getOrigin","parameters":{"Origin":"https://component.example"}}},"responses":{"ManagedResponse":{"description":"OK","links":{"follow":{"operationId":"getOrigin","parameters":{"Origin":"https://response.example"}}}}}},
"paths":{
"/source":{"get":{"operationId":"getSource","parameters":[{"name":"Origin","in":"header","required":true,"schema":{"type":"string"}}],"responses":{"204":{"description":"OK","links":{"trace":{"operationId":"getTrace","parameters":{"X-Trace":"$request.header.Origin"}},"origin":{"operationId":"getOrigin","parameters":{"Origin":"https://literal.example"}}}}}}},
"/component-source":{"get":{"operationId":"getComponentSource","responses":{"204":{"description":"OK","links":{"follow":{"$ref":"#/components/links/ManagedOrigin"}}}}}},
"/response-source":{"get":{"operationId":"getResponseSource","responses":{"204":{"$ref":"#/components/responses/ManagedResponse"}}}},
"/trace":{"get":{"operationId":"getTrace","parameters":[{"name":"X-Trace","in":"header","schema":{"type":"string"}}],"responses":{"204":{"description":"OK"}}}},
"/origin":{"get":{"operationId":"getOrigin","parameters":[{"name":"Origin","in":"header","required":true,"schema":{"type":"string"}}],"responses":{"204":{"description":"OK"}}}}
}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	root := testGeneratedModule(t, plan, map[string]string{"component_test.go": componentLinkConsumer})
	for _, candidate := range []struct{ name, source string }{
		{"wrong_operation_source", `package consumer_test; import sdk "example.test/consumer/api"; func invalid(c *sdk.Client,s *sdk.GetOriginRawResponse){c.Links.GetSource.Trace.Follow(s)}`},
		{"fabricated_source", `package consumer_test; import sdk "example.test/consumer/api"; func invalid(c *sdk.Client){c.Links.GetSource.Trace.Follow(struct{}{})}`},
		{"wrong_target_override", `package consumer_test; import sdk "example.test/consumer/api"; import "example.test/consumer/api/types"; func invalid(c *sdk.Client,s *sdk.GetSourceRawResponse){c.Links.GetSource.Trace.Follow(s,sdk.GetSourceTraceLinkInvocation{Input:types.Some(sdk.GetOriginLinkOverride{})})}`},
	} {
		t.Run(candidate.name, func(t *testing.T) {
			file := filepath.Join(root, "invalid_test.go")
			if err := os.WriteFile(file, []byte(candidate.source), 0600); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Remove(file) })
			command := exec.Command("go", "test", "-mod=readonly", "-run", "^$", ".")
			command.Dir = root
			command.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
			output, err := command.CombinedOutput()
			if err == nil || !strings.Contains(string(output), "cannot use") {
				t.Fatalf("expected Go type rejection: %v\n%s", err, output)
			}
		})
	}
}

const componentLinkConsumer = `package consumer_test
import("context";"net/http";"net/http/httptest";"reflect";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
type emptySource struct{sdk.GetSourceLinkSource}
func TestComponentsAndNativeHeaders(t *testing.T){
 var seen []string;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){seen=append(seen,r.URL.Path+"|"+r.Header.Get("Origin")+"|"+r.Header.Get("X-Trace"));w.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};ctx:=context.Background();input:=sdk.GetSourceInput{Header:sdk.GetSourceHeader{Origin:"https://source.example"}}
 source,err:=client.Operations.GetSource(input).Raw(ctx);if err!=nil{t.Fatal(err)}
 if _,err:=client.Links.GetSource.Trace.Follow(source,sdk.GetSourceTraceLinkInvocation{SourceInput:types.Some(input)}).Do(ctx);err!=nil{t.Fatal(err)}
 if _,err:=client.Links.GetSource.Origin.Follow(source).Do(ctx);err!=nil{t.Fatal(err)}
 component,err:=client.Operations.GetComponentSource().Raw(ctx);if err!=nil{t.Fatal(err)};if _,err:=client.Links.GetComponentSource.Follow.Follow(component).Do(ctx);err!=nil{t.Fatal(err)}
 response,err:=client.Operations.GetResponseSource().Raw(ctx);if err!=nil{t.Fatal(err)};if _,err:=client.Links.GetResponseSource.Follow.Follow(response).Do(ctx);err!=nil{t.Fatal(err)}
 expected:=[]string{"/source|https://source.example|","/trace||https://source.example","/origin|https://literal.example|","/component-source||","/origin|https://component.example|","/response-source||","/origin|https://response.example|"};if !reflect.DeepEqual(seen,expected){t.Fatal(seen)}
 before:=len(seen);if _,err:=client.Links.GetSource.Trace.Follow(emptySource{}).Do(ctx);err==nil||len(seen)!=before{t.Fatal("empty embedded source accepted",err)}
}
`
