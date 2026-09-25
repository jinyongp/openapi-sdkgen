package typescript

import (
	"fmt"

	"openapi-sdkgen/internal/compiler/ir"
)

type registryIdentifierNames struct {
	base, pagination, links, stream                            string // Imported factories.
	value, baseValue, paginationValue, linksValue, streamValue string
}

func planRegistryIdentifiers(owner string, manifest Manifest, operations map[string]ir.Operation, links, streams map[string]bool) (map[string]registryIdentifierNames, error) {
	plan := newAggregateIdentifierPlan(owner)
	if err := plan.reserve("assignCallableProperties", "RequestFunction", "WireSchemas", "defineOwnDataProperty", "Routes", "CallableRegistry", "Route", "createCallableRegistry", "request", "inputSchemas", "outputSchemas", "completed", "operations", "linkCalls"); err != nil {
		return nil, err
	}
	entities := make(map[string]aggregateEntityKey)
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" {
			continue
		}
		route := manifestRouteKey(operation)
		if _, exists := entities[route]; exists {
			return nil, fmt.Errorf("registry %q has duplicate route %q", owner, route)
		}
		compiled, exists := operations[route]
		if !exists || operationRouteKey(compiled) != route {
			return nil, fmt.Errorf("registry %q has no matching compiled operation for route %q", owner, route)
		}
		entity := newAggregateEntity("route", route)
		entities[route] = entity
		roles := []aggregateIdentifierRole{aggregateBaseFactory, aggregateBaseValue, aggregateOperationValue}
		if operations[route].PaginationPlan != nil {
			roles = append(roles, aggregatePaginationFactory, aggregatePaginationValue)
		}
		if links[route] {
			roles = append(roles, aggregateLinksFactory, aggregateLinksValue)
		}
		if streams[route] {
			roles = append(roles, aggregateStreamFactory, aggregateStreamValue)
		}
		if err := plan.request(entity, roles...); err != nil {
			return nil, err
		}
	}
	if err := plan.freeze(); err != nil {
		return nil, err
	}
	result := make(map[string]registryIdentifierNames, len(entities))
	for route, entity := range entities {
		var names registryIdentifierNames
		bindings := []struct {
			role        aggregateIdentifierRole
			destination *string
		}{
			{aggregateBaseFactory, &names.base}, {aggregateBaseValue, &names.baseValue}, {aggregateOperationValue, &names.value},
		}
		if operations[route].PaginationPlan != nil {
			bindings = append(bindings, struct {
				role        aggregateIdentifierRole
				destination *string
			}{aggregatePaginationFactory, &names.pagination}, struct {
				role        aggregateIdentifierRole
				destination *string
			}{aggregatePaginationValue, &names.paginationValue})
		}
		if links[route] {
			bindings = append(bindings, struct {
				role        aggregateIdentifierRole
				destination *string
			}{aggregateLinksFactory, &names.links}, struct {
				role        aggregateIdentifierRole
				destination *string
			}{aggregateLinksValue, &names.linksValue})
		}
		if streams[route] {
			bindings = append(bindings, struct {
				role        aggregateIdentifierRole
				destination *string
			}{aggregateStreamFactory, &names.stream}, struct {
				role        aggregateIdentifierRole
				destination *string
			}{aggregateStreamValue, &names.streamValue})
		}
		for _, binding := range bindings {
			name, err := plan.resolve(entity, binding.role)
			if err != nil {
				return nil, err
			}
			*binding.destination = name
		}
		result[route] = names
	}
	return result, nil
}

type schemaWireIdentifierNames struct{ input, output string }

func planSchemaWireIdentifiers(module *semanticModulePlan) (map[string]schemaWireIdentifierNames, error) {
	if module == nil {
		return nil, fmt.Errorf("schema wire identifiers require semantic modules")
	}
	plan := newAggregateIdentifierPlan(module.fixed["schema-wire"])
	if err := plan.reserve("WireSchemas", "inputSchemas", "outputSchemas"); err != nil {
		return nil, err
	}
	entities := make(map[string]aggregateEntityKey)
	for _, schema := range module.schemas {
		if !schema.inputWire && !schema.outputWire {
			continue
		}
		if _, exists := entities[schema.name]; exists {
			return nil, fmt.Errorf("schema wire registry has duplicate component %q", schema.name)
		}
		entity := newAggregateEntity("schema", schema.name)
		entities[schema.name] = entity
		if schema.inputWire {
			if err := plan.request(entity, aggregateInputWire); err != nil {
				return nil, err
			}
		}
		if schema.outputWire {
			if err := plan.request(entity, aggregateOutputWire); err != nil {
				return nil, err
			}
		}
	}
	if err := plan.freeze(); err != nil {
		return nil, err
	}
	result := make(map[string]schemaWireIdentifierNames, len(entities))
	for _, schema := range module.schemas {
		entity, exists := entities[schema.name]
		if !exists {
			continue
		}
		var names schemaWireIdentifierNames
		var err error
		if schema.inputWire {
			names.input, err = plan.resolve(entity, aggregateInputWire)
			if err != nil {
				return nil, err
			}
		}
		if schema.outputWire {
			names.output, err = plan.resolve(entity, aggregateOutputWire)
			if err != nil {
				return nil, err
			}
		}
		result[schema.name] = names
	}
	return result, nil
}

func planEnumIdentifiers(values []enumValuesPlan) error {
	plan := newAggregateIdentifierPlan("internal/enums.ts")
	if err := plan.reserve("Enums", "EnumValue", "isEnumValue", "EnumValues", "Value", "Name", "__sdkgen_createJSONRecord", "__sdkgen_createEnumValues", "__sdkgen_enumValueEquals", "entries", "values", "enumValues", "value", "left", "right", "seen", "compared", "candidate", "leftArray", "rightArray", "leftPrototype", "rightPrototype", "leftKeys", "rightKeys", "key", "rightKey", "leftItem", "rightItem", "index"); err != nil {
		return err
	}
	entities := make([]aggregateEntityKey, len(values))
	seen := make(map[aggregateEntityKey]bool, len(values))
	for i, value := range values {
		entity := newAggregateEntity("schema", value.name)
		if seen[entity] {
			return fmt.Errorf("enum registry has duplicate component %q", value.name)
		}
		seen[entity] = true
		entities[i] = entity
		if err := plan.request(entity, aggregateEnumValues, aggregateEnumRecord); err != nil {
			return err
		}
	}
	if err := plan.freeze(); err != nil {
		return err
	}
	for i, entity := range entities {
		var err error
		values[i].valuesBinding, err = plan.resolve(entity, aggregateEnumValues)
		if err != nil {
			return err
		}
		values[i].enumBinding, err = plan.resolve(entity, aggregateEnumRecord)
		if err != nil {
			return err
		}
	}
	return nil
}

func callbackIdentifierEntity(callback callbackDefinition) aggregateEntityKey {
	kind := "route"
	if callback.sourceRouteKey == "" {
		kind = "component"
	}
	return newAggregateEntity("callback", kind, callback.sourceRouteKey, callback.componentName, callback.callbackName, callback.expression, callback.method)
}

func webhookIdentifierEntity(webhook webhookDefinition) aggregateEntityKey {
	return newAggregateEntity("webhook", webhook.name, webhook.method)
}

func newServerIdentifierPlan(owner string) (*aggregateIdentifierPlan, error) {
	plan := newAggregateIdentifierPlan(owner)
	if err := plan.reserve(
		"collectInboundSecurityCandidates", "decodeInboundBody", "decodeInboundParameters", "matchInboundRoute", "InboundRequestError",
		"normalizeInboundMediaCodecs", "normalizeInboundStreamCodecs", "requiresInboundAuthentication", "responseFromHandler", "Authenticate",
		"InboundParameterValues", "InboundRequestContext", "InboundResponse", "InboundParameterDefinition", "InboundSchemas", "InboundSecuritySchemes",
		"MediaCodec", "StreamCodec", "WireSchemas", "Contract", "__sdkgen_Properties", "inputSchemas", "inputWireSchemas", "outputSchemas", "securitySchemes",
		"Callbacks", "RouteCallbacks", "ComponentCallbacks", "CallbackHandlers", "RouteCallbackHandlers", "ComponentCallbackHandlers", "CallbackPathParameters", "RouteCallbackPathParameters", "ComponentCallbackPathParameters",
		"CallbackEndpoints", "RouteCallbackEndpoints", "ComponentCallbackEndpoints", "CallbackEndpoint", "CallbackOptions", "createCallbacks", "WebhookHandlers", "Webhooks", "WebhookRoutes", "WebhookRouter", "WebhookRouterOptions", "createWebhookRouter",
		"handlers", "options", "routes", "inboundCodecs", "inboundStreamCodecs", "registrations", "request", "path", "pathParams", "pathname", "key", "handler", "params", "context", "body", "denied", "error",
	); err != nil {
		return nil, err
	}
	return plan, nil
}

// Bindings are assigned on an emission-local copy. Prepared semantic definitions
// remain reusable and contain no artifact-dependent compact spelling.
func planCallbackIdentifiers(callbacks []callbackDefinition) ([]callbackDefinition, error) {
	plan, err := newServerIdentifierPlan("server/callbacks.ts")
	if err != nil {
		return nil, err
	}
	result := append([]callbackDefinition(nil), callbacks...)
	entities := make([]aggregateEntityKey, len(result))
	seen := make(map[aggregateEntityKey]bool, len(result))
	for i, callback := range result {
		entity := callbackIdentifierEntity(callback)
		if seen[entity] {
			return nil, fmt.Errorf("callback module has duplicate exact source %q", callback.name)
		}
		seen[entity] = true
		entities[i] = entity
		if err := plan.request(entity, aggregateCallbackType, aggregateCallbackDefinition, aggregateCallbackEndpoint); err != nil {
			return nil, err
		}
	}
	if err := plan.freeze(); err != nil {
		return nil, err
	}
	for i, entity := range entities {
		result[i].typeName, err = plan.resolve(entity, aggregateCallbackType)
		if err != nil {
			return nil, err
		}
		result[i].definitionSymbol, err = plan.resolve(entity, aggregateCallbackDefinition)
		if err != nil {
			return nil, err
		}
		result[i].endpointSymbol, err = plan.resolve(entity, aggregateCallbackEndpoint)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func planWebhookIdentifiers(webhooks []webhookDefinition) ([]webhookDefinition, error) {
	plan, err := newServerIdentifierPlan("server/webhooks.ts")
	if err != nil {
		return nil, err
	}
	result := append([]webhookDefinition(nil), webhooks...)
	entities := make([]aggregateEntityKey, len(result))
	seen := make(map[aggregateEntityKey]bool, len(result))
	for i, webhook := range result {
		entity := webhookIdentifierEntity(webhook)
		if seen[entity] {
			return nil, fmt.Errorf("webhook module has duplicate exact source %q %q", webhook.name, webhook.method)
		}
		seen[entity] = true
		entities[i] = entity
		if err := plan.request(entity, aggregateWebhookType, aggregateWebhookDefinition); err != nil {
			return nil, err
		}
	}
	if err := plan.freeze(); err != nil {
		return nil, err
	}
	for i, entity := range entities {
		result[i].typeName, err = plan.resolve(entity, aggregateWebhookType)
		if err != nil {
			return nil, err
		}
		result[i].definitionSymbol, err = plan.resolve(entity, aggregateWebhookDefinition)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
