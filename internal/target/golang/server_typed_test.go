package golang

import (
	"fmt"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedTypedInboundHandlers(t *testing.T) {
	document, err := compiler.Compile([]byte(inboundScopeFixture))
	if err != nil {
		t.Fatal(err)
	}
	addons, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	options, err := addons.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	options.Golang = &generator.GoOptions{ImportPath: "example.test/consumer/api"}
	plan, findings, err := (Generator{}).Prepare(document, options)
	if err != nil || diagnostic.HasErrors(findings) {
		t.Fatal(err, findings)
	}
	names, err := planNames(document)
	if err != nil {
		t.Fatal(err)
	}
	entries, findings := prepareInboundOperations(document, inboundScopes(document, nil, nil), names, new(schemaPlanner))
	if diagnostic.HasErrors(findings) {
		t.Fatal(findings)
	}
	files := make(map[string]string)
	for _, entry := range entries {
		pkg := "webhooks"
		if entry.Definition.Kind == "callback" {
			pkg = "callbacks"
		}
		files["api/server/"+pkg+"/handler_test.go"] = fmt.Sprintf(`package %s_test
import("context";"net/http";"net/http/httptest";"testing";"example.test/consumer/api/server/%s")
func TestTypedHandler(t *testing.T){called:=false;handler,err:=%s.New%sHandler("/host/{id}",%s.Options{},func(_ context.Context,input %s.%sInput)(%s.%sResponse,error){called=true;if input.Context.Request.Body!=http.NoBody{t.Fatal("metadata exposes body")};return %s.New%s%sResponse(),nil});if err!=nil{t.Fatal(err)};recorder:=httptest.NewRecorder();handler.ServeHTTP(recorder,httptest.NewRequest("POST","/host/42",nil));if !called||recorder.Code!=204{t.Fatal(called,recorder.Code,recorder.Body.String())};info:=%s.%sInfo();if info.Identity!=%q||info.Expression!=%q{t.Fatal(info)}}
`, pkg, pkg, pkg, entry.PublicName, pkg, pkg, entry.PublicName, pkg, entry.PublicName, pkg, entry.PublicName, entry.Operation.Responses[0].Name, pkg, entry.PublicName, entry.Definition.Identity, entry.Definition.Expression)
	}
	root := testGeneratedModule(t, plan, files)
	checkInboundImportGraph(t, root, "webhooks", true)
	checkInboundImportGraph(t, root, "callbacks", true)
}

func TestGeneratedTypedInboundJSON(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Inbound JSON","version":"1"},"paths":{},"webhooks":{"received":{"post":{"operationId":"receive","parameters":[{"in":"path","name":"id","required":true,"schema":{"type":"integer"}},{"in":"query","name":"count","required":true,"schema":{"type":"integer"}}],"requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","required":["message"],"properties":{"message":{"type":"string"}}}}}},"responses":{"202":{"description":"Accepted","content":{"application/json":{"schema":{"type":"object","required":["ok"],"properties":{"ok":{"type":"boolean"}}}}}}}}}},"security":[{"Key":[]}],"components":{"securitySchemes":{"Key":{"type":"apiKey","in":"header","name":"X-Key"}}}}`
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
	inputBody := ""
	for _, field := range entry.Operation.InputFields {
		if field.Name == "Body" {
			inputBody = field.Type
		}
	}
	consumer := strings.NewReplacer("PUBLIC", entry.PublicName, "BODYMODEL", inputBody, "OUTPUTMODEL", entry.Operation.Responses[0].Model, "VARIANT", entry.Operation.Responses[0].Name).Replace(`package consumer_test
import("context";"io";"net/http";"net/http/httptest";"strings";"testing";"example.test/consumer/api/models";"example.test/consumer/api/types";"example.test/consumer/api/server/runtime";"example.test/consumer/api/server/webhooks")
func TestJSONBoundary(t *testing.T){calls,auth:=0,0;handler,err:=webhooks.NewPUBLICHandler("/events/{id}",webhooks.Options{Authenticate:func(ctx context.Context,input runtime.InboundRequestContext)(*runtime.InboundRejection,error){auth++;query,_:=input.Params.Get("query");count,_:=query.Get("count");if n,ok:=count.Number();!ok||n!=2{t.Fatal(count)};if len(input.SecurityCandidates)!=1||input.SecurityCandidates[0].Value!="secret"{t.Fatal(input.SecurityCandidates)};clone,err:=input.CloneBody(ctx);if err!=nil{return nil,err};defer clone.Close();data,err:=io.ReadAll(clone);if err!=nil||string(data)!="{\"message\":\"hello\"}"{t.Fatal(string(data),err)};return nil,nil}},func(ctx context.Context,input webhooks.PUBLICInput)(webhooks.PUBLICResponse,error){calls++;value,err:=models.EncodeBODYMODEL(ctx,input.Body);if err!=nil{t.Fatal(err)};message,_:=value.Get("message");if text,_:=message.String();text!="hello"{t.Fatal(value)};logical,_:=types.ParseValue([]byte("{\"ok\":true}"));output,err:=models.DecodeOUTPUTMODEL(ctx,logical);if err!=nil{t.Fatal(err)};return webhooks.NewPUBLICVARIANTResponse(output),nil});if err!=nil{t.Fatal(err)};server:=httptest.NewServer(handler);defer server.Close();request,_:=http.NewRequest("POST",server.URL+"/events/42?count=2",strings.NewReader("{\"message\":\"hello\"}"));request.Header.Set("Content-Type","application/json");request.Header.Set("X-Key","secret");response,err:=server.Client().Do(request);if err!=nil{t.Fatal(err)};data,_:=io.ReadAll(response.Body);response.Body.Close();if response.StatusCode!=202||string(data)!="{\"ok\":true}"||calls!=1||auth!=1{t.Fatal(response.StatusCode,string(data),calls,auth)};request,_=http.NewRequest("POST",server.URL+"/events/42?count=invalid",strings.NewReader("{\"message\":\"hello\"}"));request.Header.Set("Content-Type","application/json");response,err=server.Client().Do(request);if err!=nil{t.Fatal(err)};response.Body.Close();if response.StatusCode!=400||calls!=1||auth!=1{t.Fatal(response.StatusCode,calls,auth)}}
`)
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer})
}
