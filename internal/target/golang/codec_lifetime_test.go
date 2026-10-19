package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedMediaCodecFailuresAndHeaderDirection(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Codec lifetime","version":"1"},"paths":{
"/value":{"post":{"operationId":"sendValue","requestBody":{"required":true,"content":{"application/x-value":{"schema":{"type":"string","minLength":1}}}},"responses":{"200":{"description":"Done","content":{"application/x-value":{"schema":{"type":"string","minLength":1}}}}}}},
"/headers":{"get":{"operationId":"readHeaders","responses":{"204":{"description":"Done","headers":{"X-Custom":{"required":true,"content":{"application/x-header; version=1":{"schema":{"type":"integer","minimum":2}}}}}}}}}
}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"codec_lifetime_test.go": codecLifetimeConsumer})
}

const codecLifetimeConsumer = `package consumer_test
import("compress/gzip";"context";"errors";"io";"net/http";"net/http/httptest";"strings";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
type codecTransport func(*http.Request)(*http.Response,error)
func(transport codecTransport)Do(request *http.Request,_ types.TransportOptions)(*http.Response,error){return transport(request)}
func(codecTransport)Capabilities()types.TransportCapabilities{return types.TransportCapabilities{}}
type codecProbe struct{*strings.Reader;closed atomic.Int32;failure error}
func(body *codecProbe)Read(buffer []byte)(int,error){if body.failure!=nil{return 0,body.failure};return body.Reader.Read(buffer)}
func(body *codecProbe)Close()error{body.closed.Add(1);return nil}
type codecProbeSource struct{body *codecProbe;openError error;opens atomic.Int32}
func(*codecProbeSource)Metadata()types.BinaryMetadata{return types.BinaryMetadata{}}
func(source *codecProbeSource)Open(context.Context)(io.ReadCloser,error){source.opens.Add(1);return source.body,source.openError}
func TestCodecEncodeNilAndSourceFailures(t *testing.T){
 sentinel:=errors.New("source failed")
 for _,name:=range []string{"nil","typed-nil","open","read"}{t.Run(name,func(t *testing.T){
 var calls atomic.Int32;body:=&codecProbe{Reader:strings.NewReader("value")};source:=&codecProbeSource{body:body}
 codec:=types.MediaCodec{Encode:func(context.Context,types.Value,types.MediaContext)(types.BinarySource,error){switch name{case "nil":return nil,nil;case "typed-nil":var missing *codecProbeSource;return missing,nil;case "open":source.openError=sentinel;case "read":body.failure=sentinel};return source,nil}}
 transport:=codecTransport(func(request *http.Request)(*http.Response,error){calls.Add(1);_,err:=io.ReadAll(request.Body);return nil,err})
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"https://example.test",Transport:transport,Codecs:map[string]types.MediaCodec{"application/x-value":codec}});if err!=nil{t.Fatal(err)}
 _,err=client.Operations.SendValue(sdk.SendValueInput{Body:"value"}).Do(context.Background());var apiError *types.APIError;if !errors.As(err,&apiError)||apiError.Code!=types.RequestEncodeFailed{t.Fatal("encode failure phase",err)}
 if name=="nil"||name=="typed-nil"{if calls.Load()!=0||source.opens.Load()!=0||body.closed.Load()!=0{t.Fatal("nil source touched transport/resource")}}else{if !errors.Is(err,sentinel)||source.opens.Load()!=1||body.closed.Load()!=1{t.Fatal("source cause/lifetime",err,source.opens.Load(),body.closed.Load())};expected:=int32(0);if name=="read"{expected=1};if calls.Load()!=expected{t.Fatal("source preflight",calls.Load())}}
 })}
}
func TestCodecResponseOwnershipOnDecodeFailure(t *testing.T){
 sentinel:=errors.New("decode failed")
 for _,name:=range []string{"success","error","schema","missing","cancel"}{t.Run(name,func(t *testing.T){
 responseBody:=&codecProbe{Reader:strings.NewReader("response")};sourceBody:=&codecProbe{Reader:strings.NewReader("input")};source:=&codecProbeSource{body:sourceBody};ctx,cancel:=context.WithCancel(context.Background());defer cancel()
 codec:=types.MediaCodec{Encode:func(context.Context,types.Value,types.MediaContext)(types.BinarySource,error){return source,nil},Decode:func(_ context.Context,response *http.Response,metadata types.MediaContext)(types.Value,error){if response.Body!=metadata.Response.Body{t.Fatal("borrowed response bodies differ")};var first [1]byte;if _,err:=response.Body.Read(first[:]);err!=nil{return types.Value{},err};if responseBody.closed.Load()!=0{t.Fatal("body closed before callback returned")};switch name{case "error":return types.Value{},sentinel;case "schema":return types.ValueFrom(1);case "cancel":cancel()};return types.ValueFrom("result")}}
 if name=="missing"{codec.Decode=nil}
 transport:=codecTransport(func(request *http.Request)(*http.Response,error){if _,err:=io.Copy(io.Discard,request.Body);err!=nil{return nil,err};return &http.Response{StatusCode:200,Header:http.Header{"Content-Type":{"application/x-value; version=1"}},Body:responseBody,Request:request},nil})
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"https://example.test",Transport:transport,Codecs:map[string]types.MediaCodec{"application/x-value":codec}});if err!=nil{t.Fatal(err)}
 result,err:=client.Operations.SendValue(sdk.SendValueInput{Body:"input"}).Do(ctx)
 if name=="success"{if err!=nil||result!="result"{t.Fatal(result,err)}}else{var apiError *types.APIError;expected:=types.ResponseDecodeFailed;if name=="cancel"{expected=types.RequestAborted};if !errors.As(err,&apiError)||apiError.Code!=expected{t.Fatal("decode failure phase",err)};if name=="error"&&!errors.Is(err,sentinel){t.Fatal("decode cause lost",err)}}
 if responseBody.closed.Load()!=1||sourceBody.closed.Load()!=1{t.Fatal("SDK body ownership",responseBody.closed.Load(),sourceBody.closed.Load())}
 })}
}
func TestDecodeParameterHTTPMetadataAndSchema(t *testing.T){
 sentinel:=errors.New("header decode failed")
 for _,name:=range []string{"success","schema","error","missing"}{t.Run(name,func(t *testing.T){
 responseBody:=&codecProbe{Reader:strings.NewReader("")}
 codec:=types.MediaCodec{DecodeParameter:func(ctx context.Context,text string,metadata types.MediaContext)(types.Value,error){if text!="header:7"||metadata.ContentType!="application/x-header; version=1"||metadata.Request==nil||metadata.Request.Method!="GET"||metadata.Request.Body!=http.NoBody||metadata.Response==nil||metadata.Response.Body!=http.NoBody||metadata.Response.StatusCode!=204||ctx.Err()!=nil{t.Fatal("header codec HTTP metadata",text,metadata)};metadata.Response.Header.Set("X-Custom","snapshot");if name=="error"{return types.Value{},sentinel};value:=7;if name=="schema"{value=1};return types.ValueFrom(value)}}
 if name=="missing"{codec.DecodeParameter=nil}
 transport:=codecTransport(func(request *http.Request)(*http.Response,error){return &http.Response{StatusCode:204,Header:http.Header{"X-Custom":{"header:7"}},Body:responseBody,Request:request},nil})
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"https://example.test",Transport:transport,Codecs:map[string]types.MediaCodec{"application/x-header":codec}});if err!=nil{t.Fatal(err)}
 response,err:=client.Operations.ReadHeaders().Raw(context.Background());if name=="success"{if err!=nil{t.Fatal(err)};headers,present:=response.As204Headers();if !present||headers.XCustom!=7||response.Headers.Get("X-Custom")!="header:7"{t.Fatal("header result/snapshot",headers,response.Headers)}}else{var apiError *types.APIError;if !errors.As(err,&apiError)||apiError.Code!=types.ResponseDecodeFailed{t.Fatal("header phase",err)};if name=="error"&&!errors.Is(err,sentinel){t.Fatal("header cause",err)}}
 if responseBody.closed.Load()!=1{t.Fatal("response header body ownership",responseBody.closed.Load())}
 })}
}
func TestCodecSeesFinalTrailersWithoutSharingMetadata(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){io.Copy(io.Discard,request.Body);writer.Header().Set("Content-Type","application/x-value");writer.Header().Set("Trailer","X-Checksum");if request.Header.Get("X-Compression")=="gzip"{writer.Header().Set("Content-Encoding","gzip");compressor:=gzip.NewWriter(writer);compressor.Write([]byte("response"));compressor.Close()}else{io.WriteString(writer,"response")};writer.Header().Set("X-Checksum","done")}));defer server.Close()
 codec:=types.MediaCodec{Encode:func(context.Context,types.Value,types.MediaContext)(types.BinarySource,error){return types.BinaryFromBytes([]byte("input")),nil},Decode:func(_ context.Context,response *http.Response,metadata types.MediaContext)(types.Value,error){
 savedTrailers:=metadata.Response.Trailer;body,err:=io.ReadAll(metadata.Response.Body);if err!=nil{return types.Value{},err};if string(body)!="response"||response.Trailer.Get("X-Checksum")!="done"||metadata.Response.Trailer.Get("X-Checksum")!="done"||savedTrailers.Get("X-Checksum")!="done"{t.Error("final hook trailers",string(body),response.Trailer,metadata.Response.Trailer,savedTrailers)}
 metadata.Response.Trailer.Set("X-Checksum","changed");if response.Trailer.Get("X-Checksum")!="done"{t.Error("trailer snapshots alias")};if len(metadata.Response.TransferEncoding)!=0{metadata.Response.TransferEncoding[0]="changed";if response.TransferEncoding[0]=="changed"{t.Error("transfer encoding snapshots alias")}}
 return types.ValueFrom("response")
 }}
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Codecs:map[string]types.MediaCodec{"application/x-value":codec}});if err!=nil{t.Fatal(err)}
 for _,compression:=range []string{"plain","gzip"}{response,err:=client.Operations.SendValue(sdk.SendValueInput{Body:"input"}).WithOptions(types.RequestOptions{Headers:http.Header{"X-Compression":{compression}}}).Raw(context.Background());if err!=nil{t.Fatal(compression,err)};if response.OriginalResponse.Trailer.Get("X-Checksum")!="done"||len(response.OriginalResponse.TransferEncoding)==0||response.OriginalResponse.TransferEncoding[0]!="chunked"{t.Fatal("final response snapshot changed",compression,response.OriginalResponse.Trailer,response.OriginalResponse.TransferEncoding)}}
}
`
