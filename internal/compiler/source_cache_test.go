package sdkgen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecodedSourceCacheKeepsOneImmutableSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.yaml")
	if err := os.WriteFile(path, []byte("type: string\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache := newDecodedSourceCache()
	first, err := cache.load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("type: integer\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := cache.load(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(second.data) != string(first.data) {
		t.Fatalf("cached source changed from %q to %q", first.data, second.data)
	}
	value, _ := second.value.(map[string]any)
	if value["type"] != "string" {
		t.Fatalf("cached decoded source = %#v", second.value)
	}
}
