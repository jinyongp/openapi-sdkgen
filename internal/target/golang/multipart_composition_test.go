package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedMultipartUnionTupleAndNullableParts(t *testing.T) {
	document, err := compiler.Compile([]byte(`{"openapi":"3.2.0","info":{"title":"Multipart composition","version":"1"},"paths":{
 "/combine":{"post":{"operationId":"combine","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","anyOf":[{"type":"object","required":["file"],"properties":{"file":{"type":"string","format":"binary"}}},{"type":"object","required":["enabled"],"properties":{"enabled":{"type":"boolean"}}}]}}}},"responses":{"204":{"description":"Done"}}}},
 "/tuple":{"post":{"operationId":"sendTuple","requestBody":{"required":true,"content":{"multipart/mixed":{"schema":{"type":"array","minItems":2,"prefixItems":[{"type":"string","format":"binary"},{"type":"string"}],"items":false}}}},"responses":{"204":{"description":"Done"}}}},
 "/unique":{"post":{"operationId":"sendUnique","requestBody":{"required":true,"content":{"multipart/form-data":{"schema":{"type":"object","required":["files"],"properties":{"files":{"type":"array","uniqueItems":true,"items":{"type":["string","null"],"format":"binary"}}}}}}},"responses":{"204":{"description":"Done"}}}}
 }}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"multipart_composition_test.go": multipartCompositionConsumer})
	testPartitionedModule(t, plan, map[string]string{"multipart_composition_test.go": multipartCompositionConsumer})
}

const multipartCompositionConsumer = `package consumer_test
import("context";"encoding/json";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestMultipartCompoundViewsPreserveSources(t *testing.T){
 binary:=types.BinaryFromBytes([]byte{1,2,3})
 enabled,_:=types.ValueFrom(true)
 original,err:=models.CombineBodyFromAnyOf1(models.CombineBodyAnyOf1{File:binary,AdditionalProperties:map[string]types.MultipartValue{"enabled":types.MultipartFromValue(enabled)}});if err!=nil{t.Fatal(err)}
 file,ok:=original.AsAnyOf1();if !ok{t.Fatal("binary branch lost")};if size,present:=file.File.Metadata().Size.Get();!present||size!=3{t.Fatal("binary size lost")}
 changed,err:=original.WithAnyOf2(models.CombineBodyAnyOf2{Enabled:false});if err!=nil{t.Fatal(err)}
 oldView,ok:=original.AsAnyOf2();if !ok||!oldView.Enabled{t.Fatal("With mutated original")}
 newView,ok:=changed.AsAnyOf2();if !ok||newView.Enabled{t.Fatal("With did not update branch")}
 if _,ok:=changed.AsAnyOf1();!ok{t.Fatal("With dropped unrelated binary branch")}
 prepared,err:=models.EncodeMediaCombineBody(context.Background(),changed);if err!=nil||len(prepared.Slots)!=1{t.Fatal("union source slots",err,len(prepared.Slots))}
 if _,err:=json.Marshal(changed);err==nil{t.Fatal("binary union became JSON")}
 tuple:=sdk.SendTupleBody{Item0:binary,Item1:"text"};parts,err:=models.EncodeMediaSendTupleBody(context.Background(),tuple);if err!=nil{t.Fatal(err)}
 if _,present:=parts.Slots["/0"];!present{t.Fatal("tuple source path lost")}
 decoded,err:=models.DecodeMediaSendTupleBody(context.Background(),parts);if err!=nil||decoded.Item1!="text"{t.Fatal("tuple decode",decoded,err)}
 unique:=sdk.SendUniqueBody{Files:[]types.Nullable[types.Binary]{types.NonNull(binary),types.Null[types.Binary]()}}
 if _,err:=models.EncodeMediaSendUniqueBody(context.Background(),unique);err!=nil{t.Fatal("opaque binary confused with null",err)}
 unique.Files=append(unique.Files,types.Null[types.Binary]());if _,err:=models.EncodeMediaSendUniqueBody(context.Background(),unique);err==nil{t.Fatal("duplicate null ignored")}
}
`
