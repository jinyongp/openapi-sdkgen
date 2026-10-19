package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"strings"
	"testing"
)

func TestGeneratedTypedInboundRawResponseHeaders(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Headers","version":"1"},"paths":{},"webhooks":{"received":{"post":{"responses":{"204":{"description":"Accepted","headers":{"X-Count":{"schema":{"type":"integer","minimum":1}}}}}}}}}`
	document, err := compiler.Compile([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	addons, _ := generator.NewAddonRegistry(generator.AddonServer)
	options, _ := addons.Resolve([]string{"server"})
	options.Golang = &generator.GoOptions{ImportPath: "example.test/consumer/api"}
	plan, findings, err := (Generator{}).Prepare(document, options)
	if err != nil || diagnostic.HasErrors(findings) {
		t.Fatal(err, findings)
	}
	names, _ := planNames(document)
	entries, findings := prepareInboundOperations(document, inboundScopes(document, nil, nil), names, new(schemaPlanner))
	if diagnostic.HasErrors(findings) || len(entries) != 1 {
		t.Fatal(findings)
	}
	entry := entries[0]
	consumer := strings.NewReplacer("PUBLIC", entry.PublicName, "HEADERMODEL", entry.Operation.Headers[0].Model, "VARIANT", entry.Operation.Responses[0].Name).Replace(`package consumer_test
import("context";"net/http";"net/http/httptest";"testing";"example.test/consumer/api/models";"example.test/consumer/api/server/webhooks")
func TestRawResponseHeaders(t *testing.T){for _,test:=range []struct{raw string;status int}{ {"3",204},{"invalid",500},{"0",500} }{handler,err:=webhooks.NewPUBLICHandler("/",webhooks.Options{},func(context.Context,webhooks.PUBLICInput)(webhooks.PUBLICResponse,error){return webhooks.NewPUBLICVARIANTResponse(models.HEADERMODEL{}).WithHeaders(http.Header{"X-Count":{test.raw}}),nil});if err!=nil{t.Fatal(err)};recorder:=httptest.NewRecorder();handler.ServeHTTP(recorder,httptest.NewRequest("POST","/",nil));if recorder.Code!=test.status{t.Fatal(test,recorder.Code,recorder.Body.String())}}}
`)
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer})
}
