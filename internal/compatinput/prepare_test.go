package compatinput

import (
	"encoding/json"
	compiler "openapi-sdkgen/internal/compiler"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestOperationBundlePreservesContractsAndOpaqueData(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "openapi.json")
	write(t, input, `{"openapi":"3.1.1","info":{"title":"Authored","version":"1"},"paths":{"/items":{"get":{"$ref":"operations/get.json"}}}}`)
	write(t, filepath.Join(root, "operations/get.json"), `{"operationId":"getItems","parameters":[{"name":"token","in":"query","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"OK","content":{"text/plain":{"schema":{"$ref":"../schemas/text.json"}}}}},"x-codeSamples":[{"$ref":"do-not-read.json"}]}`)
	write(t, filepath.Join(root, "schemas/text.json"), `{"type":"string"}`)
	data, contracts, err := Bundle(input)
	if err != nil || len(contracts) != 1 {
		t.Fatal(err, contracts)
	}
	again, _, err := Bundle(input)
	if err != nil || Digest(data) != Digest(again) {
		t.Fatal("nondeterministic bundle", err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	operation := document["paths"].(map[string]any)["/items"].(map[string]any)["get"].(map[string]any)
	if operation["operationId"] != "getItems" || !strings.Contains(string(data), `"$ref":"schemas/text.json"`) || !strings.Contains(string(data), `"$ref":"do-not-read.json"`) {
		t.Fatal(string(data))
	}
	prepared, err := Prepare(input, &Spec{Kind: OperationReferencesV1, SHA256: Digest(data)})
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	compiled, err := compiler.CompileFile(prepared.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := prepared.Verify(compiled); err != nil || !prepared.Evidence.ContractsVerified {
		t.Fatal(err)
	}
	compiled.Operations[0].OperationID = ""
	if err := prepared.Verify(compiled); err == nil {
		t.Fatal("lost identity accepted")
	}
	compiled.Operations[0].OperationID = "getItems"
	compiled.Operations[0].Parameters[0].Required = false
	if err := prepared.Verify(compiled); err == nil {
		t.Fatal("lost required parameter accepted")
	}
	compiled.Operations[0].Parameters[0].Required = true
	compiled.Operations[0].Responses[0].Content = nil
	if err := prepared.Verify(compiled); err == nil {
		t.Fatal("lost response schema accepted")
	}
	if _, err := Prepare(input, &Spec{Kind: OperationReferencesV1, SHA256: strings.Repeat("0", 64)}); err == nil {
		t.Fatal("unmatched preparation pin accepted")
	}
}
func TestAuthorReferenceRejectsCycleEscapeAndSiblings(t *testing.T) {
	for _, reference := range []string{"../outside.json", "https://example.test/op.json", "cycle.json"} {
		root := t.TempDir()
		input := filepath.Join(root, "openapi.json")
		write(t, input, `{"paths":{"/items":{"get":{"$ref":"`+reference+`"}}}}`)
		write(t, filepath.Join(root, "cycle.json"), `{"$ref":"cycle.json"}`)
		if _, _, err := Bundle(input); err == nil {
			t.Fatal("invalid author reference accepted", reference)
		}
	}
	root := t.TempDir()
	input := filepath.Join(root, "openapi.json")
	write(t, input, `{"paths":{"/items":{"get":{"$ref":"op.json","operationId":"sibling"}}}}`)
	if _, _, err := Bundle(input); err == nil {
		t.Fatal("ambiguous operation siblings accepted")
	}
}

func TestBundleRebasesDiscriminatorAndHonorsParameterOverride(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "api.json")
	write(t, input, `{"openapi":"3.1.1","info":{"title":"Test","version":"1"},"paths":{"/items":{"parameters":[{"name":"q","in":"query","required":true,"schema":{"type":"string"}}],"get":{"$ref":"ops/get.json"}}}}`)
	write(t, filepath.Join(root, "ops/get.json"), `{"operationId":"getItems","parameters":[{"name":"q","in":"query","required":false,"schema":{"type":"string"}}],"responses":{"200":{"$ref":"response.json"}}}`)
	write(t, filepath.Join(root, "ops/response.json"), `{"description":"OK","headers":{"x-request-id":{"$ref":"header.json"}},"content":{"application/json":{"schema":{"type":"object","discriminator":{"propertyName":"kind","mapping":{"example":"../schema.json"}}}}}}`)
	write(t, filepath.Join(root, "ops/header.json"), `{"schema":{"type":"string"}}`)
	write(t, filepath.Join(root, "schema.json"), `{"type":"object"}`)
	data, contracts, err := Bundle(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(contracts) != 1 || len(contracts[0].Parameters) != 1 || contracts[0].Parameters[0].Required {
		t.Fatal(contracts)
	}
	if !strings.Contains(string(data), `"example":"./schema.json"`) || !strings.Contains(string(data), `"x-request-id":{"schema":{"type":"string"}}`) {
		t.Fatal(string(data))
	}
}
func TestPointerRejectsMalformedEscapesAndArrayIndices(t *testing.T) {
	for _, fragment := range []string{"/a~2b", "/a~", "/list/01", "/list/+0"} {
		if _, err := pointer(map[string]any{"a~2b": true, "a~": true, "list": []any{true, true}}, fragment); err == nil {
			t.Fatal(fragment)
		}
	}
	value, err := pointer(map[string]any{"a/b~c": true}, "/a~1b~0c")
	if err != nil || value != true {
		t.Fatal(value, err)
	}
}
