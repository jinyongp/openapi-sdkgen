package sdkgen

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/openapiwalk"
)

const reservedExtensionPrefix = "x-sdkgen-"

func reservedExtensionDiagnostics(data []byte, source string) ([]diagnostic.Diagnostic, error) {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("decode source document for extension scan: %w", err)
	}
	return reservedExtensionDiagnosticsValue(value, source), nil
}

func reservedExtensionDiagnosticsValue(value any, source string) []diagnostic.Diagnostic {
	var result []diagnostic.Diagnostic
	scanExtensionKeywords(value, nil, source, &result)
	return diagnostic.Sort(result)
}

func scanExtensionKeywords(value any, path []string, source string, result *[]diagnostic.Diagnostic) {
	switch typed := value.(type) {
	case map[string]any:
		names := make([]string, 0, len(typed))
		for name := range typed {
			names = append(names, name)
		}
		sort.Strings(names)
		namedMap := openapiwalk.IsNamedMap(path)
		for _, name := range names {
			child := typed[name]
			if openapiwalk.IsExtensionKey(path, name) {
				if strings.HasPrefix(name, reservedExtensionPrefix) {
					*result = append(*result, diagnostic.Diagnostic{
						Severity: diagnostic.SeverityError,
						Code:     "SDKGEN-E160",
						Phase:    diagnostic.PhaseOpenAPI,
						Location: diagnostic.Location{Source: source, Pointer: sourceJSONPointer(append(path, name))},
						Message:  fmt.Sprintf("Extension keyword %q uses the compiler-reserved x-sdkgen-* namespace.", name),
						Hint:     "Rename the vendor extension; exact property, header, and component names remain legal.",
					})
				}
				// Extension values are vendor-owned payloads. Keys nested inside
				// them are data, not OpenAPI or JSON Schema extension keywords.
				continue
			}
			if !namedMap && openapiwalk.IsOpaqueDataField(name, child) {
				continue
			}
			scanExtensionKeywords(child, append(path, name), source, result)
		}
	case []any:
		for index, child := range typed {
			scanExtensionKeywords(child, append(path, fmt.Sprintf("%d", index)), source, result)
		}
	}
}

func sourceJSONPointer(path []string) string {
	pointer := "#"
	for _, token := range path {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1")
		pointer += "/" + token
	}
	return pointer
}

func scanLocalReferenceDocuments(source inputSource, collector *diagnostic.Collector) error {
	var value any
	if err := yaml.Unmarshal(source.data, &value); err != nil {
		return nil
	}
	return scanLocalReferenceDocumentsValue(source, value, collector, nil)
}

func scanLocalReferenceDocumentsValue(source inputSource, value any, collector *diagnostic.Collector, cache *decodedSourceCache) error {
	if source.fileBase == "" {
		return nil
	}
	root, err := filepath.EvalSymlinks(source.fileBase)
	if err != nil {
		return err
	}
	visited := map[string]bool{}
	if source.filePath != "" {
		if resolved, resolveErr := filepath.EvalSymlinks(source.filePath); resolveErr == nil {
			visited[resolved] = true
		}
	}
	return scanLocalReferenceValue(value, source.fileBase, root, visited, collector, cache)
}

func scanLocalReferences(data []byte, directory, root string, visited map[string]bool, collector *diagnostic.Collector) error {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil
	}
	return scanLocalReferenceValue(value, directory, root, visited, collector, nil)
}

func scanLocalReferenceValue(value any, directory, root string, visited map[string]bool, collector *diagnostic.Collector, cache *decodedSourceCache) error {
	var references []string
	collectExternalReferences(value, nil, &references)
	sort.Strings(references)
	for _, reference := range references {
		name, _, _ := strings.Cut(reference, "#")
		if name == "" || filepath.IsAbs(name) || strings.Contains(name, "://") || strings.HasPrefix(name, "file:") {
			continue
		}
		target, err := filepath.EvalSymlinks(filepath.Join(directory, filepath.FromSlash(name)))
		if err != nil || requireContainedPath(target, root) != nil || visited[target] {
			continue
		}
		visited[target] = true
		source, err := cache.load(target)
		if err != nil {
			continue
		}
		referencedValue := source.value
		collector.Extend(reservedExtensionDiagnosticsValue(referencedValue, target))
		if err := scanLocalReferenceValue(referencedValue, filepath.Dir(target), root, visited, collector, cache); err != nil {
			return err
		}
	}
	return nil
}

func collectExternalReferences(value any, path []string, result *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		if reference, _ := typed["$ref"].(string); reference != "" && !strings.HasPrefix(reference, "#") {
			*result = append(*result, reference)
		}
		for name, child := range typed {
			if name == "$ref" || referenceTraversalOpaque(path, name, child) {
				continue
			}
			collectExternalReferences(child, append(path, name), result)
		}
	case []any:
		for index, child := range typed {
			collectExternalReferences(child, append(path, fmt.Sprint(index)), result)
		}
	}
}

func referenceTraversalOpaque(path []string, name string, value any) bool {
	return openapiwalk.IsExtensionKey(path, name) ||
		(!openapiwalk.IsNamedMap(path) && openapiwalk.IsOpaqueDataField(name, value))
}
