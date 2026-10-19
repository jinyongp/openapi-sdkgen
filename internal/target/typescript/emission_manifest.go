package typescript

import (
	"fmt"

	"openapi-sdkgen/internal/generator"
)

// EmissionRoutes returns a snapshot of the visible exact routes in the manifest
// consumed by the emitter. Consumers reporting emitted coverage must call this
// only after emission succeeds; a prepared manifest alone proves no delivery.
func EmissionRoutes(plan generator.Plan) ([]string, error) {
	value, err := plan.Value("typescript")
	if err != nil {
		return nil, err
	}
	prepared, ok := value.(*sourcePlan)
	if !ok || prepared == nil || prepared.manifest == nil {
		return nil, fmt.Errorf("TypeScript plan has no emission manifest")
	}
	routes := make([]string, 0, len(prepared.manifest.Operations))
	for _, operation := range prepared.manifest.Operations {
		if operation.Visibility != "hidden" && !operation.dependencyOnly {
			routes = append(routes, manifestRouteKey(operation))
		}
	}
	return routes, nil
}

// GenerationSelection reports public roots separately from private execution
// dependencies. Nil identifies a complete-document generation plan.
type GenerationSelection struct {
	Routes           []string `json:"routes"`
	DependencyRoutes []string `json:"dependencyRoutes"`
}

func SelectionDetails(plan generator.Plan) (*GenerationSelection, error) {
	value, err := plan.Value("typescript")
	if err != nil {
		return nil, err
	}
	prepared, ok := value.(*sourcePlan)
	if !ok || prepared == nil {
		return nil, fmt.Errorf("TypeScript plan has no selection details")
	}
	if prepared.selection == nil {
		return nil, nil
	}
	return &GenerationSelection{Routes: sortedStringKeys(prepared.selection.direct), DependencyRoutes: sortedStringKeys(prepared.selection.dependencies)}, nil
}

// EmissionInbound snapshots the receive definitions used by server emission.
// Call only after successful emission when reporting delivered contracts.
func EmissionInbound(plan generator.Plan) ([]generator.InboundOperationInspection, error) {
	value, err := plan.Value("typescript")
	if err != nil {
		return nil, err
	}
	prepared, ok := value.(*sourcePlan)
	if !ok || prepared == nil {
		return nil, fmt.Errorf("TypeScript plan has no inbound emission manifest")
	}
	result := make([]generator.InboundOperationInspection, 0, len(prepared.webhooks)+len(prepared.callbacks))
	for _, definition := range prepared.webhooks {
		result = append(result, generator.InboundOperationInspection{Kind: "webhook", Identity: definition.schemaSource.path, Name: definition.name, Method: definition.method})
	}
	for _, definition := range prepared.callbacks {
		result = append(result, generator.InboundOperationInspection{Kind: "callback", Identity: definition.schemaSource.path, Name: definition.callbackName, Method: definition.method, Expression: definition.expression, SourceRoute: definition.sourceRouteKey, SourceOperationID: definition.sourceOperationID, ComponentName: definition.componentName})
	}
	return result, nil
}
