package typescript

import (
	"fmt"
	"slices"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

// Features are semantic lowering facts, not searches through emitted source.
// Direction remains in the feature identity so encoders and decoders can be
// composed independently. Schema features are shared by both directions.
type runtimeFeature string

type runtimeFeatureSet map[runtimeFeature]bool

func (features runtimeFeatureSet) merge(other runtimeFeatureSet) bool {
	changed := false
	for feature := range other {
		if !features[feature] {
			features[feature] = true
			changed = true
		}
	}
	return changed
}

func (features runtimeFeatureSet) sorted() []runtimeFeature {
	result := make([]runtimeFeature, 0, len(features))
	for feature := range features {
		result = append(result, feature)
	}
	slices.Sort(result)
	return result
}

func (wire *wireRenderContext) recordRuntimeFeature(feature runtimeFeature) {
	if wire.execution == nil {
		return
	}
	if wire.execution.features == nil {
		wire.execution.features = make(runtimeFeatureSet)
	}
	wire.execution.features[feature] = true
}

// Called only at the actual descriptor field emission site, after projection
// filtering and reference lowering. Opaque const/enum values are never visited.
func (wire *wireRenderContext) recordRuntimeSchemaField(field string) {
	if wire.execution == nil {
		return
	}
	wire.recordRuntimeFeature(runtimeFeature("schema." + field))
}

func (wire *wireRenderContext) recordRuntimeSchemaValue(kind, value string) {
	if wire.execution != nil {
		wire.recordRuntimeFeature(runtimeFeature("schema." + kind + "." + value))
	}
}

func (wire *wireRenderContext) recordRuntimeMedia(role, contentType string, framing ir.StreamFraming, incremental bool) {
	if wire.execution == nil || contentType == "" {
		return
	}
	media := normalizedExecutionMedia(contentType)
	kind := "custom"
	switch {
	case strings.Contains(media, "*"):
		kind = "open"
	case isJSONMediaType(media):
		kind = "json"
	case executionXMLMedia(media):
		kind = "xml"
	case media == "application/x-www-form-urlencoded":
		kind = "form"
	case strings.HasPrefix(media, "multipart/"):
		kind = "multipart"
	case strings.HasPrefix(media, "text/"):
		kind = "text"
	case media == "application/octet-stream":
		kind = "binary"
	}
	wire.recordRuntimeFeature(runtimeFeature(role + ".media." + kind))
	if framing != "" && framing != ir.StreamFramingNone {
		mode := "complete"
		if incremental {
			mode = "incremental"
		}
		wire.recordRuntimeFeature(runtimeFeature(role + ".framing." + mode + "." + string(framing)))
		// Public protocol overrides can replace a built-in protocol, too.
		wire.recordRuntimeFeature(runtimeFeature(role + ".protocol-override"))
	}
	if kind == "multipart" {
		// Actual part Content-Type is an open dispatch boundary. In particular,
		// a default JSON inbound part can legally arrive as XML.
		wire.recordRuntimeFeature(runtimeFeature(role + ".open-part-media"))
	}
}

type runtimeFeatureRoot struct {
	kind     string
	identity string
	features []runtimeFeature
}

type runtimeFeaturePlan struct {
	operations map[string][]runtimeFeature
	inbound    []runtimeFeatureRoot
	shared     []runtimeFeature
	reasons    map[runtimeFeature][]string
	// Supported generic server helpers remain an independent compatibility
	// root. Router-specific compositions must not import this value facade.
	genericServer bool
}

func (planner *executionPlanner) runtimeSchemaClosure(facts executionSchemaFacts) (runtimeFeatureSet, error) {
	if _, _, _, err := planner.schemaClosure(facts); err != nil {
		return nil, err
	}
	return planner.loadedRuntimeFeatures(facts), nil
}

// schemaClosure has already completed cycle-safe feature propagation. Reading
// direct roots reuses that frozen summary without traversing the graph again.
func (planner *executionPlanner) loadedRuntimeFeatures(facts executionSchemaFacts) runtimeFeatureSet {
	result := make(runtimeFeatureSet)
	result.merge(facts.features)
	for reference := range facts.references {
		result.merge(planner.schemas[reference].features)
	}
	return result
}

func runtimeSecurityFeatures(document *ir.Document, requirements []operationSecurityRequirement, result runtimeFeatureSet) {
	for _, requirement := range requirements {
		for _, requested := range requirement.schemes {
			scheme, exists := securityScheme(document, requested.Name)
			if !exists {
				continue // Existing lowering owns diagnostics for unresolved schemes.
			}
			kind := scheme.Type
			if kind == "http" {
				kind += "." + strings.ToLower(scheme.Scheme)
			} else if kind == "apiKey" {
				kind += "." + scheme.Location
			}
			result[runtimeFeature("security."+kind)] = true
		}
	}
}

func (wire *wireRenderContext) recordInboundRuntimeSecurity(document *ir.Document, security any) {
	if wire.execution == nil {
		return
	}
	values, _ := security.([]any)
	for _, value := range values {
		requirement, _ := value.(map[string]any)
		for _, name := range sortedAnyKeys(requirement) {
			scheme, exists := securityScheme(document, name)
			if !exists {
				wire.recordRuntimeFeature("security.open")
				continue
			}
			wire.recordRuntimeFeature("inbound.authentication")
			features := make(runtimeFeatureSet)
			runtimeSecurityFeatures(document, []operationSecurityRequirement{{schemes: []ir.SecurityRequirementScheme{{Name: scheme.Name}}}}, features)
			for feature := range features {
				wire.recordRuntimeFeature(feature)
			}
		}
	}
}

func prepareRuntimeFeatures(plan *sourcePlan) (*runtimeFeaturePlan, error) {
	planner := newExecutionPlanner(plan.document)
	result := &runtimeFeaturePlan{operations: make(map[string][]runtimeFeature), reasons: make(map[runtimeFeature][]string), genericServer: plan.includeServer}
	union := make(runtimeFeatureSet)
	origins := make(map[runtimeFeature]map[string]bool)
	compositions := make(map[string]runtimeComposition)
	wireHandlers := make(map[string]runtimeWireHandlers)
	securityHandlers := make(map[string]runtimeSecurityHandlers)
	linkedRoutes := make(map[string]bool, len(plan.links))
	for _, link := range plan.links {
		linkedRoutes[operationRouteKey(link.SourceOperation)] = true
	}
	addRoot := func(identity string, features runtimeFeatureSet) {
		union.merge(features)
		for feature := range features {
			if origins[feature] == nil {
				origins[feature] = make(map[string]bool)
			}
			origins[feature][identity] = true
		}
	}
	for _, item := range plan.manifest.Operations {
		if item.Visibility == "hidden" {
			continue
		}
		features := make(runtimeFeatureSet)
		// Complex HTTP contracts retain the general request implementation.
		// This guard reads normalized contracts, never generated TypeScript.
		for _, parameter := range item.compiled.Parameters {
			if parameter.Location == "query" {
				features["http.query"] = true
			}
			if parameter.Location != "path" && parameter.Location != "query" || parameter.ContentType != "" {
				features["http.general"] = true
			}
		}
		if body := item.compiled.RequestBody; body != nil && len(body.Content) != 1 {
			features["http.general"] = true
		}
		for _, response := range item.compiled.Responses {
			if headers, ok := response.Raw["headers"].(map[string]any); ok && len(headers) > 0 {
				features["http.general"] = true
			}
		}
		for _, feature := range plan.executions[manifestRouteKey(item)].features {
			features[feature] = true
		}
		requirements, _, err := operationSecurityRequirements(plan.document, item.compiled)
		if err != nil {
			return nil, err
		}
		runtimeSecurityFeatures(plan.document, requirements, features)
		if item.compiled.PaginationPlan != nil {
			features["callable.pagination"] = true
		}
		if linkedRoutes[manifestRouteKey(item)] {
			features["callable.links"] = true
		}
		frozen := features.sorted()
		result.operations[manifestRouteKey(item)] = frozen
		execution := plan.executions[manifestRouteKey(item)]
		execution.features = frozen
		var key strings.Builder
		for _, feature := range frozen {
			key.WriteString(string(feature))
			key.WriteByte(0)
		}
		handlerKey := key.String()
		if _, exists := wireHandlers[handlerKey]; !exists {
			wireHandlers[handlerKey] = prepareWireHandlers(frozen)
			securityHandlers[handlerKey] = prepareSecurityHandlers(frozen)
		}
		execution.wireHandlers = wireHandlers[handlerKey]
		execution.securityHandlers = securityHandlers[handlerKey]
		inline := execution.inputBundle == "" && len(execution.inputSchemas) > 0 || execution.outputBundle == "" && len(execution.outputSchemas) > 0
		key.WriteString(fmt.Sprintf("%t/%t", execution.hasStream, inline))
		compositionKey := key.String()
		composition, exists := compositions[compositionKey]
		if !exists {
			composition = prepareRuntimeComposition(frozen, execution.hasStream)
			if inline {
				composition.wireTypes = append(composition.wireTypes, "WireSchemas")
			}
			compositions[compositionKey] = composition
		}
		execution.composition = composition
		plan.executions[manifestRouteKey(item)] = execution
		addRoot("operation "+manifestRouteKey(item), features)
	}
	addInbound := func(kind, identity string, facts executionSchemaFacts) error {
		features, err := planner.runtimeSchemaClosure(facts)
		if err != nil {
			return fmt.Errorf("%s %q runtime features: %w", kind, identity, err)
		}
		result.inbound = append(result.inbound, runtimeFeatureRoot{kind: kind, identity: identity, features: features.sorted()})
		addRoot(kind+" "+identity, features)
		return nil
	}
	for _, callback := range plan.callbacks {
		if err := addInbound("callback", callbackIdentity(callback), callback.runtimeFacts); err != nil {
			return nil, err
		}
	}
	for _, webhook := range plan.webhooks {
		if err := addInbound("webhook", webhook.name, webhook.runtimeFacts); err != nil {
			return nil, err
		}
	}
	result.shared = union.sorted()
	for feature, sources := range origins {
		result.reasons[feature] = sortedStringKeys(sources)
	}
	return result, nil
}

// Artifact and factory planning need only the client union, without rebuilding
// the operation/provenance indexes already frozen on the full feature plan.
func clientRuntimeFeatures(shared *runtimeFeaturePlan, manifest *Manifest) []runtimeFeature {
	union := make(runtimeFeatureSet)
	for _, item := range manifest.Operations {
		for _, feature := range shared.operations[manifestRouteKey(item)] {
			union[feature] = true
		}
	}
	return union.sorted()
}

// Client views reuse frozen operation summaries, but own their root union.
func scopedRuntimeFeatures(shared *runtimeFeaturePlan, manifest *Manifest, server bool) *runtimeFeaturePlan {
	if shared == nil {
		return nil
	}
	result := &runtimeFeaturePlan{operations: make(map[string][]runtimeFeature), reasons: make(map[runtimeFeature][]string), genericServer: server}
	union := make(runtimeFeatureSet)
	add := func(identity string, features []runtimeFeature) {
		for _, feature := range features {
			union[feature] = true
			result.reasons[feature] = append(result.reasons[feature], identity)
		}
	}
	for _, item := range manifest.Operations {
		route := manifestRouteKey(item)
		result.operations[route] = shared.operations[route]
		add("operation "+route, shared.operations[route])
	}
	if server {
		result.inbound = shared.inbound
		for _, root := range shared.inbound {
			add(root.kind+" "+root.identity, root.features)
		}
	}
	result.shared = union.sorted()
	for feature, origins := range result.reasons {
		values := make(map[string]bool)
		for _, origin := range origins {
			values[origin] = true
		}
		result.reasons[feature] = sortedStringKeys(values)
	}
	return result
}
