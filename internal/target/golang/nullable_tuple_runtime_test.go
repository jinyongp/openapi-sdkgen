package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedNullableEnumsTuplesAndRootCodecs(t *testing.T) {
	const input = `openapi: 3.1.0
info: {title: Values, version: "1"}
paths: {}
components:
  schemas:
    MaybeState:
      type: [string, 'null']
      enum: [null, open, closed]
    StrictState:
      type: [string, 'null']
      enum: [open]
    NullOnly:
      type: [string, 'null']
      const: null
    Pair:
      type: array
      minItems: 1
      prefixItems:
        - {type: string}
        - {type: integer}
      items: false
    OpenTuple:
      type: array
      prefixItems:
        - {type: string}
      items: {type: integer}
    Strings:
      type: array
      items: {type: string}
    Text:
      type: string
      minLength: 1
`
	document, err := compiler.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"api/models/nullable_tuple_test.go": nullableTupleContractsTest})
}

const nullableTupleContractsTest = `package models
import("context";"encoding/json";"testing";"example.test/consumer/api/types")
func TestNullableEnums(t *testing.T){
 values:=MaybeStateOutputValues();if len(values)!=3{t.Fatal("nullable enum members missing")}
 if !MaybeStateOutputNull().IsNull(){t.Fatal("nullable factory did not preserve null")}
 open:=MaybeStateOutputStringOpen();item,present:=open.Get();if !present||string(item)!="open"{t.Fatal("non-null factory failed")}
 constructed:=MaybeStateOutputFromNullable(types.NonNull(MaybeStateOutputNonNullStringOpen))
 if !open.Equal(constructed){t.Fatal("nullable enum equality failed")}
 values[1]=MaybeStateOutputNull();if MaybeStateOutputValues()[1].IsNull(){t.Fatal("nullable enum values are mutable")}
 parsed,_:=types.ParseValue([]byte("\"open\""));if !IsMaybeStateOutput(parsed){t.Fatal("nullable enum predicate failed")}
 if _,err:=ParseMaybeStateOutput(parsed);err!=nil{t.Fatal(err)}
 var invalid StrictStateOutput
 if _,err:=json.Marshal(invalid);err==nil{t.Fatal("nullable type bypassed its enum's null exclusion")}
 if _,err:=DecodeStrictStateOutput(context.Background(),types.Value{});err==nil{t.Fatal("null exclusion lost during decode")}
 if len(StrictStateOutputValues())!=1{t.Fatal("null was added to a restricted enum")}
 if !NullOnlyOutputNull().IsNull(){t.Fatal("nullable null const failed")}
 if _,err:=DecodeNullOnlyOutputNonNull(context.Background(),types.Value{});err==nil{t.Fatal("non-null projection admitted null")}
 if _,err:=types.NonNull(types.Value{}).JSONValue();err==nil{t.Fatal("non-null wrapper encoded null")}
}
func TestTuplesAndRootCodecs(t *testing.T){
 pair:=PairOutput{Item0:"x",Item1:types.Some(float64(2))}
 data,err:=json.Marshal(pair);if err!=nil||string(data)!="[\"x\",2]"{t.Fatalf("tuple: %s %v",data,err)}
 pair.Item1=types.Optional[float64]{}
 data,err=json.Marshal(pair);if err!=nil||string(data)!="[\"x\"]"{t.Fatalf("short tuple: %s %v",data,err)}
 tuple:=OpenTupleOutput{Rest:[]float64{1}}
 if _,err:=json.Marshal(tuple);err==nil{t.Fatal("tuple allowed a gap before its rest")}
 tuple.Item0=types.Some("x")
 data,err=json.Marshal(tuple);if err!=nil||string(data)!="[\"x\",1]"{t.Fatalf("tuple rest: %s %v",data,err)}
 var array StringsOutput
 data,err=json.Marshal(array);if err!=nil||string(data)!="[]"{t.Fatalf("nil non-null root array: %s %v",data,err)}
 text:=TextOutput(string([]byte{0xed,0xa0,0x80}))
 data,err=json.Marshal(text);if err!=nil||string(data)!="\"\\ud800\""{t.Fatalf("scalar surrogate: %s %v",data,err)}
 var decoded TextOutput
 if err:=json.Unmarshal(data,&decoded);err!=nil||decoded!=text{t.Fatalf("scalar decoder: %v",err)}
 if _,err:=json.Marshal(TextOutput(""));err==nil{t.Fatal("scalar root codec bypassed validation")}
}
`
