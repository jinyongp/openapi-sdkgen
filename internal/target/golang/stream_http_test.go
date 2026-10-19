package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedNativeStreamHTTP(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Native streams","version":"1"},"paths":{"/numbers":{"get":{"operationId":"readNumbers","responses":{"200":{"description":"Done","content":{"application/x-ndjson":{"schema":{"type":"array","items":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}},"itemSchema":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}}}}}}},"/events":{"get":{"operationId":"readEvents","responses":{"200":{"description":"Done","content":{"text/event-stream":{"itemSchema":{"type":"object","required":["data"],"properties":{"data":{"type":"string","contentMediaType":"application/json","contentSchema":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}},"id":{"type":"string"}}}}}}}}},"/buffered":{"get":{"operationId":"bufferedEvents","responses":{"200":{"description":"Done","content":{"text/event-stream":{"schema":{"type":"array","items":{"type":"object","required":["data"],"properties":{"data":{"type":"string"},"id":{"type":"string"}}}}}}}}}},"/seq":{"get":{"operationId":"readSequence","responses":{"200":{"description":"Done","content":{"application/json-seq":{"itemSchema":{"type":"integer"}}}}}}},"/slow":{"get":{"operationId":"slowNumbers","responses":{"200":{"description":"Done","content":{"application/x-ndjson":{"itemSchema":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}}}}}}},"/gzip":{"get":{"operationId":"gzipNumbers","responses":{"200":{"description":"Done","content":{"application/x-ndjson":{"itemSchema":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}}}}}}},"/invalid":{"get":{"operationId":"invalidNumbers","responses":{"200":{"description":"Done","content":{"application/x-ndjson":{"itemSchema":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}}}}}}},"/large":{"get":{"operationId":"largeNumbers","responses":{"200":{"description":"Done","content":{"application/x-ndjson":{"itemSchema":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}}}}}}},"/mixed":{"get":{"operationId":"mixedEvents","responses":{"200":{"description":"Done","content":{"application/x-ndjson":{"itemSchema":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}}}},"201":{"description":"Done","content":{"text/event-stream":{"itemSchema":{"type":"object","required":["data"],"properties":{"data":{"type":"string"},"id":{"type":"string"}}}}}}}}},"/finite":{"get":{"operationId":"finiteFallback","responses":{"200":{"description":"Done","content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}}}},"201":{"description":"Done","content":{"application/x-ndjson":{"itemSchema":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}}}}}}},"/custom":{"get":{"operationId":"customNumbers","responses":{"200":{"description":"Done","content":{"application/vnd.frames":{"itemSchema":{"type":"object","required":["id"],"properties":{"id":{"type":"integer"}}}}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"native_stream_http_test.go": nativeStreamHTTPConsumer, "stream_extension_test.go": streamExtensionConsumer})
}

const nativeStreamHTTPConsumer = `package consumer_test
import("compress/gzip";"context";"errors";"fmt";"io";"net/http";"net/http/httptest";"reflect";"strings";"sync/atomic";"testing";"time";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func streamHTTPClient(t *testing.T,configs ...sdk.ClientOptions)(*sdk.Client,*atomic.Int32){t.Helper();calls:=new(atomic.Int32);server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){calls.Add(1);writer.Header().Set("X-Response","headers");switch request.URL.Path{
case "/custom":writer.Header().Set("Content-Type","application/vnd.frames; version=1");io.WriteString(writer,"{\"id\":4}|{\"id\":5}|")
case "/numbers":writer.Header().Set("Content-Type","application/x-ndjson");io.WriteString(writer,"{\"id\":1}\n{\"id\":2}\n")
case "/events":writer.Header().Set("Content-Type","text/event-stream");io.WriteString(writer,"id: first\ndata: {\"id\":1}\n\ndata: {\"id\":2}\n\n")
case "/buffered":writer.Header().Set("Content-Type","text/event-stream");io.WriteString(writer,"data: first\n\ndata: second\n\n")
case "/seq":writer.Header().Set("Content-Type","application/json-seq");io.WriteString(writer,"\x1e3\n\x1e4\n")
case "/gzip":writer.Header().Set("Content-Type","application/x-ndjson");writer.Header().Set("Content-Encoding","gzip");compressed:=gzip.NewWriter(writer);io.WriteString(compressed,"{\"id\":9}\n");compressed.Close()
case "/invalid":writer.Header().Set("Content-Type","application/x-ndjson");io.WriteString(writer,"{\"id\":\"bad\"}\n")
case "/large":writer.Header().Set("Content-Type","application/x-ndjson");io.WriteString(writer,"{\"id\":123456}\n")
case "/slow":writer.Header().Set("Content-Type","application/x-ndjson");writer.WriteHeader(200);writer.(http.Flusher).Flush();<-request.Context().Done()
case "/mixed":if request.Header.Get("Accept")=="text/event-stream"{writer.Header().Set("Content-Type","text/event-stream");writer.WriteHeader(201);io.WriteString(writer,"data: selected\n\n")}else{writer.Header().Set("Content-Type","application/x-ndjson");io.WriteString(writer,"{\"id\":8}\n")}
case "/finite":writer.Header().Set("Content-Type","application/json");io.WriteString(writer,"{\"id\":5}")
default:t.Error("unexpected path",request.URL.Path);writer.WriteHeader(500)
}}));t.Cleanup(server.Close);options:=sdk.ClientOptions{};if len(configs)!=0{options=configs[0]};options.BaseURL=server.URL;client,err:=sdk.NewClient(options);if err!=nil{t.Fatal(err)};return client,calls}
func TestTypedStreamLazyHeadersAndBuffering(t *testing.T){
 client,calls:=streamHTTPClient(t);handle:=client.Operations.ReadNumbers().Stream(context.Background());if calls.Load()!=0{t.Fatal("Stream performed HTTP eagerly")};metadata,err:=handle.Response();if err!=nil||metadata.Header.Get("X-Response")!="headers"||metadata.StatusCode!=200{t.Fatal("response metadata",metadata,err)};if calls.Load()!=1{t.Fatal("request count",calls.Load())};var ids []float64;for handle.Next(){var item sdk.ReadNumbersStreamItem=handle.Item();ids=append(ids,item.ID)};if handle.Err()!=nil||!reflect.DeepEqual(ids,[]float64{1,2}){t.Fatal("typed items",ids,handle.Err())};handle.Close()
 result,err:=client.Operations.ReadNumbers().Do(context.Background());if err!=nil||len(result)!=2||result[0].ID!=1{t.Fatal("complete body",result,err)}
 buffered,err:=client.Operations.BufferedEvents().Do(context.Background());if err!=nil||len(buffered)!=2||buffered[0].Data!="first"{t.Fatal("schema-only SSE",buffered,err)};if _,present:=reflect.TypeOf(client.Operations.BufferedEvents()).MethodByName("Stream");present{t.Fatal("schema-only call exposes incremental API")}
 raw,err:=client.Operations.ReadNumbers().Raw(context.Background());if err!=nil{t.Fatal(err)};data,err:=io.ReadAll(raw.Body);raw.Close();if err!=nil||!strings.Contains(string(data),"\"id\":1"){t.Fatal("raw stream body",string(data),err)}
}
func TestSSEStringsSequencesAndMediaUnions(t *testing.T){
 client,_:=streamHTTPClient(t);events:=client.Operations.ReadEvents().Stream(context.Background());defer events.Close();count:=0;for events.Next(){count++;event:=events.Item();if event.Data!=fmt.Sprintf("{\"id\":%d}",count){t.Fatal("SSE data changed to JSON",event.Data)};id,present:=event.ID.Get();if !present||id!="first"{t.Fatal("SSE persistent id",id,present)}};if count!=2||events.Err()!=nil{t.Fatal("SSE",count,events.Err())}
 sequence:=client.Operations.ReadSequence().Stream(context.Background());defer sequence.Close();count=0;for sequence.Next(){count++;if sequence.Item()!=3&&sequence.Item()!=4{t.Fatal("sequence",sequence.Item())}};if count!=2||sequence.Err()!=nil{t.Fatal("sequence end",count,sequence.Err())}
 mixed:=client.Operations.MixedEvents().WithOptions(types.RequestOptions{Accept:types.Some("text/event-stream")}).Stream(context.Background());defer mixed.Close();if !mixed.Next(){t.Fatal(mixed.Err())};item,present:=mixed.Item().As201Stream();if !present||item.Data!="selected"{t.Fatal("media union",item,present)};if mixed.Next()||mixed.Err()!=nil{t.Fatal("union EOF",mixed.Err())}
}
func TestStreamFailurePhasesAndCompressedFrames(t *testing.T){
 client,_:=streamHTTPClient(t);invalid:=client.Operations.InvalidNumbers().Stream(context.Background());defer invalid.Close();if invalid.Next(){t.Fatal("invalid item accepted")};var failure *sdk.APIError;if !errors.As(invalid.Err(),&failure)||failure.Code!=types.ResponseDecodeFailed{t.Fatal("item validation phase",invalid.Err())}
 large:=client.Operations.LargeNumbers().WithOptions(types.RequestOptions{MaxStreamFrameBytes:types.Some(8)}).Stream(context.Background());defer large.Close();if large.Next(){t.Fatal("oversized frame accepted")};var frame *types.StreamFrameError;if !errors.As(large.Err(),&frame)||frame.Kind!="limit"{t.Fatal("frame limit cause",large.Err())}
 compressed:=client.Operations.GzipNumbers().Stream(context.Background());defer compressed.Close();metadata,err:=compressed.Response();if err!=nil||metadata.Header.Get("Content-Encoding")!="gzip"{t.Fatal("compression metadata",metadata,err)};if !compressed.Next()||compressed.Item().ID!=9||compressed.Next()||compressed.Err()!=nil{t.Fatal("compressed frame",compressed.Err())}
 finite:=client.Operations.FiniteFallback().Stream(context.Background());defer finite.Close();if _,err:=finite.Response();!errors.As(err,&failure)||failure.Code!=types.ResponseDecodeFailed{t.Fatal("finite response accepted as stream",err)}
}
func TestStreamCloseExternalCancelAndTimeout(t *testing.T){
 for _,mode:=range []string{"close","cancel","timeout"}{t.Run(mode,func(t *testing.T){client,_:=streamHTTPClient(t);ctx,cancel:=context.WithTimeout(context.Background(),3*time.Second);defer cancel();call:=client.Operations.SlowNumbers();if mode=="timeout"{call=call.WithOptions(types.RequestOptions{Timeout:types.Some(250*time.Millisecond)})};handle:=call.Stream(ctx);defer handle.Close();if _,err:=handle.Response();err!=nil{t.Fatal(err)};done:=make(chan bool,1);go func(){done<-handle.Next()}();if mode=="close"{handle.Close()}else if mode=="cancel"{cancel()};select{case value:=<-done:if value{t.Fatal("unexpected item")};case <-time.After(4*time.Second):t.Fatal("stream did not stop")};if mode=="close"{if handle.Err()!=nil{t.Fatal("explicit Close error",handle.Err())}}else{var failure *sdk.APIError;expected:=types.RequestAborted;if mode=="timeout"{expected=types.RequestTimeout};if !errors.As(handle.Err(),&failure)||failure.Code!=expected||failure.OperationID!="slowNumbers"{t.Fatal("cancellation phase",mode,handle.Err())}}})}
}
`
