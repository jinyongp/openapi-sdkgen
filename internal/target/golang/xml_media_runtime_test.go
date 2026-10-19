package golang

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedXMLHTTPResponses(t *testing.T) {
	for _, media := range []string{"application/xml", "text/xml", "application/problem+xml"} {
		t.Run(media, func(t *testing.T) {
			schema := map[string]any{"type": "object", "required": []string{"id", "name", "tags"}, "xml": map[string]any{"name": "thing", "prefix": "x", "namespace": "urn:test"}, "properties": map[string]any{
				"id":   map[string]any{"type": "integer", "xml": map[string]any{"attribute": true}},
				"name": map[string]any{"type": "string"},
				"tags": map[string]any{"type": "array", "xml": map[string]any{"wrapped": true, "name": "tags"}, "items": map[string]any{"type": "string", "xml": map[string]any{"name": "tag"}}},
			}}
			raw := map[string]any{"openapi": "3.2.0", "info": map[string]any{"title": "XML media", "version": "1"}, "paths": map[string]any{"/xml": map[string]any{"get": map[string]any{"operationId": "getXML", "responses": map[string]any{"200": map[string]any{"description": "XML", "content": map[string]any{media: map[string]any{"schema": schema}}}}}}}}

			paths := raw["paths"].(map[string]any)
			paths["/xml"].(map[string]any)["post"] = map[string]any{"operationId": "sendXML", "requestBody": map[string]any{"required": true, "content": map[string]any{media: map[string]any{"schema": schema}}}, "responses": map[string]any{"204": map[string]any{"description": "Done"}}}
			paths["/multi"] = map[string]any{"post": map[string]any{"operationId": "sendMulti", "requestBody": map[string]any{"required": true, "content": map[string]any{media: map[string]any{"schema": schema}, "application/json": map[string]any{"schema": schema}}}, "responses": map[string]any{"204": map[string]any{"description": "Done"}}}}
			data, err := json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			document, err := compiler.Compile(data)
			if err != nil {
				t.Fatal(err)
			}
			plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
			if err != nil || len(diagnostics) != 0 {
				t.Fatalf("prepare: %v %v", err, diagnostics)
			}
			testGeneratedModule(t, plan, map[string]string{"xml_test.go": "package consumer_test\n" + strings.ReplaceAll(xmlHTTPConsumer, "__XML_MEDIA__", strconv.Quote(media))})
		})
	}
}

const xmlHTTPConsumer = `
import("bytes";"compress/gzip";"context";"encoding/json";"errors";"io";"net/http";"net/http/httptest";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types";"example.test/consumer/api/models")
const xmlMedia=__XML_MEDIA__
func TestXMLHTTP(t *testing.T){
 const body="<x:thing xmlns:x=\"urn:test\" id=\"7\"><name>Hello</name><tags><tag>one</tag><tag>two</tag></tags></x:thing>"
 var corrupt atomic.Bool;var codecCalls atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){if request.Method=="POST"{data,err:=io.ReadAll(request.Body);if err!=nil||string(data)!=body||request.Header.Get("Content-Type")!=xmlMedia{t.Error("XML encoding",string(data),err,request.Header)};writer.WriteHeader(204);return};writer.Header().Set("Content-Type",xmlMedia+"; charset=utf-8");writer.Header().Set("Content-Encoding","gzip");var buffer bytes.Buffer;compressed:=gzip.NewWriter(&buffer);if corrupt.Load(){io.WriteString(compressed,"<broken>")}else{io.WriteString(compressed,body)};compressed.Close();writer.Write(buffer.Bytes())}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,Codecs:map[string]types.MediaCodec{xmlMedia:{Decode:func(context.Context,*http.Response,types.MediaContext)(types.Value,error){codecCalls.Add(1);return types.Value{},errors.New("native XML must take priority")}}}});if err!=nil{t.Fatal(err)}
 result,err:=client.Operations.GetXML().Do(context.Background());if err!=nil{t.Fatal(err)}
 if result.ID!=7||result.Name!="Hello"||len(result.Tags)!=2||result.Tags[1]!="two"{t.Fatal("XML typed projection",result)}
 encoded,err:=json.Marshal(result);if err!=nil||string(encoded)!="{\"id\":7,\"name\":\"Hello\",\"tags\":[\"one\",\"two\"]}"{t.Fatal("XML JSON DTO",string(encoded),err)}
 raw,err:=client.Operations.GetXML().Raw(context.Background());if err!=nil{t.Fatal(err)};if string(raw.Bytes())!=body||raw.Headers.Get("Content-Encoding")!="gzip"||raw.OriginalResponse.Body!=http.NoBody{t.Fatal("XML Raw bytes/headers/lease")}
 if value,ok:=raw.As200XML();!ok||value.ID!=7{t.Fatal("XML Raw accessor",ok,value)}
 if codecCalls.Load()!=0{t.Fatal("native XML invoked host codec")}
 value,err:=types.ParseValue(encoded);if err!=nil{t.Fatal(err)}
 input,err:=models.DecodeSendXMLBody(context.Background(),value);if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendXML(sdk.SendXMLInput{Body:input}).Do(context.Background());err!=nil{t.Fatal(err)}
 multi,err:=models.DecodeSendMultiXMLBody(context.Background(),value);if err!=nil{t.Fatal(err)}
 if _,err:=client.Operations.SendMulti(sdk.SendMultiInput{Body:sdk.NewSendMultiXMLBody(multi)}).Do(context.Background());err!=nil{t.Fatal(err)}
 corrupt.Store(true);_,err=client.Operations.GetXML().Do(context.Background());var failure *types.APIError;if !errors.As(err,&failure)||failure.Code!=types.ResponseDecodeFailed{t.Fatal("malformed XML classification",err)}
}
`
