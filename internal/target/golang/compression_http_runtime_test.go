package golang

import (
	"context"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestGeneratedCompressionHTTPHeadersAndRaw(t *testing.T) {
	document, _ := healthPlan(t)
	document.Operations[0].Responses[0].Raw["headers"] = map[string]any{
		"Content-Encoding": map[string]any{"required": true, "schema": map[string]any{"type": "string"}},
		"Content-Length":   map[string]any{"required": true, "schema": map[string]any{"type": "integer", "minimum": 0}},
	}
	original := document.Operations[0]
	binary := original
	binary.OperationID, binary.Path, binary.RouteKey = "getBytes", "/bytes", "GET /bytes"
	binary.Responses = []ir.Response{{Status: "200", Raw: original.Responses[0].Raw, Content: []ir.MediaType{{ContentType: "application/octet-stream", Schema: map[string]any{"type": "string", "format": "binary"}}}}}
	custom := original
	custom.OperationID, custom.Path, custom.RouteKey = "getCustom", "/custom", "GET /custom"
	custom.Responses = []ir.Response{{Status: "200", Raw: original.Responses[0].Raw, Content: []ir.MediaType{{ContentType: "application/example", Schema: original.Responses[0].Content[0].Schema}}}}
	document.Operations = append(document.Operations, binary, custom)
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	root := testGeneratedModule(t, plan, map[string]string{"consumer_test.go": compressionHTTPConsumer, "encoded.json": compressionFixtures(t)})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "test", "-mod=readonly", "./...")
	command.Dir = root
	command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOWORK=off", "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("CGO-disabled generated HTTP consumer: %v\n%s", err, output)
	}
}

const compressionHTTPConsumer = `package consumer_test
import("bytes";"compress/gzip";"context";"encoding/base64";"encoding/json";"errors";"io";"net/http";"net/http/httptest";"os";"strconv";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
type hostDecode func(*http.Request)(*http.Response,error)
func(call hostDecode)RoundTrip(request *http.Request)(*http.Response,error){return call(request)}
func TestHTTPBodiesAndHeadersBelongToHost(t *testing.T){
 payload:=[]byte("{\"ok\":true}");var fixtures map[string]string;data,err:=os.ReadFile("encoded.json");if err!=nil{t.Fatal(err)};if err:=json.Unmarshal(data,&fixtures);err!=nil{t.Fatal(err)}
 for coding,text:=range fixtures{t.Run(coding,func(t *testing.T){
  encoded,err:=base64.StdEncoding.DecodeString(text);if err!=nil{t.Fatal(err)};header:=coding;if header=="raw-deflate"{header="deflate"}
  server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.Header.Get("Accept-Encoding")!=""{t.Error("SDK added compression negotiation")};contentType:="application/json";if r.URL.Path=="/bytes"{contentType="application/octet-stream"};if r.URL.Path=="/custom"{contentType="application/example"};w.Header().Set("Content-Type",contentType);w.Header().Set("Content-Encoding",header);w.Header().Set("Content-Length",strconv.Itoa(len(encoded)));w.Write(encoded)}));defer server.Close()
  host:=&http.Transport{DisableCompression:true};defer host.CloseIdleConnections()
  client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,HTTPClient:&http.Client{Transport:host}});if err!=nil{t.Fatal(err)}
  raw,err:=client.Operations.GetBytes().Raw(context.Background());if err!=nil{t.Fatal(err)}
  if !bytes.Equal(raw.Bytes(),encoded)||raw.Headers.Get("Content-Encoding")!=header||raw.Headers.Get("Content-Length")!=strconv.Itoa(len(encoded)){t.Fatal("changed returned host bytes or headers")}
  typed,ok:=raw.As200Headers();if !ok||typed.ContentEncoding!=header||typed.ContentLength!=float64(len(encoded)){t.Fatal("typed host headers",typed,ok)}
  if coding!="identity"{if _,err:=client.Operations.GetHealth().Raw(context.Background());err==nil{t.Fatal("JSON Raw silently skipped its media decoder")}}
  codecs:=map[string]types.MediaCodec{"application/example":{Decode:func(ctx context.Context,response *http.Response,metadata types.MediaContext)(types.Value,error){data,err:=io.ReadAll(response.Body);if err!=nil{return types.Value{},err};if !bytes.Equal(data,encoded)||response.Header.Get("Content-Encoding")!=header||metadata.Response.Header.Get("Content-Encoding")!=header{t.Fatal("custom codec did not receive host bytes and headers")};return types.ParseValue(payload)}}}
  custom,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,HTTPClient:&http.Client{Transport:host},Codecs:codecs});if err!=nil{t.Fatal(err)}
  value,err:=custom.Operations.GetCustom().Do(context.Background());if err!=nil||!value.Ok{t.Fatal("custom codec result",value,err)}
  hostClient:=&http.Client{Transport:hostDecode(func(r *http.Request)(*http.Response,error){return &http.Response{StatusCode:200,Header:http.Header{"Content-Type":{"application/json"},"Content-Encoding":{header},"Content-Length":{strconv.Itoa(len(encoded))}},Body:io.NopCloser(bytes.NewReader(payload)),Uncompressed:true,ContentLength:-1,Request:r},nil})}
  decoded,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL,HTTPClient:hostClient});if err!=nil{t.Fatal(err)}
  health,err:=decoded.Operations.GetHealth().Do(context.Background());if err!=nil||!health.Ok{t.Fatal("host-decoded typed response rejected",health,err)}
 })}
}
func TestHTTPRemovedRequiredHeadersAreValidated(t *testing.T){
 var encoded bytes.Buffer;writer:=gzip.NewWriter(&encoded);writer.Write([]byte("{\"ok\":true}"));writer.Close()
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");w.Header().Set("Content-Encoding","gzip");w.Write(encoded.Bytes())}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 _,err=client.Operations.GetHealth().Do(context.Background());var apiError *sdk.APIError;if !errors.As(err,&apiError)||apiError.Code!=types.ResponseDecodeFailed{t.Fatal("missing required returned header was not rejected",err)}
}
`
