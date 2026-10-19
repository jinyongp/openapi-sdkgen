package golang

import (
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedTypedInboundDuplex(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Inbound duplex","version":"1"},"paths":{},"webhooks":{"echo":{"post":{"operationId":"echo","requestBody":{"required":true,"content":{"application/x-ndjson":{"itemSchema":{"type":"integer"}}}},"responses":{"200":{"description":"Echo","content":{"application/x-ndjson":{"itemSchema":{"type":"integer"}}}}}}}}}`
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
	response := entry.Operation.Responses[0]
	consumer := strings.NewReplacer("PUBLIC", entry.PublicName, "INPUTITEM", entry.Operation.RequestStreams[0].ItemModel, "OUTPUTITEM", response.ItemModel, "VARIANT", response.Name).Replace(`package consumer_test
import("context";"errors";"io";"net/http";"net/http/httptest";"testing";"sync/atomic";"strings";"time";"example.test/consumer/api/models";"example.test/consumer/api/types";"example.test/consumer/api/server/webhooks";"example.test/consumer/api/server/runtime")
func TestTypedDuplex(t *testing.T){for _,http2:=range []bool{false,true}{t.Run(map[bool]string{false:"http1",true:"http2"}[http2],func(t *testing.T){handler,err:=webhooks.NewPUBLICHandler("/",webhooks.Options{},func(ctx context.Context,input webhooks.PUBLICInput)(webhooks.PUBLICResponse,error){source:=types.MapSource(ctx,input.Body,func(ctx context.Context,item models.INPUTITEM)(models.OUTPUTITEM,error){raw,err:=models.EncodeINPUTITEM(ctx,item);if err!=nil{return models.OUTPUTITEM(0),err};return models.DecodeOUTPUTITEM(ctx,raw)});return webhooks.NewPUBLICVARIANTResponse(source),nil});if err!=nil{t.Fatal(err)};done:=make(chan struct{});server:=httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){defer close(done);handler.ServeHTTP(w,r)}));server.EnableHTTP2=http2;if http2{server.StartTLS()}else{server.Start()};defer server.Close();ctx,cancel:=context.WithTimeout(context.Background(),3*time.Second);defer cancel();reader,writer:=io.Pipe();defer reader.Close();defer writer.Close();stop:=context.AfterFunc(ctx,func(){reader.CloseWithError(ctx.Err())});defer stop();request,_:=http.NewRequestWithContext(ctx,"POST",server.URL,reader);request.Header.Set("Content-Type","application/x-ndjson");response,err:=server.Client().Do(request);if err!=nil{t.Fatal("headers waited for input",err)};defer response.Body.Close();if http2&&response.ProtoMajor!=2{t.Fatal("HTTP/2 not negotiated")};if _,err:=io.WriteString(writer,"1\n2\n");err!=nil{t.Fatal(err)};writer.Close();data,err:=io.ReadAll(response.Body);if err!=nil||response.StatusCode!=200||string(data)!="1\n2\n"{t.Fatal(response.StatusCode,string(data),err)};cancel();select{case <-done:case <-time.After(3*time.Second):t.Fatal("handler cleanup blocked")}})}}
type failingOutput struct{index int;closes atomic.Int32}
func(source *failingOutput)Next()bool{source.index++;return source.index==1}
func(source *failingOutput)Item()models.OUTPUTITEM{return models.OUTPUTITEM(1)}
func(source *failingOutput)Err()error{return errors.New("private stream failure")}
func(source *failingOutput)Close()error{source.closes.Add(1);return nil}
func TestFailureAfterHeadersAbortsWire(t *testing.T){for _,http2:=range []bool{false,true}{t.Run(map[bool]string{false:"http1",true:"http2"}[http2],func(t *testing.T){source:=new(failingOutput);reported:=make(chan error,1);handler,err:=webhooks.NewPUBLICHandler("/",webhooks.Options{OnError:func(_ context.Context,err error){reported<-err}},func(_ context.Context,input webhooks.PUBLICInput)(webhooks.PUBLICResponse,error){return webhooks.NewPUBLICVARIANTResponse(source),nil});if err!=nil{t.Fatal(err)};server:=httptest.NewUnstartedServer(handler);server.EnableHTTP2=http2;if http2{server.StartTLS()}else{server.Start()};defer server.Close();request,_:=http.NewRequest("POST",server.URL,strings.NewReader("1\n"));request.Header.Set("Content-Type","application/x-ndjson");response,err:=server.Client().Do(request);if err!=nil{t.Fatal(err)};data,err:=io.ReadAll(response.Body);response.Body.Close();if response.StatusCode!=200||string(data)!="1\n"||err==nil{t.Fatal("partial response ended as success",response.StatusCode,string(data),err)};select{case err:=<-reported:var failure *runtime.Error;if !errors.As(err,&failure)||failure.Code!=runtime.InboundResponseInvalid{t.Fatal(err)};case <-time.After(3*time.Second):t.Fatal("failure not reported")};if source.closes.Load()!=1{t.Fatal("source close count",source.closes.Load())}})}}

`)

	custom := consumer[strings.Index(consumer, "func TestTypedDuplex"):strings.Index(consumer, "type failingOutput")]
	custom = strings.Replace(custom, "func TestTypedDuplex", "func TestTypedCustomDuplex", 1)
	custom = strings.Replace(custom, "handler,err:=", "protocol:=new(observedProtocol);adapter:=new(observedAdapter);handler,err:=", 1)
	custom = strings.Replace(custom, "webhooks.Options{}", `webhooks.Options{StreamCodecs:map[string]types.StreamCodec{"application/x-ndjson":types.NewStreamCodec[types.Value,types.Value](protocol,adapter)}}`, 1)
	custom = strings.Replace(custom, `case <-done:`, `case <-done:if protocol.decodes.Load()!=1||protocol.encodes.Load()!=1||adapter.decodes.Load()!=1||adapter.encodes.Load()!=1{t.Fatal("custom stream stages",protocol.decodes.Load(),protocol.encodes.Load(),adapter.decodes.Load(),adapter.encodes.Load())};`, 1)
	consumer += custom + observedInboundStreamCodec
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer})
}

const observedInboundStreamCodec = `
type observedProtocol struct{decodes,encodes atomic.Int32}
func(protocol *observedProtocol)Decode(ctx context.Context,reader types.StreamReader,info types.StreamContext)(types.Source[types.Value],error){protocol.decodes.Add(1);return (runtime.JSONProtocol{}).Decode(ctx,reader,info)}
func(protocol *observedProtocol)Encode(ctx context.Context,source types.Source[types.Value],info types.StreamContext)(io.ReadCloser,error){protocol.encodes.Add(1);return (runtime.JSONProtocol{}).Encode(ctx,source,info)}
type observedAdapter struct{decodes,encodes atomic.Int32}
func(adapter *observedAdapter)Decode(ctx context.Context,source types.Source[types.Value],info types.StreamContext)(types.Source[types.Value],error){adapter.decodes.Add(1);return types.MapSource(ctx,source,func(_ context.Context,value types.Value)(types.Value,error){number,_:=value.Number();return types.ValueFrom(number*10)}),nil}
func(adapter *observedAdapter)Encode(ctx context.Context,source types.Source[types.Value],info types.StreamContext)(types.Source[types.Value],error){adapter.encodes.Add(1);return types.MapSource(ctx,source,func(_ context.Context,value types.Value)(types.Value,error){number,_:=value.Number();return types.ValueFrom(number/10)}),nil}
`
