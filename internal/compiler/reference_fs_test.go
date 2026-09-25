package sdkgen

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"openapi-sdkgen/internal/openapiwalk"
)

func TestReferenceSourceFSUsesImmutableCacheAndValidatedFileSet(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "openapi.yaml")
	schema := filepath.Join(root, "schemas", "thing.yaml")
	unrelated := filepath.Join(root, "unrelated.yaml")
	if err := os.MkdirAll(filepath.Dir(schema), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entry, []byte("openapi: 3.1.2\ninfo: {title: Root, version: '1'}\npaths: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(schema, []byte("type: string\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unrelated, []byte("type: boolean\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache := newDecodedSourceCache()
	snapshot, err := cache.load(schema)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(schema, []byte("type: integer\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	session := newCompatibilitySession(map[string]any{"openapi": "3.1.2"}, nil)
	session.registerSourceContext(schema, openapiwalk.ObjectSchema)
	filesystem, err := newReferenceSourceFS(root, []string{"openapi.yaml", "schemas/thing.yaml"}, cache, session)
	if err != nil {
		t.Fatal(err)
	}
	data, err := fs.ReadFile(filesystem, "schemas/thing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(snapshot.data) || string(data) != "type: string\n" {
		t.Fatalf("virtual source = %q, cached = %q", data, snapshot.data)
	}
	if _, err := fs.ReadFile(filesystem, "unrelated.yaml"); !os.IsNotExist(err) {
		t.Fatalf("unvalidated file was visible: %v", err)
	}
	entries, err := fs.ReadDir(filesystem, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != "openapi.yaml" || entries[1].Name() != "schemas" {
		t.Fatalf("root entries = %#v", entries)
	}
}

func TestReferenceSourceFSPreservesReferencedRootContext(t *testing.T) {
	root := t.TempDir()
	schema := filepath.Join(root, "schema.yaml")
	if err := os.WriteFile(schema, []byte("type: string\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache := newDecodedSourceCache()
	session := newCompatibilitySession(map[string]any{"openapi": "3.1.2"}, nil)
	session.registerSourceContext(schema, openapiwalk.ObjectSchema)
	if _, err := newReferenceSourceFS(root, []string{"schema.yaml"}, cache, session); err != nil {
		t.Fatal(err)
	}
	if got := session.sourceContext(schema); got != openapiwalk.ObjectSchema {
		t.Fatalf("source context = %q", got)
	}
}
