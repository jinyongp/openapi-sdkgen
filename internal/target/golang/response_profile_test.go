package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedResponsePropertyProfile(t *testing.T) {
	document, err := compiler.CompileFile("../../../test/fixtures/response-properties.json")
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"response_profile_test.go": responseProfileConsumer})
}

const responseProfileConsumer = `package consumer_test
import("context";"encoding/json";"net/http";"net/http/httptest";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/models";"example.test/consumer/api/types")
func TestTolerantResponsesAndStrictModels(t *testing.T){var requests atomic.Int32;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1);w.Header().Set("Content-Type","application/json");switch r.URL.Path{case "/closed":json.NewEncoder(w).Encode(map[string]any{"id":"known","secret":"preserve","extra":"from-extra"});case "/choice":json.NewEncoder(w).Encode(map[string]any{"left":"selected","right":2});case "/echo/from-extra":w.Header().Set("Content-Type","text/plain");w.Write([]byte("linked"));default:t.Error("unexpected request",r.Method,r.URL.Path)}}));defer server.Close();client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 raw,err:=client.Operations.ReadClosed().Raw(context.Background());if err!=nil{t.Fatal(err)};if item,present:=raw.Data.Get("secret");!present{t.Fatal(item,present)};typed,ok:=raw.As200JSON();if !ok||typed.ID!="known"{t.Fatal(typed,ok)};if _,err:=models.DecodeReadClosedOutput(context.Background(),raw.Data);err==nil{t.Fatal("standalone output decoding became tolerant")}
 linked,err:=client.Links.ReadClosed.Extra.Follow(raw).Do(context.Background());if err!=nil||linked!="linked"{t.Fatal("Link lost undeclared source body",linked,err)}
 output,err:=client.Operations.ReadChoice().Do(context.Background());if err!=nil{t.Fatal("strict branch preference",err)};branch,ok:=output.AsOneOf1();if !ok||branch.Left!="selected"{t.Fatal(branch,ok)};encoded,err:=json.Marshal(output);if err!=nil{t.Fatal("response union JSON",err)};if len(encoded)==0{t.Fatal("empty union")}
 input,_:=types.ParseValue([]byte("{\"id\":\"known\",\"extra\":\"reject\"}"));if _,err:=models.DecodeSendClosedBody(context.Background(),input);err==nil{t.Fatal("request decoding became tolerant")};before:=requests.Load();if before!=3{t.Fatal("unexpected HTTP",before)}
}
`
