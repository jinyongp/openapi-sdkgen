package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedCustomMediaDirections(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Custom","version":"1"},"paths":{"/widget":{"post":{"operationId":"createWidget","requestBody":{"required":true,"content":{"application/vnd.acme.widget":{"schema":{"type":"string","minLength":1}}}},"responses":{"200":{"description":"OK","content":{"application/vnd.acme.widget":{"schema":{"type":"string","minLength":1}}}}}}},"/records/{record}":{"get":{"operationId":"customParameters","parameters":[{"name":"record","in":"path","required":true,"content":{"application/cbor":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}},{"name":"filter","in":"query","content":{"application/cbor":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}},{"name":"whole","in":"querystring","content":{"application/cbor":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}},{"name":"X-Filter","in":"header","content":{"application/cbor":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}},{"name":"crumb","in":"cookie","content":{"application/cbor":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}}}],"responses":{"204":{"description":"No content"}}}}}}`
	document, err := compiler.Compile([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": customMediaConsumer})
}

const customMediaConsumer = `package consumer_test
import("context";"errors";"io";"net/http";"net/http/httptest";"strings";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types";"example.test/consumer/api/models")
type customSource struct{value string;closed *atomic.Int32}
func(source customSource)Metadata()types.BinaryMetadata{return types.BinaryMetadata{Size:types.Some(int64(len(source.value)))}}
func(source customSource)Open(ctx context.Context)(io.ReadCloser,error){if err:=ctx.Err();err!=nil{return nil,err};return &customReader{Reader:strings.NewReader(source.value),closed:source.closed},nil}
type customReader struct{io.Reader;closed *atomic.Int32}
func(reader *customReader)Close()error{reader.closed.Add(1);return nil}
func TestCustomMedia(t *testing.T){
 var requests,closed atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1)
  if r.URL.Path=="/widget"{body,err:=io.ReadAll(r.Body);if err!=nil||string(body)!="wire:input"||r.Header.Get("Content-Type")!="application/vnd.acme.widget"{t.Error(string(body),err,r.Header)};w.Header().Set("Content-Type","application/vnd.acme.widget; version=1");w.Write([]byte("output"));return}
  if r.URL.EscapedPath()!="/records/cbor%3Apath"||r.URL.RawQuery!="filter=cbor%3Aquery&cbor%3Awhole"||r.Header.Get("X-Filter")!="cbor:header"||r.Header.Get("Cookie")!="crumb=cbor%3Acookie"{t.Error("custom parameters",r.URL,r.Header)};w.WriteHeader(204)
 }));defer server.Close()
 codec:=types.MediaCodec{
  Encode:func(ctx context.Context,value types.Value,metadata types.MediaContext)(types.BinarySource,error){if ctx.Err()!=nil||metadata.Request==nil||metadata.Request.Method!="POST"||metadata.Request.URL.Path!="/widget"||metadata.Request.Body!=http.NoBody||metadata.ContentType!="application/vnd.acme.widget"{t.Error("encode HTTP metadata",metadata)};text,ok:=value.String();if !ok{t.Fatal("unvalidated encode value")};return customSource{value:"wire:"+text,closed:&closed},nil},
  Decode:func(ctx context.Context,response *http.Response,metadata types.MediaContext)(types.Value,error){if metadata.Response==nil||metadata.Response.Body!=response.Body||metadata.Response.StatusCode!=200||metadata.Request==nil||metadata.ContentType!="application/vnd.acme.widget; version=1"{t.Error("decode HTTP metadata",metadata)};data,err:=io.ReadAll(metadata.Response.Body);if err!=nil{return types.Value{},err};if err:=ctx.Err();err!=nil{return types.Value{},err};return types.ValueFrom("decoded:"+string(data))},
 }
 parameterCodec:=types.MediaCodec{EncodeParameter:func(ctx context.Context,value types.Value,metadata types.MediaContext)(string,error){if err:=ctx.Err();err!=nil{return "",err};if metadata.ContentType!="application/cbor"||metadata.Request==nil||metadata.Request.Method!="GET"||metadata.Request.URL.Host==""||metadata.Request.Body!=http.NoBody||metadata.Response!=nil{t.Error("parameter metadata",metadata)};metadata.Request.Header.Set("X-Snapshot-Only","yes");metadata.Request.URL.Host="invalid.test";id,_:=value.Get("id");text,_:=id.String();return "cbor:"+text,nil}}
 codecs:=map[string]types.MediaCodec{"APPLICATION/VND.ACME.WIDGET":codec,"application/cbor":parameterCodec}
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Codecs:codecs});if err!=nil{t.Fatal(err)};delete(codecs,"APPLICATION/VND.ACME.WIDGET")
 result,err:=client.Operations.CreateWidget(sdk.CreateWidgetInput{Body:"input"}).Do(context.Background());if err!=nil||result!="decoded:output"{t.Fatal("codec result",result,err)};if closed.Load()!=1{t.Fatal("encode source lifetime",closed.Load())}
 parse:=func(text string)types.Value{value,err:=types.ParseValue([]byte(text));if err!=nil{t.Fatal(err)};return value}
 path,err:=models.DecodeCustomParametersPath(context.Background(),parse("{\"record\":{\"id\":\"path\"}}"));if err!=nil{t.Fatal(err)}
 query,err:=models.DecodeCustomParametersQuery(context.Background(),parse("{\"filter\":{\"id\":\"query\"}}"));if err!=nil{t.Fatal(err)}
 whole,err:=models.DecodeCustomParametersQuerystring(context.Background(),parse("{\"whole\":{\"id\":\"whole\"}}"));if err!=nil{t.Fatal(err)}
 header,err:=models.DecodeCustomParametersHeader(context.Background(),parse("{\"X-Filter\":{\"id\":\"header\"}}"));if err!=nil{t.Fatal(err)}
 cookie,err:=models.DecodeCustomParametersCookie(context.Background(),parse("{\"crumb\":{\"id\":\"cookie\"}}"));if err!=nil{t.Fatal(err)}
 input:=sdk.CustomParametersInput{Path:path,Query:types.Some(query),Querystring:types.Some(whole),Header:types.Some(header),Cookie:types.Some(cookie)}
 if _,err:=client.Operations.CustomParameters(input).Do(context.Background());err!=nil{t.Fatal(err)}
 missing,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};before:=requests.Load()
 _,err=missing.Operations.CustomParameters(input).Do(context.Background());var apiError *sdk.APIError;if !errors.As(err,&apiError)||apiError.Code!=types.RequestEncodeFailed||before!=requests.Load(){t.Fatal("missing parameter codec",err)}
 sentinel:=errors.New("codec failure")
 codec.Decode=func(context.Context,*http.Response,types.MediaContext)(types.Value,error){return types.Value{},sentinel}
 failing,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Codecs:map[string]types.MediaCodec{"application/vnd.acme.widget":codec}});if err!=nil{t.Fatal(err)}
 _,err=failing.Operations.CreateWidget(sdk.CreateWidgetInput{Body:"input"}).Do(context.Background());if !errors.Is(err,sentinel)||!errors.As(err,&apiError)||apiError.Code!=types.ResponseDecodeFailed{t.Fatal("decode failure",err)}
 if _,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Codecs:map[string]types.MediaCodec{"application/json":{},"APPLICATION/JSON":{}}});err==nil{t.Fatal("ambiguous codec map accepted")}
}
`
