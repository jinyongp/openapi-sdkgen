package golang

import (
	"encoding/json"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedOpenMediaRepresentations(t *testing.T) {
	object := map[string]any{"type": "object", "required": []string{"names"}, "properties": map[string]any{"names": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}}
	paths := make(map[string]any)
	for _, entry := range []struct {
		name   string
		schema any
	}{{"Open", object}, {"TextOpen", map[string]any{"type": []string{"string", "integer"}}}, {"NumberOpen", map[string]any{"type": "integer"}}} {
		paths["/"+entry.name] = map[string]any{"get": map[string]any{"operationId": "get" + entry.name, "responses": map[string]any{"200": map[string]any{"description": "Open", "content": map[string]any{"*/*": map[string]any{"schema": entry.schema}}}}}}
	}
	paths["/Send"] = map[string]any{"post": map[string]any{"operationId": "sendOpen", "requestBody": map[string]any{"required": true, "content": map[string]any{"*/*": map[string]any{"schema": object}}}, "responses": map[string]any{"204": map[string]any{"description": "No content"}}}}
	encoded, err := json.Marshal(map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Open media", "version": "1"}, "paths": paths})
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
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": openMediaConsumer})
}

const openMediaConsumer = `package consumer_test
import("bytes";"context";"encoding/json";"io";"net/http";"net/http/httptest";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestOpenMedia(t *testing.T){
 var requests atomic.Int32
 binary:=[]byte{0,255,42}
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1)
  if r.Method=="POST"{data,err:=io.ReadAll(r.Body);if err!=nil{t.Error(err)};switch r.Header.Get("Content-Type"){case "application/problem+json":if string(data)!="{\"names\":[\"input\"]}"{t.Error("open JSON input",string(data))};case "application/octet-stream":if !bytes.Equal(data,binary){t.Error("open binary input",data)};default:t.Error("input content type",r.Header)};w.WriteHeader(204);return}
  media,body:="Application/Problem+JSON; version=1",[]byte("{\"names\":[\"original\"]}")
  switch r.URL.Path{case "/TextOpen":media,body="text/plain",[]byte("003");case "/NumberOpen":media,body="text/plain",append([]byte{0xef,0xbb,0xbf},'3')}
  switch r.Header.Get("X-Case"){case "binary":media,body="application/octet-stream",binary;case "custom":media,body="application/custom",[]byte("ignored");case "bad-utf8":media,body="text/plain",[]byte{0xed,0xa0,0x80}}
  w.Header().Set("Content-Type",media);w.Write(body)
 }));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Codecs:map[string]types.MediaCodec{"application/custom":{Decode:func(context.Context,*http.Response,types.MediaContext)(types.Value,error){return types.ParseValue([]byte("{\"names\":[\"decoded\"]}"))}}}});if err!=nil{t.Fatal(err)}
 options:=func(name string)sdk.RequestOptions{return sdk.RequestOptions{Headers:http.Header{"X-Case":[]string{name}}}}
 result,err:=client.Operations.GetOpen().Do(context.Background());if err!=nil{t.Fatal(err)}
 first,ok:=result.AsValue();if !ok||first.Names[0]!="original"||result.ContentType()!="Application/Problem+JSON; version=1"{t.Fatal("open JSON",first,ok)}
 first.Names[0]="changed";again,_:=result.AsValue();if again.Names[0]!="original"{t.Fatal("shared JSON snapshot")}
 if _,ok:=result.AsBinary();ok{t.Fatal("JSON classified as binary")}
 result,err=client.Operations.GetOpen().WithOptions(options("binary")).Do(context.Background());if err!=nil{t.Fatal(err)}
 data,ok:=result.AsBinary();if !ok||!bytes.Equal(data,binary){t.Fatal("open binary",data,ok)};data[0]=1;againBytes,_:=result.AsBinary();if againBytes[0]!=0{t.Fatal("shared binary snapshot")};if _,ok:=result.AsValue();ok{t.Fatal("binary classified as JSON")}
 raw,err:=client.Operations.GetOpen().WithOptions(options("binary")).Raw(context.Background());if err!=nil{t.Fatal(err)};branch,ok:=raw.As200Media();if !ok{t.Fatal("open Raw branch")};data,ok=branch.AsBinary();if !ok||!bytes.Equal(data,binary){t.Fatal("open Raw binary",data,ok)}
 result,err=client.Operations.GetOpen().WithOptions(options("custom")).Do(context.Background());if err!=nil{t.Fatal(err)};custom,ok:=result.AsValue();if !ok||custom.Names[0]!="decoded"{t.Fatal("open custom codec",custom,ok)}
 text,err:=client.Operations.GetTextOpen().Do(context.Background());if err!=nil{t.Fatal(err)};textValue,ok:=text.AsValue();if !ok{t.Fatal("open text")};encoded,err:=json.Marshal(textValue);if err!=nil||string(encoded)!="\"003\""{t.Fatal("text string priority",string(encoded),err)}
 text,err=client.Operations.GetTextOpen().WithOptions(options("bad-utf8")).Do(context.Background());if err!=nil{t.Fatal(err)};textValue,_=text.AsValue();encoded,err=json.Marshal(textValue);if err!=nil||string(encoded)!="\"���\""{t.Fatal("transport UTF-8 replacement",string(encoded),err)}
 number,err:=client.Operations.GetNumberOpen().Do(context.Background());if err!=nil{t.Fatal(err)};numberValue,ok:=number.AsValue();if !ok||numberValue!=3{t.Fatal("BOM text number",numberValue,ok)}
 for _,body:=range []sdk.SendOpenBody{sdk.NewSendOpenMediaBody("application/problem+json",sdk.SendOpenMediaBody{Names:[]string{"input"}}),sdk.NewSendOpenMediaBinaryBody("application/octet-stream",types.BinaryFromBytes(binary))}{if _,err:=client.Operations.SendOpen(sdk.SendOpenInput{Body:body}).Do(context.Background());err!=nil{t.Fatal(err)}}
 before:=requests.Load();if _,err:=client.Operations.SendOpen(sdk.SendOpenInput{Body:sdk.NewSendOpenMediaBinaryBody("application/json",types.BinaryFromBytes(binary))}).Do(context.Background());err==nil{t.Fatal("binary bypassed JSON schema")};if before!=requests.Load(){t.Fatal("invalid binary representation reached HTTP")}
}
`
