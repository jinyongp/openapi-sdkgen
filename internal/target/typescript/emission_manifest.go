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
