package golang

import (
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedTypedInboundMultipartDuplex(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Inbound MIME duplex","version":"1"},"paths":{},"webhooks":{"echo":{"post":{"requestBody":{"required":true,"content":{"multipart/mixed":{"itemSchema":true}}},"responses":{"200":{"description":"Echo","content":{"multipart/mixed":{"itemSchema":true}}}}}}}}`
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
	consumer := strings.NewReplacer("PUBLIC", entry.PublicName, "INPUTITEM", entry.Operation.RequestStreams[0].ItemModel, "OUTPUTITEM", entry.Operation.Responses[0].ItemModel, "VARIANT", entry.Operation.Responses[0].Name).Replace(`package consumer_test
import("context";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"testing";"time";"example.test/consumer/api/models";"example.test/consumer/api/types";"example.test/consumer/api/server/webhooks")
type mediaAdapter struct{}
func(mediaAdapter)Decode(ctx context.Context,source types.Source[types.MultipartPart],_ types.StreamContext)(types.Source[types.MultipartData],error){return types.MapSource(ctx,source,func(_ context.Context,part types.MultipartPart)(types.MultipartData,error){_,parameters,err:=mime.ParseMediaType(part.Headers.Get("Content-Disposition"));if err!=nil{return types.MultipartData{},err};logical,_:=types.ValueFrom(map[string]any{});return types.MultipartData{Value:logical,Slots:map[string]types.Binary{"":types.BinaryFromBytesWithMetadata(part.Data,types.BinaryMetadata{Filename:parameters["filename"],ContentType:part.Headers.Get("Content-Type")})}},nil}),nil}
func(mediaAdapter)Encode(ctx context.Context,source types.Source[types.MultipartData],_ types.StreamContext)(types.Source[types.MultipartPart],error){return types.MapSource(ctx,source,func(ctx context.Context,item types.MultipartData)(types.MultipartPart,error){binary:=item.Slots[""];reader,err:=binary.Open(ctx);if err!=nil{return types.MultipartPart{},err};defer reader.Close();data,err:=io.ReadAll(reader);return types.MultipartPart{Data:data,Headers:http.Header{"Content-Type":{binary.Metadata().ContentType},"Content-Disposition":{"attachment; filename=\""+binary.Metadata().Filename+"\""}}},err}),nil}
func TestMIMEDuplex(t *testing.T){for _,http2:=range []bool{false,true}{for _,custom:=range []bool{false,true}{name:=map[bool]string{false:"http1",true:"http2"}[http2]+map[bool]string{false:"-native",true:"-media-adapter"}[custom];t.Run(name,func(t *testing.T){codecs:=map[string]types.StreamCodec{};if custom{codecs["multipart/mixed"]=types.NewStreamCodec[types.MultipartPart,types.MultipartData](nil,mediaAdapter{})};handler,err:=webhooks.NewPUBLICHandler("/",webhooks.Options{StreamCodecs:codecs,OnError:func(_ context.Context,err error){t.Error(err)}},func(ctx context.Context,input webhooks.PUBLICInput)(webhooks.PUBLICResponse,error){source:=types.MapSource(ctx,input.Body,func(ctx context.Context,item models.INPUTITEM)(models.OUTPUTITEM,error){raw,err:=models.EncodeMediaINPUTITEM(ctx,item);if err!=nil{var zero models.OUTPUTITEM;return zero,err};return models.DecodeMediaOUTPUTITEM(ctx,raw)});return webhooks.NewPUBLICVARIANTResponse(source),nil});if err!=nil{t.Fatal(err)};server:=httptest.NewUnstartedServer(handler);server.EnableHTTP2=http2;if http2{server.StartTLS()}else{server.Start()};defer server.Close();ctx,cancel:=context.WithTimeout(context.Background(),3*time.Second);defer cancel();reader,writer:=io.Pipe();defer reader.Close();defer writer.Close();stop:=context.AfterFunc(ctx,func(){reader.CloseWithError(ctx.Err())});defer stop();request,_:=http.NewRequestWithContext(ctx,"POST",server.URL,reader);request.Header.Set("Content-Type","multipart/mixed; boundary=b");response,err:=server.Client().Do(request);if err!=nil{t.Fatal("MIME headers waited for input",err)};defer response.Body.Close();if _,err:=io.WriteString(writer,"--b\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=\"a.bin\"\r\n\r\nbytes\r\n--b--\r\n");err!=nil{t.Fatal(err)};writer.Close();_,parameters,err:=mime.ParseMediaType(response.Header.Get("Content-Type"));if err!=nil{t.Fatal(err)};parts:=multipart.NewReader(response.Body,parameters["boundary"]);part,err:=parts.NextRawPart();if err!=nil{t.Fatal(response.StatusCode,err)};data,err:=io.ReadAll(part);if err!=nil||string(data)!="bytes"||part.FileName()!="a.bin"{t.Fatal(string(data),part.Header,err)};part.Close();if _,err:=parts.NextRawPart();err!=io.EOF{t.Fatal(err)}})}}}
`)
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer})
}
