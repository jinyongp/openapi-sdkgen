package sdkgen

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/openapiwalk"
)

type referenceDiscovery struct {
	root           string
	cache          *decodedSourceCache
	session        *compatibilitySession
	remote         *remoteReferenceResolver
	remoteFailures map[string]error
	visited        map[compatibilitySourceKey]bool
	coverage       []diagnostic.AnalysisCoverage
}

func collectReferenceGraphDiagnostics(
	source inputSource,
	effective any,
	options *CompileOptions,
) ([]diagnostic.Diagnostic, []diagnostic.AnalysisCoverage, error) {
	state, err := ensureReferenceResolutionState(source, options)
	if err != nil {
		value := referenceConfigurationDiagnostic(source.display, err)
		return []diagnostic.Diagnostic{value}, []diagnostic.AnalysisCoverage{
			referenceConfigurationCoverage(source.display, err),
		}, nil
	}

	root := ""
	directory := source.fileBase
	currentSource := source.display
	if source.fileBase != "" {
		root, err = filepath.EvalSymlinks(source.fileBase)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve OpenAPI input directory: %w", err)
		}
		directory = root
	}
	if source.filePath != "" {
		resolved, resolveErr := filepath.EvalSymlinks(source.filePath)
		if resolveErr != nil {
			return nil, nil, fmt.Errorf("resolve OpenAPI reference file %s: %w", source.filePath, resolveErr)
		}
		currentSource = resolved
		directory = filepath.Dir(resolved)
	}

	discovery := &referenceDiscovery{
		root:           root,
		cache:          options.sourceCache,
		session:        options.compatibilitySession,
		remote:         state.remote,
		remoteFailures: make(map[string]error),
		visited:        make(map[compatibilitySourceKey]bool),
	}
	values, err := discovery.scanValue(effective, currentSource, directory, source.remoteBase)
	if err != nil {
		return nil, nil, err
	}
	return diagnostic.Sort(values), discovery.coverage, nil
}

func (discovery *referenceDiscovery) scanValue(
	value any,
	source string,
	directory string,
	remoteBase *url.URL,
) ([]diagnostic.Diagnostic, error) {
	var occurrences []externalReferenceOccurrence
	collectExternalReferenceOccurrences(value, nil, &occurrences)
	sort.Slice(occurrences, func(i, j int) bool {
		if occurrences[i].Reference != occurrences[j].Reference {
			return occurrences[i].Reference < occurrences[j].Reference
		}
		if occurrences[i].Context != occurrences[j].Context {
			return occurrences[i].Context < occurrences[j].Context
		}
		return occurrences[i].Pointer < occurrences[j].Pointer
	})

	for _, occurrence := range occurrences {
		remoteSource, _, remote, err := resolveRemoteReferenceSource(occurrence.Reference, remoteBase)
		if err == nil && remote {
			discovery.session.registerSourceContext(remoteSource, occurrence.Context)
		}
	}

	var result []diagnostic.Diagnostic
	for _, occurrence := range occurrences {
		name, _, _ := strings.Cut(occurrence.Reference, "#")
		if name == "" {
			continue
		}

		remoteSource, nextRemoteBase, remote, err := resolveRemoteReferenceSource(occurrence.Reference, remoteBase)
		if err != nil {
			result = append(result, referenceSourceDiagnostic(source, occurrence, err))
			discovery.coverage = append(discovery.coverage, referenceSourceCoverage(source, occurrence, err))
			continue
		}
		if remote {
			values, scanErr := discovery.scanRemoteOccurrence(source, occurrence, remoteSource, nextRemoteBase)
			if scanErr != nil {
				return nil, scanErr
			}
			result = append(result, values...)
			continue
		}

		values, scanErr := discovery.scanLocalOccurrence(source, directory, occurrence)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, values...)
	}
	return result, nil
}

func (discovery *referenceDiscovery) scanLocalOccurrence(
	source string,
	directory string,
	occurrence externalReferenceOccurrence,
) ([]diagnostic.Diagnostic, error) {
	if discovery.root == "" {
		err := fmt.Errorf("relative OpenAPI reference %q requires an input base", occurrence.Reference)
		discovery.coverage = append(discovery.coverage, referenceSourceCoverage(source, occurrence, err))
		return []diagnostic.Diagnostic{referenceSourceDiagnostic(source, occurrence, err)}, nil
	}

	target, err := resolveContainedReference(occurrence.Reference, directory, discovery.root, false)
	if err != nil {
		discovery.coverage = append(discovery.coverage, referenceSourceCoverage(source, occurrence, err))
		return []diagnostic.Diagnostic{referenceSourceDiagnostic(source, occurrence, err)}, nil
	}
	if target == "" {
		return nil, nil
	}

	discovery.session.registerSourceContext(target, occurrence.Context)
	key := compatibilitySourceKey{source: target, context: normalizedReferenceContext(occurrence.Context)}
	if discovery.visited[key] {
		return nil, nil
	}
	discovery.visited[key] = true

	input, err := discovery.cache.load(target)
	if err != nil {
		failureErr := fmt.Errorf("read OpenAPI reference file %s: %w", target, err)
		discovery.coverage = append(discovery.coverage, referenceSourceCoverage(source, occurrence, failureErr))
		return []diagnostic.Diagnostic{referenceSourceDiagnostic(source, occurrence, failureErr)}, nil
	}
	effective, err := discovery.session.effectiveSource(target, input, occurrence.Context)
	if err != nil {
		return nil, err
	}

	result := reservedExtensionDiagnosticsValue(effective.value, target)
	result = append(result, pathItemReferenceDiagnostics(effective.value, target, true)...)
	result = append(result, unresolvedLocalReferenceDiagnostics(effective.value, target)...)
	nested, err := discovery.scanValue(effective.value, target, filepath.Dir(target), nil)
	if err != nil {
		return nil, err
	}
	return append(result, nested...), nil
}

func (discovery *referenceDiscovery) scanRemoteOccurrence(
	source string,
	occurrence externalReferenceOccurrence,
	remoteSource string,
	remoteBase *url.URL,
) ([]diagnostic.Diagnostic, error) {
	if discovery.remote == nil {
		err := fmt.Errorf("OpenAPI reference %q must stay inside the input directory unless its HTTPS origin is allowlisted", occurrence.Reference)
		discovery.coverage = append(discovery.coverage, referenceSourceCoverage(source, occurrence, err))
		return []diagnostic.Diagnostic{referenceSourceDiagnostic(source, occurrence, err)}, nil
	}

	key := compatibilitySourceKey{source: remoteSource, context: normalizedReferenceContext(occurrence.Context)}
	if discovery.visited[key] {
		return nil, nil
	}
	discovery.visited[key] = true

	if previous, exists := discovery.remoteFailures[remoteSource]; exists {
		discovery.coverage = append(discovery.coverage, referenceSourceCoverage(source, occurrence, previous))
		return []diagnostic.Diagnostic{referenceSourceDiagnostic(source, occurrence, previous)}, nil
	}
	canonical, err := discovery.remote.prefetch(remoteSource)
	if err != nil {
		discovery.remoteFailures[remoteSource] = err
		discovery.coverage = append(discovery.coverage, referenceSourceCoverage(source, occurrence, err))
		return []diagnostic.Diagnostic{referenceSourceDiagnostic(source, occurrence, err)}, nil
	}
	body, ok := discovery.remote.source(canonical)
	if !ok {
		return nil, fmt.Errorf("remote reference %s was not retained after discovery", canonical)
	}
	decoded, err := discovery.remote.decodedSourceSnapshot(canonical, body)
	if err != nil {
		discovery.coverage = append(discovery.coverage, referenceSourceCoverage(source, occurrence, err))
		return []diagnostic.Diagnostic{referenceSourceDiagnostic(source, occurrence, err)}, nil
	}
	effective, err := discovery.session.effectiveSource(canonical, decoded, occurrence.Context)
	if err != nil {
		return nil, err
	}
	registerRemoteReferenceContexts(discovery.session, effective.value, canonical)

	result := reservedExtensionDiagnosticsValue(effective.value, canonical)
	result = append(result, pathItemReferenceDiagnostics(effective.value, canonical, true)...)
	result = append(result, unresolvedLocalReferenceDiagnostics(effective.value, canonical)...)
	nested, err := discovery.scanValue(effective.value, canonical, "", remoteBase)
	if err != nil {
		return nil, err
	}
	return append(result, nested...), nil
}

func resolveRemoteReferenceSource(reference string, base *url.URL) (string, *url.URL, bool, error) {
	name, _, _ := strings.Cut(reference, "#")
	if name == "" {
		return "", nil, false, nil
	}
	parsed, err := url.Parse(name)
	if err != nil {
		return "", nil, false, fmt.Errorf("parse OpenAPI reference %q: %w", reference, err)
	}
	if !parsed.IsAbs() {
		if base == nil {
			return "", nil, false, nil
		}
		parsed = base.ResolveReference(parsed)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		parsed.Fragment = ""
		return parsed.String(), documentURLBase(parsed), true, nil
	default:
		return "", nil, false, nil
	}
}

func normalizedReferenceContext(value openapiwalk.ObjectContext) openapiwalk.ObjectContext {
	if value == "" {
		return openapiwalk.ObjectUnknown
	}
	return value
}

func referenceConfigurationDiagnostic(source string, err error) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.SeverityError,
		Code:     "SDKGEN-E120",
		Phase:    diagnostic.PhaseReferences,
		Scope:    failure.ScopeDocument,
		Effect:   failure.EffectBlock,
		Location: diagnostic.Location{Source: source, Pointer: "#"},
		Message:  "Unable to configure OpenAPI reference resolution.",
		Cause:    sanitizeDiagnosticCause(err.Error()),
	}
}

func referenceConfigurationCoverage(source string, err error) diagnostic.AnalysisCoverage {
	return diagnostic.AnalysisCoverage{
		Phase:    diagnostic.PhaseReferences,
		Analyzer: "reference.graph",
		Status:   diagnostic.CoverageSkipped,
		Location: &diagnostic.Location{Source: source, Pointer: "#"},
		Scope:    failure.ScopeDocument,
		Prerequisites: []diagnostic.CoveragePrerequisite{
			{Name: "effective-source", Available: true},
		},
		Reason:    "reference resolution configuration is unavailable: " + sanitizeDiagnosticCause(err.Error()),
		BlockedBy: []string{"SDKGEN-E120"},
	}
}

func referenceSourceDiagnostic(source string, occurrence externalReferenceOccurrence, err error) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Severity: diagnostic.SeverityError,
		Code:     "SDKGEN-E120",
		Phase:    diagnostic.PhaseReferences,
		Scope:    failure.ScopeDocument,
		Effect:   failure.EffectBlock,
		Location: diagnostic.Location{Source: source, Pointer: occurrence.Pointer},
		Message:  fmt.Sprintf("Unable to resolve OpenAPI reference %q.", sanitizeDiagnosticCause(occurrence.Reference)),
		Cause:    sanitizeDiagnosticCause(err.Error()),
	}
}

func referenceSourceCoverage(source string, occurrence externalReferenceOccurrence, err error) diagnostic.AnalysisCoverage {
	return diagnostic.AnalysisCoverage{
		Phase:    diagnostic.PhaseReferences,
		Analyzer: "reference.source",
		Status:   diagnostic.CoverageSkipped,
		Location: &diagnostic.Location{Source: source, Pointer: occurrence.Pointer},
		Scope:    failure.ScopeDocument,
		Prerequisites: []diagnostic.CoveragePrerequisite{
			{Name: "effective-source", Available: true},
		},
		Reason:    "referenced source unavailable; its descendants were not inspected: " + sanitizeDiagnosticCause(err.Error()),
		BlockedBy: []string{"SDKGEN-E120"},
	}
}
