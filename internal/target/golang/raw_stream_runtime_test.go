package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedUntypedSequentialRaw(t *testing.T) {
	const source = `{"openapi":"3.1.0","info":{"title":"Raw stream","version":"1"},"paths":{"/events":{"get":{"operationId":"events","responses":{"200":{"description":"Events","content":{"text/event-stream":{}}}}}}}}`
	document, err := compiler.Compile([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": rawStreamConsumer})
}

const rawStreamConsumer = `package consumer_test
import("bytes";"compress/gzip";"context";"errors";"io";"net/http";"net/http/httptest";"sync/atomic";"testing";"time";sdk "example.test/consumer/api";"example.test/consumer/api/types")
type rawBody struct{io.Reader;reads,closes atomic.Int32}
func(body *rawBody)Read(buffer []byte)(int,error){body.reads.Add(1);return body.Reader.Read(buffer)}
func(body *rawBody)Close()error{body.closes.Add(1);return nil}
type rawTransport struct{body *rawBody;ctx context.Context}
func(host *rawTransport)Capabilities()types.TransportCapabilities{return types.TransportCapabilities{ResponseBody:types.Encoded}}
func(host *rawTransport)Do(request *http.Request,_ types.TransportOptions)(*http.Response,error){host.ctx=request.Context();return &http.Response{StatusCode:200,Header:http.Header{"Content-Type":[]string{"text/event-stream"},"Content-Encoding":[]string{"gzip"}},Body:host.body,Request:request},nil}
func TestSequentialRawIsUnreadUntilConsumption(t *testing.T){
 var encoded bytes.Buffer;writer:=gzip.NewWriter(&encoded);writer.Write([]byte("data: hello\n\n"));writer.Close()
 for _,consume:=range []bool{false,true}{host:=&rawTransport{body:&rawBody{Reader:bytes.NewReader(encoded.Bytes())}};client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"http://example.test",Transport:host});if err!=nil{t.Fatal(err)}
  raw,err:=client.Operations.Events().Raw(context.Background());if err!=nil{t.Fatal(err)}
  if host.body.reads.Load()!=0||host.ctx.Err()!=nil||raw.OriginalResponse.Body!=http.NoBody||raw.ContentType!="text/event-stream"{t.Fatal("eager read or premature cancel")}
  if consume{data,err:=io.ReadAll(raw.Body);if err!=nil||string(data)!="data: hello\n\n"{t.Fatal(string(data),err)};if host.ctx.Err()!=context.Canceled{t.Fatal("EOF retained request context")}}
  if consume{var buffer [1]byte;if count,err:=raw.Body.Read(buffer[:]);count!=0||err!=io.EOF{t.Fatal("EOF became cleanup cancellation on repeated Read",count,err)}}
  if err:=raw.Close();err!=nil{t.Fatal(err)};if err:=raw.Close();err!=nil{t.Fatal(err)}
  if host.body.closes.Load()!=1||host.ctx.Err()!=context.Canceled{t.Fatal("body/context ownership",host.body.closes.Load(),host.ctx.Err())}
 }
 host:=&rawTransport{body:&rawBody{Reader:bytes.NewReader([]byte("not gzip"))}};client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"http://example.test",Transport:host});if err!=nil{t.Fatal(err)}
 raw,err:=client.Operations.Events().Raw(context.Background());if err!=nil{t.Fatal(err)}
 _,err=io.ReadAll(raw.Body);var apiError *sdk.APIError
 if !errors.As(err,&apiError)||apiError.Code!=types.ResponseDecodeFailed||apiError.Status!=200{t.Fatal("decode failure became own cleanup cancellation",err)}
 if host.body.closes.Load()!=1||host.ctx.Err()!=context.Canceled{t.Fatal("malformed compressed Raw was not finalized")}
}
func TestSequentialRawLiveHTTPAndTimeout(t *testing.T){
 stopped:=make(chan struct{},4)
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){defer func(){stopped<-struct{}{}}();w.Header().Set("Content-Type","text/event-stream");w.WriteHeader(200);w.(http.Flusher).Flush();<-r.Context().Done()}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 ctx,cancel:=context.WithTimeout(context.Background(),time.Second);defer cancel()
 raw,err:=client.Operations.Events().Raw(ctx);if err!=nil{t.Fatal(err)}
 done:=make(chan error,1);go func(){_,err:=io.Copy(io.Discard,raw.Body);done<-err}()
 if err:=raw.Close();err!=nil{t.Fatal(err)};select{case <-done:case <-time.After(time.Second):t.Fatal("Close did not interrupt Read")}
 select{case <-stopped:case <-time.After(time.Second):t.Fatal("Close did not cancel HTTP")}
 raw,err=client.Operations.Events().WithOptions(sdk.RequestOptions{Timeout:types.Some(100*time.Millisecond)}).Raw(context.Background());if err!=nil{t.Fatal(err)}
 _,err=io.ReadAll(raw.Body);var apiError *sdk.APIError;if !errors.Is(err,context.DeadlineExceeded)||!errors.As(err,&apiError)||apiError.Code!=types.RequestTimeout{t.Fatal("lost stream timeout",err)}
 raw.Close()
}
`
