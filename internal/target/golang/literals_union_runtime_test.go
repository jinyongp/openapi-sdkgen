package golang

import (
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedLiteralsAndUnionSnapshots(t *testing.T) {
	const input = `{"openapi":"3.1.0","info":{"title":"Literals","version":"1"},"paths":{"/choice":{"get":{"operationId":"getChoice","responses":{"200":{"description":"Choice","content":{"application/json":{"schema":{"oneOf":[{"type":"object","required":["kind","items"],"properties":{"kind":{"const":"a"},"items":{"type":"array","items":{"type":"string"}},"optional":{"type":"string"}}},{"type":"object","required":["kind","count"],"properties":{"kind":{"const":"b"},"count":{"type":"integer"}}}]}}}}}}}},"components":{"schemas":{"Literal":{"enum":[null,"a",1,{"x":1,"items":[true,null]},[1,"a"]]},"State":{"type":"string","enum":["open","closed"]},"Constant":{"const":{"x":1}}}}}`
	document, err := compiler.Compile([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	// The public component-only document also exercises literal models that no
	// operation references. Operation selection is covered by its later work unit.
	const componentInput = `{"openapi":"3.1.0","info":{"title":"Literals","version":"1"},"paths":{},"components":{"schemas":{"Literal":{"enum":[null,"a",1,{"x":1,"items":[true,null]},[1,"a"]]},"State":{"type":"string","enum":["open","closed"]},"Constant":{"const":{"x":1}}}}}`
	components, err := compiler.Compile([]byte(componentInput))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		documentInput bool
		source        string
	}{{true, unionSnapshotTest}, {false, literalSnapshotTest}} {
		source := document
		if !test.documentInput {
			source = components
		}
		plan, diagnostics, err := (Generator{}).Prepare(source, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
		if err != nil || len(diagnostics) != 0 {
			t.Fatalf("prepare: %v %v", err, diagnostics)
		}
		testGeneratedModule(t, plan, map[string]string{"api/models/snapshots_test.go": test.source})
	}
}

const literalSnapshotTest = `package models
import("context";"sync";"testing";"example.test/consumer/api/types")
func TestLiteralFactories(t *testing.T){
 var workers sync.WaitGroup
 for index:=0;index<16;index++{workers.Go(func(){for sample:=0;sample<20;sample++{if !ConstantOutputObject1().Equal(ConstantOutputValues()[0]){t.Error("concurrent literal initialization")}}})};workers.Wait()
 values:=LiteralOutputValues()
 if len(values)!=5{t.Fatal("lost enum members")}
 object:=LiteralOutputObject1()
 if !values[3].Equal(object){t.Fatal("enum factory mismatch")}
 values[3]=LiteralOutputNull()
 fields:=object.Value().Fields();fields["x"]=types.Value{}
 if !LiteralOutputValues()[3].Equal(object){t.Fatal("enum values share mutable state")}
 reordered,_:=types.ParseValue([]byte("{\"items\":[true,null],\"x\":1}"))
 parsed,err:=ParseLiteralOutput(reordered);if err!=nil||!parsed.Equal(object){t.Fatalf("enum structural equality: %v",err)}
 wrong,_:=types.ParseValue([]byte("{\"items\":[null,true],\"x\":1}"))
 if IsLiteralOutput(wrong){t.Fatal("ignored array ordering")}
 numeric,_:=types.ParseValue([]byte("1"));text,_:=types.ParseValue([]byte("\"1\""))
 if !IsLiteralOutput(numeric)||IsLiteralOutput(text){t.Fatal("coerced enum types")}
 if !StateOutputStringOpen.Equal(StateOutput("open")){t.Fatal("scalar enum identity")}
 states:=StateOutputValues();states[0]=StateOutput("invalid")
 if StateOutputValues()[0]!=StateOutputStringOpen{t.Fatal("scalar enum values are shared")}
 if _,err:=EncodeStateOutput(context.Background(),StateOutput("invalid"));err==nil{t.Fatal("invalid scalar enum encoded")}
 if _,err:=types.ValueFrom(StateOutput("invalid"));err==nil{t.Fatal("ValueFrom bypassed named enum validation")}
 if !ConstantOutputObject1().Equal(ConstantOutputValues()[0]){t.Fatal("object const factory mismatch")}
}
`

const unionSnapshotTest = `package models
import("context";"testing";"example.test/consumer/api/types")
func TestUnionSnapshotsAndUpdates(t *testing.T){
 parsed,_:=types.ParseValue([]byte("{\"kind\":\"a\",\"items\":[\"x\"],\"optional\":\"old\",\"extra\":{\"n\":1}}"))
 value,err:=DecodeGetChoiceOutput(context.Background(),parsed);if err!=nil{t.Fatal(err)}
 first,ok:=value.AsOneOf1();if !ok{t.Fatal("lost matching branch")}
 first.Items[0]="changed";first.Optional=types.Optional[string]{}
 first.AdditionalProperties["new"],_=types.ValueFrom(true)
 second,ok:=value.AsOneOf1();if !ok||second.Items[0]!="x"||!second.Optional.IsSet(){t.Fatal("As exposes mutable branch cache")}
 if _,ok:=second.AdditionalProperties["new"];ok{t.Fatal("As shares map storage")}
 updated,err:=value.WithOneOf1(first);if err!=nil{t.Fatal(err)}
 branch,ok:=updated.AsOneOf1();if !ok||branch.Items[0]!="changed"||branch.Optional.IsSet(){t.Fatal("With failed to update branch keys and remove absent optional")}
 if _,ok:=branch.AdditionalProperties["extra"];!ok{t.Fatal("With dropped unrelated additional property")}
 original,_:=value.AsOneOf1();if original.Items[0]!="x"||!original.Optional.IsSet(){t.Fatal("With mutated original union")}
 if _,ok:=value.AsOneOf2();ok{t.Fatal("converted to invalid branch")}
 next,err:=GetChoiceOutputFromOneOf2(GetChoiceOutputOneOf2{Kind:"b",Count:2});if err!=nil{t.Fatal(err)}
 if _,ok:=next.AsOneOf2();!ok{t.Fatal("branch constructor failed")}
}
`
