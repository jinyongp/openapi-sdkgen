package sdkgen

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestDecodedSourceCacheSupportsConcurrentLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema.yaml")
	if err := os.WriteFile(path, []byte("type: string\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache := newDecodedSourceCache()

	const workers = 32
	results := make(chan decodedSource, workers)
	errors := make(chan error, workers)
	var ready sync.WaitGroup
	var workersDone sync.WaitGroup
	start := make(chan struct{})
	ready.Add(workers)
	workersDone.Add(workers)
	for range workers {
		go func() {
			defer workersDone.Done()
			ready.Done()
			<-start
			source, err := cache.load(path)
			if err != nil {
				errors <- err
				return
			}
			results <- source
		}()
	}
	ready.Wait()
	close(start)
	workersDone.Wait()
	close(results)
	close(errors)

	for err := range errors {
		t.Fatal(err)
	}
	count := 0
	for source := range results {
		count++
		if string(source.data) != "type: string\n" {
			t.Fatalf("concurrent cached source = %q", source.data)
		}
		value, _ := source.value.(map[string]any)
		if value["type"] != "string" {
			t.Fatalf("concurrent decoded source = %#v", source.value)
		}
	}
	if count != workers {
		t.Fatalf("concurrent loads = %d, want %d", count, workers)
	}
}

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
