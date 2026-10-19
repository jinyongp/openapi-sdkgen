package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedMultipartNativeXMLAndCustomParts(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Multipart codecs","version":"1"},"paths":{"/parts":{"post":{"operationId":"sendParts","requestBody":{"required":true,"content":{"multipart/mixed":{"schema":{"type":"array","minItems":3,"maxItems":3,"prefixItems":[{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}},{"type":"object","xml":{"name":"record"},"required":["id"],"properties":{"id":{"type":"integer","xml":{"attribute":true}}}},{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}]},"prefixEncoding":[{"contentType":"application/json; charset=utf-8"},{"contentType":"application/xml"},{"contentType":"application/x-custom; version=1"}]}}},"responses":{"204":{"description":"Done"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"multipart_codecs_test.go": multipartCodecsConsumer})
}

const multipartCodecsConsumer = `package consumer_test
import("context";"errors";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"strings";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
type partCodecSource struct{closed *atomic.Int32}
func(partCodecSource)Metadata()types.BinaryMetadata{return types.BinaryMetadata{Size:types.Some(int64(8))}}
func(source partCodecSource)Open(context.Context)(io.ReadCloser,error){return &partCodecReader{Reader:strings.NewReader("custom:3"),closed:source.closed},nil}
type partCodecReader struct{*strings.Reader;closed *atomic.Int32}
func(reader *partCodecReader)Close()error{reader.closed.Add(1);return nil}
type partPreflightTransport struct{calls atomic.Int32}
func(transport *partPreflightTransport)Do(request *http.Request,_ types.TransportOptions)(*http.Response,error){transport.calls.Add(1);return &http.Response{StatusCode:204,Header:make(http.Header),Body:http.NoBody,Request:request},nil}
func(*partPreflightTransport)Capabilities()types.TransportCapabilities{return types.TransportCapabilities{ResponseBody:types.DecodedWithOriginalHeaders}}
type negativePartSource struct{}
func(negativePartSource)Metadata()types.BinaryMetadata{return types.BinaryMetadata{Size:types.Some(int64(-1))}}
func(negativePartSource)Open(context.Context)(io.ReadCloser,error){panic("negative-size source must not be opened")}
func TestPartCodecs(t *testing.T){
 var requests,closed,encoded atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){requests.Add(1);_,parameters,err:=mime.ParseMediaType(request.Header.Get("Content-Type"));if err!=nil{t.Error(err);writer.WriteHeader(500);return};reader:=multipart.NewReader(request.Body,parameters["boundary"])
 for index,want:=range []string{"{\"id\":1}","<record id=\"2\"></record>","custom:3"}{part,err:=reader.NextPart();if err!=nil{t.Error(err);break};data,err:=io.ReadAll(part);if err!=nil||string(data)!=want{t.Error("part codec wire",index,string(data),err)};expected:=[]string{"application/json; charset=utf-8","application/xml","application/x-custom; version=1"};if part.Header.Get("Content-Type")!=expected[index]{t.Error("part metadata",index,part.Header)}};writer.WriteHeader(204)
 }));defer server.Close()
 codec:=types.MediaCodec{Encode:func(ctx context.Context,value types.Value,metadata types.MediaContext)(types.BinarySource,error){encoded.Add(1);id,_:=value.Get("id");number,_:=id.Number();if number!=3||metadata.ContentType!="application/x-custom; version=1"||metadata.Request==nil||metadata.Request.URL.Path!="/parts"||metadata.Request.Body!=http.NoBody||ctx.Err()!=nil{t.Error("part encode metadata",value,metadata)};return partCodecSource{closed:&closed},nil}}
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Codecs:map[string]types.MediaCodec{"application/x-custom":codec,"application/xml":{Encode:func(context.Context,types.Value,types.MediaContext)(types.BinarySource,error){t.Error("native XML delegated");return nil,errors.New("native XML delegated")}}}});if err!=nil{t.Fatal(err)}
 value,_:=types.ParseValue([]byte("[{\"id\":1},{\"id\":2},{\"id\":3}]"));body,err:=models.DecodeSendPartsBody(context.Background(),value);if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendParts(sdk.SendPartsInput{Body:body}).Do(context.Background());err!=nil{t.Fatal(err)}
 if requests.Load()!=1||encoded.Load()!=1||closed.Load()!=1{t.Fatal("part source lifetime",requests.Load(),encoded.Load(),closed.Load())}
 for _,name:=range []string{"nil","typed-nil","negative"}{t.Run(name,func(t *testing.T){
 transport:=&partPreflightTransport{};invalid:=types.MediaCodec{Encode:func(context.Context,types.Value,types.MediaContext)(types.BinarySource,error){switch name{case "nil":return nil,nil;case "typed-nil":var source *partCodecSource;return source,nil;default:return negativePartSource{},nil}}}
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Transport:transport,Codecs:map[string]types.MediaCodec{"application/x-custom":invalid}});if err!=nil{t.Fatal(err)}
 _,err=client.Operations.SendParts(sdk.SendPartsInput{Body:body}).Do(context.Background());var apiError *types.APIError;if !errors.As(err,&apiError)||apiError.Code!=types.RequestEncodeFailed||transport.calls.Load()!=0{t.Fatal("invalid codec source bypassed multipart preflight",err,transport.calls.Load())}
 })}
}
`
