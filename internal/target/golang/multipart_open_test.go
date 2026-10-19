package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedOpenMultipartValuesKeepJSONStrict(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Open multipart","version":"1"},"components":{"schemas":{"Envelope":{"type":"object","required":["label"],"properties":{"label":{"type":"string"}}}}},"paths":{
"/open":{"post":{"operationId":"sendOpen","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":true}}},"responses":{"204":{"description":"Done"}}}},
"/envelope":{"post":{"operationId":"sendEnvelope","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"$ref":"#/components/schemas/Envelope"}}}},"responses":{"204":{"description":"Done"}}}},
"/json":{"post":{"operationId":"sendEnvelopeJSON","requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Envelope"}}}},"responses":{"204":{"description":"Done"}}}},
"/reference":{"post":{"operationId":"sendReferencedEnvelope","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["payload"],"properties":{"payload":{"$ref":"#/components/schemas/Envelope"}}}}}},"responses":{"204":{"description":"Done"}}}}
}}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"multipart_open_test.go": multipartOpenConsumer})
}

const multipartOpenConsumer = `package consumer_test
import("context";"encoding/json";"io";"mime";"mime/multipart";"net/http";"net/http/httptest";"strings";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestOpenMultipartAndContainerSnapshots(t *testing.T){
 var calls atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter,request *http.Request){calls.Add(1);_,parameters,err:=mime.ParseMediaType(request.Header.Get("Content-Type"));if err!=nil{t.Error(err);writer.WriteHeader(500);return};reader:=multipart.NewReader(request.Body,parameters["boundary"]);names:=[]string{};values:=[]string{};for{part,err:=reader.NextPart();if err==io.EOF{break};if err!=nil{t.Error(err);break};data,err:=io.ReadAll(part);if err!=nil{t.Error(err)};names=append(names,part.FormName());values=append(values,string(data))};if strings.Join(names,",")!="file,label,values,values"||strings.Join(values,"|")!="binary|label|1|two"{t.Error("open multipart wire",names,values)};writer.WriteHeader(204)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 one,_:=types.ValueFrom(1);two,_:=types.ValueFrom("two");label,_:=types.ValueFrom("label")
 entries:=[]types.MultipartValue{types.MultipartFromValue(one),types.MultipartFromValue(two)};array:=types.MultipartFromArray(entries);entries[0]=types.MultipartFromValue(two)
 fields:=map[string]types.MultipartValue{"file":types.MultipartFromBinary(types.BinaryFromBytes([]byte("binary"))),"label":types.MultipartFromValue(label),"values":array}
 value,err:=types.MultipartFromObject(fields);if err!=nil{t.Fatal(err)};delete(fields,"file")
 file,present:=value.Get("file");if !present{t.Fatal("object aliased its input")};if _,present:=file.Binary();!present{t.Fatal("source getter missing")}
 if _,err:=json.Marshal(value);err==nil{t.Fatal("binary multipart became JSON")};if _,err:=types.ValueFrom(value);err==nil{t.Fatal("binary multipart escaped into Value")}
 prepared,err:=value.WireValue();if err!=nil{t.Fatal(err)};delete(prepared.Slots,"/file");again,err:=value.WireValue();if err!=nil||len(again.Slots)!=1{t.Fatal("wire export mutated value",err)}
 if _,err:=client.Operations.SendOpen(sdk.SendOpenInput{Body:sdk.SendOpenBody(value)}).Do(context.Background());err!=nil{t.Fatal(err)}
 input:=sdk.SendEnvelopeBody{Label:"label",AdditionalProperties:map[string]types.MultipartValue{"file":file,"values":array}}
 if _,err:=client.Operations.SendEnvelope(sdk.SendEnvelopeInput{Body:input}).Do(context.Background());err!=nil{t.Fatal(err)}
 normal:=models.EnvelopeInput{Label:"label",AdditionalProperties:map[string]types.Value{"number":one}}
 if _,err:=models.EncodeEnvelopeInput(context.Background(),normal);err!=nil{t.Fatal("ordinary component changed",err)}
 if calls.Load()!=2{t.Fatal("wrong executions",calls.Load())}
 invalid:=types.MultipartFromBinary(types.BinaryFromString(string([]byte{0xff})));if _,err:=invalid.WireValue();err==nil{t.Fatal("invalid text silently accepted")};if _,err:=types.MultipartFromObject(map[string]types.MultipartValue{"invalid":invalid});err==nil{t.Fatal("invalid child ignored")};if _,err:=types.MultipartFromArray([]types.MultipartValue{invalid}).WireValue();err==nil{t.Fatal("invalid array child ignored")}
 envelope,err:=types.MultipartFromObject(map[string]types.MultipartValue{"label":types.MultipartFromValue(label),"file":file});if err!=nil{t.Fatal(err)};referenced,err:=types.MultipartFromObject(map[string]types.MultipartValue{"payload":envelope});if err!=nil{t.Fatal(err)};wireValue,err:=referenced.WireValue();if err!=nil{t.Fatal(err)}
 if _,err:=models.DecodeMediaSendReferencedEnvelopeBody(context.Background(),wireValue);err==nil{t.Fatal("JSON-only shared component silently dropped a binary source")}
 valid,err:=value.WireValue();if err!=nil{t.Fatal(err)}
 for _,path:=range []string{"missing","/missing","/file/~2"}{modified:=valid;modified.Slots=map[string]types.Binary{path:types.BinaryFromBytes(nil)};if _,err:=models.DecodeMediaSendOpenBody(context.Background(),modified);err==nil{t.Fatal("invalid source path accepted",path)};if _,err:=models.EncodeMediaSendOpenBody(context.Background(),sdk.SendOpenBody(types.MultipartFromWireValue(modified)));err==nil{t.Fatal("invalid source path encoded",path)}}
 overlap:=valid;overlap.Slots=map[string]types.Binary{"":types.BinaryFromBytes(nil),"/file":types.BinaryFromBytes(nil)};if _,err:=models.DecodeMediaSendOpenBody(context.Background(),overlap);err==nil{t.Fatal("overlapping sources accepted")}
 if _,err:=client.Operations.SendOpen(sdk.SendOpenInput{Body:sdk.SendOpenBody(types.MultipartFromBinary(types.BinaryFromBytes([]byte("lost"))))}).Do(context.Background());err==nil{t.Fatal("raw source became an empty multipart body")};if calls.Load()!=2{t.Fatal("invalid source mapping sent HTTP",calls.Load())}
}
`
