package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedContentSchemasAndMediaRootPolicy(t *testing.T) {
	const input = `openapi: 3.1.0
info: {title: Content, version: "1"}
paths:
  /root:
    get:
      operationId: getRoot
      responses:
        '200':
          description: Root
          content:
            application/json:
              schema: {$ref: '#/components/schemas/Mismatched'}
  /choice:
    get:
      operationId: getChoice
      responses:
        '200':
          description: Choice
          content:
            application/json:
              schema: {$ref: '#/components/schemas/Choice'}
  /nested:
    get:
      operationId: getNested
      responses:
        '200':
          description: Nested
          content:
            application/json:
              schema:
                type: object
                required: [payload]
                properties:
                  payload: {$ref: '#/components/schemas/Mismatched'}
  /encoded:
    get:
      operationId: getEncoded
      responses:
        '200':
          description: Encoded
          content:
            application/json:
              schema: {$ref: '#/components/schemas/Encoded'}
  /string:
    get:
      operationId: getString
      responses:
        '200':
          description: String
          content:
            application/json:
              schema: {$ref: '#/components/schemas/ContentString'}
components:
  schemas:
    Encoded:
      type: string
      contentEncoding: base64
      contentMediaType: application/json
      contentSchema:
        type: object
        required: [code]
        properties:
          code: {type: string}
    ContentString:
      type: string
      contentMediaType: application/json
      contentSchema: {type: string, minLength: 1, maxLength: 1}
    Mismatched:
      type: string
      contentMediaType: application/xml
      contentSchema: {type: object}
    Choice:
      anyOf:
        - {$ref: '#/components/schemas/Mismatched'}
        - {type: integer}
`
	document, err := compiler.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"api/models/content_test.go": contentContractsTest, "api/internal/schema/transport_test.go": transportJSONTest})
}

const contentContractsTest = `package models
import("context";"encoding/json";"strings";"testing";"example.test/consumer/api/types";"example.test/consumer/api/internal/wire")
func TestEncodedContent(t *testing.T){
 ctx:=context.Background()
 for _,test:=range []struct{text string;valid bool}{
  {"eyJjb2RlIjoiT0sifQ==",true},
  {"eyJjb2RlIjoiT0sifQ",true},
  {" eyJjb2RlIjoiT0sifQ==\n",true},
  {"e30=",false},{"%%%%",false},{"eyJjb2RlIjoxfQ==",false},
 }{
  value,err:=DecodeEncodedOutput(ctx,wire.Text(test.text))
  if (err==nil)!=test.valid{t.Errorf("%q: %v",test.text,err)}
  if err==nil&&string(value)!=test.text{t.Fatal("content schema changed the outer string")}
 }
 inner:="\""+string([]byte{0xed,0xa0,0x80})+"\""
 value,err:=DecodeContentStringOutput(ctx,wire.Text(inner));if err!=nil{t.Fatal(err)}
 encoded,err:=json.Marshal(value);if err!=nil||!strings.Contains(string(encoded),"\\ud800"){t.Fatalf("content parser lost lone surrogate: %s %v",encoded,err)}
}
func TestMediaRootPolicy(t *testing.T){
 ctx:=context.Background()
 root,err:=DecodeGetRootOutput(ctx,wire.Text("raw text"));if err!=nil{t.Fatal(err)}
 if encoded,err:=json.Marshal(root);err!=nil||string(encoded)!="\"raw text\""{t.Fatalf("operation DTO lost its media policy: %s %v",encoded,err)}
 if _,err:=types.ValueFrom(root);err!=nil{t.Fatal("ValueFrom lost the operation's media policy",err)}
 if _,err:=DecodeMismatchedOutput(ctx,wire.Text("raw text"));err==nil{t.Fatal("generic schema unexpectedly inherited operation media policy")}
 nested,_:=types.ParseValue([]byte("{\"payload\":\"raw text\"}"))
 if _,err:=DecodeGetNestedOutput(ctx,nested);err==nil{t.Fatal("root media override weakened nested schema")}
 choice,err:=DecodeGetChoiceOutput(ctx,wire.Text("raw text"));if err!=nil{t.Fatal(err)}
 branch,ok:=choice.AsMismatchedOutput();if !ok{t.Fatal("union branch lost its media-root policy")}
 if _,err:=json.Marshal(branch);err!=nil{t.Fatal("projected DTO lost its media policy",err)}
}
`

const transportJSONTest = `package schema
import("testing";"example.test/consumer/api/internal/wire")
func TestTransportBOM(t *testing.T){
 data:=append([]byte{0xef,0xbb,0xbf},[]byte("{\"ok\":true}")...)
 value,err:=DecodeJSON(data);if err!=nil||value.Kind()!=wire.Object{t.Fatalf("transport BOM: %v",err)}
 if _,err:=wire.ParseText("\ufeff{}");err==nil{t.Fatal("decoded JSON text unexpectedly discarded its own BOM")}
}
`
