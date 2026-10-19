package golang

import "testing"

func TestGeneratedHTTPTransportIntegration(t *testing.T) {
	_, plan := healthPlan(t)
	testGeneratedModule(t, plan, map[string]string{"api/internal/transport/adapter_test.go": httpAdapterConsumer, "http_compression_test.go": httpCompressionConsumer})
}

const httpAdapterConsumer = `package transport
import("bytes";"context";"io";"net/http";"net/http/cookiejar";"net/http/httptest";"net/http/httptrace";"net/url";"strings";"sync/atomic";"testing";"time";"example.test/consumer/api/types")
type adapterRoundTrip func(*http.Request)(*http.Response,error)
func(call adapterRoundTrip)RoundTrip(request *http.Request)(*http.Response,error){return call(request)}
func TestConfiguredDefaultClientTransportIsPreserved(t *testing.T){
 previous:=http.DefaultClient;t.Cleanup(func(){http.DefaultClient=previous})
 called:=false
 http.DefaultClient=&http.Client{Transport:adapterRoundTrip(func(request *http.Request)(*http.Response,error){called=true;return &http.Response{StatusCode:200,Header:make(http.Header),Body:http.NoBody,Request:request},nil}),Timeout:time.Second}
 adapter:=NewHTTPClient(nil);request,_:=http.NewRequest("GET","http://example.test",nil)
 response,err:=adapter.Do(request,types.TransportOptions{AllowRedirects:true});if err!=nil{t.Fatal(err)};response.Body.Close()
 if !called||adapter.client.Timeout!=time.Second{t.Fatal("configured default HTTP client was replaced")}
}
func TestClientSettingsAndCapability(t *testing.T){
 var seenEncoding string
 original:=adapterRoundTrip(func(request *http.Request)(*http.Response,error){seenEncoding=request.Header.Get("Accept-Encoding");return &http.Response{StatusCode:200,Header:make(http.Header),Body:io.NopCloser(strings.NewReader("ok")),Request:request},nil})
 jar,_:=cookiejar.New(nil)
 provided:=&http.Client{Transport:original,Jar:jar,Timeout:2*time.Second,CheckRedirect:func(*http.Request,[]*http.Request)error{return http.ErrUseLastResponse}}
 adapter:=NewHTTPClient(provided)
 request,_:=http.NewRequest("GET","https://example.test/path",nil)
 response,err:=adapter.Do(request,types.TransportOptions{Credentials:types.Omit,AllowRedirects:true});if err!=nil{t.Fatal(err)};response.Body.Close()
 if seenEncoding!="br, gzip, deflate"||request.Header.Get("Accept-Encoding")!=""{t.Fatal("default encoding or request clone",seenEncoding)}
 request.Header.Set("Range","bytes=0-3")
 response,err=adapter.Do(request,types.TransportOptions{AllowRedirects:true});if err!=nil{t.Fatal(err)};response.Body.Close()
 if seenEncoding!="identity"{t.Fatal("Range did not select identity")}
 if provided.Timeout!=2*time.Second||provided.Jar!=jar||provided.CheckRedirect==nil{t.Fatal("mutated injected HTTPClient settings")}
 if caps:=adapter.Capabilities();caps.ResponseBody!=types.Encoded||!caps.CookieJar||!caps.CookieHeaders{t.Fatalf("capabilities %+v",caps)}
}
func TestActualCookieModesAndCrossOriginRedirect(t *testing.T){
 var firstCookie,lastCookie string
 target:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){lastCookie=request.Header.Get("Cookie");io.WriteString(writer,"target")}));defer target.Close()
 first:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){firstCookie=request.Header.Get("Cookie");http.Redirect(writer,request,target.URL,http.StatusFound)}));defer first.Close()
 jar,_:=cookiejar.New(nil);origin,_:=url.Parse(first.URL);jar.SetCookies(origin,[]*http.Cookie{{Name:"ambient",Value:"cookie",Path:"/"}})
 adapter:=NewHTTPClient(&http.Client{Jar:jar})
 for _,test:=range []struct{mode types.CredentialsMode;origin string;first,last bool}{{types.Omit,first.URL,false,false},{types.SameOrigin,"",false,false},{types.SameOrigin,first.URL,true,false},{types.Include,"",true,true}}{
  firstCookie,lastCookie="","";request,_:=http.NewRequest("GET",first.URL,nil)
  response,err:=adapter.Do(request,types.TransportOptions{Credentials:test.mode,Origin:test.origin,AllowRedirects:true});if err!=nil{t.Fatal(err)};io.Copy(io.Discard,response.Body);response.Body.Close()
  if (firstCookie!="")!=test.first||(lastCookie!="")!=test.last{t.Errorf("mode %s origin %s cookies %q %q",test.mode,test.origin,firstCookie,lastCookie)}
 }
}
func TestActualConnectionReuse(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){io.WriteString(writer,"ok")}));defer server.Close()
 adapter:=NewHTTPClient(nil)
 for sample:=0;sample<2;sample++{
  reused:=false
  trace:=&httptrace.ClientTrace{GotConn:func(info httptrace.GotConnInfo){reused=info.Reused}}
  request,_:=http.NewRequestWithContext(httptrace.WithClientTrace(context.Background(),trace),"GET",server.URL,nil)
  response,err:=adapter.Do(request,types.TransportOptions{AllowRedirects:true});if err!=nil{t.Fatal(err)}
  normalized,_,err:=NewResponseBody(request.Context(),response,"GET",adapter.Capabilities().ResponseBody);if err!=nil{t.Fatal(err)}
  if _,err:=io.Copy(io.Discard,normalized.Body);err!=nil{t.Fatal(err)}
  if sample==1&&!reused{t.Fatal("buffered body cleanup prevented connection reuse")}
 }
}
func TestRedirectFailureAndImplicitDecompressionOwnership(t *testing.T){
 var closed atomic.Int32
 adapter:=NewHTTPClient(&http.Client{Transport:adapterRoundTrip(func(request *http.Request)(*http.Response,error){return &http.Response{StatusCode:302,Header:http.Header{"Location":{"https://example.test/next"}},Body:&adapterBody{Reader:bytes.NewReader(nil),closed:&closed},Request:request},nil})})
 request,_:=http.NewRequest("GET","https://example.test/first",nil)
 response,err:=adapter.Do(request,types.TransportOptions{AllowRedirects:false});if err==nil{t.Fatal("redirect accepted")};if response!=nil{response.Body.Close()}
 if closed.Load()!=1{t.Fatalf("redirect failure closed original %d times",closed.Load())}
 closed.Store(0)
 adapter=NewHTTPClient(&http.Client{Transport:adapterRoundTrip(func(request *http.Request)(*http.Response,error){return &http.Response{StatusCode:200,Header:make(http.Header),Body:&adapterBody{Reader:strings.NewReader("decoded"),closed:&closed},Request:request,Uncompressed:true},nil})})
 response,err=adapter.Do(request,types.TransportOptions{AllowRedirects:true});if err==nil{t.Fatal("implicit decompression accepted")};if response!=nil{response.Body.Close()}
 if closed.Load()!=1{t.Fatalf("implicit decoding closed original %d times",closed.Load())}
}
type adapterBody struct{io.Reader;closed *atomic.Int32}
func(body *adapterBody)Close()error{body.closed.Add(1);return nil}
`

const httpCompressionConsumer = `package consumer_test
import("bytes";"compress/gzip";"context";"encoding/json";"io";"net/http";"net/http/httptest";"strconv";"testing";sdk "example.test/consumer/api")
func TestActualSDKGzipPreservesHeaders(t *testing.T){
 var compressed bytes.Buffer;writer:=gzip.NewWriter(&compressed);io.WriteString(writer,"{\"ok\":true}");writer.Close()
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){if request.Header.Get("Accept-Encoding")!="gzip, deflate"{t.Error("unexpected compression negotiation",request.Header)};writer.Header().Set("Content-Type","application/json");writer.Header().Set("Content-Encoding","gzip");writer.Header().Set("Content-Length",strconv.Itoa(compressed.Len()));writer.Write(compressed.Bytes())}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 result,err:=client.Operations.GetHealth().Do(context.Background());if err!=nil{t.Fatal(err)};encoded,_:=json.Marshal(result);if string(encoded)!="{\"ok\":true}"{t.Fatal("decoded result",string(encoded))}
 response,err:=client.Operations.GetHealth().Raw(context.Background());if err!=nil{t.Fatal(err)}
 data:=response.Bytes();if string(data)!="{\"ok\":true}"{t.Fatalf("Raw decoded bytes: %s",data)}
 if !response.OriginalResponse.Uncompressed||response.OriginalResponse.ContentLength!=-1||response.Headers.Get("Content-Encoding")!="gzip"||response.Headers.Get("Content-Length")!=strconv.Itoa(compressed.Len())||response.OriginalResponse.Body!=http.NoBody{t.Fatal("lost original compressed headers or body ownership",response)}
}
`
