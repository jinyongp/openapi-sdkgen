package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedMultipartBinaryDTOProjection(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.1.1","info":{"title":"Multipart projection","version":"1"},
 "components":{"schemas":{"File":{"type":"string","format":"binary","minLength":999},"Upload":{"type":"object","propertyNames":{"type":"string","minLength":1},"required":["file","files","nullable","count"],"properties":{"file":{"$ref":"#/components/schemas/File"},"files":{"type":"array","items":{"$ref":"#/components/schemas/File"}},"nullable":{"type":["string","null"],"format":"binary"},"count":{"type":"integer","minimum":1}}}}},
 "paths":{"/upload":{"post":{"operationId":"sendUpload","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"$ref":"#/components/schemas/Upload"}}}},"responses":{"204":{"description":"Done"}}}},
 "/json":{"post":{"operationId":"sendJSON","requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Upload"}}}},"responses":{"204":{"description":"Done"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"multipart_projection_test.go": multipartProjectionConsumer})
}

const multipartProjectionConsumer = `package consumer_test
import("context";"encoding/json";"errors";"io";"strings";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
type projectionSource struct{opened atomic.Int32}
func(*projectionSource)Metadata()types.BinaryMetadata{return types.BinaryMetadata{Filename:"file.txt",ContentType:"application/octet-stream"}}
func(value *projectionSource)Open(context.Context)(io.ReadCloser,error){value.opened.Add(1);return nil,errors.New("projection must not open sources")}
func TestMultipartProjectionIsSeparateFromJSON(t *testing.T){
 source:=&projectionSource{};binary:=types.BinaryFromSource(source)
 input:=sdk.SendUploadBody{File:binary,Files:[]types.Binary{types.BinaryFromBytes([]byte{1,2,3})},Nullable:types.Null[types.Binary](),Count:1}
 prepared,err:=models.EncodeMediaSendUploadBody(context.Background(),input);if err!=nil{t.Fatal(err)}
 if source.opened.Load()!=0||len(prepared.Slots)!=2{t.Fatal("eager source or wrong slots",source.opened.Load(),len(prepared.Slots))}
 if _,found:=prepared.Slots["/file"];!found{t.Fatal("file path missing")};if _,found:=prepared.Slots["/files/0"];!found{t.Fatal("array path missing")}
 decoded,err:=models.DecodeMediaSendUploadBody(context.Background(),prepared);if err!=nil{t.Fatal(err)}
 if decoded.File.Metadata().Filename!="file.txt"||len(decoded.Files)!=1||!decoded.Nullable.IsNull(){t.Fatal("lost binary DTO",decoded)}
 if _,err:=json.Marshal(input);err==nil{t.Fatal("binary DTO claimed a JSON representation")}
 if _,err:=types.ValueFrom(input);err==nil{t.Fatal("binary DTO escaped into strict JSON Value")}
 input.Count=0;if _,err:=models.EncodeMediaSendUploadBody(context.Background(),input);err==nil{t.Fatal("ordinary schema assertion bypassed")};input.Count=1
 input.File=types.BinaryFromString("short");if _,err:=models.EncodeMediaSendUploadBody(context.Background(),input);err==nil{t.Fatal("raw text ignored its string assertion")}
 input.File=types.BinaryFromString(strings.Repeat("x",999));if _,err:=models.EncodeMediaSendUploadBody(context.Background(),input);err!=nil{t.Fatal("valid raw text",err)}
 value,err:=models.EncodeFileInput(context.Background(),models.FileInput(strings.Repeat("x",999)));if err!=nil{t.Fatal(err)};if _,stringValue:=value.String();!stringValue{t.Fatal("shared JSON component became binary")}
 if source.opened.Load()!=0{t.Fatal("DTO codec consumed host source")}
}
`
