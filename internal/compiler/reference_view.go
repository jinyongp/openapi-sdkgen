package sdkgen

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"openapi-sdkgen/internal/openapiwalk"
)

const opaqueReferenceMarkerPrefix = "x-openapi-sdkgen-opaque-key-"

// opaqueReferenceEscaper owns one compile's reversible key mapping for data
// passed to libopenapi. Exact source/effective trees never contain these keys.
type opaqueReferenceEscaper struct {
	mu         sync.Mutex
	next       int
	markers    map[string]string
	byOriginal map[string]string
	reserved   map[string]struct{}
}

func newOpaqueReferenceEscaper() *opaqueReferenceEscaper {
	return &opaqueReferenceEscaper{
		markers:    make(map[string]string),
		byOriginal: make(map[string]string),
		reserved:   make(map[string]struct{}),
	}
}

func (escaper *opaqueReferenceEscaper) data(value any, original []byte) ([]byte, error) {
	if escaper == nil {
		return original, nil
	}
	escaped, changed, err := escaper.escape(value)
	if err != nil {
		return nil, err
	}
	if !changed {
		return original, nil
	}
	data, err := json.Marshal(escaped)
	if err != nil {
		return nil, fmt.Errorf("encode OpenAPI reference-resolution view: %w", err)
	}
	return data, nil
}

func (escaper *opaqueReferenceEscaper) escape(value any) (any, bool, error) {
	if escaper == nil {
		return value, false, nil
	}
	escaper.mu.Lock()
	defer escaper.mu.Unlock()

	escaper.reserveKeysLocked(value, nil, false)
	return escaper.escapeValueLocked(value, nil, false)
}

func (escaper *opaqueReferenceEscaper) restore(value any) (any, error) {
	if escaper == nil {
		return value, nil
	}
	escaper.mu.Lock()
	defer escaper.mu.Unlock()

	restored, _, err := escaper.restoreValueLocked(value, nil, false)
	return restored, err
}

func (escaper *opaqueReferenceEscaper) reserveKeysLocked(value any, path []string, opaque bool) {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedOpaqueMapKeys(typed) {
			child := typed[key]
			escaper.reserved[key] = struct{}{}
			childOpaque := opaque || openapiwalk.ReferenceChildOpaque(path, key, child)
			escaper.reserveKeysLocked(child, append(path, key), childOpaque)
		}
	case []any:
		for index, child := range typed {
			escaper.reserveKeysLocked(child, append(path, strconv.Itoa(index)), opaque)
		}
	}
}

func (escaper *opaqueReferenceEscaper) escapeValueLocked(value any, path []string, opaque bool) (any, bool, error) {
	switch typed := value.(type) {
	case map[string]any:
		type entry struct {
			key   string
			value any
		}
		entries := make([]entry, 0, len(typed))
		outputs := make(map[string]struct{}, len(typed))
		changed := false
		for _, key := range sortedOpaqueMapKeys(typed) {
			child := typed[key]
			outputKey := key
			if opaque && (key == "$ref" || escaper.isMarkerLocked(key)) {
				outputKey = escaper.markerForLocked(key)
			}
			childOpaque := opaque || openapiwalk.ReferenceChildOpaque(path, key, child)
			escapedChild, childChanged, err := escaper.escapeValueLocked(child, append(path, key), childOpaque)
			if err != nil {
				return nil, false, err
			}
			if _, exists := outputs[outputKey]; exists {
				return nil, false, fmt.Errorf("opaque reference marker %q collides inside one object", outputKey)
			}
			outputs[outputKey] = struct{}{}
			entries = append(entries, entry{key: outputKey, value: escapedChild})
			changed = changed || outputKey != key || childChanged
		}
		if !changed {
			return value, false, nil
		}
		result := make(map[string]any, len(entries))
		for _, entry := range entries {
			result[entry.key] = entry.value
		}
		return result, true, nil
	case []any:
		var copied []any
		for index, child := range typed {
			escapedChild, childChanged, err := escaper.escapeValueLocked(child, append(path, strconv.Itoa(index)), opaque)
			if err != nil {
				return nil, false, err
			}
			if !childChanged {
				continue
			}
			if copied == nil {
				copied = append([]any(nil), typed...)
			}
			copied[index] = escapedChild
		}
		if copied == nil {
			return value, false, nil
		}
		return copied, true, nil
	default:
		return value, false, nil
	}
}

func (escaper *opaqueReferenceEscaper) restoreValueLocked(value any, path []string, opaque bool) (any, bool, error) {
	switch typed := value.(type) {
	case map[string]any:
		type entry struct {
			key   string
			value any
		}
		entries := make([]entry, 0, len(typed))
		outputs := make(map[string]struct{}, len(typed))
		changed := false
		for _, key := range sortedOpaqueMapKeys(typed) {
			child := typed[key]
			outputKey := key
			if opaque {
				if original, exists := escaper.markers[key]; exists {
					outputKey = original
				}
			}
			childOpaque := opaque || openapiwalk.ReferenceChildOpaque(path, outputKey, child)
			restoredChild, childChanged, err := escaper.restoreValueLocked(child, append(path, outputKey), childOpaque)
			if err != nil {
				return nil, false, err
			}
			if _, exists := outputs[outputKey]; exists {
				return nil, false, fmt.Errorf("restore opaque reference key %q: duplicate object key", outputKey)
			}
			outputs[outputKey] = struct{}{}
			entries = append(entries, entry{key: outputKey, value: restoredChild})
			changed = changed || outputKey != key || childChanged
		}
		if !changed {
			return value, false, nil
		}
		result := make(map[string]any, len(entries))
		for _, entry := range entries {
			result[entry.key] = entry.value
		}
		return result, true, nil
	case []any:
		var copied []any
		for index, child := range typed {
			restoredChild, childChanged, err := escaper.restoreValueLocked(child, append(path, strconv.Itoa(index)), opaque)
			if err != nil {
				return nil, false, err
			}
			if !childChanged {
				continue
			}
			if copied == nil {
				copied = append([]any(nil), typed...)
			}
			copied[index] = restoredChild
		}
		if copied == nil {
			return value, false, nil
		}
		return copied, true, nil
	default:
		return value, false, nil
	}
}

func (escaper *opaqueReferenceEscaper) isMarkerLocked(key string) bool {
	_, exists := escaper.markers[key]
	return exists
}

func (escaper *opaqueReferenceEscaper) markerForLocked(original string) string {
	if marker, exists := escaper.byOriginal[original]; exists {
		return marker
	}
	for {
		marker := opaqueReferenceMarkerPrefix + strconv.Itoa(escaper.next)
		escaper.next++
		if _, exists := escaper.reserved[marker]; exists {
			continue
		}
		escaper.reserved[marker] = struct{}{}
		escaper.markers[marker] = original
		escaper.byOriginal[original] = marker
		return marker
	}
}

func sortedOpaqueMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
