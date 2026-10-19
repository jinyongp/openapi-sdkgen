package golang

import (
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedTypedInboundMultipart(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Inbound files","version":"1"},"paths":{},"webhooks":{"upload":{"post":{"requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["count","tags","file","meta","bundle"],"properties":{"count":{"type":"integer"},"tags":{"type":"array","items":{"type":"string"}},"file":{"type":"string","format":"binary"},"meta":{"type":"object","xml":{"name":"record"},"required":["id"],"properties":{"id":{"type":"integer"}}},"bundle":{"type":"array","items":{"type":"object","xml":{"name":"entry"},"required":["id"],"properties":{"id":{"type":"integer"}}}}}},"encoding":{"meta":{"contentType":"application/xml"},"bundle":{"contentType":"multipart/mixed; boundary=n","itemEncoding":{"contentType":"application/xml"}},"file":{"contentType":"application/octet-stream","headers":{"X-Part":{"required":true,"schema":{"type":"integer","minimum":1}}}}}}}},"responses":{"204":{"description":"Accepted"}}}}}}`
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
	consumer := strings.NewReplacer("PUBLIC", entry.PublicName, "BODYMODEL", entry.Operation.InputFields[0].Type, "VARIANT", entry.Operation.Responses[0].Name).Replace(`package consumer_test
import("bytes";"context";"errors";"io";"mime/multipart";"net/http/httptest";"net/textproto";"testing";"example.test/consumer/api/models";"example.test/consumer/api/server/webhooks";"example.test/consumer/api/server/runtime")
func TestFilesAndLimits(t *testing.T){calls:=0;callback:=func(ctx context.Context,input webhooks.PUBLICInput)(webhooks.PUBLICResponse,error){calls++;raw,err:=models.EncodeMediaBODYMODEL(ctx,input.Body);if err!=nil{t.Fatal(err)};meta,_:=raw.Value.Get("meta");id,_:=meta.Get("id");if n,_:=id.Number();n!=9{t.Fatal("XML part",meta)};bundle,_:=raw.Value.Get("bundle");first,_:=bundle.At(0);id,_=first.Get("id");if n,_:=id.Number();bundle.Len()!=1||n!=4{t.Fatal("nested XML part",bundle)};count,_:=raw.Value.Get("count");if n,_:=count.Number();n!=5{t.Fatal(count)};tags,_:=raw.Value.Get("tags");encoded,_:=tags.MarshalJSON();if string(encoded)!="[\"a\",\"b\"]"{t.Fatal(string(encoded))};binary,present:=raw.Slots["/file"];data,buffered:=binary.Bytes();if !present||!buffered||string(data)!="bytes"||binary.Metadata().Filename!="example.bin"{t.Fatal(present,buffered,string(data),binary.Metadata())};return webhooks.NewPUBLICVARIANTResponse(),nil};for _,test:=range []struct{name,header string;maximum int64;status int}{ {"valid","1",0,204},{"invalid-header","0",0,400},{"missing-header","",0,400},{"byte-limit","1",32,413} }{t.Run(test.name,func(t *testing.T){var body bytes.Buffer;writer:=multipart.NewWriter(&body);writer.WriteField("count","5");writer.WriteField("tags","a");writer.WriteField("tags","b");headers:=textproto.MIMEHeader{"Content-Disposition":{"form-data; name=\"file\"; filename=\"example.bin\""},"Content-Type":{"application/octet-stream"}};if test.header!=""{headers.Set("X-Part",test.header)};part,err:=writer.CreatePart(headers);if err!=nil{t.Fatal(err)};io.WriteString(part,"bytes");meta,err:=writer.CreatePart(textproto.MIMEHeader{"Content-Disposition":{"form-data; name=\"meta\""},"Content-Type":{"application/xml"}});if err!=nil{t.Fatal(err)};io.WriteString(meta,"<record><id>9</id></record>");bundle,err:=writer.CreatePart(textproto.MIMEHeader{"Content-Disposition":{"form-data; name=\"bundle\""},"Content-Type":{"multipart/mixed; boundary=n"}});if err!=nil{t.Fatal(err)};io.WriteString(bundle,"--n\r\nContent-Type: application/xml\r\n\r\n<entry><id>4</id></entry>\r\n--n--\r\n");writer.Close();handler,err:=webhooks.NewPUBLICHandler("/",webhooks.Options{MaxBodyBytes:test.maximum,OnError:func(_ context.Context,err error){if test.status==204{var failure *runtime.Error;if errors.As(err,&failure){t.Errorf("unexpected failure: %s: %v",failure.Code,failure.Cause)}else{t.Error(err)}}}},callback);if err!=nil{t.Fatal(err)};request:=httptest.NewRequest("POST","/",bytes.NewReader(body.Bytes()));request.Header.Set("Content-Type",writer.FormDataContentType());recorder:=httptest.NewRecorder();handler.ServeHTTP(recorder,request);if recorder.Code!=test.status{t.Fatal(recorder.Code,recorder.Body.String())}})};if calls!=1{t.Fatal(calls)}}
`)
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer})
}
