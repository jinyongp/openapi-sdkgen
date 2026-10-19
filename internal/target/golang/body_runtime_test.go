package golang

import (
	"encoding/json"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedFiniteRequestBodies(t *testing.T) {
	paths := make(map[string]any)
	for _, entry := range []struct {
		name, media string
		schema      any
		required    bool
	}{
		{"JSON", "application/json", map[string]any{"type": "object", "required": []string{"title"}, "properties": map[string]any{"title": map[string]any{"type": "string", "minLength": 1}, "readOnly": map[string]any{"type": "string", "readOnly": true}}}, true},
		{"Text", "text/plain", map[string]any{"type": "integer", "minimum": 0}, true},
		{"Binary", "application/octet-stream", map[string]any{"type": "string", "format": "binary"}, true},
		{"Optional", "application/json", map[string]any{"type": "boolean"}, false},
	} {
		paths["/"+entry.name] = map[string]any{"post": map[string]any{"operationId": "send" + entry.name, "requestBody": map[string]any{"required": entry.required, "content": map[string]any{entry.media: map[string]any{"schema": entry.schema}}}, "responses": map[string]any{"204": map[string]any{"description": "No content"}}}}
	}
	paths["/Media"] = map[string]any{"post": map[string]any{"operationId": "sendMedia", "requestBody": map[string]any{"required": true, "content": map[string]any{
		"application/json":         map[string]any{"schema": map[string]any{"type": "object", "required": []string{"title"}, "properties": map[string]any{"title": map[string]any{"type": "string", "minLength": 1}}}},
		"text/plain":               map[string]any{"schema": map[string]any{"type": "integer", "minimum": 0}},
		"application/octet-stream": map[string]any{"schema": map[string]any{"type": "string", "format": "binary"}},
	}}, "responses": map[string]any{"204": map[string]any{"description": "No content"}}}}
	paths["/Wildcard"] = map[string]any{"post": map[string]any{"operationId": "sendWildcard", "requestBody": map[string]any{"required": true, "content": map[string]any{"application/*+json": map[string]any{"schema": map[string]any{"type": "object", "required": []string{"title"}, "properties": map[string]any{"title": map[string]any{"type": "string"}}}}}}, "responses": map[string]any{"204": map[string]any{"description": "No content"}}}}
	encoded, err := json.Marshal(map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Bodies", "version": "1"}, "paths": paths})
	if err != nil {
		t.Fatal(err)
	}
	document, err := compiler.Compile(encoded)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": bodyConsumer})
}

const bodyConsumer = `package consumer_test
import("bytes";"context";"errors";"io";"net/http";"net/http/httptest";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestRequestBodies(t *testing.T){
 var count atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){count.Add(1);data,err:=io.ReadAll(r.Body);if err!=nil&&r.Header.Get("X-Case")!="source-error"{t.Error(err)}
  switch r.URL.Path{
  case "/JSON":if string(data)!="{\"title\":\"Hello\"}"||r.Header.Get("Content-Type")!="application/json"{t.Error("JSON",string(data),r.Header)}
  case "/Text":if string(data)!="3"||r.Header.Get("Content-Type")!="text/plain"{t.Error("Text",string(data),r.Header)}
  case "/Binary":if r.Header.Get("X-Case")!="source-error"&&(!bytes.Equal(data,[]byte{0,255,42})||r.Header.Get("Content-Type")!="application/octet-stream"){t.Error("Binary",data,r.Header)}
   switch r.Header.Get("X-Case"){case "length":if r.ContentLength!=3||r.Header.Get("Content-Length")!="3"{t.Error("native Content-Length ignored",r.ContentLength,r.Header)};case "chunked","trailers":if len(r.TransferEncoding)!=1||r.TransferEncoding[0]!="chunked"||r.ContentLength!=-1{t.Error("native transfer encoding ignored",r.TransferEncoding,r.ContentLength)};if r.Header.Get("X-Case")=="trailers"{if _,present:=r.Trailer["X-Checksum"];!present{t.Error("native trailer declaration ignored",r.Trailer)}}}
  case "/Optional":if len(data)==0{if r.Header.Get("Content-Type")!=""{t.Error("absent body has Content-Type")}}else if string(data)!="false"{t.Error("optional zero body",string(data))}
  case "/Media":switch r.Header.Get("Content-Type"){case "application/json":if string(data)!="{\"title\":\"Hello\"}"{t.Error("selected JSON",string(data))};case "text/plain":if string(data)!="3"{t.Error("selected text",string(data))};case "application/octet-stream":if !bytes.Equal(data,[]byte{0,255,42}){t.Error("selected binary",data)};default:t.Error("unselected body media")}
  case "/Wildcard":if r.Header.Get("Content-Type")!="Application/Problem+JSON; version=1"||string(data)!="{\"title\":\"Hello\"}"{t.Error("wildcard body",r.Header,string(data))}
  };w.WriteHeader(204)
 }));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 for repeat:=0;repeat<2;repeat++{if _,err:=client.Operations.SendJSON(sdk.SendJSONInput{Body:sdk.SendJSONBody{Title:"Hello"}}).Do(context.Background());err!=nil{t.Fatal(err)}}
 if _,err:=client.Operations.SendText(sdk.SendTextInput{Body:3}).Do(context.Background());err!=nil{t.Fatal(err)}
 binary:=types.BinaryFromBytes([]byte{0,255,42});call:=client.Operations.SendBinary(sdk.SendBinaryInput{Body:binary})
 for repeat:=0;repeat<2;repeat++{if _,err:=call.Do(context.Background());err!=nil{t.Fatal(err)}}
 for _,entry:=range []struct{name,key,value string}{{"length","Content-Length","3"},{"chunked","Transfer-Encoding","chunked"},{"trailers","Trailer","X-Checksum"}}{if _,err:=call.WithOptions(sdk.RequestOptions{Headers:http.Header{"X-Case":[]string{entry.name},entry.key:[]string{entry.value}}}).Do(context.Background());err!=nil{t.Fatal("native header",entry.name,err)}}
 unknown:=client.Operations.SendBinary(sdk.SendBinaryInput{Body:types.BinaryFromReader(io.NopCloser(bytes.NewReader([]byte{0,255,42})),types.BinaryMetadata{})})
 if _,err:=unknown.WithOptions(sdk.RequestOptions{Headers:http.Header{"X-Case":[]string{"length"},"Content-Length":[]string{"3"}}}).Do(context.Background());err!=nil{t.Fatal("unknown source length bridge",err)}
 beforeInvalidHeader:=count.Load();for _,headers:=range []http.Header{{"Content-Length":[]string{"4"}},{"Content-Length":[]string{"3"},"Transfer-Encoding":[]string{"chunked"}},{"Transfer-Encoding":[]string{"gzip"}},{"Trailer":[]string{"Content-Length"}}}{if _,err:=call.WithOptions(sdk.RequestOptions{Headers:headers}).Do(context.Background());err==nil{t.Fatal("invalid native header accepted",headers)}};if beforeInvalidHeader!=count.Load(){t.Fatal("invalid native headers sent HTTP")}
 if _,err:=client.Operations.SendOptional().Do(context.Background());err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendOptional(sdk.SendOptionalInput{Body:types.Some(sdk.SendOptionalBody(false))}).Do(context.Background());err!=nil{t.Fatal(err)}
 for _,body:=range []sdk.SendMediaBody{sdk.NewSendMediaJSONBody(sdk.SendMediaJSONBody{Title:"Hello"}),sdk.NewSendMediaTextBody(3),sdk.NewSendMediaBinaryBody(types.BinaryFromBytes([]byte{0,255,42}))}{if _,err:=client.Operations.SendMedia(sdk.SendMediaInput{Body:body}).Do(context.Background());err!=nil{t.Fatal(err)}}
 if _,err:=client.Operations.SendWildcard(sdk.SendWildcardInput{Body:sdk.NewSendWildcardJSONBody("Application/Problem+JSON; version=1",sdk.SendWildcardJSONBody{Title:"Hello"})}).Do(context.Background());err!=nil{t.Fatal(err)}
 beforeWildcard:=count.Load();for _,media:=range []string{"","application/*+json","text/plain"}{if _,err:=client.Operations.SendWildcard(sdk.SendWildcardInput{Body:sdk.NewSendWildcardJSONBody(media,sdk.SendWildcardJSONBody{Title:"Hello"})}).Do(context.Background());err==nil{t.Fatal("invalid wildcard media",media)}};if beforeWildcard!=count.Load(){t.Fatal("invalid wildcard media reached HTTP")}
 unselectedCount:=count.Load();if _,err:=client.Operations.SendMedia(sdk.SendMediaInput{}).Do(context.Background());err==nil{t.Fatal("unselected media accepted")};if unselectedCount!=count.Load(){t.Fatal("unselected media sent HTTP")}
 before:=count.Load();if _,err:=client.Operations.SendJSON(sdk.SendJSONInput{}).Do(context.Background());err==nil{t.Fatal("invalid body accepted")};if before!=count.Load(){t.Fatal("invalid body sent HTTP")}
 reader:=&countedReader{Reader:bytes.NewReader([]byte{0,255,42})}
 oneShot:=client.Operations.SendBinary(sdk.SendBinaryInput{Body:types.BinaryFromReader(reader,types.BinaryMetadata{Size:types.Some(int64(3))})})
 if _,err:=oneShot.Do(context.Background());err!=nil{t.Fatal(err)}
 _,err=oneShot.Do(context.Background());var apiError *sdk.APIError;var detail *sdk.SDKError
 if !errors.As(err,&apiError)||!errors.As(err,&detail)||apiError.Code!=types.RequestEncodeFailed||detail.Code!=types.RequestBodyConsumed{t.Fatal("one-shot failure",err)}
 if reader.closed.Load()!=1{t.Fatal("reader closed more than once",reader.closed.Load())}
 sentinel:=errors.New("source failed");broken:=&brokenReader{failure:sentinel}
 _,err=client.Operations.SendBinary(sdk.SendBinaryInput{Body:types.BinaryFromReader(broken,types.BinaryMetadata{})}).WithOptions(sdk.RequestOptions{Headers:http.Header{"X-Case":[]string{"source-error"}}}).Do(context.Background())
 if !errors.Is(err,sentinel)||!errors.As(err,&apiError)||apiError.Code!=types.RequestEncodeFailed||broken.closed.Load()!=1{t.Fatal("source read failure phase or ownership",err,broken.closed.Load())}
}
type countedReader struct{*bytes.Reader;closed atomic.Int32}
func(reader *countedReader)Close()error{reader.closed.Add(1);return nil}
type brokenReader struct{failure error;closed atomic.Int32}
func(reader *brokenReader)Read([]byte)(int,error){return 0,reader.failure}
func(reader *brokenReader)Close()error{reader.closed.Add(1);return nil}
`
