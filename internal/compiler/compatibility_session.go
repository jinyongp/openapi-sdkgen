package sdkgen

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"

	"go.yaml.in/yaml/v4"

	"openapi-sdkgen/internal/compiler/compatibility"
	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"openapi-sdkgen/internal/openapiwalk"
)

// compatibilitySession owns one compile's entry OpenAPI line and policy.
// Referenced OpenAPI objects inherit this version line; Schema dialect/resource
// semantics remain handled by the compiler schema layer.
type compatibilitySession struct {
	version openapidoc.VersionLine
	policy  compatibility.Policy

	mu       sync.Mutex
	findings []compatibility.Finding
	ledger   []compatibility.LedgerEntry
	contexts map[string]map[openapiwalk.ObjectContext]struct{}
}

func newCompatibilitySession(root any, policy compatibility.Policy) *compatibilitySession {
	if policy == nil {
		policy = compatibility.NoopPolicy{}
	}
	var version openapidoc.VersionLine
	if object, ok := root.(map[string]any); ok {
		if declared, ok := object["openapi"].(string); ok {
			version, _ = openapidoc.DetectVersionLine(declared)
		}
	}
	return &compatibilitySession{version: version, policy: policy, contexts: make(map[string]map[openapiwalk.ObjectContext]struct{})}
}

func (session *compatibilitySession) effectiveValue(source string, value any) (any, bool, error) {
	return session.effectiveValueInContext(source, value, openapiwalk.ObjectOpenAPI)
}

func (session *compatibilitySession) effectiveValueInContext(source string, value any, root openapiwalk.ObjectContext) (any, bool, error) {
	if session == nil {
		return value, false, nil
	}
	session.registerSourceContext(source, root)
	effective, omitted, changed, err := session.walk(source, value, nil, root)
	if err != nil {
		return nil, false, err
	}
	if omitted {
		return nil, true, fmt.Errorf("compatibility policy cannot omit the OpenAPI entry document")
	}
	return effective, changed, nil
}

func (session *compatibilitySession) walk(source string, value any, path []string, root openapiwalk.ObjectContext) (any, bool, bool, error) {
	object := openapiwalk.ObjectContextAt(path)
	if len(path) == 0 && root != "" && root != openapiwalk.ObjectUnknown {
		object = root
	}
	result := session.policy.Apply(compatibility.Context{
		Version: session.version,
		Object:  object,
		Source:  source,
		Pointer: sourceJSONPointer(path),
	}, value)
	session.record(result)
	if result.Omit {
		return nil, true, true, nil
	}
	current := result.Value
	changed := result.Changed

	switch typed := current.(type) {
	case map[string]any:
		names := make([]string, 0, len(typed))
		for name := range typed {
			names = append(names, name)
		}
		sort.Strings(names)
		var copied map[string]any
		for _, name := range names {
			child := typed[name]
			if name == "$ref" || referenceTraversalOpaque(path, name, child) {
				continue
			}
			effective, omitted, childChanged, err := session.walk(source, child, append(path, name), root)
			if err != nil {
				return nil, false, false, err
			}
			if !omitted && !childChanged {
				continue
			}
			if copied == nil {
				copied = make(map[string]any, len(typed))
				for key, original := range typed {
					copied[key] = original
				}
			}
			if omitted {
				delete(copied, name)
			} else {
				copied[name] = effective
			}
			changed = true
		}
		if copied != nil {
			current = copied
		}
	case []any:
		var copied []any
		for index, child := range typed {
			effective, omitted, childChanged, err := session.walk(source, child, append(path, fmt.Sprint(index)), root)
			if err != nil {
				return nil, false, false, err
			}
			if !omitted && !childChanged {
				continue
			}
			if copied == nil {
				copied = append([]any(nil), typed...)
			}
			if omitted {
				copied[index] = omittedArrayValue{}
			} else {
				copied[index] = effective
			}
			changed = true
		}
		if copied != nil {
			filtered := copied[:0]
			for _, child := range copied {
				if _, omitted := child.(omittedArrayValue); omitted {
					continue
				}
				filtered = append(filtered, child)
			}
			current = filtered
		}
	}
	return current, false, changed, nil
}

type omittedArrayValue struct{}

func (session *compatibilitySession) record(result compatibility.Result) {
	if len(result.Findings) == 0 && len(result.Ledger) == 0 {
		return
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	session.findings = append(session.findings, result.Findings...)
	session.ledger = append(session.ledger, result.Ledger...)
}

func (session *compatibilitySession) effectiveSource(source string, input decodedSource, context openapiwalk.ObjectContext) (decodedSource, error) {
	effective, changed, err := session.effectiveValueInContext(source, input.value, context)
	if err != nil {
		return decodedSource{}, err
	}
	if !changed {
		return input, nil
	}
	data, err := json.Marshal(effective)
	if err != nil {
		return decodedSource{}, fmt.Errorf("encode effective OpenAPI source %s: %w", source, err)
	}
	var normalized any
	if err := yaml.Unmarshal(data, &normalized); err != nil {
		return decodedSource{}, fmt.Errorf("decode effective OpenAPI source %s: %w", source, err)
	}
	return decodedSource{data: data, value: normalized}, nil
}

func (session *compatibilitySession) registerSourceContext(source string, context openapiwalk.ObjectContext) {
	if session == nil || source == "" {
		return
	}
	if context == "" {
		context = openapiwalk.ObjectUnknown
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	values := session.contexts[source]
	if values == nil {
		values = make(map[openapiwalk.ObjectContext]struct{})
		session.contexts[source] = values
	}
	values[context] = struct{}{}
}

func (session *compatibilitySession) sourceContext(source string) openapiwalk.ObjectContext {
	if session == nil {
		return openapiwalk.ObjectUnknown
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	values := session.contexts[source]
	if len(values) != 1 {
		return openapiwalk.ObjectUnknown
	}
	for value := range values {
		return value
	}
	return openapiwalk.ObjectUnknown
}

func (session *compatibilitySession) sourceContexts(source string) []openapiwalk.ObjectContext {
	if session == nil {
		return nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	values := session.contexts[source]
	result := make([]openapiwalk.ObjectContext, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func registerRemoteReferenceContexts(session *compatibilitySession, value any, baseSource string) {
	if session == nil {
		return
	}
	var base *url.URL
	if baseSource != "" {
		base, _ = url.Parse(baseSource)
	}
	var occurrences []externalReferenceOccurrence
	collectExternalReferenceOccurrences(value, nil, &occurrences)
	for _, occurrence := range occurrences {
		if strings.HasPrefix(occurrence.Reference, "#") {
			continue
		}
		reference, err := url.Parse(occurrence.Reference)
		if err != nil {
			continue
		}
		if !reference.IsAbs() {
			if base == nil {
				continue
			}
			reference = base.ResolveReference(reference)
		}
		if reference.Scheme != "http" && reference.Scheme != "https" {
			continue
		}
		reference.Fragment = ""
		session.registerSourceContext(reference.String(), occurrence.Context)
	}
}

func (session *compatibilitySession) evidence() ([]compatibility.Finding, []compatibility.LedgerEntry) {
	if session == nil {
		return nil, nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return append([]compatibility.Finding(nil), session.findings...), append([]compatibility.LedgerEntry(nil), session.ledger...)
}

func prepareCompatibilityValue(source string, value any, options *CompileOptions) (any, bool, error) {
	if options.compatibilitySession == nil {
		options.compatibilitySession = newCompatibilitySession(value, options.compatibilityPolicy)
	}
	return options.compatibilitySession.effectiveValue(source, value)
}
