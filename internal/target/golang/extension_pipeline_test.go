package golang

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestGeneratedTrustedExtensionIsLoweredBeforeGoEmission(t *testing.T) {
	root := t.TempDir()
	script := `#!/bin/sh
read request
case "$request" in
 *'"method":"describe"'*) printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"vocabularies":["https://example.test/vocab"]}}' ;;
 *'"method":"lower"'*) printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"schema":{"type":"string","minLength":3}}}' ;;
 *) exit 1 ;;
esac
`
	executable := filepath.Join(root, "extension")
	if err := os.WriteFile(executable, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(script))
	manifest := filepath.Join(root, "extension.json")
	text := `{"version":1,"extensions":[{"vocabularies":["https://example.test/vocab"],"command":"extension","sha256":"` + hex.EncodeToString(hash[:]) + `"}]}`
	if err := os.WriteFile(manifest, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(root, "input.json")
	text = `{"openapi":"3.1.0","info":{"title":"Lowered extension","version":"1"},"paths":{"/widgets":{"post":{"operationId":"createWidget","requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Widget"}}}},"responses":{"204":{"description":"Created"}}}}},"components":{"schemas":{"Widget":{"$vocabulary":{"https://example.test/vocab":true},"x-assertion":"three-characters"}}}}`
	if err := os.WriteFile(input, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	document, err := compiler.CompileFileWithOptions(input, compiler.CompileOptions{SchemaExtensionManifests: []string{manifest}, UpdateRefLock: true})
	if err != nil {
		t.Fatal(err)
	}
	plan, findings, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(findings) != 0 {
		t.Fatal(err, findings)
	}
	// Emission and actual SDK execution must need only the lowered schema.
	if err := os.Remove(executable); err != nil {
		t.Fatal(err)
	}
	testGeneratedModule(t, plan, map[string]string{"extension_test.go": extensionPipelineConsumer})
}

const extensionPipelineConsumer = `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"sync/atomic";"testing";api "example.test/consumer/api";"example.test/consumer/api/types")
func TestLoweredRuleWithoutExecutable(t *testing.T){var calls atomic.Int32;server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){calls.Add(1);body,err:=io.ReadAll(r.Body);if err!=nil||string(body)!="\"abc\""{t.Error(string(body),err)};w.WriteHeader(204)}));defer server.Close();client,err:=api.NewClient(api.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 for _,test:=range []struct{value string;valid bool}{{"abc",true},{"ab",false}}{call:=client.Operations.CreateWidget(api.CreateWidgetInput{Body:types.Some(api.CreateWidgetBody(test.value))});_,err:=call.Do(context.Background());if (err==nil)!=test.valid{t.Fatal(test,err)}};if calls.Load()!=1{t.Fatal("lowered assertion bypassed",calls.Load())}}
`
