package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedDTOFieldPresenceAndCodecs(t *testing.T) {
	const input = `{"openapi":"3.1.0","info":{"title":"Models","version":"1"},"paths":{"/model":{"get":{"operationId":"getModel","responses":{"200":{"description":"Model","content":{"application/json":{"schema":{"type":"object","required":["requiredPlain","requiredNullable","items"],"properties":{"requiredPlain":{"type":"boolean"},"optionalPlain":{"type":"boolean"},"requiredNullable":{"type":["boolean","null"]},"optionalNullable":{"type":["boolean","null"]},"items":{"type":"array","items":{"type":"string"}},"count":{"type":"integer","minimum":0},"details":{"type":"object","required":["label"],"properties":{"label":{"type":"string","minLength":1}}}}}}}}}}}}}`
	document, err := compiler.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"api/models/contracts_test.go": modelContractsTest})
}

const modelContractsTest = `package models
import("context";"encoding/json";"testing";"example.test/consumer/api/types")
func TestPresenceAndSnapshots(t *testing.T){
 ctx:=context.Background()
 value:=GetModelOutput{RequiredNullable:types.Null[bool](),Items:nil}
 encoded,err:=EncodeGetModelOutput(ctx,value);if err!=nil{t.Fatal(err)}
 want,_:=types.ParseValue([]byte("{\"requiredPlain\":false,\"requiredNullable\":null,\"items\":[]}"))
 if !encoded.Equal(want){data,_:=encoded.MarshalJSON();t.Fatalf("zero field contracts: %s",data)}
 value.OptionalPlain=types.Some(false)
 value.OptionalNullable=types.Some(types.NonNull(false))
 value.Count=types.Some(float64(0))
 encoded,err=types.ValueFrom(value);if err!=nil{t.Fatal(err)}
 for _,key:=range []string{"optionalPlain","optionalNullable","count"}{if _,ok:=encoded.Get(key);!ok{t.Errorf("dropped present zero field %s",key)}}
 value.OptionalNullable=types.Some(types.Null[bool]())
 encoded,err=EncodeGetModelOutput(ctx,value);if err!=nil{t.Fatal(err)}
 optional,_:=encoded.Get("optionalNullable");if optional.Kind()!=types.ValueNull{t.Fatal("lost explicit null")}
 for _,input:=range []string{"{}","{\"RequiredPlain\":true,\"requiredNullable\":null,\"items\":[]}","{\"requiredPlain\":null,\"requiredNullable\":null,\"items\":[]}"}{
  parsed,_:=types.ParseValue([]byte(input));if _,err:=DecodeGetModelOutput(ctx,parsed);err==nil{t.Errorf("accepted invalid fields %s",input)}
 }
 parsed,err:=types.ParseValue([]byte("{\"requiredPlain\":false,\"requiredNullable\":true,\"items\":[\"a\"],\"extra\":{\"x\":1},\"details\":{\"label\":\"ok\"}}"));if err!=nil{t.Fatal(err)}
 first,err:=DecodeGetModelOutput(ctx,parsed);if err!=nil{t.Fatal(err)}
 second,err:=DecodeGetModelOutput(ctx,parsed);if err!=nil{t.Fatal(err)}
 first.Items[0]="changed";first.AdditionalProperties["other"]=types.Value{}
 if second.Items[0]!="a"{t.Fatal("decoded slices share mutable storage")}
 if _,ok:=second.AdditionalProperties["other"];ok{t.Fatal("decoded maps share mutable storage")}
 first.AdditionalProperties["optionalPlain"]=types.Value{}
 if _,err:=EncodeGetModelOutput(ctx,first);err==nil{t.Fatal("allowed additional property to collide with an absent declared field")}
 details,ok:=second.Details.Get();if !ok{t.Fatal("lost details")}
 details.Label=""
 if _,err:=json.Marshal(details);err==nil{t.Fatal("nested DTO skipped its own schema constraints")}
 details.Label=string([]byte{0xff})
 if _,err:=json.Marshal(details);err==nil{t.Fatal("nested DTO accepted a malformed Go string")}
 value.AdditionalProperties=map[string]types.Value{string([]byte{0xff}):types.Value{}}
 if _,err:=EncodeGetModelOutput(ctx,value);err==nil{t.Fatal("accepted malformed additional-property key")}
 pair:=string([]byte{0xed,0xa0,0xbd,0xed,0xb8,0x80})
 value.AdditionalProperties=map[string]types.Value{pair:types.Value{},"😀":types.Value{}}
 if _,err:=EncodeGetModelOutput(ctx,value);err==nil{t.Fatal("additional-property key normalization lost a collision")}
 second.Count=types.Some(float64(-1))
 if _,err:=json.Marshal(second);err==nil{t.Fatal("DTO encoding skipped validation")}
 canceled,cancel:=context.WithCancel(ctx);cancel()
 if _,err:=EncodeGetModelOutput(canceled,value);err==nil{t.Fatal("encoding ignored cancellation")}
}
`
