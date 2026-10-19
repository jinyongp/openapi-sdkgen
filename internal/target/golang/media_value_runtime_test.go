package golang

import "testing"

func TestGeneratedTransportAndUSVTextPolicies(t *testing.T) {
	_, plan := healthPlan(t)
	testGeneratedModule(t, plan, map[string]string{"api/internal/media/value_test.go": mediaTextConsumer})
}

const mediaTextConsumer = `package media
import("context";"io";"testing";"example.test/consumer/api/types")
func TestUnicodePolicies(t *testing.T){
 value,err:=types.ParseValue([]byte("\"\\udc00\\udc01\""));if err!=nil{t.Fatal(err)}
 binary,err:=EncodeText(value);if err!=nil{t.Fatal(err)};reader,err:=binary.Open(context.Background());if err!=nil{t.Fatal(err)};defer reader.Close()
 data,err:=io.ReadAll(reader);if err!=nil||string(data)!="��"{t.Fatal("TextEncoder USV replacement",string(data),err)}
 object,err:=types.ValueFrom(map[string]types.Value{"p":value});if err!=nil{t.Fatal(err)}
 if got:=Form(object);got!="p=%EF%BF%BD%EF%BF%BD"{t.Fatal("URLSearchParams USV replacement",got)}
 decoded,err:=DecodeText([]byte{0xed,0xa0,0x80});if err!=nil{t.Fatal(err)};text,_:=decoded.String();if text!="���"{t.Fatal("transport UTF-8 replacement",text)}
 bom,err:=types.ValueFrom("\ufefftext");if err!=nil{t.Fatal(err)};encoded,err:=EncodeText(bom);if err!=nil{t.Fatal(err)};opened,err:=encoded.Open(context.Background());if err!=nil{t.Fatal(err)};defer opened.Close();data,err=io.ReadAll(opened);if err!=nil||string(data)!="\ufefftext"{t.Fatal("encoder removed BOM",string(data),err)}
 decoded,err=DecodeText(data);if err!=nil{t.Fatal(err)};text,_=decoded.String();if text!="text"{t.Fatal("decoder retained initial BOM",text)}
 null,err:=types.ParseValue([]byte("null"));if err!=nil{t.Fatal(err)};if String(null)!="null"{t.Fatal("JavaScript null string")}
}
`
