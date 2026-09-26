package sdkgen

import (
	"os"
	"path/filepath"
	"sync"
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
	metrics *compilationMetrics
}

func newDecodedSourceCache(metrics ...*compilationMetrics) *decodedSourceCache {
	var value *compilationMetrics
	if len(metrics) != 0 {
		value = metrics[0]
	}
	return &decodedSourceCache{sources: make(map[string]decodedSource), metrics: value}
}

func (cache *decodedSourceCache) remember(path string, source decodedSource) error {
	if cache == nil || path == "" {
		return nil
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if _, exists := cache.sources[resolved]; exists {
		return nil
	}
	// remember is used only with bytes already owned by the compiler input
	// loader. Treat that buffer as immutable and share it with the per-compile
	// cache instead of retaining a second full source copy.
	cache.sources[resolved] = source
	return nil
}

func (cache *decodedSourceCache) load(path string) (decodedSource, error) {
	if cache == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return decodedSource{}, err
		}
		value, err := decodeInputValue(data, nil)
		if err != nil {
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
	if cache.metrics != nil {
		cache.metrics.ReferenceSourceDecodes++
	}
	value, err := decodeInputValue(data, nil)
	if err != nil {
		return decodedSource{}, err
	}
	source := decodedSource{data: data, value: value}
	cache.sources[path] = source
	return source, nil
}
