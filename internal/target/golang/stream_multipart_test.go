package golang

import "testing"

func TestGeneratedSequentialMultipartProtocol(t *testing.T) {
	testGeneratedModule(t, streamRuntimePlan(t, "stream-json", "stream-sse", "stream-multipart"), map[string]string{"api/internal/stream/protocol_test.go": streamProtocolConsumer, "api/internal/stream/multipart_test.go": streamMultipartConsumer})
}

const streamMultipartConsumer = `package stream
import("context";"errors";"io";"net/http";"strings";"sync/atomic";"testing";"example.test/consumer/api/types")
func multipartFrames(t *testing.T,text string,step,maximum int)([]types.MultipartPart,error,int32){t.Helper();body:=&chunkBody{data:[]byte(text),step:step};reader,_:=NewReader(context.Background(),body,maximum);source,err:=MultipartProtocol{}.Decode(context.Background(),reader,types.StreamContext{ContentType:"multipart/mixed; boundary=known",MaxFrameBytes:maximum});if err!=nil{t.Fatal(err)};if body.reads.Load()!=0{t.Fatal("multipart read eagerly")};var parts []types.MultipartPart;for source.Next(){parts=append(parts,source.Item())};err=source.Err();source.Close();reader.Close();return parts,err,body.closes.Load()}
func TestMultipartFramesAcrossBoundaries(t *testing.T){
 input:="preamble\r\n--known\r\nContent-Type: application/json\r\nX-Count: 1\r\nX-Count: 2\r\n\r\n{\"id\":1}\r\n--known\r\nContent-Type: text/plain\r\n\r\né😀\r\n--known--\r\nepilogue"
 for _,step:=range []int{1,2,7,4096}{parts,err,closes:=multipartFrames(t,input,step,64);if err!=nil||len(parts)!=2||closes!=1{t.Fatal("frames",step,len(parts),err,closes)};if string(parts[0].Data)!="{\"id\":1}"||len(parts[0].Headers.Values("X-Count"))!=2||string(parts[1].Data)!="é😀"{t.Fatal("part ownership or headers",parts)}}
 parts,err,closes:=multipartFrames(t,"--known--",1,8);if err!=nil||len(parts)!=0||closes!=1{t.Fatal("empty multipart",parts,err,closes)}
}
func TestMultipartFrameFailuresAndBounds(t *testing.T){
 cases:=[]string{"--knownXX","--known\r\nBadHeader\r\n\r\nx\r\n--known--","--known\r\nX: one\r\n\r\nx","--known\r\nX: one\r\n\r\nx\r\n--knownXX","--known\r\nX: "+strings.Repeat("h",8193)+"\r\n\r\nx\r\n--known--","--known\r\nX: one\r\n\r\n"+strings.Repeat("x",9)+"\r\n--known--"}
 for _,input:=range cases{_,err,closes:=multipartFrames(t,input,1,8);var failure *types.StreamFrameError;if !errors.As(err,&failure)||closes!=1{t.Fatal("frame error/close",err,closes)}}
 parts,err,_:=multipartFrames(t,"--known\r\nX: one\r\n\r\n12345678\r\n--known--",1,8);if err!=nil||len(parts)!=1{t.Fatal("exact multipart bound",parts,err)}
}
type partSource struct{parts []types.MultipartPart;index int;next,closes atomic.Int32}
func(source *partSource)Next()bool{source.next.Add(1);if source.index==len(source.parts){return false};source.index++;return true}
func(source *partSource)Item()types.MultipartPart{return source.parts[source.index-1]}
func(*partSource)Err()error{return nil}
func(source *partSource)Close()error{source.closes.Add(1);return nil}
func TestMultipartEncoderIsLazyAndUsesMatchingBoundaries(t *testing.T){
 for _,empty:=range []bool{false,true}{source:=&partSource{};if !empty{source.parts=[]types.MultipartPart{ {Headers:http.Header{"Content-Type":{"application/json"}},Data:[]byte("{\"n\":1}")},{Headers:http.Header{"Content-Type":{"text/plain"}},Data:[]byte("done")} }};reader,err:=MultipartProtocol{}.Encode(context.Background(),source,types.StreamContext{ContentType:"multipart/mixed; boundary=known",MaxFrameBytes:64});if err!=nil{t.Fatal(err)};if source.next.Load()!=0{t.Fatal("multipart pulled eagerly")};data,err:=io.ReadAll(reader);reader.Close();if err!=nil||source.closes.Load()!=1{t.Fatal("encode close",err,source.closes.Load())};parts,err,_:=multipartFrames(t,string(data),1,64);if err!=nil||len(parts)!=len(source.parts){t.Fatal("roundtrip",string(data),parts,err)}}
 contentType,err:=MultipartContentType("multipart/mixed");if err!=nil||!strings.Contains(contentType,"boundary="){t.Fatal("boundary selection",contentType,err)}
}
`
