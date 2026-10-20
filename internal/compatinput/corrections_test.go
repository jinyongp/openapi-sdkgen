package compatinput

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/goruntime/ecmaregexp"
)

func TestTagCorrectionBundlesExternalSchemasAndKeepsOpaquePatterns(t *testing.T) {
	var hash string
	for range 2 {
		root := t.TempDir()
		input := filepath.Join(root, "openapi.json")
		write(t, input, `{"openapi":"3.0.3","info":{"title":"Tags","version":"1"},"paths":{"/tags":{"post":{"$ref":"operation.json"}}}}`)
		write(t, filepath.Join(root, "operation.json"), `{"operationId":"tag","requestBody":{"required":true,"content":{"application/json":{"schema":{"$ref":"tag.json"}}}},"responses":{"204":{"description":"Done"}},"x-data":{"pattern":"^[a-zA-Z0-9_\\-\\:]+$"}}`)
		write(t, filepath.Join(root, "tag.json"), `{"type":"string","pattern":"^[a-zA-Z0-9_\\-\\:]+$"}`)
		original, _ := os.ReadFile(input)
		data, contracts, err := BundleWithKind(input, DigitalOceanTagsV1)
		if err != nil || len(contracts) != 1 {
			t.Fatal(err, contracts)
		}
		if hash != "" && Digest(data) != hash {
			t.Fatal("provider bundle depends on its host directory")
		}
		hash = Digest(data)
		if !strings.Contains(string(data), `"pattern":"^[a-zA-Z0-9_\\-:]+$"`) || !strings.Contains(string(data), `"pattern":"^[a-zA-Z0-9_\\-\\:]+$"`) {
			t.Fatal("schema correction or opaque data lost")
		}
		prepared, err := Prepare(input, &Spec{Kind: DigitalOceanTagsV1, SHA256: hash, Operations: 1})
		if err != nil {
			t.Fatal(err)
		}
		compiled, err := compiler.CompileFile(prepared.Path)
		if err != nil || prepared.Verify(compiled) != nil {
			t.Fatal(err)
		}
		prepared.Close()
		after, _ := os.ReadFile(input)
		if string(original) != string(after) {
			t.Fatal("original input changed")
		}
	}
	if _, err := ecmaregexp.Compile(`^[a-zA-Z0-9_\-:]+$`); err != nil {
		t.Fatal(err)
	}
}

func TestParameterBodyCorrectionPreservesRealBodyAndRejectsLoss(t *testing.T) {
	source := `{"openapi":"3.0.3","info":{"title":"Parameters","version":"1"},"paths":{"/items/{id}":{"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"delete":{"operationId":"remove","parameters":[{"name":"hash","in":"query","required":true,"schema":{"type":"string"}}],"requestBody":{"required":true,"content":{"":{"schema":{"type":"object","required":["id","hash"],"properties":{"id":{"type":"string"},"hash":{"type":"string"}}}}}},"responses":{"204":{"description":"Done"}}},"post":{"operationId":"update","requestBody":{"required":true,"content":{"":{"schema":{"type":"object","properties":{"id":{"type":"string"}}}},"application/json":{"schema":{"type":"integer","minimum":1}}}},"responses":{"204":{"description":"Done"}}}}}}`
	input := filepath.Join(t.TempDir(), "openapi.json")
	write(t, input, source)
	data, contracts, err := BundleWithKind(input, SpoonacularParametersV1)
	if err != nil || len(contracts) != 2 {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range compiled.Operations {
		if operation.Method == "DELETE" && (operation.RequestBody != nil || len(operation.Parameters) != 2) {
			t.Fatal("parameters were lost or the placeholder body survived")
		}
		if operation.Method == "POST" && (operation.RequestBody == nil || len(operation.RequestBody.Content) != 1 || operation.RequestBody.Content[0].ContentType != "application/json") {
			t.Fatal("real JSON body was lost")
		}
	}
	write(t, input, strings.Replace(source, `"properties":{"id":{"type":"string"}}`, `"properties":{}`, 1))
	if _, _, err := BundleWithKind(input, SpoonacularParametersV1); err != nil {
		t.Fatal("empty metadata placeholder blocked the real JSON body", err)
	}
	for _, replacement := range []string{`"hash":{"type":"integer"}`, `"extra":{"type":"string"}`} {
		invalid := strings.Replace(source, `"hash":{"type":"string"}`, replacement, 1)
		write(t, input, invalid)
		if _, _, err := BundleWithKind(input, SpoonacularParametersV1); err == nil {
			t.Fatal("nonredundant body field was discarded")
		}
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
}
