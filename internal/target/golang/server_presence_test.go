package golang

import (
	"fmt"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedTypedInboundBodyPresence(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Presence","version":"1"},"paths":{},"webhooks":{"optional":{"post":{"requestBody":{"content":{"application/json":{"schema":{"type":["string","null"]}},"text/plain":{"schema":{"type":"string"}},"application/x-www-form-urlencoded":{"schema":{"type":"object"}},"application/octet-stream":{}}},"responses":{"204":{"description":"Accepted"}}}},"required":{"post":{"requestBody":{"required":true,"content":{"application/json":{"schema":{"type":["string","null"]}},"text/plain":{"schema":{"type":"string"}},"application/x-www-form-urlencoded":{"schema":{"type":"object"}},"application/octet-stream":{}}},"responses":{"204":{"description":"Accepted"}}}}}}`
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
	if diagnostic.HasErrors(findings) {
		t.Fatal(findings)
	}
	var consumer strings.Builder
	consumer.WriteString(`package consumer_test
import("context";"net/http/httptest";"strings";"testing";"example.test/consumer/api/server/webhooks")
`)
	for _, entry := range entries {
		required := entry.BodyRequired
		fmt.Fprintf(&consumer, `func Test%s(t *testing.T){present:=false;calls:=0;handler,err:=webhooks.New%sHandler("/",webhooks.Options{},func(_ context.Context,input webhooks.%sInput)(webhooks.%sResponse,error){calls++;`, entry.PublicName, entry.PublicName, entry.PublicName, entry.PublicName)
		if required {
			consumer.WriteString(`present=true;`)
		} else {
			consumer.WriteString(`present=input.Body.IsSet();`)
		}
		fmt.Fprintf(&consumer, `return webhooks.New%s%sResponse(),nil});if err!=nil{t.Fatal(err)}
for _,test:=range []struct{media,body string;missing bool}{
{"application/json","",true},{"application/json"," \t\n",true},{"application/json","null",false},{"text/plain","",true},{"text/plain","   ",false},{"application/x-www-form-urlencoded"," \t",true},{"application/x-www-form-urlencoded","a=b",false},{"application/octet-stream","",true},{"application/octet-stream","x",false},
}{calls=0;present=false;request:=httptest.NewRequest("POST","/",strings.NewReader(test.body));request.Header.Set("Content-Type",test.media);request.ContentLength=0;recorder:=httptest.NewRecorder();handler.ServeHTTP(recorder,request);expected:=204;if %t&&test.missing{expected=400};if recorder.Code!=expected{t.Fatal(test.media,test.body,recorder.Code,recorder.Body.String())};if expected==204{wantPresent:=!test.missing||test.media=="application/octet-stream";if calls!=1||present!=wantPresent{t.Fatal(test.media,test.body,calls,present,wantPresent)}}}}
`, entry.PublicName, entry.Operation.Responses[0].Name, required)
	}
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer.String()})
}
