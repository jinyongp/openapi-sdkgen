package golang

import (
	"fmt"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"strings"
	"testing"
)

// An empty authoring key previously produced an unused sectionBody variable:
// its DTO was encoded but no media encoder consumed it. Reject that contract
// before emission instead of generating a module that cannot compile.
func TestPrepareRejectsEmptyRequestMedia(t *testing.T) {
	for _, required := range []bool{false, true} {
		for _, media := range []string{"", " "} {
			t.Run(fmt.Sprintf("%t/%q", required, media), func(t *testing.T) {
				source := fmt.Sprintf(`{"openapi":"3.0.3","info":{"title":"Test","version":"1"},"paths":{"/items":{"post":{"operationId":"send","requestBody":{"required":%t,"content":{%q:{"schema":{"type":"object"}}}},"responses":{"204":{"description":"Done"}}}}}}`, required, media)
				document, err := compiler.Compile([]byte(source))
				if err != nil {
					t.Fatal(err)
				}
				_, _, err = (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
				if err == nil || !strings.Contains(err.Error(), "non-empty media type or media range") {
					t.Fatal("invalid request media contract accepted", err)
				}
			})
		}
	}
}

// A complete binary body has no field/part model. Previously the multipart
// template called an EncodeMedia function that was never generated for it.
func TestPrepareWholeBinaryMultipart(t *testing.T) {
	addons, _ := generator.NewAddonRegistry(generator.AddonServer)
	options, _ := addons.Resolve([]string{"server"})
	options.Golang = &generator.GoOptions{ImportPath: "example.test/consumer/api"}
	for _, version := range []string{"3.0.3", "3.1.1", "3.2.0"} {
		for _, required := range []bool{false, true} {
			for _, union := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/required=%t/union=%t", version, required, union), func(t *testing.T) {
					binary := `"contentEncoding":"binary"`
					if version == "3.0.3" {
						binary = `"format":"binary"`
					}
					other := ""
					if union {
						other = `,"application/json":{"schema":{"type":"object"}}`
					}
					source := fmt.Sprintf(`{"openapi":%q,"info":{"title":"Test","version":"1"},"paths":{"/items":{"post":{"operationId":"send","requestBody":{"required":%t,"content":{"multipart/form-data":{"schema":{"type":"string",%s}}%s}},"responses":{"204":{"description":"Done"}}}}}}`, version, required, binary, other)
					document, err := compiler.Compile([]byte(source))
					if err != nil {
						t.Fatal(err)
					}
					plan, _, err := (Generator{}).Prepare(document, options)
					if err != nil {
						t.Fatal(err)
					}
					body := "binary"
					if union {
						body = "sdk.NewSendBinaryBody(binary)"
					}
					if !required {
						body = "types.Some(" + body + ")"
					}
					testGeneratedModule(t, plan, map[string]string{"multipart_test.go": strings.ReplaceAll(binaryMultipartConsumer, "BODY", body)})
				})
			}
		}
	}
}

const binaryMultipartConsumer = `package consumer_test
import("bytes";"context";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestWholeMultipart(t *testing.T){
 var requests atomic.Int32
 payload:=[]byte("--fixture-boundary\r\nContent-Disposition: form-data; name=\"upload\"; filename=\"sample.bin\"\r\nContent-Type: application/octet-stream\r\n\r\n\x00\xff\r\n--fixture-boundary--\r\n")
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1);raw,err:=io.ReadAll(r.Body);if err!=nil||!bytes.Equal(raw,payload){t.Error("body changed",raw,err)};media,params,err:=mime.ParseMediaType(r.Header.Get("Content-Type"));if err!=nil||media!="multipart/form-data"{t.Error("media",media,err)};reader:=multipart.NewReader(bytes.NewReader(raw),params["boundary"]);part,err:=reader.NextPart();if err!=nil{t.Error(err);w.WriteHeader(400);return};data,err:=io.ReadAll(part);if err!=nil||part.FormName()!="upload"||part.FileName()!="sample.bin"||!bytes.Equal(data,[]byte{0,255}){t.Error("part",part.Header,data,err)};if _,err:=reader.NextPart();err!=io.EOF{t.Error("unexpected part",err)};w.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 for _,contentType:=range []string{"multipart/form-data; boundary=fixture-boundary","multipart/form-data","multipart/form-data; boundary=bad@boundary","multipart/mixed; boundary=fixture-boundary","application/octet-stream"}{
  binary:=types.BinaryFromBytesWithMetadata(payload,types.BinaryMetadata{ContentType:contentType})
  _,err:=client.Operations.Send(sdk.SendInput{Body:BODY}).Do(context.Background())
  if contentType=="multipart/form-data; boundary=fixture-boundary"{if err!=nil{t.Fatal(err)}}else if err==nil{t.Fatal("invalid framing accepted",contentType)}
 };if requests.Load()!=1{t.Fatal("invalid body reached server",requests.Load())}
}
type multipartSource struct{contentType string;opens,closes atomic.Int32}
func(source *multipartSource)Metadata()types.BinaryMetadata{return types.BinaryMetadata{ContentType:source.contentType}}
func(source *multipartSource)Open(context.Context)(io.ReadCloser,error){source.opens.Add(1);return &multipartReader{ReadCloser:io.NopCloser(bytes.NewReader([]byte("--fixture--\r\n"))),closes:&source.closes},nil}
type multipartReader struct{io.ReadCloser;closes *atomic.Int32}
func(reader *multipartReader)Close()error{reader.closes.Add(1);return reader.ReadCloser.Close()}
func TestMultipartSourceReplay(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){raw,err:=io.ReadAll(r.Body);if err!=nil||string(raw)!="--fixture--\r\n"{t.Error(raw,err)};w.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 source:=&multipartSource{contentType:"multipart/form-data"};binary:=types.BinaryFromSource(source)
 call:=client.Operations.Send(sdk.SendInput{Body:BODY});if source.opens.Load()!=0{t.Fatal("source opened during preparation")}
 if _,err:=call.Do(context.Background());err==nil||source.opens.Load()!=0{t.Fatal("invalid metadata opened source",err,source.opens.Load())}
 source.contentType="multipart/form-data; boundary=fixture\r\n"
 if _,err:=call.Do(context.Background());err==nil||source.opens.Load()!=0{t.Fatal("invalid header opened source",err,source.opens.Load())}
 source.contentType="multipart/form-data; boundary=fixture"
 for count:=int32(1);count<=2;count++{if _,err:=call.Do(context.Background());err!=nil{t.Fatal(err)};if source.opens.Load()!=count||source.closes.Load()!=count{t.Fatal("source lifetime",source.opens.Load(),source.closes.Load())}}
}
`
