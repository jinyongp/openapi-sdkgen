package golang

import "testing"

func TestGeneratedStreamCodecContextOwnership(t *testing.T) {
	testGeneratedModule(t, streamRuntimePlan(t, "types-stream"), map[string]string{"codec_context_test.go": streamCodecContextConsumer})
}

const streamCodecContextConsumer = `package consumer_test
import("context";"errors";"io";"sync";"sync/atomic";"testing";"example.test/consumer/api/types")
type codecProbe struct{ctx context.Context;closes atomic.Int32;cancelled atomic.Bool;failure error;requireCancel bool}
func(probe *codecProbe)close()error{probe.closes.Add(1);if probe.ctx!=nil&&probe.ctx.Err()!=nil{probe.cancelled.Store(true)};if probe.requireCancel&&!probe.cancelled.Load(){return errors.New("codec context was live during cleanup")};return probe.failure}
type codecSource struct{codecProbe}
func(*codecSource)Next()bool{return false}
func(*codecSource)Item()int{return 0}
func(*codecSource)Err()error{return nil}
func(source *codecSource)Close()error{return source.close()}
type codecReader struct{codecProbe;once sync.Once;closeErr error}
func(*codecReader)Read([]byte)(int,error){return 0,io.EOF}
func(reader *codecReader)Close()error{reader.once.Do(func(){reader.closeErr=reader.close()});return reader.closeErr}
type codecBoundReader struct{*codecReader}
func(*codecBoundReader)Read(int)([]byte,error){return nil,io.EOF}
type contextProtocol struct{source *codecSource;reader *codecReader;failure error}
func(protocol contextProtocol)Decode(ctx context.Context,reader types.StreamReader,info types.StreamContext)(types.Source[int],error){protocol.source.ctx=ctx;return protocol.source,protocol.failure}
func(protocol contextProtocol)Encode(ctx context.Context,source types.Source[int],info types.StreamContext)(io.ReadCloser,error){protocol.reader.ctx=ctx;return protocol.reader,protocol.failure}
type contextAdapter struct{source *codecSource;failure error}
func(adapter contextAdapter)Decode(ctx context.Context,source types.Source[int],info types.StreamContext)(types.Source[int],error){adapter.source.ctx=ctx;return adapter.source,adapter.failure}
func(adapter contextAdapter)Encode(ctx context.Context,source types.Source[int],info types.StreamContext)(types.Source[int],error){adapter.source.ctx=ctx;return adapter.source,adapter.failure}
func TestCodecConstructorFailuresCancelBeforeCleanup(t *testing.T){
 for _,phase:=range []string{"protocol-decode","adapter-decode","protocol-encode","adapter-encode"}{t.Run(phase,func(t *testing.T){
  constructor,cleanup:=errors.New("constructor"),errors.New("cleanup");frames:=&codecSource{codecProbe:codecProbe{requireCancel:true}};items:=&codecSource{codecProbe:codecProbe{requireCancel:true}};encoded:=&codecReader{codecProbe:codecProbe{requireCancel:true}};protocol:=contextProtocol{source:frames,reader:encoded};var adapter types.StreamAdapter[int,int];observed:=&frames.codecProbe
  switch phase{case "protocol-decode":protocol.failure=constructor;frames.failure=cleanup;case "adapter-decode":adapter=contextAdapter{source:items,failure:constructor};items.failure=cleanup;observed=&items.codecProbe;case "protocol-encode":protocol.failure=constructor;encoded.failure=cleanup;observed=&encoded.codecProbe;case "adapter-encode":adapter=contextAdapter{source:items,failure:constructor};items.failure=cleanup;observed=&items.codecProbe}
  codec:=types.NewStreamCodec[int,int](protocol,adapter);var err error
  if phase=="protocol-decode"||phase=="adapter-decode"{reader:=&codecBoundReader{codecReader:&codecReader{}};_,err=types.DecodeStreamCodec(context.Background(),codec,types.StreamProtocol[int](nil),reader,types.StreamContext{MaxFrameBytes:64},func(_ context.Context,value int)(int,error){return value,nil});if reader.closes.Load()!=1{t.Fatal("input reader close count",reader.closes.Load())}
  }else{input:=&codecSource{};_,err=types.EncodeStreamCodec(context.Background(),codec,types.StreamProtocol[int](nil),input,types.StreamContext{MaxFrameBytes:64},func(_ context.Context,value int)(int,error){return value,nil});if input.closes.Load()!=1{t.Fatal("input source close count",input.closes.Load())}}
  if !errors.Is(err,constructor)||!errors.Is(err,cleanup)||observed.closes.Load()!=1||!observed.cancelled.Load(){t.Fatal("constructor cleanup ownership",err,observed.closes.Load(),observed.cancelled.Load())}
 })}
}
func TestCodecNormalCloseAndEOFCancelOwnedContext(t *testing.T){
 for _,adapterEnabled:=range []bool{false,true}{frames:=&codecSource{codecProbe:codecProbe{requireCancel:true}};items:=&codecSource{codecProbe:codecProbe{requireCancel:true}};protocol:=contextProtocol{source:frames,reader:&codecReader{codecProbe:codecProbe{requireCancel:true}}};var adapter types.StreamAdapter[int,int];if adapterEnabled{adapter=contextAdapter{source:items}};codec:=types.NewStreamCodec[int,int](protocol,adapter)
  reader:=&codecBoundReader{codecReader:&codecReader{}};source,err:=types.DecodeStreamCodec(context.Background(),codec,types.StreamProtocol[int](nil),reader,types.StreamContext{MaxFrameBytes:64},func(_ context.Context,value int)(int,error){return value,nil});if err!=nil{t.Fatal(err)};if source.Next()||source.Err()!=nil{t.Fatal("EOF cleanup",source.Err())};if err:=source.Close();err!=nil{t.Fatal(err)};if frames.closes.Load()!=1||!frames.cancelled.Load()||reader.closes.Load()!=1{t.Fatal("frame lifetime",frames.closes.Load(),frames.cancelled.Load(),reader.closes.Load())};if adapterEnabled&&(items.closes.Load()!=1||!items.cancelled.Load()){t.Fatal("adapter lifetime")}
  input:=&codecSource{};encoded,err:=types.EncodeStreamCodec(context.Background(),codec,types.StreamProtocol[int](nil),input,types.StreamContext{MaxFrameBytes:64},func(_ context.Context,value int)(int,error){return value,nil});if err!=nil{t.Fatal(err)};if err:=encoded.Close();err!=nil{t.Fatal(err)};encoded.Close();if input.closes.Load()!=1||protocol.reader.closes.Load()!=1||!protocol.reader.cancelled.Load(){t.Fatal("encoded lifetime",input.closes.Load(),protocol.reader.closes.Load(),protocol.reader.cancelled.Load())}
 }
}
type flushSource struct{ctx context.Context;frames types.Source[int];emitted bool;failure error}
func(source *flushSource)Next()bool{if source.emitted{return false};if source.frames.Next(){source.failure=errors.New("expected empty frame source");return false};if err:=source.frames.Err();err!=nil{source.failure=err;return false};if err:=source.frames.Close();err!=nil{source.failure=err;return false};if err:=source.ctx.Err();err!=nil{source.failure=err;return false};source.emitted=true;return true}
func(*flushSource)Item()int{return 9}
func(source *flushSource)Err()error{return source.failure}
func(source *flushSource)Close()error{if source.ctx.Err()==nil{return errors.New("adapter closed before cancellation")};return source.frames.Close()}
type flushAdapter struct{}
func(flushAdapter)Decode(ctx context.Context,source types.Source[int],info types.StreamContext)(types.Source[int],error){return &flushSource{ctx:ctx,frames:source},nil}
func(flushAdapter)Encode(context.Context,types.Source[int],types.StreamContext)(types.Source[int],error){return nil,nil}
type tailReader struct{ctx context.Context;source types.Source[int];sent bool}
func(reader *tailReader)Read(buffer []byte)(int,error){if len(buffer)==0{return 0,nil};if reader.sent{return 0,io.EOF};if reader.source.Next(){return 0,errors.New("expected empty input")};if err:=reader.source.Err();err!=nil{return 0,err};if err:=reader.source.Close();err!=nil{return 0,err};if err:=reader.ctx.Err();err!=nil{return 0,err};buffer[0]='!';reader.sent=true;return 1,nil}
func(reader *tailReader)Close()error{if reader.ctx.Err()==nil{return errors.New("protocol reader closed before cancellation")};return reader.source.Close()}
type tailProtocol struct{}
func(tailProtocol)Decode(context.Context,types.StreamReader,types.StreamContext)(types.Source[int],error){return nil,nil}
func(tailProtocol)Encode(ctx context.Context,source types.Source[int],info types.StreamContext)(io.ReadCloser,error){return &tailReader{ctx:ctx,source:source},nil}
func TestCodecStagesCanFlushAfterTheirInputEOF(t *testing.T){
 frames:=&codecSource{codecProbe:codecProbe{requireCancel:true}};codec:=types.NewStreamCodec[int,int](contextProtocol{source:frames},flushAdapter{});reader:=&codecBoundReader{codecReader:&codecReader{}};source,err:=types.DecodeStreamCodec(context.Background(),codec,types.StreamProtocol[int](nil),reader,types.StreamContext{MaxFrameBytes:64},func(_ context.Context,value int)(int,error){return value,nil});if err!=nil{t.Fatal(err)};if !source.Next()||source.Item()!=9{t.Fatal("adapter final item canceled by frame EOF",source.Err())};if source.Next()||source.Err()!=nil{t.Fatal("adapter end",source.Err())};source.Close()
 for _,adapt:=range []bool{false,true}{var adapter types.StreamAdapter[int,int];if adapt{adapter=contextAdapter{source:&codecSource{codecProbe:codecProbe{requireCancel:true}}}};codec:=types.NewStreamCodec[int,int](tailProtocol{},adapter);encoded,err:=types.EncodeStreamCodec(context.Background(),codec,types.StreamProtocol[int](nil),&codecSource{},types.StreamContext{MaxFrameBytes:64},func(_ context.Context,value int)(int,error){return value,nil});if err!=nil{t.Fatal(err)};data,err:=io.ReadAll(encoded);closeError:=encoded.Close();if err!=nil||closeError!=nil||string(data)!="!"{t.Fatal("protocol tail canceled by item EOF",adapt,string(data),err,closeError)}}
}
`
