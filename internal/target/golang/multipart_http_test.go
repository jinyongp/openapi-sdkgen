package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedMultipartHTTPAndSourceLifetime(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Multipart streaming","version":"1"},"paths":{
"/upload":{"post":{"operationId":"upload","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["file","files","label","number","payload"],"properties":{"file":{"type":"string","format":"binary"},"files":{"type":"array","items":{"type":"string","format":"binary"}},"label":{"type":"string"},"number":{"type":"integer"},"payload":{"type":"object"}}},"encoding":{"file":{"contentType":"text/plain, application/octet-stream","headers":{"X-Part":{"required":true,"schema":{"type":"integer","minimum":2}}}},"payload":{"contentType":"application/json"}}}}},"responses":{"204":{"description":"Done"}}}},
"/multi":{"post":{"operationId":"uploadMulti","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["file"],"properties":{"file":{"type":"string","format":"binary"}}}},"application/json":{"schema":{"type":"object"}}}},"responses":{"204":{"description":"Done"}}}}
}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"multipart_http_test.go": multipartHTTPConsumer, "api/internal/media/multipart_test.go": multipartSourceConsumer})
}

const multipartHTTPConsumer = `package consumer_test
import("context";"errors";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"strings";"sync";"sync/atomic";"testing";"time";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
type uploadSource struct{opens,closes atomic.Int32}
func(*uploadSource)Metadata()types.BinaryMetadata{return types.BinaryMetadata{Filename:"a\\\"b.txt",ContentType:"text/plain",Size:types.Some(int64(7))}}
func(source *uploadSource)Open(context.Context)(io.ReadCloser,error){source.opens.Add(1);return &uploadReader{Reader:strings.NewReader("payload"),source:source},nil}
type uploadReader struct{*strings.Reader;source *uploadSource}
func(reader *uploadReader)Close()error{reader.source.closes.Add(1);return nil}
func TestMultipartHTTP(t *testing.T){
 var requests atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){
 requests.Add(1);media,parameters,err:=mime.ParseMediaType(request.Header.Get("Content-Type"));if err!=nil||media!="multipart/form-data"||parameters["boundary"]==""{t.Error("multipart content type",request.Header,err);writer.WriteHeader(500);return}
 reader:=multipart.NewReader(request.Body,parameters["boundary"]);names:=[]string{};values:=[]string{}
 for{part,err:=reader.NextPart();if err==io.EOF{break};if err!=nil{t.Error(err);break};data,err:=io.ReadAll(part);if err!=nil{t.Error(err)};names=append(names,part.FormName());values=append(values,string(data));if part.FormName()=="file"{expected:="text/plain";if request.URL.Path=="/multi"{expected="application/octet-stream"};if part.FileName()!="a\\\"b.txt"||part.Header.Get("Content-Type")!=expected{t.Error("file metadata",part.Header)}};part.Close()}
 if request.URL.Path=="/upload"{if strings.Join(names,",")!="file,files,files,label,number,payload"||strings.Join(values,"|")!="payload|one|two|label|3|{\"x\":1}"{t.Error("multipart wire",names,values)}}else if strings.Join(names,",")!="file"||strings.Join(values,"|")!="payload"{t.Error("union body",names,values)}
 writer.WriteHeader(204)
 }));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 source:=&uploadSource{};payloadValue,_:=types.ParseValue([]byte("{\"x\":1}"));payload,err:=models.DecodeUploadBodyPayload(context.Background(),payloadValue);if err!=nil{t.Fatal(err)}
 input:=sdk.UploadInput{Body:sdk.UploadBody{File:types.BinaryFromSource(source),Files:[]types.Binary{types.BinaryFromBytes([]byte("one")),types.BinaryFromString("two")},Label:"label",Number:3,Payload:payload}}
 options:=types.RequestOptions{MultipartHeaders:map[string]http.Header{"file":{"X-Part":{"3"}}}}
 if _,err:=client.Operations.Upload(input).WithOptions(options).Do(context.Background());err!=nil{t.Fatal(err)}
 if source.opens.Load()!=1||source.closes.Load()!=1{t.Fatal("source ownership",source.opens.Load(),source.closes.Load())}
 body:=sdk.UploadMultiMultipartFormDataBody{File:types.BinaryFromSource(source)}
 if _,err:=client.Operations.UploadMulti(sdk.UploadMultiInput{Body:sdk.NewUploadMultiMultipartFormDataBody(body)}).Do(context.Background());err!=nil{t.Fatal(err)}
 if source.opens.Load()!=2||source.closes.Load()!=2||requests.Load()!=2{t.Fatal("union source ownership",source.opens.Load(),source.closes.Load(),requests.Load())}
 for _,headers:=range []map[string]http.Header{nil,{"file":{"X-Part":{"1"}}},{"file":{"X-Part":{"3"},"X-Undeclared":{"x"}}},{"file":{"X-Part":{"3"},"Content-Type":{"text/plain"}}},{"file":{"X-Part":{"3\r\nInjected: value"}}}}{
 if _,err:=client.Operations.Upload(input).WithOptions(types.RequestOptions{MultipartHeaders:headers}).Do(context.Background());err==nil{t.Fatal("accepted invalid or missing multipart header",headers)}
 }
 if _,err:=client.Operations.Upload(input).WithOptions(types.RequestOptions{MultipartHeaders:options.MultipartHeaders,MultipartContentTypes:map[string]string{"file":"image/png"}}).Do(context.Background());err==nil{t.Fatal("accepted disallowed part content type")}
 if source.opens.Load()!=2||requests.Load()!=2{t.Fatal("invalid header opened source or sent request",source.opens.Load(),requests.Load())}
}
type earlyUploadSource struct{opened,released chan struct{};opens,closes atomic.Int32;once sync.Once}
func(*earlyUploadSource)Metadata()types.BinaryMetadata{return types.BinaryMetadata{Filename:"big.bin",Size:types.Some(int64(64<<20))}}
func(source *earlyUploadSource)Open(context.Context)(io.ReadCloser,error){source.opens.Add(1);close(source.opened);return &earlyUploadReader{source:source},nil}
type earlyUploadReader struct{source *earlyUploadSource}
func(reader *earlyUploadReader)Read([]byte)(int,error){<-reader.source.released;return 0,io.ErrClosedPipe}
func(reader *earlyUploadReader)Close()error{reader.source.once.Do(func(){reader.source.closes.Add(1);close(reader.source.released)});return nil}
func TestEarlyHTTPResponseClosesActiveMultipartSource(t *testing.T){
 source:=&earlyUploadSource{opened:make(chan struct{}),released:make(chan struct{})}
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){select{case <-source.opened:case <-time.After(5*time.Second):t.Error("source never opened")};writer.Header().Set("Connection","close");writer.WriteHeader(204);writer.(http.Flusher).Flush()}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
 body:=sdk.UploadMultiMultipartFormDataBody{File:types.BinaryFromSource(source)}
 if _,err:=client.Operations.UploadMulti(sdk.UploadMultiInput{Body:sdk.NewUploadMultiMultipartFormDataBody(body)}).Do(ctx);err!=nil{var apiError *types.APIError;if errors.As(err,&apiError)&&apiError.Code==types.RequestTimeout{t.Fatal("early response left source blocked",err)};t.Fatal("early response",err)}
 if source.opens.Load()!=1||source.closes.Load()!=1{t.Fatal("early response source lifetime",source.opens.Load(),source.closes.Load())}
}
`

const multipartSourceConsumer = `package media
import("context";"errors";"io";"net/textproto";"strings";"sync";"sync/atomic";"testing";"time";"example.test/consumer/api/types")
type lazyPartSource struct{opens,closes,reads atomic.Int32;size int64;block bool;fail bool;opened chan struct{};released chan struct{};once sync.Once}
func(source *lazyPartSource)Metadata()types.BinaryMetadata{return types.BinaryMetadata{Size:types.Some(source.size)}}
func(source *lazyPartSource)Open(context.Context)(io.ReadCloser,error){source.opens.Add(1);if source.fail{return nil,errors.New("source open failed")};if source.opened!=nil{close(source.opened)};return &lazyPartReader{source:source,remaining:source.size},nil}
type lazyPartReader struct{source *lazyPartSource;remaining int64}
func(reader *lazyPartReader)Read(buffer []byte)(int,error){reader.source.reads.Add(1);if reader.source.block{<-reader.source.released;return 0,errors.New("closed")};if reader.remaining==0{return 0,io.EOF};count:=len(buffer);if int64(count)>reader.remaining{count=int(reader.remaining)};clear(buffer[:count]);reader.remaining-=int64(count);return count,nil}
func(reader *lazyPartReader)Close()error{reader.source.closes.Add(1);if reader.source.released!=nil{reader.source.once.Do(func(){close(reader.source.released)})};return nil}
func TestMultipartSourceDoesNotReadAhead(t *testing.T){
 source:=&lazyPartSource{size:64<<20};binary,err:=EncodeMultipart("multipart/form-data; boundary=known",[]MultipartPart{ {Header:textproto.MIMEHeader{"Content-Type":{"application/octet-stream"}},Body:types.BinaryFromSource(source)} });if err!=nil{t.Fatal(err)}
 if source.opens.Load()!=0||source.reads.Load()!=0{t.Fatal("encode read source")};reader,err:=binary.Open(context.Background());if err!=nil{t.Fatal(err)}
 if source.opens.Load()!=0{t.Fatal("Open eagerly opened a part")};header:=make([]byte,1);if _,err:=reader.Read(header);err!=nil{t.Fatal(err)};if source.opens.Load()!=0{t.Fatal("header read eagerly opened source")}
 count,err:=io.Copy(io.Discard,reader);count+=int64(len(header));expected,present:=binary.Metadata().Size.Get();if err!=nil||count<64<<20||!present||count!=expected{t.Fatal("large upload and computed size",count,expected,present,err)};if err:=reader.Close();err!=nil{t.Fatal(err)}
 if source.opens.Load()!=1||source.closes.Load()!=1{t.Fatal("source lifetime",source.opens.Load(),source.closes.Load())}
}
func TestMultipartCancellationClosesBlockedPart(t *testing.T){
 source:=&lazyPartSource{size:1,block:true,opened:make(chan struct{}),released:make(chan struct{})};ctx,cancel:=context.WithCancel(context.Background());defer cancel()
 binary,err:=EncodeMultipart("multipart/mixed",[]MultipartPart{ {Body:types.BinaryFromSource(source)} });if err!=nil{t.Fatal(err)};reader,err:=binary.Open(ctx);if err!=nil{t.Fatal(err)}
 result:=make(chan error,1);go func(){_,err:=io.Copy(io.Discard,reader);result<-err}();select{case <-source.opened:case <-time.After(5*time.Second):t.Fatal("part not opened")};cancel()
 select{case err:=<-result:if err==nil{t.Fatal("cancellation ignored")};case <-time.After(5*time.Second):t.Fatal("blocked read not closed")};reader.Close()
 if source.opens.Load()!=1||source.closes.Load()!=1{t.Fatal("cancel lifetime",source.opens.Load(),source.closes.Load())}
}
func TestMultipartFailureAndHeaderValidation(t *testing.T){
 bad:=&lazyPartSource{size:1,fail:true};next:=&lazyPartSource{size:1}
 binary,err:=EncodeMultipart("multipart/mixed",[]MultipartPart{ {Body:types.BinaryFromSource(bad)},{Body:types.BinaryFromSource(next)} });if err!=nil{t.Fatal(err)};reader,err:=binary.Open(context.Background());if err!=nil{t.Fatal(err)};_,err=io.Copy(io.Discard,reader);if err==nil||!strings.Contains(err.Error(),"source open failed"){t.Fatal("open failure lost",err)};reader.Close();if next.opens.Load()!=0{t.Fatal("opened later source after failure")}
 for _,header:=range []textproto.MIMEHeader{ {"Bad\rName":{"x"}},{"X-Test":{"one\r\ntwo"}},{"X-Test":{"one\x01two"}},{"X-Test":{"one\x7ftwo"}} }{if _,err:=EncodeMultipart("multipart/mixed",[]MultipartPart{ {Header:header} });err==nil{t.Fatal("accepted header injection")}}
}
type waitingOpenSource struct{opened chan struct{};canceled atomic.Int32}
func(*waitingOpenSource)Metadata()types.BinaryMetadata{return types.BinaryMetadata{}}
func(source *waitingOpenSource)Open(ctx context.Context)(io.ReadCloser,error){close(source.opened);<-ctx.Done();source.canceled.Add(1);return nil,ctx.Err()}
func TestMultipartCloseCancelsPartOpen(t *testing.T){
 source:=&waitingOpenSource{opened:make(chan struct{})};binary,err:=EncodeMultipart("multipart/mixed",[]MultipartPart{ {Body:types.BinaryFromSource(source)} });if err!=nil{t.Fatal(err)};reader,err:=binary.Open(context.Background());if err!=nil{t.Fatal(err)}
 result:=make(chan error,1);go func(){_,err:=io.Copy(io.Discard,reader);result<-err}();select{case <-source.opened:case <-time.After(5*time.Second):t.Fatal("part Open not reached")};if err:=reader.Close();err!=nil{t.Fatal(err)}
 select{case err:=<-result:if err==nil{t.Fatal("Close ignored")};case <-time.After(5*time.Second):t.Fatal("Close did not cancel Open")};if source.canceled.Load()!=1{t.Fatal("part Open context not canceled")}
}
type slowClosePart struct{opened,started,release chan struct{};closes atomic.Int32}
func(*slowClosePart)Metadata()types.BinaryMetadata{return types.BinaryMetadata{}}
func(source *slowClosePart)Open(context.Context)(io.ReadCloser,error){close(source.opened);return &slowCloseReader{source:source},nil}
type slowCloseReader struct{source *slowClosePart}
func(reader *slowCloseReader)Read([]byte)(int,error){<-reader.source.release;return 0,io.ErrClosedPipe}
func(reader *slowCloseReader)Close()error{reader.source.closes.Add(1);close(reader.source.started);<-reader.source.release;return nil}
func TestMultipartCloseWaitsForCancellationCleanup(t *testing.T){
 source:=&slowClosePart{opened:make(chan struct{}),started:make(chan struct{}),release:make(chan struct{})};ctx,cancel:=context.WithCancel(context.Background());defer cancel()
 binary,err:=EncodeMultipart("multipart/mixed",[]MultipartPart{ {Body:types.BinaryFromSource(source)} });if err!=nil{t.Fatal(err)};reader,err:=binary.Open(ctx);if err!=nil{t.Fatal(err)}
 reading:=make(chan error,1);go func(){_,err:=io.Copy(io.Discard,reader);reading<-err}();select{case <-source.opened:case <-time.After(5*time.Second):t.Fatal("part not opened")};cancel();select{case <-source.started:case <-time.After(5*time.Second):t.Fatal("cancel did not close part")}
 closing:=make(chan error,1);attempt:=make(chan struct{});go func(){close(attempt);closing<-reader.Close()}();<-attempt
 select{case <-closing:close(source.release);t.Fatal("Close returned before active source cleanup finished");case <-time.After(50*time.Millisecond):}
 close(source.release);select{case <-closing:case <-time.After(5*time.Second):t.Fatal("Close did not complete")};select{case <-reading:case <-time.After(5*time.Second):t.Fatal("read did not finish")};if source.closes.Load()!=1{t.Fatal("source closed more than once",source.closes.Load())}
}
`
