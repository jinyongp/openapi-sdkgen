package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedStreamResponseMedia(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Response media","version":"1"},"paths":{"/different":{"get":{"operationId":"different","responses":{"200":{"description":"Done","content":{"application/x-ndjson":{"schema":{"type":"array","prefixItems":[{"type":"integer"}],"items":false},"itemSchema":{"type":"string"}}}}}}},"/binary":{"get":{"operationId":"binaryParts","responses":{"200":{"description":"Done","content":{"multipart/mixed":{"schema":{"type":"array","items":true},"itemSchema":true}}}}}},"/string":{"get":{"operationId":"stringParts","responses":{"200":{"description":"Done","content":{"multipart/mixed":{"itemSchema":{"type":"string"}}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"response_media_test.go": streamResponseMediaConsumer})
}

const streamResponseMediaConsumer = `package consumer_test
import("context";"errors";"io";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestCompleteAndIncrementalSchemasAreIndependent(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/x-ndjson");io.WriteString(w,"7\n")}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 value,err:=client.Operations.Different().Do(context.Background());if err!=nil{t.Fatal("complete response incorrectly used itemSchema",err)};_ = value
 stream:=client.Operations.Different().Stream(context.Background());defer stream.Close();if stream.Next(){t.Fatal("incremental response ignored itemSchema")};var failure *sdk.APIError;if !errors.As(stream.Err(),&failure)||failure.Code!=types.ResponseDecodeFailed{t.Fatal(stream.Err())}
}
func TestMultipartBytesArePreservedAndCopied(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","multipart/mixed; boundary=b");io.WriteString(w,"--b\r\nContent-Type: application/octet-stream\r\n\r\nabc\r\n--b--\r\n")}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 stream:=client.Operations.BinaryParts().Stream(context.Background());defer stream.Close();if !stream.Next(){t.Fatal(stream.Err())};item:=stream.Item();data,present:=item.AsBinary();if !present||string(data)!="abc"{t.Fatal("lost binary",data,present)};data[0]='x';again,_:=item.AsBinary();if string(again)!="abc"{t.Fatal("mutable response bytes",again)};if _,present:=item.As200Stream();present{t.Fatal("JSON view silently dropped binary")};if stream.Next()||stream.Err()!=nil{t.Fatal(stream.Err())}
 complete,err:=client.Operations.BinaryParts().Do(context.Background());if err!=nil{t.Fatal(err)};media,present:=complete.AsMultipart();if !present||media.Len()!=1{t.Fatal("complete multipart",media,present)};part,present:=media.At(0);binary,present:=part.Binary();data,buffered:=binary.Bytes();if !present||!buffered||string(data)!="abc"{t.Fatal("complete binary",data,present,buffered)}
 invalid:=client.Operations.StringParts().Stream(context.Background());defer invalid.Close();if invalid.Next()||invalid.Err()==nil{t.Fatal("binary accepted as string",invalid.Err())}
}
`

func TestGeneratedMultipartResponsePartContracts(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Part contracts","version":"1"},"paths":{"/parts":{"get":{"operationId":"parts","responses":{"200":{"description":"Done","content":{"multipart/mixed":{"schema":{"type":"array","prefixItems":[{"type":"integer"},{"type":"string"},{"type":"object","xml":{"name":"record"},"required":["id"],"properties":{"id":{"type":"integer"}}},{"type":"array","items":true}],"items":false},"itemSchema":true,"prefixEncoding":[{"contentType":"application/json","headers":{"X-First":{"required":true,"schema":{"type":"integer","minimum":1}}}},{"contentType":"text/plain"},{"contentType":"application/xml"},{"contentType":"multipart/mixed; boundary=n","itemEncoding":{"contentType":"application/octet-stream"}}],"itemEncoding":{"contentType":"application/json, text/plain","headers":{"X-Item":{"required":true,"schema":{"type":"string","enum":["yes"]}}}}}}}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"part_contracts_test.go": multipartResponsePartConsumer})
}

const multipartResponsePartConsumer = `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
type partAdapter struct{}
func(partAdapter)Decode(ctx context.Context,source types.Source[types.MultipartPart],info types.StreamContext)(types.Source[types.Value],error){return types.MapSource(ctx,source,func(context.Context,types.MultipartPart)(types.Value,error){return types.ValueFrom(7)}),nil}
func(partAdapter)Encode(context.Context,types.Source[types.Value],types.StreamContext)(types.Source[types.MultipartPart],error){return nil,nil}
func TestPartHeadersDefaultsXMLAndNestedBytes(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","multipart/mixed; boundary=b");switch r.Header.Get("Mode"){
 case "item":io.WriteString(w,"--b\r\nX-Item: yes\r\n\r\n7\r\n--b--\r\n")
 case "missing":io.WriteString(w,"--b\r\nOther: value\r\n\r\n7\r\n--b--\r\n")
 case "invalid":io.WriteString(w,"--b\r\nX-Item: no\r\n\r\n7\r\n--b--\r\n")
 case "complete-invalid":io.WriteString(w,"--b\r\nX-First: 0\r\n\r\n7\r\n--b--\r\n")
 default:io.WriteString(w,"--b\r\nX-First: 1\r\n\r\n7\r\n--b\r\nX: value\r\n\r\nhello\r\n--b\r\nX: value\r\n\r\n<record><id>9</id></record>\r\n--b\r\nContent-Type: multipart/mixed; boundary=n\r\n\r\n--n\r\nX: inner\r\n\r\nabc\r\n--n--\r\n\r\n--b--\r\n")
 }}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 output,err:=client.Operations.Parts().Do(context.Background());if err!=nil{t.Fatal("complete prefix/header/XML/nested",err)};media,present:=output.AsMultipart();if !present||media.Len()!=4{t.Fatal(media,present)};first,_:=media.At(0);jsonValue,err:=first.JSONValue();if err!=nil{t.Fatal(err)};data,_:=jsonValue.MarshalJSON();if string(data)!="7"{t.Fatal("prefix default media",string(data))};xml,_:=media.At(2);record,err:=xml.JSONValue();if err!=nil{t.Fatal(err)};id,_:=record.Get("id");data,_=id.MarshalJSON();if string(data)!="9"{t.Fatal("XML output",string(data))};nested,_:=media.At(3);part,_:=nested.At(0);binary,present:=part.Binary();data,buffered:=binary.Bytes();if !present||!buffered||string(data)!="abc"{t.Fatal("nested binary",data,present,buffered)}
 item:=client.Operations.Parts().WithOptions(types.RequestOptions{Headers:http.Header{"Mode":{"item"}}}).Stream(context.Background());defer item.Close();if !item.Next(){t.Fatal("item default",item.Err())};value,present:=item.Item().AsMultipart();logical,err:=value.JSONValue();data,_=logical.MarshalJSON();if !present||err!=nil||string(data)!="7"{t.Fatal("item encoding",string(data),present,err)};if item.Next()||item.Err()!=nil{t.Fatal(item.Err())}
 codec:=types.NewStreamCodec[types.MultipartPart,types.Value](nil,partAdapter{})
 for _,override:=range []*types.StreamCodec{nil,&codec}{for _,mode:=range []string{"missing","invalid"}{stream:=client.Operations.Parts().WithOptions(types.RequestOptions{Headers:http.Header{"Mode":{mode}},StreamCodec:override}).Stream(context.Background());if stream.Next()||stream.Err()==nil{t.Fatal("item header ignored",mode,stream.Err())};stream.Close()}}
 if _,err:=client.Operations.Parts().WithOptions(types.RequestOptions{Headers:http.Header{"Mode":{"complete-invalid"}}}).Do(context.Background());err==nil{t.Fatal("complete prefix header ignored")}
}
`

func TestGeneratedRecursiveMultipartResponse(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Recursive parts","version":"1"},"paths":{"/recursive":{"get":{"operationId":"recursive","responses":{"200":{"description":"Done","content":{"multipart/mixed":{"itemSchema":{"$ref":"#/components/schemas/Node"}}}}}}}},"components":{"schemas":{"Node":{"anyOf":[{"type":"array","items":{"$ref":"#/components/schemas/Node"}},{"type":"object"}]}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"recursive_media_test.go": recursiveMultipartConsumer})
}

const recursiveMultipartConsumer = `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"strings";"testing";sdk "example.test/consumer/api")
func TestRecursiveNativeMultipartDoesNotExpandGeneration(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","multipart/mixed; boundary=b");io.WriteString(w,"--b\r\nContent-Type: multipart/mixed; boundary=n\r\n\r\n--n\r\nX-Long: "+strings.Repeat("x",9000)+"\r\nContent-Type: application/octet-stream\r\n\r\nabc\r\n--n--\r\n\r\n--b--\r\n")}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};stream:=client.Operations.Recursive().Stream(context.Background());defer stream.Close();if !stream.Next(){t.Fatal(stream.Err())};value,present:=stream.Item().AsMultipart();if !present||value.Len()!=1{t.Fatal("nested value",value,present)};part,_:=value.At(0);binary,present:=part.Binary();data,buffered:=binary.Bytes();if !present||!buffered||string(data)!="abc"{t.Fatal("recursive bytes",data,present,buffered)};if stream.Next()||stream.Err()!=nil{t.Fatal(stream.Err())}
}
`
