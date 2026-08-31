package sdkgen

import (
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
	"openapi-sdkgen/internal/compiler/ir"
)

func attachDocumentProvenanceValue(document *ir.Document, source inputSource, value any, remoteSources map[string][]byte, localSources *decodedSourceCache) {
	if document == nil {
		return
	}
	display := safeInputDisplay(source.display)
	document.Provenance = make(map[string]ir.Provenance)
	references := referenceProvenanceValue(value, display, source.effective, source.fileBase, remoteSources, localSources)
	document.ProvenanceIndex = newProvenanceIndex(document.Raw, display, references)
}

func referenceProvenance(data []byte, displaySource, resolutionSource, directory string, remoteSources map[string][]byte) map[string]ir.Provenance {
	var value any
	if yaml.Unmarshal(data, &value) != nil {
		return nil
	}
	return referenceProvenanceValue(value, displaySource, resolutionSource, directory, remoteSources, nil)
}

func referenceProvenanceValue(value any, displaySource, resolutionSource, directory string, remoteSources map[string][]byte, localSources *decodedSourceCache) map[string]ir.Provenance {
	if resolutionSource == "" {
		resolutionSource = displaySource
	}
	result := make(map[string]ir.Provenance)
	visiting := make(map[string]bool)
	var visit func(any, []string, string, string, string, string)
	visit = func(current any, path []string, currentSource, currentDisplay, currentDirectory, currentPointer string) {
		switch typed := current.(type) {
		case map[string]any:
			if reference, _ := typed["$ref"].(string); reference != "" && !strings.HasPrefix(reference, "#") {
				name, fragment, _ := strings.Cut(reference, "#")
				targetSource := name
				targetDirectory := ""
				if parsed, err := url.Parse(name); err == nil && parsed.IsAbs() {
					targetSource = parsed.String()
				} else if base, baseErr := url.Parse(currentSource); baseErr == nil && base.IsAbs() && (strings.EqualFold(base.Scheme, "http") || strings.EqualFold(base.Scheme, "https")) {
					targetSource = base.ResolveReference(parsed).String()
				} else if currentDirectory != "" {
					if absolute, err := filepath.Abs(filepath.Join(currentDirectory, filepath.FromSlash(name))); err == nil {
						if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
							absolute = resolved
						}
						targetSource = absolute
						targetDirectory = filepath.Dir(absolute)
					}
				}
				targetPointer := "#"
				if fragment != "" {
					if decoded, err := url.PathUnescape(fragment); err == nil {
						targetPointer += decoded
					}
				}
				pointer := sourceJSONPointer(path)
				result[pointer] = ir.Provenance{
					Primary: ir.SourceLocation{Source: targetSource, Pointer: targetPointer},
					Related: []ir.SourceLocation{{Source: currentDisplay, Pointer: currentPointer + "/$ref"}},
				}
				identity := targetSource + targetPointer
				if !visiting[identity] {
					visiting[identity] = true
					referenced := remoteSources[targetSource]
					var referencedValue any
					if targetDirectory != "" {
						if source, err := localSources.load(targetSource); err == nil {
							referenced = source.data
							referencedValue = source.value
						}
					}
					if len(referenced) != 0 {
						if referencedValue != nil || yaml.Unmarshal(referenced, &referencedValue) == nil {
							targetValue := referencedValue
							found := targetPointer == "#"
							if !found {
								targetValue, found = resolveLocalReference(referencedValue, targetPointer)
							}
							if found {
								visit(targetValue, path, targetSource, targetSource, targetDirectory, targetPointer)
							}
						}
					}
					delete(visiting, identity)
				}
			}
			for key, child := range typed {
				if key == "$ref" || referenceTraversalOpaque(path, key, child) {
					continue
				}
				token := strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
				visit(child, append(path, key), currentSource, currentDisplay, currentDirectory, currentPointer+"/"+token)
			}
		case []any:
			for index, child := range typed {
				token := itoa(index)
				visit(child, append(path, token), currentSource, currentDisplay, currentDirectory, currentPointer+"/"+token)
			}
		}
	}
	visit(value, nil, resolutionSource, displaySource, directory, "#")
	return result
}

type provenanceRange struct {
	bundledPrefix string
	sourcePrefix  string
	provenance    ir.Provenance
}

type provenanceIndex struct {
	rootSource string
	exact      map[string]ir.Provenance
	ranges     []provenanceRange
}

func newProvenanceIndex(raw map[string]any, rootSource string, references map[string]ir.Provenance) *provenanceIndex {
	index := &provenanceIndex{rootSource: rootSource, exact: references}
	pointers := make([]string, 0, len(references))
	for pointer := range references {
		pointers = append(pointers, pointer)
	}
	sort.Slice(pointers, func(left, right int) bool {
		if len(pointers[left]) == len(pointers[right]) {
			return pointers[left] < pointers[right]
		}
		return len(pointers[left]) < len(pointers[right])
	})
	type redirect struct {
		from string
		to   string
	}
	var redirects []redirect
	for _, pointer := range pointers {
		resolvedPointer := pointer
		value, found := resolveLocalReference(raw, resolvedPointer)
		if !found {
			for index := len(redirects) - 1; index >= 0; index-- {
				candidate := redirects[index]
				if pointer == candidate.from || strings.HasPrefix(pointer, candidate.from+"/") {
					resolvedPointer = candidate.to + strings.TrimPrefix(pointer, candidate.from)
					value, found = resolveLocalReference(raw, resolvedPointer)
					if found {
						break
					}
				}
			}
		}
		if !found {
			continue
		}
		provenance := references[pointer]
		if object, ok := value.(map[string]any); ok {
			if local, _ := object["$ref"].(string); strings.HasPrefix(local, "#/") {
				if _, targetFound := resolveLocalReference(raw, local); targetFound {
					redirects = append(redirects, redirect{from: pointer, to: local})
					index.ranges = append(index.ranges, provenanceRange{bundledPrefix: local, sourcePrefix: provenance.Primary.Pointer, provenance: provenance})
					continue
				}
			}
		}
		index.ranges = append(index.ranges, provenanceRange{bundledPrefix: resolvedPointer, sourcePrefix: provenance.Primary.Pointer, provenance: provenance})
	}
	sort.Slice(index.ranges, func(left, right int) bool {
		if len(index.ranges[left].bundledPrefix) == len(index.ranges[right].bundledPrefix) {
			return index.ranges[left].bundledPrefix < index.ranges[right].bundledPrefix
		}
		return len(index.ranges[left].bundledPrefix) > len(index.ranges[right].bundledPrefix)
	})
	return index
}

func (index *provenanceIndex) LookupProvenance(pointer string) (ir.Provenance, bool) {
	if index == nil {
		return ir.Provenance{}, false
	}
	if value, exists := index.exact[pointer]; exists {
		return cloneProvenance(value), true
	}
	for _, candidate := range index.ranges {
		if pointer != candidate.bundledPrefix && !strings.HasPrefix(pointer, candidate.bundledPrefix+"/") {
			continue
		}
		value := cloneProvenance(candidate.provenance)
		value.Primary.Pointer = candidate.sourcePrefix + strings.TrimPrefix(pointer, candidate.bundledPrefix)
		return value, true
	}
	return ir.Provenance{Primary: ir.SourceLocation{Source: index.rootSource, Pointer: pointer}}, true
}

func cloneProvenance(value ir.Provenance) ir.Provenance {
	value.Related = append([]ir.SourceLocation(nil), value.Related...)
	return value
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
