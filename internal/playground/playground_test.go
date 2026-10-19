package playground

import (
	"strings"
	"testing"
)

func TestGenerateEmitsTypeScriptFiles(t *testing.T) {
	result, err := Generate([]byte(`{
		"openapi":"3.1.0",
		"info":{"title":"Todo API","version":"1.0.0"},
		"paths":{"/todos":{"get":{"operationId":"listTodos","responses":{"200":{"description":"OK"}}}}}
	}`), "typescript")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Diagnostics != "" {
		t.Fatalf("Generate() diagnostics = %q", result.Diagnostics)
	}
	if len(result.Artifacts) == 0 {
		t.Fatal("Generate() emitted no artifacts")
	}
	found := false
	for _, artifact := range result.Artifacts {
		if strings.HasPrefix(artifact.Path, "internal/operations/") && strings.Contains(artifact.Content, "listTodos") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("generated semantic operation tree does not contain listTodos")
	}
}

func TestGenerateReturnsAuthorDiagnostics(t *testing.T) {
	result, err := Generate([]byte(`{"openapi":"3.1.0","info":{"title":"Broken","version":"1"},"paths":[]}`), "typescript")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Diagnostics == "" {
		t.Fatal("Generate() returned no diagnostics")
	}
	if len(result.Artifacts) != 0 {
		t.Fatalf("Generate() emitted %d artifacts after an author error", len(result.Artifacts))
	}
}

func TestGenerateRejectsUnknownTarget(t *testing.T) {
	if _, err := Generate([]byte(`{}`), "swift"); err == nil || !strings.Contains(err.Error(), "unsupported SDK target") {
		t.Fatalf("Generate() error = %v", err)
	}
}

func TestGenerateGoWithOptions(t *testing.T) {
	const document = `{"openapi":"3.2.0","info":{"title":"Hooks","version":"1"},"paths":{},"webhooks":{"received":{"post":{"responses":{"204":{"description":"Accepted"}}}}}}`
	result, err := Generate([]byte(document), "golang", Options{GoImportPath: "example.com/consumer/internal/api", GoPackage: "client", Addons: []string{"server", "metadata"}})
	if err != nil || result.Diagnostics != "" {
		t.Fatal(err, result.Diagnostics)
	}
	found := map[string]bool{}
	for _, artifact := range result.Artifacts {
		if strings.HasSuffix(artifact.Path, ".ts") {
			t.Fatal("Go emitted TypeScript", artifact.Path)
		}
		found[artifact.Path] = true
		if artifact.Path == "client.go" && (!strings.Contains(artifact.Content, "package client") || !strings.Contains(artifact.Content, "example.com/consumer/internal/api/types")) {
			t.Fatal(artifact.Content)
		}
	}
	if !found["client.go"] || !found["server/runtime/handler.go"] {
		t.Fatal(found)
	}
	var metadata, webhook bool
	for path := range found {
		metadata = metadata || strings.HasPrefix(path, "metadata/")
		webhook = webhook || strings.HasPrefix(path, "server/webhooks/")
	}
	if !metadata || !webhook {
		t.Fatal("addon files missing", metadata, webhook)
	}
}

func TestGenerateOptionErrors(t *testing.T) {
	const document = `{"openapi":"3.1.0","info":{"title":"Empty","version":"1"},"paths":{}}`
	for _, options := range [][]Options{{{GoImportPath: "invalid path"}}, {{Addons: []string{"unknown"}}}, {{}, {}}} {
		if result, err := Generate([]byte(document), "golang", options...); err == nil && result.Diagnostics == "" {
			t.Fatal("invalid options accepted", options)
		}
	}
}
