package golang

import (
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedTypedInboundOpenMedia(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Inbound open media","version":"1"},"paths":{},"webhooks":{"echo":{"post":{"requestBody":{"required":true,"content":{"*/*":{"schema":{"anyOf":[{"type":"object","xml":{"name":"payload"},"required":["message"],"properties":{"message":{"type":"string"},"file":{"type":"string","format":"binary"}}},{"type":"string"}]}}}},"responses":{"200":{"description":"Echo","content":{"*/*":{"schema":{"anyOf":[{"type":"object","xml":{"name":"payload"},"required":["message"],"properties":{"message":{"type":"string"},"file":{"type":"string","format":"binary"}}},{"type":"string"}]}}}}}}}}}`
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
	consumer := strings.NewReplacer("PUBLIC", entry.PublicName, "INPUTMODEL", entry.Operation.RequestBodies[0].Model, "OUTPUTMODEL", entry.Operation.Responses[0].Model, "BODYVARIANT", entry.Operation.RequestBodies[0].Name, "VARIANT", entry.Operation.Responses[0].Name).Replace(`package consumer_test
import("context";"io";"mime";"mime/multipart";"net/http/httptest";"strings";"testing";"example.test/consumer/api/models";"example.test/consumer/api/types";"example.test/consumer/api/server/webhooks")
func TestOpenMedia(t *testing.T){calls:=0;handler,err:=webhooks.NewPUBLICHandler("/",webhooks.Options{},func(ctx context.Context,input webhooks.PUBLICInput)(webhooks.PUBLICResponse,error){calls++;body,present:=input.Body.AsBODYVARIANT();if !present{t.Fatal("open body variant missing")};contentType:=body.ContentType();if native,present:=body.AsMultipart();present{file,found:=native.Slots["/file"];if !found||file.Metadata().Filename!="hello.txt"{t.Fatal(native,file.Metadata())};return webhooks.NewPUBLICVARIANTResponse(types.MediaValueFromMultipart(contentType,native,models.DecodeMediaOUTPUTMODEL)),nil};if data,binary:=body.AsBinary();binary{return webhooks.NewPUBLICVARIANTResponse(types.MediaValueFromBytes[models.OUTPUTMODEL](contentType,data)),nil};typed,present:=body.AsValue();if !present{t.Fatal("validated value missing")};raw,err:=models.EncodeINPUTMODEL(ctx,typed);if err!=nil{t.Fatal(err)};return webhooks.NewPUBLICVARIANTResponse(types.MediaValueFromValue(contentType,raw,models.DecodeOUTPUTMODEL)),nil});if err!=nil{t.Fatal(err)};for _,test:=range []struct{media,body string;status int}{ {"application/json","{\"message\":\"hello\"}",200},{"application/xml","<payload><message>hello</message></payload>",200},{"text/plain; charset=utf-8","hello",200},{"image/png","binary",200},{"multipart/form-data; boundary=input","--input\r\nContent-Disposition: form-data; name=\"message\"\r\n\r\nhello\r\n--input\r\nContent-Disposition: form-data; name=\"file\"; filename=\"hello.txt\"\r\nContent-Type: application/octet-stream\r\n\r\nfilebytes\r\n--input--\r\n",200},{"application/json","{\"message\":7}",400},{"application/unknown","custom",415},{"*/*","invalid",415} }{request:=httptest.NewRequest("POST","/",strings.NewReader(test.body));request.Header.Set("Content-Type",test.media);recorder:=httptest.NewRecorder();handler.ServeHTTP(recorder,request);response:=recorder.Result();data,_:=io.ReadAll(response.Body);response.Body.Close();if response.StatusCode!=test.status{t.Fatal(test.media,response.StatusCode,string(data))};if test.status==200&&strings.HasPrefix(test.media,"multipart/"){_,params,err:=mime.ParseMediaType(response.Header.Get("Content-Type"));if err!=nil{t.Fatal(err)};reader:=multipart.NewReader(strings.NewReader(string(data)),params["boundary"]);parts:=0;for{part,err:=reader.NextPart();if err==io.EOF{break};if err!=nil{t.Fatal(err)};payload,_:=io.ReadAll(part);if part.FormName()=="file"&&(part.FileName()!="hello.txt"||string(payload)!="filebytes"){t.Fatal(part.Header,string(payload))};parts++};if parts!=2{t.Fatal(parts)};continue};if test.status==200&&(response.Header.Get("Content-Type")!=test.media||string(data)!=test.body){t.Fatal(test.media,response.Header,string(data))}};if calls!=5{t.Fatal(calls)}}
`)
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer})
}
