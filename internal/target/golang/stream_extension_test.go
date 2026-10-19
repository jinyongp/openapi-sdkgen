package golang

const streamExtensionConsumer = `package consumer_test
import("context";"errors";"io";"net/http";"strconv";"strings";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
type scaleAdapter struct{delta float64}
func(adapter scaleAdapter)Decode(ctx context.Context,source types.Source[types.Value],info types.StreamContext)(types.Source[types.Value],error){return types.MapSource(ctx,source,func(_ context.Context,value types.Value)(types.Value,error){id,_:=value.Get("id");number,_:=id.Number();return types.ValueFrom(map[string]any{"id":number+adapter.delta})}),nil}
func(adapter scaleAdapter)Encode(ctx context.Context,source types.Source[types.Value],info types.StreamContext)(types.Source[types.Value],error){return source,nil}
type concreteAdapter struct{fraction bool}
func(adapter concreteAdapter)Decode(ctx context.Context,source types.Source[types.Value],info types.StreamContext)(types.Source[sdk.ReadNumbersStreamItem],error){return types.MapSource(ctx,source,func(_ context.Context,value types.Value)(sdk.ReadNumbersStreamItem,error){id,_:=value.Get("id");number,_:=id.Number();if adapter.fraction{number=1.25};return sdk.ReadNumbersStreamItem{ID:number},nil}),nil}
func(concreteAdapter)Encode(ctx context.Context,source types.Source[sdk.ReadNumbersStreamItem],info types.StreamContext)(types.Source[types.Value],error){return types.MapSource(ctx,source,func(_ context.Context,value sdk.ReadNumbersStreamItem)(types.Value,error){return types.ValueFrom(map[string]any{"id":value.ID})}),nil}
type wrongAdapter struct{}
func(wrongAdapter)Decode(ctx context.Context,source types.Source[types.Value],info types.StreamContext)(types.Source[string],error){return types.MapSource(ctx,source,func(context.Context,types.Value)(string,error){return "wrong item",nil}),nil}
func(wrongAdapter)Encode(context.Context,types.Source[string],types.StreamContext)(types.Source[types.Value],error){return nil,errors.New("unused direction")}
func TestStreamCodecNativeAdapterAndRequestPriority(t *testing.T){
 defaults:=map[string]types.StreamCodec{"Application/X-NDJSON":types.NewStreamCodec[types.Value,types.Value](nil,scaleAdapter{delta:100})};client,_:=streamHTTPClient(t,sdk.ClientOptions{StreamCodecs:defaults});defaults["Application/X-NDJSON"]=types.StreamCodec{}
 handle:=client.Operations.ReadNumbers().Stream(context.Background());defer handle.Close();if !handle.Next()||handle.Item().ID!=101{t.Fatal("client adapter or map copy",handle.Item(),handle.Err())};handle.Close()
 override:=types.NewStreamCodec[types.Value,types.Value](nil,scaleAdapter{delta:200});call:=client.Operations.ReadNumbers().WithOptions(types.RequestOptions{StreamCodec:&override});override=types.StreamCodec{};selected:=call.Stream(context.Background());defer selected.Close();if !selected.Next()||selected.Item().ID!=201{t.Fatal("request codec precedence or pointer copy",selected.Item(),selected.Err())}
 complete,err:=client.Operations.ReadNumbers().Do(context.Background());if err!=nil||len(complete)!=2||complete[0].ID!=101{t.Fatal("buffered adapter",complete,err)}
 concrete:=types.NewStreamCodec[types.Value,sdk.ReadNumbersStreamItem](nil,concreteAdapter{});typed:=client.Operations.ReadNumbers().WithOptions(types.RequestOptions{StreamCodec:&concrete}).Stream(context.Background());defer typed.Close();if !typed.Next()||typed.Item().ID!=1{t.Fatal("concrete registered item",typed.Item(),typed.Err())}
 for _,codec:=range []types.StreamCodec{types.NewStreamCodec[types.Value,sdk.ReadNumbersStreamItem](nil,concreteAdapter{fraction:true}),types.NewStreamCodec[types.Value,string](nil,wrongAdapter{})}{failed:=client.Operations.ReadNumbers().WithOptions(types.RequestOptions{StreamCodec:&codec}).Stream(context.Background());if failed.Next()||failed.Err()==nil{t.Fatal("invalid adapter item accepted")};failed.Close()}
}
type traceKey struct{}
type pipeProtocol struct{oversized bool;nilSource bool;failure error;closes *atomic.Int32}
func(protocol pipeProtocol)Decode(ctx context.Context,reader types.StreamReader,info types.StreamContext)(types.Source[types.Value],error){
 if protocol.failure!=nil{return nil,protocol.failure};if protocol.nilSource{var source *pipeSource;return source,nil}
 if ctx.Value(traceKey{})!="trace"||!strings.HasPrefix(info.ContentType,"application/vnd.frames")||info.MaxFrameBytes!=64{return nil,errors.New("missing protocol context or limits")}
 return &pipeSource{ctx:ctx,reader:reader,info:info,oversized:protocol.oversized,closes:protocol.closes},nil
}
func(pipeProtocol)Encode(context.Context,types.Source[types.Value],types.StreamContext)(io.ReadCloser,error){return nil,errors.New("unused direction")}
type pipeSource struct{ctx context.Context;reader types.StreamReader;info types.StreamContext;oversized bool;closes *atomic.Int32;item types.Value;failure error}
func(source *pipeSource)Next()bool{var data []byte;for{maximum:=1;if source.oversized{maximum=source.info.MaxFrameBytes+1};part,err:=source.reader.Read(maximum);if err!=nil{if err!=io.EOF{source.failure=err};return false};if part[0]=='|'{source.item,source.failure=types.ParseValue(data);return source.failure==nil};data=append(data,part[0]);if len(data)>source.info.MaxFrameBytes{source.failure=errors.New("frame bound exceeded");return false}}}
func(source *pipeSource)Item()types.Value{return source.item}
func(source *pipeSource)Err()error{return source.failure}
func(source *pipeSource)Close()error{if source.closes!=nil{source.closes.Add(1)};return source.reader.Close()}
func TestUnknownStreamProtocolAndBoundedReader(t *testing.T){
 var closes atomic.Int32;codec:=types.NewStreamCodec[types.Value,types.Value](pipeProtocol{closes:&closes},scaleAdapter{delta:10});client,_:=streamHTTPClient(t,sdk.ClientOptions{StreamCodecs:map[string]types.StreamCodec{"application/vnd.frames":codec}});ctx:=context.WithValue(context.Background(),traceKey{},"trace")
 handle:=client.Operations.CustomNumbers().WithOptions(types.RequestOptions{MaxStreamFrameBytes:types.Some(64)}).Stream(ctx);var ids []string;for handle.Next(){ids=append(ids,strconv.FormatFloat(handle.Item().ID,'f',0,64))};handle.Close();if handle.Err()!=nil||strings.Join(ids,",")!="14,15"||closes.Load()!=1{t.Fatal("custom protocol",ids,handle.Err(),closes.Load())}
 missing,_:=streamHTTPClient(t);unregistered:=missing.Operations.CustomNumbers().Stream(ctx);if unregistered.Next()||unregistered.Err()==nil{t.Fatal("unknown protocol accepted")};unregistered.Close()
 sentinel:=errors.New("protocol decode failed");for _,protocol:=range []pipeProtocol{ {oversized:true},{nilSource:true},{failure:sentinel} }{override:=types.NewStreamCodec[types.Value,types.Value](protocol,nil);failed:=client.Operations.CustomNumbers().WithOptions(types.RequestOptions{MaxStreamFrameBytes:types.Some(64),StreamCodec:&override}).Stream(ctx);if failed.Next()||failed.Err()==nil{t.Fatal("protocol failure accepted")};if protocol.failure!=nil&&!errors.Is(failed.Err(),sentinel){t.Fatal("protocol cause lost",failed.Err())};failed.Close()}
}
type trackedStreamBody struct{io.Reader;closes atomic.Int32}
func(body *trackedStreamBody)Close()error{body.closes.Add(1);return nil}
type trackedStreamTransport struct{body *trackedStreamBody}
func(transport trackedStreamTransport)Capabilities()types.TransportCapabilities{return types.TransportCapabilities{ResponseBody:types.Encoded}}
func(transport trackedStreamTransport)Do(request *http.Request,_ types.TransportOptions)(*http.Response,error){return &http.Response{StatusCode:200,Header:http.Header{"Content-Type":{"application/vnd.frames"}},Body:transport.body,Request:request},nil}
func TestProtocolConstructorFailureClosesOriginalBody(t *testing.T){
 body:=&trackedStreamBody{Reader:strings.NewReader("unused")};sentinel:=errors.New("decode constructor failed");codec:=types.NewStreamCodec[types.Value,types.Value](pipeProtocol{failure:sentinel},nil);client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"https://example.test",Transport:trackedStreamTransport{body:body},StreamCodecs:map[string]types.StreamCodec{"application/vnd.frames":codec}});if err!=nil{t.Fatal(err)};handle:=client.Operations.CustomNumbers().Stream(context.Background());if handle.Next()||!errors.Is(handle.Err(),sentinel){t.Fatal("decode failure",handle.Err())};handle.Close();if body.closes.Load()!=1{t.Fatal("original body close count",body.closes.Load())}
}
`
