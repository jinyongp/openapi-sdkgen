package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateResourceReferencesAcrossInputs(t *testing.T) {
	const document = `{"openapi":"3.2.0","info":{"title":"Resource input","version":"1"},"paths":{"/tokens":{"post":{"operationId":"sendToken","requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Root"}}}},"responses":{"204":{"description":"Done"}}}}},"components":{"schemas":{"Root":{"$id":"https://schemas.example.test/root","type":"object","$defs":{"Token":{"type":"string"}},"properties":{"token":{"$ref":"#/$defs/Token"}}}}}}`
	dir := t.TempDir()
	file := filepath.Join(dir, "openapi.json")
	if err := os.WriteFile(file, []byte(document), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(document))
	}))
	defer server.Close()
	previous := standardInput
	t.Cleanup(func() { standardInput = previous })
	for _, target := range []string{"typescript", "golang"} {
		for index, input := range []string{file, "-", server.URL + "/spec/openapi.json"} {
			standardInput = strings.NewReader(document)
			output := filepath.Join(t.TempDir(), "sdk")
			args := []string{"generate", "--target", target, "--input", input, "--output", output}
			if target == "golang" {
				args = append(args, "--go-import-path", "example.test/sdk")
			}
			if err := run(args); err != nil {
				t.Fatalf("%s input %d: %v", target, index, err)
			}
		}
	}
}
