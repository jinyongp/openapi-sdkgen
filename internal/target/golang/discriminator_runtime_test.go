package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedDiscriminatorMappings(t *testing.T) {
	const input = `openapi: 3.2.0
info: {title: Pets, version: "1"}
paths:
  /pet:
    get:
      operationId: getPet
      responses:
        '200':
          description: Pet
          content:
            application/json:
              schema: {$ref: '#/components/schemas/Pet'}
  /other:
    get:
      operationId: getOther
      responses:
        '200':
          description: Other pet
          content:
            application/json:
              schema: {$ref: '#/components/schemas/FallbackPet'}
components:
  schemas:
    Pet:
      oneOf:
        - {$ref: '#/components/schemas/Cat'}
        - {$ref: '#/components/schemas/Dog'}
      discriminator:
        propertyName: kind
        mapping:
          cat: '#%2Fcomponents%2Fschemas%2FCat'
          dog: '#/components/schemas/Dog'
    Cat:
      type: object
      required: [kind, lives]
      properties:
        kind: {const: cat}
        lives: {type: integer}
    Dog:
      type: object
      required: [kind, barks]
      properties:
        kind: {const: dog}
        barks: {type: boolean}
    FallbackPet:
      anyOf:
        - {$ref: '#/components/schemas/Cat'}
        - {$ref: '#/components/schemas/OtherPet'}
      discriminator: {propertyName: kind, defaultMapping: OtherPet}
    OtherPet:
      type: object
      required: [display_name]
      properties:
        display_name: {type: string}
`
	document, err := compiler.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	const source = `package models
import("context";"testing";"example.test/consumer/api/types")
func TestMappings(t *testing.T){
 parsed,_:=types.ParseValue([]byte("{\"kind\":\"cat\",\"lives\":9}"))
 value,err:=DecodeGetPetOutput(context.Background(),parsed);if err!=nil{t.Fatal(err)}
 cat,ok:=value.AsMappedCat();if !ok||cat.Lives!=9{t.Fatal("mapped cat projection failed")}
 if _,ok:=value.AsMappedDog();ok{t.Fatal("selected wrong mapping")}
 if _,ok:=value.AsCatOutput();!ok{t.Fatal("reference alias lost typed union methods")}
 invalid,_:=types.ParseValue([]byte("{\"kind\":\"cat\",\"lives\":false}"))
 if _,err:=DecodeGetPetOutput(context.Background(),invalid);err==nil{t.Fatal("discriminator bypassed oneOf constraints")}
 fallback,_:=types.ParseValue([]byte("{\"kind\":\"unknown\",\"display_name\":\"mystery\"}"))
 other,err:=DecodeGetOtherOutput(context.Background(),fallback);if err!=nil{t.Fatal(err)}
 branch,ok:=other.AsDefaultMapping();if !ok||branch.DisplayName!="mystery"{t.Fatal("default mapping projection failed")}
 next,err:=PetOutputFromMappedDog(DogOutput{Kind:"dog",Barks:true});if err!=nil{t.Fatal(err)}
 if _,ok:=next.AsMappedDog();!ok{t.Fatal("mapped constructor failed")}
}
`
	testGeneratedModule(t, plan, map[string]string{"api/models/discriminator_test.go": source})
}
