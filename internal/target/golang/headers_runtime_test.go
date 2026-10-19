package golang

import (
	"encoding/json"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedDeclaredResponseHeaders(t *testing.T) {
	header := func(schema any, required bool) map[string]any {
		return map[string]any{"schema": schema, "required": required}
	}
	integer := map[string]any{"type": "integer", "minimum": 0}
	union := map[string]any{"anyOf": []any{map[string]any{"type": "object", "required": []string{"n", "tag"}, "properties": map[string]any{"n": integer, "tag": map[string]any{"const": "a"}}}, map[string]any{"type": "object", "required": []string{"n", "tag"}, "properties": map[string]any{"n": map[string]any{"type": "string"}, "tag": map[string]any{"const": "b"}}}}}
	headers := map[string]any{
		"X-Count":      map[string]any{"$ref": "#/components/headers/Count"},
		"X-Items":      header(map[string]any{"type": "array", "items": integer, "minItems": 2}, true),
		"X-Object":     map[string]any{"explode": true, "required": true, "schema": map[string]any{"allOf": []any{map[string]any{"type": "object", "properties": map[string]any{"n": integer}, "required": []string{"n"}}, map[string]any{"type": "object", "properties": map[string]any{"flag": map[string]any{"type": "boolean"}}, "required": []string{"flag"}}}}},
		"X-Union":      map[string]any{"explode": true, "required": true, "schema": union},
		"X-JSON":       map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": integer}}},
		"X-Text":       map[string]any{"required": true, "content": map[string]any{"text/plain": map[string]any{"schema": integer}}},
		"X-Pattern":    map[string]any{"explode": true, "required": true, "schema": map[string]any{"type": "object", "patternProperties": map[string]any{"^n": integer}, "additionalProperties": false}},
		"X-Root":       map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "string", "contentMediaType": "text/plain", "contentSchema": false}}}},
		"X-Root-Text":  map[string]any{"required": true, "content": map[string]any{"text/plain": map[string]any{"schema": map[string]any{"type": "string", "contentMediaType": "application/json", "contentSchema": false}}}},
		"X-Nested":     map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "properties": map[string]any{"inner": map[string]any{"type": "string", "contentMediaType": "text/plain", "contentSchema": false}}}}}},
		"X-Collision":  header(map[string]any{"type": "boolean"}, true),
		"X_Collision":  header(integer, true),
		"X-Enum":       header(map[string]any{"enum": []any{3}}, true),
		"X-Const":      header(map[string]any{"const": 4}, true),
		"Content-Type": header(map[string]any{"const": "never"}, true),
	}
	response := func(headers map[string]any) map[string]any {
		return map[string]any{"description": "OK", "headers": headers, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "boolean"}}}}
	}
	input := map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Headers", "version": "1"}, "components": map[string]any{"headers": map[string]any{"Count": header(map[string]any{"type": "integer", "minimum": 0, "writeOnly": true}, true)}}, "paths": map[string]any{
		"/headers": map[string]any{"get": map[string]any{"operationId": "getHeaders", "responses": map[string]any{"200": response(headers), "201": response(nil), "default": response(map[string]any{"X-Default": header(map[string]any{"type": "string"}, true)})}}},
		"/cookies": map[string]any{"get": map[string]any{"operationId": "getCookies", "responses": map[string]any{"200": response(map[string]any{"Set-Cookie": header(map[string]any{"type": "string"}, true)})}}},
	}}
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	document, err := compiler.Compile(encoded)
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"consumer_test.go": headerConsumer})
}

const headerConsumer = `package consumer_test
import("context";"encoding/json";"errors";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestDeclaredHeaders(t *testing.T){
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("Content-Type","application/json");w.Header().Set("X-Count","0x3");w.Header().Set("X-Items","1,2");w.Header().Set("X-Object","n=3,flag=false");w.Header().Set("X-Union","n=3,tag=b");w.Header().Set("X-JSON","3");w.Header().Set("X-Text","003");w.Header().Set("X-Pattern","n1=3,n2=4")
  w.Header().Set("X-Root","\"raw\"");w.Header().Set("X-Root-Text","raw");w.Header().Set("X-Collision","true");w.Header().Set("X_Collision","3")
 w.Header().Set("X-Enum","3");w.Header().Set("X-Const","4")
  status:=200
  switch r.Header.Get("X-Case"){case "missing":w.Header().Del("X-Count");case "bad-items":w.Header().Set("X-Items","1,false");case "bad-object":w.Header().Set("X-Object","n=3,flag=no");case "json-string":w.Header().Set("X-JSON","\"3\"");case "union-number":w.Header().Set("X-Union","n=3,tag=a");case "no-headers":status=201;case "default":status=208;w.Header().Set("X-Default","default");case "nested-conflict":w.Header().Set("X-Nested","{\"inner\":\"bad\"}")}
  w.WriteHeader(status);w.Write([]byte("true"))
 }));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 call:=client.Operations.GetHeaders()
 raw,err:=call.Raw(context.Background());if err!=nil{t.Fatal(err)}
 headers,ok:=raw.As200Headers();if !ok||headers.XCount!=3||headers.XJSON!=3||headers.XText!=3||len(headers.XItems)!=2{t.Fatal("typed headers",headers,ok)}
 if headers.XRoot!="raw"||headers.XRootText!="raw"{t.Fatal("physical header media roots lost",headers)}
 if headers.XEnum!=3||headers.XConst!=4{t.Fatal("numeric enum/const conversion",headers)}
 encoded,err:=json.Marshal(headers);if err!=nil{t.Fatal(err)};var values map[string]any;if err:=json.Unmarshal(encoded,&values);err!=nil{t.Fatal(err)}
 object:=values["X-Object"].(map[string]any);if object["n"]!=float64(3)||object["flag"]!=false{t.Fatal("allOf coercion",object)}
 union:=values["X-Union"].(map[string]any);if union["n"]!="3"{t.Fatal("uncorrelated union coercion",union)}
 pattern:=values["X-Pattern"].(map[string]any);if pattern["n1"]!=float64(3)||pattern["n2"]!=float64(4){t.Fatal("pattern fields",pattern)}
 headers.XItems[0]=99;again,_:=raw.As200Headers();if again.XItems[0]!=1{t.Fatal("shared typed header snapshot")}
 options:=func(name string)sdk.RequestOptions{return sdk.RequestOptions{Headers:http.Header{"X-Case":[]string{name}}}}
 for _,name:=range []string{"missing","bad-items","bad-object","json-string","nested-conflict"}{_,err:=call.WithOptions(options(name)).Raw(context.Background());var apiError *sdk.APIError;if !errors.As(err,&apiError)||apiError.Code!=types.ResponseDecodeFailed{t.Errorf("%s: %v",name,err)}}
 if _,err:=call.WithOptions(options("union-number")).Raw(context.Background());err!=nil{t.Fatal("numeric union branch",err)}
 if _,err:=call.WithOptions(options("no-headers")).Raw(context.Background());err!=nil{t.Fatal("default headers leaked into exact response",err)}
 raw,err=call.WithOptions(options("default")).Raw(context.Background());if err!=nil{t.Fatal(err)};fallback,ok:=raw.AsDefaultHeaders();if !ok||fallback.XDefault!="default"{t.Fatal("default headers",fallback,ok)}
}
type incapableTransport struct{called bool}
func(host *incapableTransport)Capabilities()types.TransportCapabilities{return types.TransportCapabilities{ResponseBody:types.Encoded}}
func(host *incapableTransport)Do(*http.Request,types.TransportOptions)(*http.Response,error){host.called=true;return nil,errors.New("unexpected HTTP")}
func TestRequiredCookieCapability(t *testing.T){host:=&incapableTransport{};client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:"http://example.test",Transport:host});if err!=nil{t.Fatal(err)};_,err=client.Operations.GetCookies().Do(context.Background());var apiError *sdk.APIError;if !errors.As(err,&apiError)||apiError.Code!=types.TransportCapabilityRequired||host.called{t.Fatal("cookie header capability",err,host.called)}}
`
