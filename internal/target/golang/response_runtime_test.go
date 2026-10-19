package golang

import (
	"encoding/json"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedResponseSelectionAndErrors(t *testing.T) {
	response := func(media string, schema any) map[string]any {
		return map[string]any{"description": "Response", "content": map[string]any{media: map[string]any{"schema": schema}}}
	}
	responses := map[string]any{
		"200":     response("application/json", map[string]any{"type": "object", "required": []string{"names"}, "properties": map[string]any{"names": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}}),
		"2XX":     response("application/json", map[string]any{"type": "integer", "minimum": 0}),
		"202":     response("application/octet-stream", map[string]any{"type": "string", "format": "binary"}),
		"204":     map[string]any{"description": "No content"},
		"206":     response("text/plain", map[string]any{"type": "integer", "minimum": 0}),
		"default": map[string]any{"description": "Fallback", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "required": []string{"code"}, "properties": map[string]any{"code": map[string]any{"type": "string"}}}}, "text/plain": map[string]any{"schema": map[string]any{"type": "string"}}}},
	}
	input := map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Responses", "version": "1"}, "paths": map[string]any{"/cases": map[string]any{"get": map[string]any{"operationId": "getCases", "responses": responses}}}}
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	document, err := compiler.Compile(encoded)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": responseConsumer})
}

const responseConsumer = `package consumer_test
import("context";"encoding/json";"errors";"io";"net/http";"net/http/httptest";"strings";"sync/atomic";"testing";"time";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestResponseSelection(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  status,media,body:=200,"application/json","{\"names\":[\"original\"]}"
  switch r.Header.Get("X-Case"){
  case "range":status,body=201,"7"
  case "default-media":media,body="text/plain; charset=utf-8","003"
  case "binary":status,media,body=202,"application/octet-stream",string([]byte{0,255,42})
  case "none":status,media,body=204,"",""
  case "text-number":status,media,body=206,"text/plain","3"
  case "text-invalid":status,media,body=206,"text/plain","3.5"
  case "invalid-json":body="{"
  case "invalid-model":body="{\"names\":false}"
  case "error":status,body=400,"{\"code\":\"BAD_INPUT\",\"secret\":\"private\"}"
  case "unknown-error":status,media,body=503,"text/html","private"
  case "missing-media":media=""
  }
  if media!=""{w.Header().Set("Content-Type",media)}else{w.Header()["Content-Type"]=nil}
  w.Header().Set("X-Request-ID","response-id");w.WriteHeader(status);io.WriteString(w,body)
 }));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 call:=client.Operations.GetCases()
 value,err:=call.Do(context.Background());if err!=nil{t.Fatal(err)}
 first,ok:=value.As200JSON();if !ok{t.Fatal("wrong success branch")};first.Names[0]="changed";second,ok:=value.As200JSON();if !ok||second.Names[0]!="original"{t.Fatal("shared output snapshot")}
 options:=func(name string)sdk.RequestOptions{return sdk.RequestOptions{Headers:http.Header{"X-Case":[]string{name}}}}
 value,err=call.WithOptions(options("range")).Do(context.Background());if err!=nil{t.Fatal(err)};number,ok:=value.As2XXJSON();if !ok||number!=7{t.Fatal("range",number,ok)}
 value,err=call.WithOptions(options("default-media")).Do(context.Background());if err!=nil{t.Fatal(err)};text,ok:=value.AsDefaultText();if !ok||text!="003"{t.Fatal("media-specific default or string coercion",text,ok)}
 value,err=call.WithOptions(options("text-number")).Do(context.Background());if err!=nil{t.Fatal(err)};integer,ok:=value.As206Text();if !ok||integer!=3{t.Fatal("text scalar fallback",integer,ok)}
 value,err=call.WithOptions(options("binary")).Do(context.Background());if err!=nil{t.Fatal(err)};binary,ok:=value.As202Binary();if !ok||string(binary)!=string([]byte{0,255,42}){t.Fatal("binary")};binary[0]=1;again,_:=value.As202Binary();if again[0]!=0{t.Fatal("shared binary snapshot")}
 value,err=call.WithOptions(options("none")).Do(context.Background());if err!=nil{t.Fatal(err)};if _,ok:=value.As204NoContent();!ok{t.Fatal("no content")}
 raw,err:=call.Raw(context.Background());if err!=nil{t.Fatal(err)};if raw.Status!=200||raw.OriginalResponse.Body!=http.NoBody||raw.Request.Body!=http.NoBody||raw.Headers.Get("X-Request-ID")!="response-id"{t.Fatal("Raw metadata",raw)}
 rawBytes:=raw.Bytes();rawBytes[0]='!';if raw.Bytes()[0]!='{'{t.Fatal("shared Raw bytes")}
 for _,name:=range []string{"invalid-json","invalid-model","text-invalid","missing-media"}{_,err:=call.WithOptions(options(name)).Do(context.Background());var apiError *sdk.APIError;var httpError *sdk.GetCasesHTTPError;if !errors.As(err,&apiError)||apiError.Code!=types.ResponseDecodeFailed||errors.As(err,&httpError){t.Errorf("%s: wrong decode error %v",name,err)}}
 _,err=call.WithOptions(options("error")).Do(context.Background());var httpError *sdk.GetCasesHTTPError;var apiError *sdk.APIError
 if !errors.As(err,&httpError)||!errors.As(err,&apiError)||apiError.Code!="BAD_INPUT"||apiError.Status!=400||apiError.Category!="http"{t.Fatal("HTTP error",err)}
 body,ok:=httpError.AsDefaultJSON();if !ok||body.Code!="BAD_INPUT"{t.Fatal("typed HTTP body",body,ok)}
 if strings.Contains(err.Error(),"private"){t.Fatal("error leaked body")}
 encodedError,marshalError:=json.Marshal(err);if marshalError!=nil||strings.Contains(string(encodedError),"private")||strings.Contains(string(encodedError),server.URL)||!strings.Contains(string(encodedError),"BAD_INPUT"){t.Fatal("HTTP error JSON leaked metadata or lost its code",string(encodedError),marshalError)}
 _,err=call.WithOptions(options("unknown-error")).Do(context.Background());if !errors.As(err,&httpError)||httpError.Code!="HTTP_503"{t.Fatal("undeclared HTTP error",err)}
}

type ownedBody struct{io.Reader;closed *atomic.Int32;failure error}
func(body *ownedBody)Close()error{body.closed.Add(1);return body.failure}
type testTransport struct{closed atomic.Int32;failure error;slow bool}
func(host *testTransport)Capabilities()types.TransportCapabilities{return types.TransportCapabilities{}}
func(host *testTransport)Do(request *http.Request,_ types.TransportOptions)(*http.Response,error){
 if host.slow{<-request.Context().Done();return nil,request.Context().Err()}
 return &http.Response{StatusCode:200,Header:http.Header{"Content-Type":[]string{"application/json"}},Body:&ownedBody{Reader:strings.NewReader("{\"names\":[]}"),closed:&host.closed,failure:host.failure},Request:request},nil
}
func TestBodyOwnershipAndDeadline(t *testing.T){
 sentinel:=errors.New("close failed")
 for _,raw:=range []bool{false,true}{host:=&testTransport{failure:sentinel};client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"http://example.test",Transport:host});if err!=nil{t.Fatal(err)};if raw{_,err=client.Operations.GetCases().Raw(context.Background())}else{_,err=client.Operations.GetCases().Do(context.Background())};var apiError *sdk.APIError;if !errors.Is(err,sentinel)||!errors.As(err,&apiError)||apiError.Code!=types.ResponseDecodeFailed{t.Fatal("close failure classification",err)};if host.closed.Load()!=1{t.Fatal("close count",host.closed.Load())}}
 host:=&testTransport{slow:true};client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"http://example.test",Transport:host});if err!=nil{t.Fatal(err)}
 _,err=client.Operations.GetCases().WithOptions(sdk.RequestOptions{Timeout:types.Some(time.Millisecond)}).Do(context.Background());var apiError *sdk.APIError
 if !errors.Is(err,context.DeadlineExceeded)||!errors.As(err,&apiError)||apiError.Code!=types.RequestTimeout{t.Fatal("deadline",err)}
}
`
