package golang

import (
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedTypedInboundMediaAndHeaders(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Inbound media","version":"1"},"paths":{},"webhooks":{"media":{"post":{"requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","required":["message"],"additionalProperties":false,"properties":{"message":{"type":"string"}}}},"application/xml":{"schema":{"type":"object","xml":{"name":"payload"},"required":["message"],"additionalProperties":false,"properties":{"message":{"type":"string"}}}},"application/x-www-form-urlencoded":{"schema":{"type":"object","required":["message"],"additionalProperties":false,"properties":{"message":{"type":"string"}}}},"text/plain":{"schema":{"type":"string"}},"application/octet-stream":{}}},"responses":{"202":{"description":"Accepted","headers":{"X-Rate":{"required":true,"schema":{"type":"integer"}},"X-JSON":{"required":true,"content":{"application/json":{"schema":{"type":"object","properties":{"n":{"type":"integer"}},"required":["n"]}}}},"X-XML":{"required":true,"content":{"application/xml":{"schema":{"type":"object","xml":{"name":"meta"},"properties":{"n":{"type":"integer"}},"required":["n"]}}}}},"content":{"application/xml":{"schema":{"type":"object","xml":{"name":"result"},"properties":{"ok":{"type":"boolean"}},"required":["ok"]}}}}}}}}}`
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
	consumer := strings.NewReplacer("PUBLIC", entry.PublicName, "OUTPUTMODEL", entry.Operation.Responses[0].Model, "VARIANT", entry.Operation.Responses[0].Name, "HEADERMODEL", entry.Operation.Headers[0].Model).Replace(`package consumer_test
import("context";"io";"net/http";"net/http/httptest";"strings";"testing";"example.test/consumer/api/models";"example.test/consumer/api/types";"example.test/consumer/api/server/webhooks")
func TestNativeMedia(t *testing.T){calls:=0;handler,err:=webhooks.NewPUBLICHandler("/",webhooks.Options{},func(ctx context.Context,input webhooks.PUBLICInput)(webhooks.PUBLICResponse,error){calls++;if input.Body.ContentType()==""{t.Fatal("body media is absent")};logical,_:=types.ParseValue([]byte("{\"ok\":true}"));output,err:=models.DecodeOUTPUTMODEL(ctx,logical);if err!=nil{t.Fatal(err)};rawHeaders,_:=types.ParseValue([]byte("{\"X-Rate\":2,\"X-JSON\":{\"n\":3},\"X-XML\":{\"n\":4}}"));headers,err:=models.DecodeHEADERMODEL(ctx,rawHeaders);if err!=nil{t.Fatal(err)};return webhooks.NewPUBLICVARIANTResponse(output,headers),nil});if err!=nil{t.Fatal(err)};server:=httptest.NewServer(handler);defer server.Close();for _,test:=range []struct{media,body string;status int}{ {"application/json","{\"message\":\"hello\"}",202},{"application/xml","<payload><message>hello</message></payload>",202},{"application/x-www-form-urlencoded","message=hello",202},{"application/x-www-form-urlencoded","message=hello&extra=no",400},{"text/plain; charset=utf-8","hello",202},{"application/octet-stream","binary",202},{"application/xml","<payload><wrong>no</wrong></payload>",400},{"application/unknown","hello",415} }{request,_:=http.NewRequest("POST",server.URL,strings.NewReader(test.body));request.Header.Set("Content-Type",test.media);response,err:=server.Client().Do(request);if err!=nil{t.Fatal(test.media,err)};data,_:=io.ReadAll(response.Body);response.Body.Close();if response.StatusCode!=test.status{t.Fatal(test.media,response.StatusCode,string(data))};if test.status==202&&(response.Header.Get("X-Rate")!="2"||response.Header.Get("X-JSON")!="{\"n\":3}"||!strings.Contains(response.Header.Get("X-XML"),"<n>4</n>")||!strings.Contains(string(data),"<ok>true</ok>")){t.Fatal(test.media,response.Header,string(data))}};if calls!=5{t.Fatal(calls)}}
`)
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer})
}
