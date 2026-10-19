package golang

import "testing"

func TestGeneratedJSONValueContracts(t *testing.T) {
	_, plan := healthPlan(t)
	testGeneratedModule(t, plan, map[string]string{"api/internal/wire/value_test.go": wireRuntimeTest, "api/types/value_test.go": typesRuntimeTest})
}

const wireRuntimeTest = `package wire
import (
 "encoding/json"
 "strings"
 "testing"
)
func TestJSONGrammarAndExactKeys(t *testing.T) {
 for _,input:=range []string{"", "01", "-01", "1.", "+1", "1e", "1e+", "NaN", "Infinity", "1e400", "true false", "[1,]", "[,1]", "{,}", "{\"x\":1,}", "{\"x\" 1}", "{\"x\":}", "\"\\uZZZZ\"", "\"\\x20\"", "\"unterminated", "\"\n\"", "[true false]"} {
  if _,err:=Parse([]byte(input));err==nil { t.Errorf("accepted invalid JSON %q",input) }
 }
 value,err:=Parse([]byte("{\"id\":1,\"ID\":2,\"id\":3,\"__proto__\":{\"ok\":true}}"))
 if err!=nil { t.Fatal(err) }
 lower,_:=value.Get("id");upper,_:=value.Get("ID")
 a,_:=lower.Number();b,_:=upper.Number()
 if a!=3||b!=2||value.Len()!=3 { t.Fatal("key casing or duplicate semantics changed") }
 for _,input:=range []string{"null","[]","{}","[true,false,null,1,\"x\",{}]","-0","1e-999","1.7976931348623157e308"} {
  value,err:=Parse([]byte(input));if err!=nil { t.Fatalf("%q: %v",input,err) }
  encoded,err:=value.MarshalJSON();if err!=nil||!json.Valid(encoded) { t.Fatalf("encoding %q: %s %v",input,encoded,err) }
  roundtrip,err:=Parse(encoded);if err!=nil||!roundtrip.Equal(value) { t.Fatalf("roundtrip %q: %v",input,err) }
 }
 nested:=strings.Repeat("[",20000)+"null"+strings.Repeat("]",20000)
 if _,err:=Parse([]byte(nested));err!=nil { t.Fatalf("iterative nesting: %v",err) }
}
func TestUnicodeAndBinary64(t *testing.T) {
 for _,test:=range []struct{ input,want string; length int }{
  {"\"\\ud800\"","\"\\ud800\"",1},
  {"\"\\udfff\"","\"\\udfff\"",1},
  {"\"\\ud83d\\ude00\"","\"😀\"",1},
  {"\"\\ud800x\\udfff\"","\"\\ud800x\\udfff\"",3},
  {"\"\\ud800\\ud800\"","\"\\ud800\\ud800\"",2},
  {"\"é\"","\"é\"",2},
  {"\"\xff\xff\"","\"��\"",2},
  {"\"\xe2\x82\"","\"�\"",1},
  {"\"\xed\xa0\x80\"","\"���\"",3},
 } {
  value,err:=Parse([]byte(test.input));if err!=nil { t.Fatalf("parse %q: %v",test.input,err) }
  encoded,err:=value.MarshalJSON();if err!=nil||string(encoded)!=test.want||value.Len()!=test.length { t.Errorf("%q: encoded %s length %d error %v",test.input,encoded,value.Len(),err) }
 }
 for _,test:=range []struct{input,want string}{ {"-0","0"},{"0.000001","0.000001"},{"1e-7","1e-7"},{"1e20","100000000000000000000"},{"1e21","1e+21"},{"9007199254740993","9007199254740992"} } {
  value,err:=Parse([]byte(test.input));if err!=nil { t.Fatal(err) };encoded,_:=value.MarshalJSON()
  if string(encoded)!=test.want { t.Errorf("binary64 %s -> %s; want %s",test.input,encoded,test.want) }
 }
}
func TestImmutableTreeAndStructuralEquality(t *testing.T) {
 items:=[]Value{Bool(true)}
 fields:=map[string]Value{"items":List(items)}
 value:=Record(fields)
 items[0]=Bool(false);fields["items"]=Value{}
 fieldsCopy:=value.Fields();fieldsCopy["other"]=Text("mutated")
 array,_:=value.Get("items");array.Items()[0]=Bool(false)
 expected,err:=Parse([]byte("{\"items\":[true]}"));if err!=nil { t.Fatal(err) }
 if !value.Equal(expected) { t.Fatal("tree exposes mutable storage") }
 first,_:=Parse([]byte("{\"a\":1,\"b\":[true,null]}"));second,_:=Parse([]byte("{\"b\":[true,null],\"a\":1}"))
 if !first.Equal(second) { t.Fatal("object equality depends on key order") }
 for _,input:=range []string{"{\"a\":\"1\",\"b\":[true,null]}","{\"a\":1,\"b\":[null,true]}","{\"a\":1}"} { other,_:=Parse([]byte(input));if first.Equal(other) { t.Errorf("structural equality coerced %s",input) } }
}
func TestDecodedSurrogatePairCanonicalization(t *testing.T){
 pair:=string([]byte{0xed,0xa0,0xbd,0xed,0xb8,0x80})
 value,err:=ParseText("\""+pair+"\"");if err!=nil{t.Fatal(err)}
 if text,_:=value.String();text!="😀"||value.Len()!=1{t.Fatalf("noncanonical decoded pair %q",text)}
 keyed,err:=ParseText("{\""+pair+"\":1,\"😀\":2}");if err!=nil{t.Fatal(err)}
 item,ok:=keyed.Get("😀");number,_:=item.Number();if !ok||keyed.Len()!=1||number!=2{t.Fatal("surrogate-pair keys lost last-duplicate identity")}
}
`

const typesRuntimeTest = `package types
import (
 "math"
 "testing"
)
func TestPresenceAndNull(t *testing.T) {
 var absent Optional[Nullable[bool]]
 if absent.IsSet() { t.Fatal("optional zero value is present") }
 if _,err:=absent.MarshalJSON();err==nil { t.Fatal("absent optional encoded as a JSON value") }
 for _,test:=range []struct{value Optional[Nullable[bool]];want string}{ {Some(Null[bool]()),"null"},{Some(NonNull(false)),"false"},{Some(NonNull(true)),"true"} } {
  encoded,err:=test.value.MarshalJSON();if err!=nil||string(encoded)!=test.want { t.Errorf("presence: %s %v",encoded,err) }
 }
 var nullable Nullable[string]
 if !nullable.IsNull() { t.Fatal("nullable zero value is non-null") }
 for _,value:=range []any{false,0,"",[]string(nil),map[string]any(nil)} { if _,err:=ValueFrom(value);err!=nil { t.Fatal(err) } }
 array,_:=ValueFrom([]string(nil));object,_:=ValueFrom(map[string]any(nil))
 a,_:=array.MarshalJSON();b,_:=object.MarshalJSON()
 if string(a)!="[]"||string(b)!="{}" { t.Fatalf("nil non-null containers: %s %s",a,b) }
}
func TestValueFromSnapshotsAndRejectsInvalidValues(t *testing.T) {
 input:=map[string]any{"a":[]any{true}}
 value,err:=ValueFrom(input);if err!=nil { t.Fatal(err) }
 input["a"].([]any)[0]=false
 want,_:=ParseValue([]byte("{\"a\":[true]}"))
 if !value.Equal(want) { t.Fatal("ValueFrom retains mutable input") }
 cyclicMap:=map[string]any{};cyclicMap["self"]=cyclicMap
 cyclicSlice:=make([]any,1);cyclicSlice[0]=cyclicSlice
 for _,input:=range []any{math.NaN(),math.Inf(1),math.Inf(-1),cyclicMap,cyclicSlice,make(chan bool),map[int]string{1:"x"},struct{X int}{1}} {
  if _,err:=ValueFrom(input);err==nil { t.Errorf("accepted invalid JSON value %T",input) }
 }
 for _,input:=range []any{string([]byte{0xff}),map[string]any{string([]byte{0xff}):true}}{if _,err:=ValueFrom(input);err==nil{t.Fatal("accepted malformed Go string outside JSON/WTF-8")}}
 pair:=string([]byte{0xed,0xa0,0xbd,0xed,0xb8,0x80})
 value,err=ValueFrom(pair);if err!=nil{t.Fatal(err)};if text,_:=value.String();text!="😀"{t.Fatal("Go surrogate pair not canonicalized")}
 if _,err:=ValueFrom(map[string]any{pair:1,"😀":2});err==nil{t.Fatal("silently merged colliding Go map keys")}
}
`
