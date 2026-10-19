package plan

import (
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"sort"
)

// InboundScope retains the declaration owner before any language rendering.
type InboundScope struct {
	Kind, Pointer string
	Values        map[string]any
	Source        *ir.Operation
	Components    bool
}

type InboundDefinition struct {
	Kind, Name, Identity, Expression, CallbackName string
	SourceRoute, SourceOperationID, ComponentName  string
	Operation                                      ir.Operation
}

// InboundScopes follows client selection for operation-owned callbacks. Reusable
// callback components and document webhooks retain their document ownership.
func InboundScopes(document *ir.Document, omitted, direct map[string]bool) []InboundScope {
	var result []InboundScope
	if values, _ := document.Raw["webhooks"].(map[string]any); len(values) != 0 {
		result = append(result, InboundScope{Kind: "webhook", Pointer: "#/webhooks", Values: values})
	}
	components, _ := document.Raw["components"].(map[string]any)
	if values, _ := components["callbacks"].(map[string]any); len(values) != 0 {
		result = append(result, InboundScope{Kind: "callback", Pointer: "#/components/callbacks", Values: values, Components: true})
	}
	for index := range document.Operations {
		operation := &document.Operations[index]
		if operation.Visibility == "hidden" || omitted[operation.RouteKey] || direct != nil && !direct[operation.RouteKey] {
			continue
		}
		if values, _ := operation.Raw["callbacks"].(map[string]any); len(values) != 0 {
			result = append(result, InboundScope{Kind: "callback", Pointer: operation.Pointer + "/callbacks", Values: values, Source: operation})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Pointer < result[j].Pointer })
	return result
}

func CollectInbound(document *ir.Document, scopes []InboundScope) ([]InboundDefinition, []error) {
	var result []InboundDefinition
	var failures []error
	appendPathItem := func(scope InboundScope, name, expression string, value any, pointer string) {
		item, ok := value.(map[string]any)
		if !ok {
			failures = append(failures, fmt.Errorf("%s must be a Path Item Object", pointer))
			return
		}
		operations, err := ir.ReadInboundOperations(document, item, pointer)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", pointer, err))
			return
		}
		for _, operation := range operations {
			if operation.OperationID == "" {
				operation.OperationID = name
			}
			definition := InboundDefinition{Kind: scope.Kind, Name: name, Identity: operation.Pointer, Expression: expression, Operation: operation}
			if scope.Kind == "callback" {
				definition.CallbackName = name
				if scope.Components {
					definition.ComponentName = name
				}
				if scope.Source != nil {
					definition.SourceRoute = scope.Source.RouteKey
					definition.SourceOperationID = scope.Source.OperationID
				}
			}
			result = append(result, definition)
		}
	}
	for _, scope := range scopes {
		for _, name := range inboundSortedKeys(scope.Values) {
			pointer := scope.Pointer + "/" + PointerToken(name)
			value := scope.Values[name]
			if scope.Kind == "webhook" {
				appendPathItem(scope, name, "", value, pointer)
				continue
			}
			callback, ok := value.(map[string]any)
			if !ok {
				failures = append(failures, fmt.Errorf("%s must be a Callback Object", pointer))
				continue
			}
			resolved, err := ResolveComponentObject(document, callback, "callbacks")
			if err != nil {
				failures = append(failures, fmt.Errorf("%s: %w", pointer, err))
				continue
			}
			for _, expression := range inboundSortedKeys(resolved) {
				appendPathItem(scope, name, expression, resolved[expression], pointer+"/"+PointerToken(expression))
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Identity < result[j].Identity })
	return result, failures
}

func inboundSortedKeys(values map[string]any) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
