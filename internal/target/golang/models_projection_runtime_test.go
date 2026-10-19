package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedProjectionsRecursiveDTOAndAllOf(t *testing.T) {
	const input = `openapi: 3.1.0
info: {title: Models, version: "1"}
paths: {}
components:
  schemas:
    Account:
      type: object
      required: [id, password, label]
      properties:
        id: {type: string, readOnly: true}
        password: {type: string, writeOnly: true}
        label: {type: string}
    Child:
      type: object
      required: [label]
      properties:
        label: {type: string, minLength: 1}
    Combined:
      allOf:
        - {$ref: '#/components/schemas/Child'}
        - type: object
          required: [count]
          properties:
            count: {type: integer, minimum: 1}
    Container:
      type: object
      required: [child]
      properties:
        child: {$ref: '#/components/schemas/Child'}
    Node:
      type: object
      required: [value]
      properties:
        value: {type: number}
        next: {$ref: '#/components/schemas/Node'}
    Dictionary:
      type: object
      additionalProperties: {type: integer}
`
	document, err := compiler.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"api/models/projections_test.go": projectionContractsTest})
}

const projectionContractsTest = `package models
import("context";"reflect";"testing";"example.test/consumer/api/types")
func TestProjectionContracts(t *testing.T){
 ctx:=context.Background()
 input:=AccountInput{Password:"secret",Label:"name"}
 output:=AccountOutput{ID:"account-1",Label:"name"}
 if _,err:=EncodeAccountInput(ctx,input);err!=nil{t.Fatal(err)}
 if _,err:=EncodeAccountOutput(ctx,output);err!=nil{t.Fatal(err)}
 if _,ok:=reflect.TypeFor[AccountInput]().FieldByName("ID");ok{t.Fatal("readOnly property leaked into input DTO")}
 if _,ok:=reflect.TypeFor[AccountOutput]().FieldByName("Password");ok{t.Fatal("writeOnly property leaked into output DTO")}
 combined:=CombinedOutput{Label:"ok",Count:1}
 if _,err:=EncodeCombinedOutput(ctx,combined);err!=nil{t.Fatal(err)}
 combined.Count=0
 if _,err:=EncodeCombinedOutput(ctx,combined);err==nil{t.Fatal("flattening removed allOf constraints")}
 container:=ContainerOutput{Child:ChildOutput{Label:"ok"}}
 if _,err:=EncodeContainerOutput(ctx,container);err!=nil{t.Fatal(err)}
 dictionary:=DictionaryOutput{AdditionalProperties:map[string]float64{"a":1,"b":2}}
 if _,err:=EncodeDictionaryOutput(ctx,dictionary);err!=nil{t.Fatal(err)}
 dictionary.AdditionalProperties["a"]=1.5
 if _,err:=EncodeDictionaryOutput(ctx,dictionary);err==nil{t.Fatal("typed additional properties bypassed validation")}
 node:=NodeOutput{Value:1}
 node.Next=types.Some(&node)
 if _,err:=EncodeNodeOutput(ctx,node);err==nil{t.Fatal("cyclic DTO encoded")}
 if _,err:=types.ValueFrom(node);err==nil{t.Fatal("ValueFrom ignored DTO cycle")}
 parsed,_:=types.ParseValue([]byte("{\"value\":1,\"next\":{\"value\":2}}"))
 first,err:=DecodeNodeOutput(ctx,parsed);if err!=nil{t.Fatal(err)}
 second,err:=DecodeNodeOutput(ctx,parsed);if err!=nil{t.Fatal(err)}
 firstNext,_:=first.Next.Get();firstNext.Value=99
 secondNext,_:=second.Next.Get();if secondNext.Value!=2{t.Fatal("decoded recursive pointers share storage")}
}
`
