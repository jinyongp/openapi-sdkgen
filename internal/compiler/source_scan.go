package sdkgen

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
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
	return scanLocalReferenceDocumentsValue(source, value, collector, nil, nil)
}

func scanLocalReferenceDocumentsValue(source inputSource, value any, collector *diagnostic.Collector, cache *decodedSourceCache, session *compatibilitySession) error {
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
	return scanLocalReferenceValue(value, source.fileBase, root, visited, collector, cache, session)
}

func scanLocalReferences(data []byte, directory, root string, visited map[string]bool, collector *diagnostic.Collector) error {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil
	}
	return scanLocalReferenceValue(value, directory, root, visited, collector, nil, nil)
}

func scanLocalReferenceValue(value any, directory, root string, visited map[string]bool, collector *diagnostic.Collector, cache *decodedSourceCache, session *compatibilitySession) error {
	var references []externalReferenceOccurrence
	collectExternalReferenceOccurrences(value, nil, &references)
	sort.Slice(references, func(i, j int) bool {
		if references[i].Reference != references[j].Reference {
			return references[i].Reference < references[j].Reference
		}
		return references[i].Context < references[j].Context
	})
	for _, occurrence := range references {
		name, _, _ := strings.Cut(occurrence.Reference, "#")
		if remote := externalReferenceSource(occurrence.Reference); remote != "" {
			session.registerSourceContext(remote, occurrence.Context)
			continue
		}
		if name == "" || filepath.IsAbs(name) || strings.HasPrefix(name, "file:") {
			continue
		}
		target, err := filepath.EvalSymlinks(filepath.Join(directory, filepath.FromSlash(name)))
		if err != nil || requireContainedPath(target, root) != nil {
			continue
		}
		session.registerSourceContext(target, occurrence.Context)
		if visited[target] {
			continue
		}
		visited[target] = true
		source, err := cache.load(target)
		if err != nil {
			continue
		}
		effective, err := session.effectiveSource(target, source, occurrence.Context)
		if err != nil {
			return err
		}
		referencedValue := effective.value
		collector.Extend(reservedExtensionDiagnosticsValue(referencedValue, target))
		if err := scanLocalReferenceValue(referencedValue, filepath.Dir(target), root, visited, collector, cache, session); err != nil {
			return err
		}
	}
	return nil
}

type externalReferenceOccurrence struct {
	Reference string
	Context   openapiwalk.ObjectContext
}

func hasExternalReference(value any, path []string) bool {
	if path == nil {
		path = make([]string, 0, 64)
	}
	switch typed := value.(type) {
	case map[string]any:
		if reference, _ := typed["$ref"].(string); reference != "" && !strings.HasPrefix(reference, "#") {
			return true
		}
		for name, child := range typed {
			if name == "$ref" || referenceTraversalOpaque(path, name, child) {
				continue
			}
			if hasExternalReference(child, append(path, name)) {
				return true
			}
		}
	case []any:
		for index, child := range typed {
			if hasExternalReference(child, append(path, strconv.Itoa(index))) {
				return true
			}
		}
	}
	return false
}

func collectExternalReferenceOccurrences(value any, path []string, result *[]externalReferenceOccurrence) {
	if path == nil {
		path = make([]string, 0, 64)
	}
	switch typed := value.(type) {
	case map[string]any:
		if reference, _ := typed["$ref"].(string); reference != "" && !strings.HasPrefix(reference, "#") {
			*result = append(*result, externalReferenceOccurrence{
				Reference: reference,
				Context:   openapiwalk.ObjectContextAt(path),
			})
		}
		for name, child := range typed {
			if name == "$ref" || referenceTraversalOpaque(path, name, child) {
				continue
			}
			collectExternalReferenceOccurrences(child, append(path, name), result)
		}
	case []any:
		for index, child := range typed {
			collectExternalReferenceOccurrences(child, append(path, strconv.Itoa(index)), result)
		}
	}
}

func referenceTraversalOpaque(path []string, name string, value any) bool {
	return openapiwalk.IsExtensionKey(path, name) ||
		(!openapiwalk.IsNamedMap(path) && openapiwalk.IsOpaqueDataField(name, value))
}
