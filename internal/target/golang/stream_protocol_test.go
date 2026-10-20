package golang

import "testing"

func TestGeneratedNativeJSONAndSSEProtocols(t *testing.T) {
	testGeneratedModule(t, streamRuntimePlan(t, "stream-json", "stream-sse"), map[string]string{"api/internal/stream/protocol_test.go": streamProtocolConsumer})
}

const streamProtocolConsumer = `package stream
import("context";"errors";"io";"strings";"sync/atomic";"testing";"example.test/consumer/api/types")
type chunkBody struct{data []byte;step int;reads,closes atomic.Int32}
func(body *chunkBody)Read(buffer []byte)(int,error){body.reads.Add(1);if len(body.data)==0{return 0,io.EOF};if len(buffer)>body.step{buffer=buffer[:body.step]};count:=copy(buffer,body.data);body.data=body.data[count:];return count,nil}
func(body *chunkBody)Close()error{body.closes.Add(1);return nil}
func parseFrame(t *testing.T,text string)types.Value{t.Helper();value,err:=types.ParseValue([]byte(text));if err!=nil{t.Fatal(err)};return value}
func decodeFrames(t *testing.T,protocol types.StreamProtocol[types.Value],text string,step,maximum int)([]types.Value,error,int32){t.Helper();body:=&chunkBody{data:[]byte(text),step:step};reader,err:=NewReader(context.Background(),body,maximum);if err!=nil{t.Fatal(err)};source,err:=protocol.Decode(context.Background(),reader,types.StreamContext{MaxFrameBytes:maximum});if err!=nil{t.Fatal(err)};if body.reads.Load()!=0{t.Fatal("decode read eagerly")};var values []types.Value;for source.Next(){values=append(values,source.Item())};err=source.Err();source.Close();reader.Close();return values,err,body.closes.Load()}
func TestJSONFramingAcrossChunks(t *testing.T){
 for _,step:=range []int{1,2,7,4096}{for _,sequence:=range []bool{false,true}{text:="\xef\xbb\xbf{\"word\":\"é😀\"}\r\n\n2\n 3 ";if sequence{text="\xef\xbb\xbf\x1e{\"word\":\"é😀\"}\n\x1e2\n\x1e 3 "};values,err,closes:=decodeFrames(t,JSONProtocol{Sequence:sequence},text,step,128);if err!=nil||len(values)!=3||closes!=1{t.Fatal("frames",sequence,step,values,err,closes)};if !values[0].Equal(parseFrame(t,"{\"word\":\"é😀\"}"))||!values[1].Equal(parseFrame(t,"2"))||!values[2].Equal(parseFrame(t,"3")){t.Fatal("JSON values",values)}}}
 values,err,_:=decodeFrames(t,JSONProtocol{},"\"\xff\"\n",1,8);if err!=nil||len(values)!=1||!values[0].Equal(parseFrame(t,"\"�\"")){t.Fatal("UTF-8 replacement",values,err)}
 if _,err,_:=decodeFrames(t,JSONProtocol{},"1\n\xef\xbb\xbf2\n",1,32);err==nil{t.Fatal("non-initial BOM was stripped")}
}
func TestJSONLimitsAndMalformedFrames(t *testing.T){
 for _,sequence:=range []bool{false,true}{for _,size:=range []int{3,4}{text:="\"é\"\n";if sequence{text="\x1e\"é\"\x1e"};values,err,closes:=decodeFrames(t,JSONProtocol{Sequence:sequence},text,1,size);if closes!=1{t.Fatal("close",closes)};if size==4{if err!=nil||len(values)!=1{t.Fatal("exact frame limit",values,err)}}else{var failure *types.StreamFrameError;if !errors.As(err,&failure)||failure.Kind!="limit"{t.Fatal("frame limit",err)}}}}
 if _,err,_:=decodeFrames(t,JSONProtocol{},"{broken}\n",1,64);err==nil{t.Fatal("malformed JSON accepted")}
 if _,err,_:=decodeFrames(t,JSONProtocol{},"\"\xff\"\n",1,4);err==nil{t.Fatal("limit ignored replacement byte expansion")}
}
func TestSSEFieldsStateAndLineEndings(t *testing.T){
 input:="\xef\xbb\xbf: comment\r\nid: first\r\nevent: delta\r\ndata: one\r\ndata: two\r\nretry: 10\r\n\r\ndata: [1]\n\nid:\rdata:\r\rid: bad\x00id\ndata: end\nretry: -2\n\ndata: discarded"
 expected:=[]string{"{\"data\":\"one\\ntwo\",\"event\":\"delta\",\"id\":\"first\",\"retry\":10}","{\"data\":\"[1]\",\"id\":\"first\"}","{\"data\":\"\",\"id\":\"\"}","{\"data\":\"end\",\"id\":\"\"}"}
 for _,step:=range []int{1,2,9,4096}{values,err,closes:=decodeFrames(t,SSEProtocol{},input,step,128);if err!=nil||len(values)!=len(expected)||closes!=1{t.Fatal("SSE",step,len(values),err,closes)};for index,text:=range expected{if !values[index].Equal(parseFrame(t,text)){actual,_:=values[index].MarshalJSON();t.Fatal("event",index,string(actual),text)}}}
 values,err,_:=decodeFrames(t,SSEProtocol{},"data: x\n\n",1,8);if err!=nil||len(values)!=1{t.Fatal("exact SSE event limit",values,err)}
 if _,err,_:=decodeFrames(t,SSEProtocol{},"data: x\r\n\r\n",1,8);err==nil{t.Fatal("CRLF frame bytes ignored")}
}
type valueSource struct{values []types.Value;index int;next,closes atomic.Int32}
func(source *valueSource)Next()bool{source.next.Add(1);if source.index>=len(source.values){return false};source.index++;return true}
func(source *valueSource)Item()types.Value{return source.values[source.index-1]}
func(*valueSource)Err()error{return nil}
func(source *valueSource)Close()error{source.closes.Add(1);return nil}
func TestNativeEncodersPullOneItemAndClose(t *testing.T){
 for _,protocol:=range []types.StreamProtocol[types.Value]{JSONProtocol{},JSONProtocol{Sequence:true},SSEProtocol{}}{source:=&valueSource{values:[]types.Value{parseFrame(t,"{\"data\":\"hello\\nworld\",\"event\":\"delta\",\"id\":\"one\",\"retry\":2}")}};reader,err:=protocol.Encode(context.Background(),source,types.StreamContext{MaxFrameBytes:128});if err!=nil{t.Fatal(err)};if source.next.Load()!=0{t.Fatal("encode pulled eagerly")};body,err:=io.ReadAll(reader);reader.Close();if err!=nil||source.closes.Load()!=1{t.Fatal("encode ownership",err,source.closes.Load())};if _,sse:=protocol.(SSEProtocol);sse&&string(body)!="event: delta\ndata: hello\ndata: world\nid: one\nretry: 2\n\n"{t.Fatal("SSE wire",string(body))}}
 for _,text:=range []string{"{\"data\":\"x\",\"retry\":9007199254740992}","{\"data\":\"x\",\"other\":true}","{\"data\":\"x\",\"id\":\"bad\\nline\"}"}{if _,err:=encodeSSE(parseFrame(t,text),128);err==nil{t.Fatal("invalid SSE encode",text)}}
}
type repeatedBody struct{remaining int;offset int;bytes int64}
func(body *repeatedBody)Read(buffer []byte)(int,error){if body.remaining==0{return 0,io.EOF};count:=0;for count<len(buffer)&&body.remaining>0{record:="{\"n\":1}\n";n:=copy(buffer[count:],record[body.offset:]);body.offset+=n;count+=n;if body.offset==len(record){body.offset=0;body.remaining--}};body.bytes+=int64(count);return count,nil}
func(*repeatedBody)Close()error{return nil}
func TestJSONBufferDoesNotGrowWithItemCount(t *testing.T){
 for _,total:=range []int{1000,100000}{body:=&repeatedBody{remaining:total};reader,err:=NewReader(context.Background(),body,1024);if err!=nil{t.Fatal(err)};value,_:=JSONProtocol{}.Decode(context.Background(),reader,types.StreamContext{MaxFrameBytes:1024});source:=value.(*jsonSource);count:=0;for source.Next(){count++;if count==1&&body.bytes>1024{t.Fatal("unbounded prefetch",body.bytes)};if cap(source.pending)>1024||cap(reader.buffer)>1024{t.Fatal("buffer grew with items",cap(source.pending),cap(reader.buffer))}};if source.Err()!=nil||count!=total{t.Fatal("large stream",total,count,source.Err())}}
}
func TestBoundedReaderRejectsOversizedReads(t *testing.T){body:=&chunkBody{data:[]byte(strings.Repeat("x",16)),step:16};reader,_:=NewReader(context.Background(),body,8);if _,err:=reader.Read(9);err==nil||body.reads.Load()!=0{t.Fatal("oversized read reached body",err)};data,err:=reader.Read(8);if err!=nil||len(data)!=8{t.Fatal("bounded read",len(data),err)};reader.Close();reader.Close();if body.closes.Load()!=1{t.Fatal("body closed twice")}}

// RFC 7464 section 2.4 requires JSON whitespace after non-self-delimiting values.
func TestJSONSequenceTermination(t *testing.T){for _,step:=range []int{1,64}{for _,text:=range []string{"\x1e123","\x1etrue","\x1efalse","\x1enull"}{values,err,closes:=decodeFrames(t,JSONProtocol{Sequence:true},text,step,1024);if err!=nil||len(values)!=0||closes!=1{t.Fatal(text,values,err,closes)}};values,err,_:=decodeFrames(t,JSONProtocol{Sequence:true},"\x1e123\x1e{}",step,1024);if err!=nil||len(values)!=1||!values[0].Equal(parseFrame(t,"{}")){t.Fatal(values,err)};for _,text:=range []string{"123\n","\x1e123\u00a0"}{if _,err,_:=decodeFrames(t,JSONProtocol{Sequence:true},text,step,1024);err==nil{t.Fatal("invalid framing accepted",text)}}}}
`
