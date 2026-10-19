package golang

import (
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedTypedInboundMultipartResponse(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Inbound file response","version":"1"},"paths":{},"webhooks":{"download":{"post":{"responses":{"200":{"description":"File","content":{"multipart/form-data":{"schema":{"type":"object","required":["count","file"],"properties":{"count":{"type":"integer"},"file":{"type":"string","format":"binary"}}},"encoding":{"file":{"contentType":"application/octet-stream","headers":{"X-Part":{"required":true,"schema":{"type":"integer","minimum":1}}}}}}}}}}}}}`
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
	consumer := strings.NewReplacer("PUBLIC", entry.PublicName, "BODYMODEL", entry.Operation.Responses[0].Model, "VARIANT", entry.Operation.Responses[0].Name).Replace(`package consumer_test
import("context";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"strings";"sync/atomic";"testing";"example.test/consumer/api/models";"example.test/consumer/api/types";"example.test/consumer/api/server/runtime";"example.test/consumer/api/server/webhooks")
type fileSource struct{opens,closes atomic.Int32}
func(source *fileSource)Metadata()types.BinaryMetadata{return types.BinaryMetadata{Filename:"example.bin",ContentType:"application/octet-stream",Size:types.Some(int64(4))}}
func(source *fileSource)Open(context.Context)(io.ReadCloser,error){source.opens.Add(1);return &fileReader{Reader:strings.NewReader("data"),source:source},nil}
type fileReader struct{io.Reader;source *fileSource}
func(reader *fileReader)Close()error{reader.source.closes.Add(1);return nil}
func TestTypedFileResponse(t *testing.T){source:=new(fileSource);handler,err:=webhooks.NewPUBLICHandler("/",webhooks.Options{OnError:func(_ context.Context,err error){t.Error(err)}},func(ctx context.Context,input webhooks.PUBLICInput)(webhooks.PUBLICResponse,error){count,_:=types.ValueFrom(5);value,err:=types.MultipartFromObject(map[string]types.MultipartValue{"count":types.MultipartFromValue(count),"file":types.MultipartFromBinary(types.BinaryFromSource(source))});if err!=nil{t.Fatal(err)};raw,err:=value.WireValue();if err!=nil{t.Fatal(err)};body,err:=models.DecodeMediaBODYMODEL(ctx,raw);if err!=nil{t.Fatal(err)};if source.opens.Load()!=0{t.Fatal("file opened during model preparation")};return webhooks.NewPUBLICVARIANTResponse(body).WithMultipartOptions(runtime.MultipartOptions{Headers:map[string]http.Header{"file":{"X-Part":{"1"}}}}),nil});if err!=nil{t.Fatal(err)};server:=httptest.NewServer(handler);defer server.Close();response,err:=server.Client().Post(server.URL,"text/plain",nil);if err!=nil{t.Fatal(err)};defer response.Body.Close();_,parameters,err:=mime.ParseMediaType(response.Header.Get("Content-Type"));if err!=nil||response.StatusCode!=200{data,_:=io.ReadAll(response.Body);t.Fatal(response.StatusCode,string(data),err)};reader:=multipart.NewReader(response.Body,parameters["boundary"]);parts:=map[string]string{};for{part,err:=reader.NextRawPart();if err==io.EOF{break};if err!=nil{t.Fatal(err)};data,err:=io.ReadAll(part);if err!=nil{t.Fatal(err)};parts[part.FormName()]=string(data);if part.FormName()=="file"&&(part.FileName()!="example.bin"||part.Header.Get("X-Part")!="1"){t.Fatal(part.Header)};part.Close()};if parts["file"]!="data"||parts["count"]!="5"||source.opens.Load()!=1||source.closes.Load()!=1{t.Fatal(parts,source.opens.Load(),source.closes.Load())}}
`)
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer})
}
