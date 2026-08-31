package sdkgen

import (
	"os"
	"sync"

	"go.yaml.in/yaml/v4"
)

type decodedSource struct {
	data  []byte
	value any
}

// decodedSourceCache owns immutable local source snapshots for one compile.
// Reference diagnostics, containment checks, and provenance share the same
// bytes and decoded YAML tree instead of reopening every referenced file.
type decodedSourceCache struct {
	mu      sync.Mutex
	sources map[string]decodedSource
}

func newDecodedSourceCache() *decodedSourceCache {
	return &decodedSourceCache{sources: make(map[string]decodedSource)}
}

func (cache *decodedSourceCache) load(path string) (decodedSource, error) {
	if cache == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return decodedSource{}, err
		}
		var value any
		if err := yaml.Unmarshal(data, &value); err != nil {
			return decodedSource{}, err
		}
		return decodedSource{data: data, value: value}, nil
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if source, exists := cache.sources[path]; exists {
		return source, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return decodedSource{}, err
	}
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return decodedSource{}, err
	}
	source := decodedSource{data: data, value: value}
	cache.sources[path] = source
	return source, nil
}
