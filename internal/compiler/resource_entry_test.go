package sdkgen

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"openapi-sdkgen/internal/diagnostic"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResourceReferencesAtInputBoundaries(t *testing.T) {
	for _, version := range []string{"3.1.1", "3.2.0"} {
		for _, reference := range []string{"#/$defs/Token", "https://schemas.example.test/root#/$defs/Token", "https://schemas.example.test/root#token"} {
			root := map[string]any{"$id": "https://schemas.example.test/root", "$defs": map[string]any{"Token": map[string]any{"type": "string", "$anchor": "token"}}, "type": "object", "properties": map[string]any{"token": map[string]any{"$ref": reference}}}
			data, _ := json.Marshal(map[string]any{"openapi": version, "info": map[string]any{"title": "Resources", "version": "1"}, "paths": map[string]any{}, "components": map[string]any{"schemas": map[string]any{"Root": root}}})
			dir := t.TempDir()
			file := filepath.Join(dir, "openapi.json")
			if err := os.WriteFile(file, data, 0600); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write(data)
			}))
			for _, mode := range []diagnostic.Mode{diagnostic.ModeFailFast, diagnostic.ModeCollect} {
				for _, input := range []string{file, "-", server.URL + "/spec/openapi.json"} {
					result, err := CompileInputResultWithOptions(input, CompileOptions{DiagnosticMode: mode, InputReader: strings.NewReader(string(data))})
					if err != nil || result.Document == nil || diagnostic.HasErrors(result.Diagnostics) {
						t.Fatalf("%s %s %s: %v %+v", version, reference, input, err, result.Diagnostics)
					}
				}
			}
			server.Close()
		}
	}
}

func TestQueryOnlyRemoteReference(t *testing.T) {
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.RequestURI() {
		case "/spec/openapi.json":
			w.Write([]byte(`{"openapi":"3.2.0","info":{"title":"Query refs","version":"1"},"paths":{},"components":{"schemas":{"Token":{"$ref":"?kind=token#/Token"}}}}`))
		case "/spec/openapi.json?kind=token":
			w.Write([]byte(`{"Token":{"type":"string"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	for _, mode := range []diagnostic.Mode{diagnostic.ModeFailFast, diagnostic.ModeCollect} {
		result, err := CompileInputResultWithOptions(server.URL+"/spec/openapi.json", CompileOptions{DiagnosticMode: mode, RefLockPath: filepath.Join(t.TempDir(), "refs.lock"), UpdateRefLock: true})
		if err != nil || result.Document == nil || diagnostic.HasErrors(result.Diagnostics) {
			t.Fatalf("mode %s requests %v: %v %+v", mode, requested, err, result.Diagnostics)
		}
	}
}

func TestOperationReferenceCannotLoseContract(t *testing.T) {
	data := []byte(`{"openapi":"3.2.0","info":{"title":"Invalid operation","version":"1"},"paths":{"/ref":{"get":{"$ref":"missing.json"}}}}`)
	result, err := CompileResult(data)
	if err != nil || result.Document != nil || !diagnostic.HasErrors(result.Diagnostics) {
		t.Fatalf("operation reference accepted: %v %+v", err, result)
	}
	if _, err := Compile(data); err == nil {
		t.Fatal("direct compile accepted an Operation $ref")
	}
}

func TestRelativeSchemaResourceBases(t *testing.T) {
	value := map[string]any{"openapi": "3.2.0", "components": map[string]any{"schemas": map[string]any{"Root": map[string]any{"$id": "schemas/root.json", "properties": map[string]any{"token": map[string]any{"$ref": "token.json"}}}}}}
	for _, c := range []struct{ base, want string }{{"", "schemas/token.json"}, {"https://example.test/spec/openapi.json", "https://example.test/spec/schemas/token.json"}} {
		view, err := schemaReferenceView(value, "openapi", c.base)
		if err != nil {
			t.Fatal(err)
		}
		raw := view.(map[string]any)["components"].(map[string]any)["schemas"].(map[string]any)["Root"].(map[string]any)["properties"].(map[string]any)["token"].(map[string]any)
		if raw["$ref"] != c.want {
			t.Fatal(raw)
		}
	}
}
