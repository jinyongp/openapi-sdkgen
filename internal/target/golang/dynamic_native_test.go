package golang

import (
	"encoding/json"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedRuntimeNativeMedia(t *testing.T) {
	var source map[string]any
	if err := json.Unmarshal([]byte(linkNativeFixture), &source); err != nil {
		t.Fatal(err)
	}
	paths := source["paths"].(map[string]any)
	paths["/events"] = map[string]any{"get": map[string]any{"operationId": "events", "responses": map[string]any{"200": map[string]any{"description": "OK", "content": map[string]any{"application/x-ndjson": map[string]any{"itemSchema": map[string]any{"type": "object", "required": []any{"id"}, "properties": map[string]any{"id": map[string]any{"type": "integer"}}, "additionalProperties": false}}}}}}}
	paths["/open-target"] = map[string]any{"post": map[string]any{"operationId": "openTarget", "requestBody": map[string]any{"required": true, "content": map[string]any{"*/*": map[string]any{"schema": map[string]any{}}}}, "responses": map[string]any{"204": map[string]any{"description": "OK"}}}}
	encoded, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	document, err := compiler.Compile(encoded)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"native_test.go": dynamicNativeConsumer})
}

const dynamicNativeConsumer = `package consumer_test
import("bufio";"bytes";"context";"encoding/json";"io";"net/http";"net/http/httptest";"reflect";"strings";"sync/atomic";"testing";"example.test/consumer/api/operations";"example.test/consumer/api/selective";"example.test/consumer/api/types")
type nativeItems struct{ctx context.Context;index int;closed *atomic.Int32}
func(source *nativeItems)Next()bool{if source.ctx.Err()!=nil{return false};source.index++;return source.index<=2}
func(source *nativeItems)Item()types.MultipartData{value,_:=types.ValueFrom(map[string]any{"id":source.index});return types.MultipartData{Value:value}}
func(source *nativeItems)Err()error{if source.index>2{return nil};return source.ctx.Err()}
func(source *nativeItems)Close()error{source.closed.Add(1);return nil}
type nativeFile struct{opens,closes atomic.Int32}
func(*nativeFile)Metadata()types.BinaryMetadata{return types.BinaryMetadata{Filename:"native.dat",ContentType:"application/octet-stream",Size:types.Some(int64(7))}}
func(source *nativeFile)Open(context.Context)(io.ReadCloser,error){source.opens.Add(1);return &nativeReader{Reader:strings.NewReader("payload"),source:source},nil}
type nativeReader struct{io.Reader;source *nativeFile}
func(reader *nativeReader)Close()error{reader.source.closes.Add(1);return nil}
func TestComputedNativeMedia(t *testing.T){var requests atomic.Int32;var binaries [][]byte;var mediaTypes []string;var streams [][]int;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1);switch r.URL.Path{
 case "/download":w.Header().Set("Content-Type","application/octet-stream");w.Write([]byte("payload"));return
 case "/events":w.Header().Set("Content-Type","application/x-ndjson");io.WriteString(w,"{\"id\":1,\"extra\":\"keep\"}\n{\"id\":2,\"extra\":\"keep\"}\n");return
 case "/target-binary","/open-target":raw,err:=io.ReadAll(r.Body);if err!=nil{t.Error(err)};binaries=append(binaries,raw);if r.URL.Path=="/open-target"{w.WriteHeader(http.StatusNoContent);return}
 case "/target-form":reader,err:=r.MultipartReader();if err!=nil{t.Error(err);break};fields:=map[string]string{};for{part,err:=reader.NextPart();if err==io.EOF{break};if err!=nil{t.Error(err);break};raw,err:=io.ReadAll(part);if err!=nil{t.Error(err)};fields[part.FormName()]=string(raw);part.Close()};if !reflect.DeepEqual(fields,map[string]string{"file":"payload","label":"native"}){t.Error(fields)}
 case "/target-media":mediaTypes=append(mediaTypes,r.Header.Get("Content-Type"));raw,err:=io.ReadAll(r.Body);if err!=nil||string(raw)!="text"{t.Error(string(raw),err)}
 case "/target-stream","/target-combined","/source-combined":scanner:=bufio.NewScanner(r.Body);var ids []int;for scanner.Scan(){var item struct{ID int};if err:=json.Unmarshal(scanner.Bytes(),&item);err!=nil{t.Error(err)};ids=append(ids,item.ID)};if err:=scanner.Err();err!=nil{t.Error(err)};streams=append(streams,ids)
 default:t.Error("unexpected request",r.URL.Path)
 };w.Header().Set("Content-Type","application/json");io.WriteString(w,"{\"ok\":true}")}));defer server.Close()
 refs:=make([]operations.Reference,0);byID:=map[string]operations.Ref[operations.DynamicOperation]{};for _,id:=range []string{"targetBinary","targetForm","targetMedia","targetStream","targetCombined","sourceCombined","openTarget","download","events"}{ref,ok:=selective.LookupOperation(id);if !ok{t.Fatal(id)};refs=append(refs,ref);byID[id]=ref};prepared,err:=selective.Prepare(refs...);if err!=nil{t.Fatal(err)};client,err:=selective.NewClient(selective.ClientOptions{BaseURL:server.URL},prepared);if err!=nil{t.Fatal(err)}
 call:=func(id string,body any)operations.DynamicCall{operation,err:=selective.Bind(client,byID[id]);if err!=nil{t.Fatal(err)};input,err:=types.ValueFrom(map[string]any{"body":body});if err!=nil{t.Fatal(err)};return operation.Call(input)}
 if _,err:=call("targetBinary",[]byte{0,255,13,10}).Do(context.Background());err!=nil{t.Fatal(err)}
 if _,err:=call("targetForm",map[string]any{"label":"native","file":"payload"}).Do(context.Background());err!=nil{t.Fatal("MIME request",err)}
 if _,err:=call("targetMedia",map[string]any{"contentType":"text/plain; charset=utf-8","value":"text"}).Do(context.Background());err!=nil{t.Fatal(err)}
 if _,err:=call("targetStream",[]map[string]any{{"id":1},{"id":2}}).Do(context.Background());err!=nil{t.Fatal(err)}
 if _,err:=call("targetCombined",map[string]any{"contentType":"application/x-ndjson","value":[]map[string]any{{"id":1},{"id":2},{"id":3}}}).Do(context.Background());err!=nil{t.Fatal(err)}
 before:=requests.Load();if _,err:=call("targetCombined",map[string]any{"contentType":"application/x-ndjson","value":[]map[string]any{{"id":1},{"id":2}}}).Do(context.Background());err==nil||requests.Load()!=before{t.Fatal("complete body constraint ignored",err)}
 download,err:=selective.Bind(client,byID["download"]);if err!=nil{t.Fatal(err)};value,err:=download.Call(types.Value{}).Do(context.Background());first,_:=value.At(0);number,_:=first.Number();if err!=nil||value.Len()!=7||number!=112{t.Fatal(value,err)};raw,err:=download.Call(types.Value{}).Raw(context.Background());if err!=nil{t.Fatal(err)};follow:=download.Links.Follow("Upload",raw);if _,err:=follow.Do(context.Background());err!=nil{t.Fatal("native computed Link",err)}
 if len(binaries)!=2||!bytes.Equal(binaries[0],[]byte{0,255,13,10})||string(binaries[1])!="payload"||!reflect.DeepEqual(mediaTypes,[]string{"text/plain; charset=utf-8"})||!reflect.DeepEqual(streams,[][]int{{1,2},{1,2,3}}){t.Fatal(binaries,mediaTypes,streams)}
 events,err:=selective.Bind(client,byID["events"]);if err!=nil||events.CanDo()||!events.CanStream(){t.Fatal("stream capabilities",err)};before=requests.Load();stream:=events.Call(types.Value{}).Stream(context.Background());if requests.Load()!=before{t.Fatal("eager dynamic stream")};defer stream.Close();count:=0;for stream.Next(){count++;extra,_:=stream.Item().Get("extra");if text,_:=extra.String();text!="keep"{t.Fatal("stream raw property lost")}};if stream.Err()!=nil||count!=2{t.Fatal(count,stream.Err())}
 sourceCombined,err:=selective.Bind(client,byID["sourceCombined"]);if err!=nil{t.Fatal(err)};var opens,closes atomic.Int32;factory:=func(ctx context.Context)(types.Source[types.MultipartData],error){opens.Add(1);return &nativeItems{ctx:ctx,closed:&closes},nil};logical,err:=types.ValueFrom(map[string]any{"body":map[string]any{"contentType":"application/x-ndjson","value":map[string]any{}}});if err!=nil{t.Fatal(err)};native:=types.NativeInput{Value:logical,Sources:map[string]types.SourceFactory[types.MultipartData]{"/body/value":factory}};request:=sourceCombined.CallNative(native);native.Sources["/body/value"]=nil;if opens.Load()!=0{t.Fatal("CallNative opened factory")};sourceRaw,err:=request.Raw(context.Background());if err!=nil||opens.Load()!=1||closes.Load()!=1{t.Fatal("native item source",err,opens.Load(),closes.Load())};native.Sources["/body/value"]=factory;followNative:=sourceCombined.Links.Follow("Copy",sourceRaw,operations.DynamicLinkInvocation{SourceNative:types.Some(native)});native.Sources["/body/value"]=nil;if opens.Load()!=1{t.Fatal("native Follow opened factory")};if _,err:=followNative.Do(context.Background());err!=nil||opens.Load()!=2||closes.Load()!=2{t.Fatal("native source Link replay",err,opens.Load(),closes.Load())}
 file:=new(nativeFile);logical,_=types.ValueFrom(map[string]any{"body":map[string]any{"label":"native","file":map[string]any{}}});form,err:=selective.Bind(client,byID["targetForm"]);if err!=nil{t.Fatal(err)};nativeFileInput:=types.NativeInput{Value:logical,Binaries:map[string]types.Binary{"/body/file":types.BinaryFromSource(file)}};fileCall:=form.CallNative(nativeFileInput);nativeFileInput.Binaries["/body/file"]=types.BinaryFromString("mutated");if file.opens.Load()!=0{t.Fatal("native call opened file")};if _,err:=fileCall.Do(context.Background());err!=nil||file.opens.Load()!=1||file.closes.Load()!=1{t.Fatal("native file ownership",err,file.opens.Load(),file.closes.Load())}
 openTarget,err:=selective.Bind(client,byID["openTarget"]);if err!=nil{t.Fatal(err)};logical,_=types.ValueFrom(map[string]any{"body":map[string]any{"contentType":"application/octet-stream","value":map[string]any{}}});if _,err:=openTarget.CallNative(types.NativeInput{Value:logical,Binaries:map[string]types.Binary{"/body/value":types.BinaryFromBytes([]byte{0,255,13,10})}}).Do(context.Background());err!=nil||!bytes.Equal(binaries[len(binaries)-1],[]byte{0,255,13,10}){t.Fatal("open native binary",err,binaries)}
 before=requests.Load();if _,err:=form.CallNative(types.NativeInput{Value:logical,Binaries:map[string]types.Binary{"/body/missing":types.BinaryFromString("bad")}}).Do(context.Background());err==nil||requests.Load()!=before{t.Fatal("invalid native pointer sent request",err)}
 onceLogical,_:=types.ValueFrom(map[string]any{"body":map[string]any{"contentType":"application/x-ndjson","value":map[string]any{}}});var onceClosed atomic.Int32;onceFactory:=types.SourceOnce[types.MultipartData](&nativeItems{ctx:context.Background(),closed:&onceClosed});onceCall:=sourceCombined.CallNative(types.NativeInput{Value:onceLogical,Sources:map[string]types.SourceFactory[types.MultipartData]{"/body/value":onceFactory}});if _,err:=onceCall.Do(context.Background());err!=nil{t.Fatal(err)};if _,err:=onceCall.Do(context.Background());err==nil||onceClosed.Load()!=1{t.Fatal("native one-shot replay",err,onceClosed.Load())}
 canceled,cancel:=context.WithCancel(context.Background());cancel();before=requests.Load();beforeOpened:=opens.Load();if _,err:=sourceCombined.CallNative(types.NativeInput{Value:onceLogical,Sources:map[string]types.SourceFactory[types.MultipartData]{"/body/value":factory}}).Do(canceled);err==nil||requests.Load()!=before||opens.Load()!=beforeOpened{t.Fatal("canceled native call opened source",err)}
 before=requests.Load();conflicting:=sourceCombined.Links.Follow("Copy",sourceRaw,operations.DynamicLinkInvocation{SourceInput:types.Some(onceLogical),SourceNative:types.Some(types.NativeInput{Value:onceLogical})});if _,err:=conflicting.Do(context.Background());err==nil||requests.Load()!=before{t.Fatal("ambiguous native Link input",err)}
}
`
