package typescript

import (
	"fmt"

	"openapi-sdkgen/internal/generator"
)

// OperationInspection reports the callable surfaces of the prepared client.
// Expressions come from the same collision-resolved manifest as emission.
type OperationInspection struct {
	Status           string  `json:"status"`
	ResourceCall     *string `json:"resourceCall"`
	Routes           bool    `json:"routes"`
	Operations       bool    `json:"operations"`
	ResourceOmission string  `json:"resourceOmission,omitempty"`
}

// InspectPlan reads the target-owned preparation result without emitting code.
func InspectPlan(plan generator.Plan) (map[string]OperationInspection, error) {
	value, err := plan.Value("typescript")
	if err != nil {
		return nil, err
	}
	prepared, ok := value.(*sourcePlan)
	if !ok || prepared.manifest == nil {
		return nil, fmt.Errorf("TypeScript analysis plan is unavailable")
	}
	result := make(map[string]OperationInspection, len(prepared.manifest.Operations))
	for _, operation := range prepared.manifest.Operations {
		route := manifestRouteKey(operation)
		public := operation.Visibility != "hidden" && !operation.dependencyOnly
		inspection := OperationInspection{Status: "hidden", Routes: public, Operations: public && operation.OperationID != ""}
		if public {
			inspection.Status = "exposed"
		}
		if public && prepared.resourceReachable[route] {
			expression := operation.CallExpression
			inspection.ResourceCall = &expression
		}
		if public && inspection.ResourceCall == nil {
			inspection.ResourceOmission = prepared.resourceOmissions[route]
			if inspection.ResourceOmission == "" {
				inspection.ResourceOmission = "visibility"
			}
		}
		result[route] = inspection
	}
	for route := range prepared.omittedOperations {
		result[route] = OperationInspection{Status: "omitted"}
	}
	return result, nil
}
