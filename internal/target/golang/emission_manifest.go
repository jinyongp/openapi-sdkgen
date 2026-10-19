package golang

import (
	"fmt"
	"openapi-sdkgen/internal/generator"
	"sort"
)

// EmissionRoutes snapshots the visible root operations used by client.go.
// Only a successful Emit/EmitTo establishes that these routes were delivered.
func EmissionRoutes(plan generator.Plan) ([]string, error) {
	value, err := plan.Value("golang")
	if err != nil {
		return nil, err
	}
	prepared, ok := value.(sourcePlan)
	if !ok {
		return nil, fmt.Errorf("Go plan has no emission manifest")
	}
	for _, file := range prepared.Files {
		if file.Path == "client.go" {
			routes := make([]string, 0, len(file.ClientOperations))
			for _, operation := range file.ClientOperations {
				routes = append(routes, operation.Route)
			}
			sort.Strings(routes)
			return routes, nil
		}
	}
	return nil, fmt.Errorf("Go plan has no root client emission file")
}

// EmissionInbound snapshots the receive contracts supplied to server emission.
// Call only after successful emission when reporting delivered contracts.
func EmissionInbound(plan generator.Plan) ([]generator.InboundOperationInspection, error) {
	value, err := plan.Value("golang")
	if err != nil {
		return nil, err
	}
	prepared, ok := value.(sourcePlan)
	if !ok {
		return nil, fmt.Errorf("Go plan has no inbound emission manifest")
	}
	return append([]generator.InboundOperationInspection(nil), prepared.Inbound...), nil
}
