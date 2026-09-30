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
		if operation.Visibility != "hidden" {
			routes = append(routes, manifestRouteKey(operation))
		}
	}
	return routes, nil
}
