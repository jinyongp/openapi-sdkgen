package typescript

import (
	"fmt"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

// executionProfile names a tested composition of the shared request services.
// Unproven media combinations retain the full implementation, not missing hooks.
type executionProfile string

const (
	executionJSON        executionProfile = "json"
	executionBufferedXML executionProfile = "buffered-xml"
	executionJSONStream  executionProfile = "json-response-stream"
	executionGeneral     executionProfile = "general"
)

type executionSchemaReference struct {
	name      string
	direction projection
}

type executionSchemaCapabilities uint8

const (
	executionSchemaXML executionSchemaCapabilities = 1 << iota
	executionSchemaDynamic
)

// Facts are collected by the existing wire lowerer, after its projection and
// opaque-data rules. They never come from parsing the generated TypeScript.
type executionSchemaFacts struct {
	capabilities executionSchemaCapabilities
	references   map[executionSchemaReference]bool
}

func (wire *wireRenderContext) recordExecutionReference(name string, direction projection) {
	if wire.execution == nil {
		return
	}
	if wire.execution.references == nil {
		wire.execution.references = make(map[executionSchemaReference]bool)
	}
	wire.execution.references[executionSchemaReference{name: name, direction: direction}] = true
}

func (wire *wireRenderContext) recordExecutionCapability(capability executionSchemaCapabilities) {
	if wire.execution != nil {
		wire.execution.capabilities |= capability
	}
}

type executionSchemaNode struct {
	facts        executionSchemaFacts
	capabilities executionSchemaCapabilities
	parents      map[executionSchemaReference]bool
	loaded       bool
}

// The cache belongs to a single prepared document. Direction is part of the key;
// dialect and resource identity are those of that immutable lowered document.
// Partial visiting states are never cached as completed transitive summaries.
type executionPlanner struct {
	document *ir.Document
	schemas  map[executionSchemaReference]*executionSchemaNode
}

func newExecutionPlanner(document *ir.Document) *executionPlanner {
	return &executionPlanner{document: document, schemas: make(map[executionSchemaReference]*executionSchemaNode)}
}

type operationExecutionPlan struct {
	profile       executionProfile
	reasons       []string
	inputSchemas  []string
	outputSchemas []string
	inputBundle   string
	outputBundle  string
	hasStream     bool
}

func (planner *executionPlanner) schemaClosure(facts executionSchemaFacts) (executionSchemaCapabilities, []string, []string, error) {
	queue := sortedExecutionReferences(facts.references)
	seen := make(map[executionSchemaReference]bool)
	changed := make([]executionSchemaReference, 0)
	for len(queue) > 0 {
		key := queue[0]
		queue = queue[1:]
		if seen[key] {
			continue
		}
		seen[key] = true
		node := planner.schemas[key]
		if node == nil {
			node = &executionSchemaNode{parents: make(map[executionSchemaReference]bool)}
			planner.schemas[key] = node
		}
		if !node.loaded {
			value, ok := planner.document.Schemas[key.name]
			var schema any
			if ok {
				schema = value.Value
			} else {
				var exists bool
				schema, exists = planner.document.ComponentSchemas[key.name]
				if !exists {
					return 0, nil, nil, fmt.Errorf("execution schema references missing component %q", key.name)
				}
			}
			wire := newWireRenderContext(wirePropertiesConstructed)
			wire.execution = &node.facts
			if _, err := wire.wireSchemaDescriptorForDocument(planner.document, schema, key.direction); err != nil {
				return 0, nil, nil, err
			}
			node.capabilities = node.facts.capabilities
			node.loaded = true
			changed = append(changed, key)
		}
		for _, child := range sortedExecutionReferences(node.facts.references) {
			dependency := planner.schemas[child]
			if dependency == nil {
				dependency = &executionSchemaNode{parents: make(map[executionSchemaReference]bool)}
				planner.schemas[child] = dependency
			}
			dependency.parents[key] = true
			combined := node.capabilities | dependency.capabilities
			if combined != node.capabilities {
				node.capabilities = combined
				changed = append(changed, key)
			}
			queue = append(queue, child)
		}
	}
	// Monotone propagation needs at most one growth per capability per node.
	// In particular, A -> B -> A plus a later XML edge updates BOTH A and B.
	for len(changed) > 0 {
		key := changed[0]
		changed = changed[1:]
		node := planner.schemas[key]
		for parent := range node.parents {
			owner := planner.schemas[parent]
			combined := owner.capabilities | node.capabilities
			if combined != owner.capabilities {
				owner.capabilities = combined
				changed = append(changed, parent)
			}
		}
	}
	capabilities := facts.capabilities
	input, output := make(map[string]bool), make(map[string]bool)
	for key := range seen {
		capabilities |= planner.schemas[key].capabilities
		if key.direction == projectionInput {
			input[key.name] = true
		} else {
			output[key.name] = true
		}
	}
	return capabilities, sortedStringKeys(input), sortedStringKeys(output), nil
}

// prepareOperationExecutions resolves every emitted operation before output
// publication. The emitter only consumes these document-owned immutable facts.
func prepareOperationExecutions(document *ir.Document, manifest Manifest, modules *semanticModulePlan, streams []generatedStream) (map[string]operationExecutionPlan, error) {
	items := make(map[string]ManifestOperation, len(manifest.Operations))
	for _, item := range manifest.Operations {
		items[manifestRouteKey(item)] = item
	}
	streamRoutes := make(map[string]bool, len(streams))
	for _, stream := range streams {
		streamRoutes[operationRouteKey(stream.Operation)] = true
	}
	planner := newExecutionPlanner(document)
	result := make(map[string]operationExecutionPlan, len(modules.operations))
	for _, module := range modules.operations {
		item, exists := items[module.routeKey]
		if !exists {
			return nil, fmt.Errorf("execution %q has no manifest operation", module.routeKey)
		}
		execution, err := planner.operation(item.compiled, item, streamRoutes[module.routeKey])
		if err != nil {
			return nil, fmt.Errorf("operation %q: %w", module.routeKey, err)
		}
		result[module.routeKey] = execution
	}
	return result, nil
}

func sortedExecutionReferences(values map[executionSchemaReference]bool) []executionSchemaReference {
	result := make([]executionSchemaReference, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].name != result[j].name {
			return result[i].name < result[j].name
		}
		return result[i].direction < result[j].direction
	})
	return result
}

func normalizedExecutionMedia(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.SplitN(value, ";", 2)[0]))
}

func executionXMLMedia(value string) bool {
	value = normalizedExecutionMedia(value)
	return value == "application/xml" || value == "text/xml" || strings.HasSuffix(value, "+xml")
}

func (planner *executionPlanner) operation(operation ir.Operation, item ManifestOperation, hasStream bool) (operationExecutionPlan, error) {
	wire := newWireRenderContext(wirePropertiesConstructed)
	facts := executionSchemaFacts{}
	wire.execution = &facts
	// This is the same semantic lowering used by the operation binder. Its text
	// is not inspected; the attached facts only observe semantic decisions.
	if _, err := wire.operationDefinition(planner.document, operation, item); err != nil {
		return operationExecutionPlan{}, err
	}
	capabilities, input, output, err := planner.schemaClosure(facts)
	if err != nil {
		return operationExecutionPlan{}, err
	}
	result := operationExecutionPlan{profile: executionJSON, inputSchemas: input, outputSchemas: output, hasStream: hasStream}
	reasons := make(map[string]bool)
	general := capabilities&executionSchemaDynamic != 0
	needsXML := capabilities&executionSchemaXML != 0
	if general {
		reasons["dynamic-schema-scope"] = true
	}
	if needsXML {
		reasons["schema-content-xml"] = true
	}
	jsonRequest := true
	for _, parameter := range item.prepared.clientParameters {
		if executionXMLMedia(parameter.ContentType) {
			needsXML = true
			reasons["xml-parameter"] = true
		}
		if strings.Contains(parameter.ContentType, "*") {
			general = true
			reasons["parameter-media-range"] = true
		}
	}
	body, err := operationRequestBody(planner.document, operation)
	if err != nil {
		return operationExecutionPlan{}, err
	}
	if body != nil {
		for _, media := range body.Content {
			name := normalizedExecutionMedia(media.ContentType)
			if !isJSONMediaType(name) {
				jsonRequest = false
			}
			if executionXMLMedia(name) {
				needsXML = true
				reasons["xml-request"] = true
			}
			if media.Stream.IsStreaming() || media.ItemSchema != nil {
				general = true
				reasons["request-stream"] = true
			}
			if strings.Contains(name, "*") || name == "" {
				general = true
				reasons["request-media-range"] = true
			}
			if strings.HasPrefix(name, "multipart/") || name == "application/x-www-form-urlencoded" {
				general = true
				reasons["structured-request-body"] = true
			}
		}
	}
	responseStream := hasStream
	responses, err := operationResponses(planner.document, operation)
	if err != nil {
		return operationExecutionPlan{}, err
	}
	for _, response := range responses {
		for _, media := range response.Content {
			name := normalizedExecutionMedia(media.ContentType)
			if executionXMLMedia(name) {
				needsXML = true
				reasons["xml-response"] = true
			}
			if strings.Contains(name, "*") {
				general = true
				reasons["response-media-range"] = true
			}
			if strings.HasPrefix(name, "multipart/") || media.Stream.Framing == ir.StreamFramingMultipart {
				general = true
				reasons["multipart-response"] = true
			}
			if media.Stream.IsStreaming() || media.ItemSchema != nil {
				responseStream = true
				reasons["response-stream"] = true
			}
		}
		headers, _ := response.Raw["headers"].(map[string]any)
		for _, name := range sortedAnyKeys(headers) {
			header, _ := headers[name].(map[string]any)
			header, err = resolveComponentObject(planner.document, header, "headers")
			if err != nil {
				return operationExecutionPlan{}, err
			}
			_, media, err := responseHeaderSchema(planner.document, header)
			if err != nil {
				return operationExecutionPlan{}, err
			}
			if executionXMLMedia(media) {
				needsXML = true
				reasons["xml-response-header"] = true
			}
			if strings.Contains(media, "*") {
				general = true
				reasons["header-media-range"] = true
			}
		}
	}
	switch {
	case general:
		result.profile = executionGeneral
	case responseStream && (needsXML || !jsonRequest):
		result.profile = executionGeneral
		reasons["mixed-stream-services"] = true
	case responseStream:
		result.profile = executionJSONStream
	case needsXML:
		result.profile = executionBufferedXML
	case !jsonRequest:
		result.profile = executionGeneral
		reasons["non-json-request-body"] = true
	}
	result.reasons = sortedStringKeys(reasons)
	return result, nil
}
