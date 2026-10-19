package golang

import (
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedTypedInboundCustomCodecs(t *testing.T) {
	const source = `{"openapi":"3.2.0","info":{"title":"Inbound codecs","version":"1"},"paths":{},"webhooks":{"receive":{"post":{"parameters":[{"in":"query","name":"q","required":true,"content":{"application/vnd.example.parameter":{"schema":{"type":"object","required":["n"],"properties":{"n":{"type":"integer"}}}}}}],"requestBody":{"required":true,"content":{"application/vnd.example.body":{"schema":{"type":"object","required":["message"],"properties":{"message":{"type":"string"}}}}}},"responses":{"200":{"description":"Reply","headers":{"X-Custom":{"required":true,"content":{"application/vnd.example.parameter":{"schema":{"type":"object","required":["n"],"properties":{"n":{"type":"integer"}}}}}}},"content":{"application/vnd.example.body":{"schema":{"type":"object","required":["message"],"properties":{"message":{"type":"string"}}}}}}}}}},"security":[{"Key":[]}],"components":{"securitySchemes":{"Key":{"type":"apiKey","in":"header","name":"X-Key"}}}}`
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
	inputBody := ""
	for _, field := range entry.Operation.InputFields {
		if field.Name == "Body" {
			inputBody = field.Type
		}
	}
	consumer := strings.NewReplacer("PUBLIC", entry.PublicName, "INPUTBODY", inputBody, "OUTPUTMODEL", entry.Operation.Responses[0].Model, "HEADERMODEL", entry.Operation.Headers[0].Model, "VARIANT", entry.Operation.Responses[0].Name).Replace(`package consumer_test
import("context";"errors";"fmt";"io";"net/http";"net/http/httptest";"strings";"sync/atomic";"testing";"example.test/consumer/api/models";"example.test/consumer/api/types";"example.test/consumer/api/server/runtime";"example.test/consumer/api/server/webhooks")
type encodedSource struct{opens,closes atomic.Int32;canceled atomic.Bool}
func(source *encodedSource)Metadata()types.BinaryMetadata{return types.BinaryMetadata{ContentType:"application/vnd.example.body",Size:types.Some(int64(len("message=accepted")))}}
func(source *encodedSource)Open(ctx context.Context)(io.ReadCloser,error){source.opens.Add(1);return &encodedReader{Reader:strings.NewReader("message=accepted"),ctx:ctx,source:source},nil}
type encodedReader struct{io.Reader;ctx context.Context;source *encodedSource}
func(reader *encodedReader)Close()error{reader.source.canceled.Store(reader.ctx.Err()!=nil);reader.source.closes.Add(1);return nil}
func TestCustomCodecRolesAndSnapshots(t *testing.T){var auth,decode,encode,parameterDecode,parameterEncode atomic.Int32;source:=new(encodedSource);codecs:=map[string]types.MediaCodec{"application/vnd.example.parameter":{DecodeParameter:func(ctx context.Context,text string,info types.MediaContext)(types.Value,error){parameterDecode.Add(1);if info.Request.Body!=http.NoBody||info.Response!=nil{t.Error("parameter metadata ownership")};if text!="p:2"{return types.ValueFrom(map[string]any{"n":"invalid"})};return types.ValueFrom(map[string]any{"n":2})},EncodeParameter:func(ctx context.Context,value types.Value,info types.MediaContext)(string,error){parameterEncode.Add(1);if info.Request.Body!=http.NoBody||info.Request.Header.Get("X-Stable")!="yes"{t.Error("header metadata was mutated")};n,_:=value.Get("n");number,_:=n.Number();return fmt.Sprintf("p:%.0f",number),nil}},"APPLICATION/VND.EXAMPLE.BODY":{Decode:func(context.Context,*http.Response,types.MediaContext)(types.Value,error){t.Error("client response decoder used for inbound");return types.Value{},errors.New("wrong codec role")},DecodeInbound:func(ctx context.Context,request *http.Request,info types.MediaContext)(types.Value,error){decode.Add(1);if request.Body!=info.Request.Body||info.Response!=nil{t.Error("borrowed inbound metadata body")};request.Header.Set("X-Stable","changed-request");info.Request.Header.Set("X-Stable","changed-metadata");data,err:=io.ReadAll(request.Body);if err!=nil{return types.Value{},err};if string(data)!="message=hello"||info.Request.Trailer.Get("X-Final")!="done"{t.Error(string(data),info.Request.Trailer)};request.Body.Close();return types.ValueFrom(map[string]any{"message":"hello"})},Encode:func(ctx context.Context,value types.Value,info types.MediaContext)(types.BinarySource,error){encode.Add(1);if info.Request.Header.Get("X-Stable")!="yes"||info.Request.Body!=http.NoBody||info.Response!=nil{t.Error("response encode metadata mutated")};message,_:=value.Get("message");if text,_:=message.String();text!="accepted"{t.Error(value)};return source,nil}}};handler,err:=webhooks.NewPUBLICHandler("/",webhooks.Options{Codecs:codecs,Authenticate:func(ctx context.Context,input runtime.InboundRequestContext)(*runtime.InboundRejection,error){auth.Add(1);clone,err:=input.CloneBody(ctx);if err!=nil{return nil,err};defer clone.Close();data,err:=io.ReadAll(clone);if err!=nil||string(data)!="message=hello"{t.Error(string(data),err)};return nil,nil}},func(ctx context.Context,input webhooks.PUBLICInput)(webhooks.PUBLICResponse,error){raw,err:=models.EncodeINPUTBODY(ctx,input.Body);if err!=nil{t.Fatal(err)};message,_:=raw.Get("message");if text,_:=message.String();text!="hello"{t.Error(raw)};logical,_:=types.ValueFrom(map[string]any{"message":"accepted"});output,err:=models.DecodeOUTPUTMODEL(ctx,logical);if err!=nil{return webhooks.PUBLICResponse{},err};logicalHeaders,_:=types.ValueFrom(map[string]any{"X-Custom":map[string]any{"n":3}});headers,err:=models.DecodeHEADERMODEL(ctx,logicalHeaders);if err!=nil{return webhooks.PUBLICResponse{},err};return webhooks.NewPUBLICVARIANTResponse(output,headers),nil});if err!=nil{t.Fatal(err)};for key:=range codecs{codecs[key]=types.MediaCodec{}};server:=httptest.NewServer(handler);defer server.Close();for _,test:=range []struct{query string;status int}{ {"p:2",200},{"invalid",400} }{request,_:=http.NewRequest("POST",server.URL+"/?q="+test.query,strings.NewReader("message=hello"));request.Header.Set("Content-Type","application/vnd.example.body");request.Header.Set("X-Key","secret");request.Header.Set("X-Stable","yes");request.Trailer=http.Header{"X-Final":{"done"}};request.ContentLength=-1;response,err:=server.Client().Do(request);if err!=nil{t.Fatal(err)};data,err:=io.ReadAll(response.Body);response.Body.Close();if err!=nil||response.StatusCode!=test.status{t.Fatal(test,response.StatusCode,string(data),err)};if test.status==200&&(string(data)!="message=accepted"||response.Header.Get("X-Custom")!="p:3"){t.Fatal(response.Header,string(data))}};if auth.Load()!=1||decode.Load()!=1||encode.Load()!=1||parameterDecode.Load()!=2||parameterEncode.Load()!=1||source.opens.Load()!=1||source.closes.Load()!=1||!source.canceled.Load(){t.Fatal(auth.Load(),decode.Load(),encode.Load(),parameterDecode.Load(),parameterEncode.Load(),source.opens.Load(),source.closes.Load(),source.canceled.Load())}}
`)
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": consumer})
}
