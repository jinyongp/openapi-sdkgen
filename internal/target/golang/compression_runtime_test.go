package golang

import (
	"os"
	"testing"
)

func TestGeneratedResponseCompressionAndLease(t *testing.T) {
	_, plan := healthPlan(t)
	testGeneratedModule(t, plan, map[string]string{"api/internal/transport/body_test.go": compressionBodyConsumer, "api/internal/transport/encoded.json": compressionFixtures(t)})
}

func compressionFixtures(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../../testdata/goruntime/http-encoded.json")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

const compressionBodyConsumer = `package transport
import("bytes";"context";"encoding/base64";"encoding/json";"errors";"io";"net/http";"os";"strconv";"strings";"sync";"sync/atomic";"testing";"time")
type observedBody struct{io.Reader;reads atomic.Int32;closed atomic.Int32;closeError error}
func(body *observedBody)Read(buffer []byte)(int,error){body.reads.Add(1);return body.Reader.Read(buffer)}
func(body *observedBody)Close()error{body.closed.Add(1);return body.closeError}
func TestResponseCodingsRemainOpaque(t *testing.T){
 var fixtures map[string]string;data,err:=os.ReadFile("encoded.json");if err!=nil{t.Fatal(err)};if err:=json.Unmarshal(data,&fixtures);err!=nil{t.Fatal(err)}
 fixtures["gzip, example"]=fixtures["gzip"];fixtures["x-gzip"]=fixtures["gzip"]
 for coding,value:=range fixtures{t.Run(coding,func(t *testing.T){
  encoded,err:=base64.StdEncoding.DecodeString(value);if err!=nil{t.Fatal(err)};header:=coding;if header=="raw-deflate"{header="deflate"}
  original:=&observedBody{Reader:bytes.NewReader(encoded)}
  response:=&http.Response{StatusCode:200,ContentLength:int64(len(encoded)),Header:http.Header{"Content-Encoding":{header},"Content-Length":{strconv.Itoa(len(encoded))}},Body:original}
  normalized,lease,err:=NewResponseBody(context.Background(),response,"GET");if err!=nil{t.Fatal(err)}
  if original.reads.Load()!=0{t.Fatal("receiving response headers read a body")}
  if normalized.Header.Get("Content-Encoding")!=header||normalized.Header.Get("Content-Length")!=strconv.Itoa(len(encoded)){t.Fatal("host coding/length header lost")}
  normalized.Header.Set("X-Local","changed");if response.Header.Get("X-Local")!=""{t.Fatal("shared header map")}
  data,err:=io.ReadAll(normalized.Body);if err!=nil{t.Fatal(err)}
  if !bytes.Equal(data,encoded){t.Fatal("SDK decoded or changed the HTTP layer body")}
  if normalized.Uncompressed||normalized.ContentLength!=response.ContentLength{t.Fatal("changed host body metadata")}
  lease.Close();lease.Close();if original.closed.Load()!=1{t.Fatalf("original closed %d times",original.closed.Load())}
 })}
}
func TestBodylessUnknownAndAlreadyDecoded(t *testing.T){
 for _,test:=range []struct{method string;status int}{{"HEAD",200},{"GET",204},{"GET",205},{"GET",304}}{
  raw:=&observedBody{Reader:strings.NewReader("not compressed")}
  response:=&http.Response{StatusCode:test.status,Header:http.Header{"Content-Encoding":{"gzip"}},Body:raw}
  _,lease,err:=NewResponseBody(context.Background(),response,test.method);if err!=nil{t.Fatal(err)};lease.Close()
  if raw.reads.Load()!=0||raw.closed.Load()!=1{t.Fatal("bodyless response was read or leaked")}
 }
 raw:=&observedBody{Reader:strings.NewReader("already decoded")}
 response:=&http.Response{StatusCode:200,Header:http.Header{"content-encoding":{"gzip"}},Body:raw,Uncompressed:true}
 normalized,_,err:=NewResponseBody(context.Background(),response,"GET");if err!=nil{t.Fatal(err)};data,err:=io.ReadAll(normalized.Body);if err!=nil||string(data)!="already decoded"{t.Fatalf("double decoding: %s %v",data,err)}
 raw=&observedBody{Reader:strings.NewReader("valid body")}
 normalized,_,err=NewResponseBody(context.Background(),&http.Response{StatusCode:200,Body:raw},"head");if err!=nil{t.Fatal(err)};data,err=io.ReadAll(normalized.Body);if err!=nil||string(data)!="valid body"{t.Fatal("treated a case-sensitive extension method as HEAD")}
}
func TestMalformedAndEarlyClose(t *testing.T){
 raw:=&observedBody{Reader:strings.NewReader("bad gzip")}
 response:=&http.Response{StatusCode:200,Header:http.Header{"Content-Encoding":{"gzip"}},Body:raw}
 normalized,lease,err:=NewResponseBody(context.Background(),response,"GET");if err!=nil{t.Fatal(err)}
 if data,err:=io.ReadAll(normalized.Body);err!=nil||string(data)!="bad gzip"{t.Fatal("SDK interpreted opaque compression data",string(data),err)};lease.Close();if raw.closed.Load()!=1{t.Fatal("malformed body leaked")}
 raw=&observedBody{Reader:strings.NewReader("unread encoded data")};response.Body=raw
 _,lease,err=NewResponseBody(context.Background(),response,"GET");if err!=nil{t.Fatal(err)};lease.Close();if raw.reads.Load()!=0||raw.closed.Load()!=1{t.Fatal("early Close read or leaked a body")}
 cause:=errors.New("close failed");raw=&observedBody{Reader:strings.NewReader("body"),closeError:cause}
 normalized,_,err=NewResponseBody(context.Background(),&http.Response{StatusCode:200,Body:raw},"GET");if err!=nil{t.Fatal(err)}
 if _,err:=io.ReadAll(normalized.Body);!errors.Is(err,cause){t.Fatalf("lost close failure: %v",err)}
}
type blockedBody struct{started chan struct{};released chan struct{};begin sync.Once;release sync.Once;closed atomic.Int32}
func(body *blockedBody)Read(buffer []byte)(int,error){body.begin.Do(func(){close(body.started)});<-body.released;return 0,io.ErrClosedPipe}
func(body *blockedBody)Close()error{body.release.Do(func(){body.closed.Add(1);close(body.released)});return nil}
func TestLeaseCancellationInterruptsInitializationAndRead(t *testing.T){
 for _,coding:=range []string{"","gzip","deflate","br","zstd"}{t.Run(coding,func(t *testing.T){
  ctx,cancel:=context.WithCancel(context.Background());defer cancel()
  raw:=&blockedBody{started:make(chan struct{}),released:make(chan struct{})}
  response:=&http.Response{StatusCode:200,Header:http.Header{"Content-Encoding":{coding}},Body:raw}
  _,lease,err:=NewResponseBody(ctx,response,"GET");if err!=nil{t.Fatal(err)}
  done:=make(chan error,1);go func(){_,err:=io.ReadAll(lease);done<-err}()
  select{case <-raw.started:case <-time.After(time.Second):t.Fatal("Read did not start")}
  cancel()
  select{case err:=<-done:if !errors.Is(err,context.Canceled){t.Fatalf("lost cancellation: %v",err)};case <-time.After(time.Second):t.Fatal("cancel failed to interrupt body read")}
  lease.Close();if raw.closed.Load()!=1{t.Fatalf("body closed %d times",raw.closed.Load())}
 })}
}
`
