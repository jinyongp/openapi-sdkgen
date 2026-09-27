package sdkgen

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"openapi-sdkgen/internal/compiler/compatibility"
	"openapi-sdkgen/internal/compiler/ir"
	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/openapiwalk"
)

// compatibilitySession owns one compile's entry OpenAPI line and policy.
// Referenced OpenAPI objects inherit this version line; Schema dialect/resource
// semantics remain handled by the compiler schema layer.
type compatibilitySession struct {
	version        openapidoc.VersionLine
	policy         compatibility.Policy
	consumerPolicy bool
	noopPolicy     bool

	mu                 sync.Mutex
	findings           []compatibility.Finding
	collectorFindingAt int
	ledger             []compatibility.LedgerEntry
	contexts           map[string]map[openapiwalk.ObjectContext]struct{}
	effectiveSources   map[compatibilitySourceKey]decodedSource
}

type compatibilitySourceKey struct {
	source  string
	context openapiwalk.ObjectContext
}

func newCompatibilitySession(root any, policy compatibility.Policy) *compatibilitySession {
	if policy == nil {
		policy = compatibility.ConsumerPolicy{}
	}
	var version openapidoc.VersionLine
	if object, ok := root.(map[string]any); ok {
		if declared, ok := object["openapi"].(string); ok {
			version, _ = openapidoc.DetectVersionLine(declared)
		}
	}
	_, consumerPolicy := policy.(compatibility.ConsumerPolicy)
	_, noopPolicy := policy.(compatibility.NoopPolicy)
	return &compatibilitySession{
		version:          version,
		policy:           policy,
		consumerPolicy:   consumerPolicy,
		noopPolicy:       noopPolicy,
		contexts:         make(map[string]map[openapiwalk.ObjectContext]struct{}),
		effectiveSources: make(map[compatibilitySourceKey]decodedSource),
	}
}

func (session *compatibilitySession) effectiveValue(source string, value any) (any, bool, error) {
	return session.effectiveValueInContext(source, value, openapiwalk.ObjectOpenAPI)
}

func (session *compatibilitySession) effectiveValueInContext(source string, value any, root openapiwalk.ObjectContext) (any, bool, error) {
	if session == nil {
		return value, false, nil
	}
	session.registerSourceContext(source, root)
	if session.noopPolicy {
		return value, false, nil
	}
	// OpenAPI 3.1+ Schema Objects use native JSON Schema semantics. Consumer
	// compatibility rules do not apply inside a source whose root is already a
	// Schema Object, so avoid recursively walking external schema resources.
	if session.consumerPolicy && root == openapiwalk.ObjectSchema && session.version != openapidoc.Version30 {
		return value, false, nil
	}
	effective, omitted, changed, err := session.walk(source, value, value, make([]string, 0, 64), root)
	if err != nil {
		return nil, false, err
	}
	if omitted {
		if root == "" || root == openapiwalk.ObjectOpenAPI {
			return nil, true, fmt.Errorf("compatibility policy cannot omit the OpenAPI entry document")
		}
		// A referenced reusable-object source may be ignored as a whole. Expose
		// an empty object to the controlled loader so the referencing occurrence
		// becomes semantically inert while exact source bytes remain in provenance.
		return map[string]any{}, true, nil
	}
	return effective, changed, nil
}

func (session *compatibilitySession) walk(source string, sourceRoot, value any, path []string, root openapiwalk.ObjectContext) (any, bool, bool, error) {
	current := value
	changed := false
	applyPolicy := true
	var position openapiwalk.StructuralPosition
	positionResolved := false
	resolvePosition := func() openapiwalk.StructuralPosition {
		if positionResolved {
			return position
		}
		position = openapiwalk.StructuralPositionAtRoot(root, path)
		positionResolved = true
		return position
	}
	if session.consumerPolicy {
		switch value.(type) {
		case map[string]any, bool:
			position := resolvePosition()
			// OpenAPI 3.1+ Schema subtrees already use native JSON Schema
			// semantics and contain no ConsumerPolicy-owned OpenAPI objects.
			// Prune them wholesale instead of walking every nested schema node.
			if position.Object == openapiwalk.ObjectSchema && session.version != openapidoc.Version30 {
				return value, false, false, nil
			}
			applyPolicy = compatibility.ConsumerPolicyMayApply(session.version, position.Object, value)
		default:
			applyPolicy = false
		}
	}
	if applyPolicy {
		position := resolvePosition()
		context := compatibility.Context{
			Version: session.version,
			Object:  position.Object,
			Keyword: position.Keyword,
			Source:  source,
			Pointer: sourceJSONPointer(path),
		}
		if classified, ok := session.resolveToClassify(context, sourceRoot, value); ok {
			session.record(classified)
			return nil, true, true, nil
		}
		result := session.policy.Apply(context, value)
		session.record(result)
		if result.Reject {
			if result.Scope != failure.ScopeNone && result.Scope != failure.ScopeDocument {
				return nil, true, true, nil
			}
			return result.Value, false, result.Changed, nil
		}
		if result.Omit {
			return nil, true, true, nil
		}
		current = result.Value
		changed = result.Changed
	}

	switch typed := current.(type) {
	case map[string]any:
		var copied map[string]any
		for name, child := range typed {
			if name == "$ref" || openapiwalk.ReferenceChildOpaque(path, name, child) {
				continue
			}
			effective, omitted, childChanged, err := session.walk(source, sourceRoot, child, append(path, name), root)
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
			effective, omitted, childChanged, err := session.walk(source, sourceRoot, child, append(path, strconv.Itoa(index)), root)
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

func (session *compatibilitySession) resolveToClassify(context compatibility.Context, sourceRoot, value any) (compatibility.Result, bool) {
	if context.Object != openapiwalk.ObjectParameter && context.Object != openapiwalk.ObjectLink {
		return compatibility.Result{}, false
	}
	object, ok := value.(map[string]any)
	if !ok {
		return compatibility.Result{}, false
	}
	if reference, _ := object["$ref"].(string); reference == "" {
		return compatibility.Result{}, false
	}
	current := value
	visited := make(map[string]bool)
	for {
		candidate, ok := current.(map[string]any)
		if !ok {
			return compatibility.Result{}, false
		}
		reference, _ := candidate["$ref"].(string)
		if reference == "" {
			break
		}
		if !strings.HasPrefix(reference, "#") || visited[reference] {
			return compatibility.Result{}, false
		}
		visited[reference] = true
		resolved, found := resolveLocalReference(sourceRoot, reference)
		if !found {
			return compatibility.Result{}, false
		}
		current = resolved
	}
	result := session.policy.Apply(context, current)
	switch context.Object {
	case openapiwalk.ObjectParameter:
		if !result.Omit {
			return compatibility.Result{}, false
		}
	case openapiwalk.ObjectLink:
		if !result.Reject || result.Scope != failure.ScopeCapability {
			return compatibility.Result{}, false
		}
	default:
		return compatibility.Result{}, false
	}
	return result, true
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
	if session == nil {
		return input, nil
	}
	if context == "" {
		context = openapiwalk.ObjectUnknown
	}
	key := compatibilitySourceKey{source: source, context: context}
	session.mu.Lock()
	cached, exists := session.effectiveSources[key]
	session.mu.Unlock()
	if exists {
		return cached, nil
	}

	effective, changed, err := session.effectiveValueInContext(source, input.value, context)
	if err != nil {
		return decodedSource{}, err
	}
	result := input
	if changed {
		data, err := json.Marshal(effective)
		if err != nil {
			return decodedSource{}, fmt.Errorf("encode effective OpenAPI source %s: %w", source, err)
		}
		// effective is already the decoded semantic view. Keep it rather than
		// decoding the just-encoded JSON into a second tree.
		result = decodedSource{data: data, value: effective}
	}
	session.mu.Lock()
	if existing, ok := session.effectiveSources[key]; ok {
		result = existing
	} else {
		session.effectiveSources[key] = result
	}
	session.mu.Unlock()
	return result, nil
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
	registerRemoteReferenceContextsAtRoot(session, value, baseSource, openapiwalk.ObjectOpenAPI)
}

func registerRemoteReferenceContextsAtRoot(session *compatibilitySession, value any, baseSource string, rootContext openapiwalk.ObjectContext) {
	if session == nil {
		return
	}
	var base *url.URL
	if baseSource != "" {
		base, _ = url.Parse(baseSource)
	}
	var occurrences []externalReferenceOccurrence
	collectExternalReferenceOccurrencesAtRoot(value, nil, rootContext, &occurrences)
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

func (session *compatibilitySession) drainCollectorDiagnostics() []diagnostic.Diagnostic {
	if session == nil {
		return nil
	}
	session.mu.Lock()
	start := session.collectorFindingAt
	if start > len(session.findings) {
		start = len(session.findings)
	}
	findings := append([]compatibility.Finding(nil), session.findings[start:]...)
	session.collectorFindingAt = len(session.findings)
	session.mu.Unlock()

	result := make([]diagnostic.Diagnostic, 0, len(findings))
	for _, finding := range findings {
		result = append(result, compatibilityDiagnostic(finding))
	}
	return diagnostic.Sort(result)
}

func (session *compatibilitySession) evidence() ([]compatibility.Finding, []compatibility.LedgerEntry) {
	if session == nil {
		return nil, nil
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	return append([]compatibility.Finding(nil), session.findings...), append([]compatibility.LedgerEntry(nil), session.ledger...)
}

func compatibilityDiagnostic(finding compatibility.Finding) diagnostic.Diagnostic {
	severity := diagnostic.SeverityWarning
	code := "SDKGEN-W140"
	hint := "Review the compatibility behavior before relying on it as portable OpenAPI semantics."
	if finding.Action == compatibility.ActionReject {
		if finding.Effect == failure.EffectOmitOperation || finding.Effect == failure.EffectOmitCapability {
			hint = "The rejected semantic scope is omitted from generated target behavior."
		} else {
			severity = diagnostic.SeverityError
			code = "SDKGEN-E140"
			hint = "Remove or rewrite the construct so its semantics can be generated safely."
		}
	}
	return diagnostic.Diagnostic{
		Severity: severity,
		Code:     code,
		Phase:    diagnostic.PhaseOpenAPI,
		Location: diagnostic.Location{Source: finding.Source, Pointer: finding.Pointer},
		Scope:    finding.Scope,
		Effect:   finding.Effect,
		Rule:     finding.RuleID,
		Action:   string(finding.Action),
		Message:  finding.Message,
		Hint:     hint,
	}
}

func semanticRestrictionForFinding(finding compatibility.Finding, ownerPointer string) (ir.SemanticRestriction, bool) {
	if finding.Action != compatibility.ActionReject ||
		finding.Scope == failure.ScopeNone || finding.Scope == failure.ScopeDocument {
		return ir.SemanticRestriction{}, false
	}
	return ir.SemanticRestriction{
		RuleID:       finding.RuleID,
		Conformance:  string(finding.Conformance),
		Disposition:  string(finding.Disposition),
		Action:       string(finding.Action),
		Impact:       string(finding.Impact),
		Scope:        finding.Scope,
		Effect:       finding.Effect,
		Location:     ir.SourceLocation{Source: finding.Source, Pointer: finding.Pointer},
		OwnerPointer: ownerPointer,
		Message:      finding.Message,
	}, true
}

func compatibilityDiagnostics(session *compatibilitySession) []diagnostic.Diagnostic {
	findings, _ := session.evidence()
	result := make([]diagnostic.Diagnostic, 0, len(findings))
	for _, finding := range findings {
		result = append(result, compatibilityDiagnostic(finding))
	}
	return diagnostic.Sort(result)
}

func compatibilityRestrictions(session *compatibilitySession) []ir.SemanticRestriction {
	findings, _ := session.evidence()
	result := make([]ir.SemanticRestriction, 0, len(findings))
	for _, finding := range findings {
		if restriction, ok := semanticRestrictionForFinding(finding, ""); ok {
			result = append(result, restriction)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Location.Source != result[j].Location.Source {
			return result[i].Location.Source < result[j].Location.Source
		}
		if result[i].Location.Pointer != result[j].Location.Pointer {
			return result[i].Location.Pointer < result[j].Location.Pointer
		}
		return result[i].RuleID < result[j].RuleID
	})
	return result
}

func isScopedCompatibilityRejectDiagnostic(value diagnostic.Diagnostic, session *compatibilitySession) bool {
	if value.Severity != diagnostic.SeverityError ||
		value.Phase != diagnostic.PhaseOpenAPI ||
		value.Action != string(compatibility.ActionReject) ||
		value.Scope == failure.ScopeNone || value.Scope == failure.ScopeDocument {
		return false
	}
	findings, _ := session.evidence()
	for _, finding := range findings {
		if finding.Action == compatibility.ActionReject &&
			finding.Scope == value.Scope &&
			finding.Effect == value.Effect &&
			finding.RuleID == value.Rule &&
			finding.Source == value.Location.Source &&
			finding.Pointer == value.Location.Pointer {
			return true
		}
	}
	return false
}

func prepareCompatibilityValue(source string, value any, options *CompileOptions) (any, bool, error) {
	if options.compatibilitySession == nil {
		options.compatibilitySession = newCompatibilitySession(value, options.compatibilityPolicy)
	}
	effective, changed, err := options.compatibilitySession.effectiveValue(source, value)
	if err != nil {
		return nil, false, err
	}
	if err := syncCompatibilityDiagnostics(options); err != nil {
		return nil, changed, err
	}
	return effective, changed, nil
}

func syncCompatibilityDiagnostics(options *CompileOptions) error {
	if options == nil || options.compatibilitySession == nil {
		return nil
	}
	if options.diagnostics != nil {
		options.diagnostics.Extend(options.compatibilitySession.drainCollectorDiagnostics())
		return nil
	}
	for _, value := range compatibilityDiagnostics(options.compatibilitySession) {
		if value.Severity == diagnostic.SeverityError {
			return phaseError(
				diagnostic.PhaseOpenAPI,
				fmt.Errorf("%s at %s%s", value.Message, safeInputDisplay(value.Location.Source), value.Location.Pointer),
			)
		}
	}
	return nil
}
