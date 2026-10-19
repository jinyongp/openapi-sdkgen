package golang

import (
	"encoding/json"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedVersionedBinaryMediaAndFalseSchemas(t *testing.T) {
	for _, version := range []string{"3.0.3", "3.1.0", "3.2.0"} {
		t.Run(version, func(t *testing.T) {
			schema := map[string]any{"type": "string", "minLength": 999}
			if version == "3.0.3" {
				schema["format"] = "BiNaRy"
			} else {
				schema["contentEncoding"] = "BiNaRy"
			}
			response := map[string]any{"200": map[string]any{"description": "File", "content": map[string]any{"Application/PDF": map[string]any{"schema": schema}}}}
			paths := map[string]any{"/file": map[string]any{"get": map[string]any{"operationId": "getFile", "responses": response}, "post": map[string]any{"operationId": "sendFile", "requestBody": map[string]any{"required": true, "content": map[string]any{"Application/PDF": map[string]any{"schema": schema}}}, "responses": map[string]any{"204": map[string]any{"description": "No content"}}}}}
			paths["/open-file"] = map[string]any{"get": map[string]any{"operationId": "getOpenFile", "responses": map[string]any{"200": map[string]any{"description": "Open file", "content": map[string]any{"*/*": map[string]any{"schema": schema}}}}}, "post": map[string]any{"operationId": "sendOpenFile", "requestBody": map[string]any{"required": true, "content": map[string]any{"*/*": map[string]any{"schema": schema}}}, "responses": map[string]any{"204": map[string]any{"description": "No content"}}}}
			if version != "3.0.3" {
				paths["/forbidden"] = map[string]any{"get": map[string]any{"operationId": "getForbidden", "responses": map[string]any{"200": map[string]any{"description": "Forbidden", "content": map[string]any{"application/octet-stream": map[string]any{"schema": false}}}}}, "post": map[string]any{"operationId": "sendForbidden", "requestBody": map[string]any{"required": true, "content": map[string]any{"application/octet-stream": map[string]any{"schema": false}}}, "responses": map[string]any{"204": map[string]any{"description": "No content"}}}}
			}
			encoded, err := json.Marshal(map[string]any{"openapi": version, "info": map[string]any{"title": "Binary media", "version": "1"}, "paths": paths})
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
			files := map[string]string{"consumer_test.go": versionedBinaryConsumer}
			if version != "3.0.3" {
				files["false_test.go"] = falseBinaryConsumer
			}
			testGeneratedModule(t, plan, files)
		})
	}
}

const versionedBinaryConsumer = `package consumer_test
import("bytes";"context";"io";"net/http";"net/http/httptest";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestVersionedRawBinary(t *testing.T){
 payload:=[]byte{0,255,42}
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.Method=="POST"{data,err:=io.ReadAll(r.Body);if err!=nil||!bytes.Equal(data,payload)||r.Header.Get("Content-Type")!="Application/PDF"{t.Error("binary request",data,err,r.Header)};w.WriteHeader(204);return};w.Header().Set("Content-Type","application/pdf");w.Write(payload)}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 data,err:=client.Operations.GetFile().Do(context.Background());if err!=nil||!bytes.Equal(data,payload){t.Fatal("binary response",data,err)}
 raw,err:=client.Operations.GetFile().Raw(context.Background());if err!=nil{t.Fatal(err)};data,ok:=raw.As200Binary();if !ok||!bytes.Equal(data,payload){t.Fatal("binary Raw",data,ok)}
 if _,err:=client.Operations.SendFile(sdk.SendFileInput{Body:types.BinaryFromBytes(payload)}).Do(context.Background());err!=nil{t.Fatal(err)}
 open,err:=client.Operations.GetOpenFile().Do(context.Background());if err!=nil{t.Fatal(err)};data,ok=open.AsBinary();if !ok||!bytes.Equal(data,payload){t.Fatal("versioned wildcard binary",data,ok)}
 if _,err:=client.Operations.SendOpenFile(sdk.SendOpenFileInput{Body:sdk.NewSendOpenFileMediaBinaryBody("Application/PDF",types.BinaryFromBytes(payload))}).Do(context.Background());err!=nil{t.Fatal(err)}
}
`
const falseBinaryConsumer = `package consumer_test
import("context";"errors";"net/http";"net/http/httptest";"sync/atomic";"testing";sdk "example.test/consumer/api";"example.test/consumer/api/types")
func TestFalseBinaryContract(t *testing.T){
 var requests atomic.Int32
 server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){requests.Add(1);w.Header().Set("Content-Type","application/octet-stream");w.Write([]byte{0,255,42})}));defer server.Close()
 client,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 _,err=client.Operations.SendForbidden(sdk.SendForbiddenInput{}).Do(context.Background());var apiError *sdk.APIError;if !errors.As(err,&apiError)||apiError.Code!=types.RequestEncodeFailed||requests.Load()!=0{t.Fatal("false input was bypassed as raw binary",err,requests.Load())}
 _,err=client.Operations.GetForbidden().Do(context.Background());if !errors.As(err,&apiError)||apiError.Code!=types.ResponseDecodeFailed{t.Fatal("false output was bypassed as raw binary",err)}
}
`
