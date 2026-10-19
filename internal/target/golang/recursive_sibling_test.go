package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedRecursiveReferenceSiblings(t *testing.T) {
	const input = `openapi: 3.1.0
info: {title: Recursive reference siblings, version: "1"}
paths: {}
components:
 schemas:
  Node:
   type: object
   required: [value]
   properties:
    value: {type: integer, minimum: 1}
    next:
     allOf:
      - {$ref: '#/components/schemas/Node'}
      - type: object
        required: [label]
        properties:
         label: {type: string, minLength: 1}
  Duplicate:
   type: object
   required: [value]
   properties:
    value: {type: integer, minimum: 1}
    next:
     allOf:
      - {$ref: '#/components/schemas/Duplicate'}
      - {$ref: '#/components/schemas/Duplicate'}
`
	document, err := compiler.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	plan, findings, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(findings) != 0 {
		t.Fatal(err, findings)
	}
	testGeneratedModule(t, plan, map[string]string{"api/models/recursive_sibling_test.go": recursiveSiblingConsumer})
}

const recursiveSiblingConsumer = `package models
import("context";"testing";"example.test/consumer/api/types")
func TestRecursiveSiblings(t *testing.T){
 ctx:=context.Background()
 duplicateRaw,_:=types.ParseValue([]byte("{\"value\":1,\"next\":{\"value\":2,\"next\":{\"value\":3}}}"))
 duplicate,err:=DecodeDuplicateInput(ctx,duplicateRaw);if err!=nil{t.Fatal(err)}
 duplicateEncoded,err:=EncodeDuplicateInput(ctx,duplicate);if err!=nil||!duplicateEncoded.Equal(duplicateRaw){t.Fatal(duplicateEncoded,err)}

 raw,_:=types.ParseValue([]byte("{\"value\":1,\"next\":{\"value\":2,\"label\":\"a\",\"next\":{\"value\":3,\"label\":\"b\"}}}"))
 item,err:=DecodeNodeInput(ctx,raw);if err!=nil{t.Fatal(err)}
 next,_:=item.Next.Get();child,_:=next.Next.Get();if child==nil||child.Label!="b"||child.Value!=3{t.Fatal(child)}
 encoded,err:=EncodeNodeInput(ctx,item);if err!=nil||!encoded.Equal(raw){t.Fatal(encoded,err)}
 child.Next=types.Some(child);if _,err:=EncodeNodeInput(ctx,item);err==nil{t.Fatal("cyclic sibling DTO accepted")}
 for _,text:=range []string{"{\"value\":1,\"next\":{\"value\":2}}","{\"value\":1,\"next\":{\"value\":0,\"label\":\"a\"}}","{\"value\":1,\"next\":{\"value\":2,\"label\":\"a\",\"next\":{\"value\":3,\"label\":\"\"}}}"}{value,_:=types.ParseValue([]byte(text));if _,err:=DecodeNodeInput(ctx,value);err==nil{t.Fatal("reference or sibling assertion removed",text)}}
}
`
