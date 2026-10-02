package typescript

import (
	"fmt"
	"strings"

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
			expression, err := inspectResourceCall(prepared, operation)
			if err != nil {
				return nil, err
			}
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

// Keep the prepared, collision-resolved callee, then use the target's input,
// options and response capability rules to show a supported invocation.
func inspectResourceCall(plan *sourcePlan, operation ManifestOperation) (string, error) {
	input := callInput(operation.compiled, operation.InputSections, len(operation.PathParameterOrder) > 0, operation.PathParameterOrder, operation.prepared.pathBindings)
	callee := strings.TrimSuffix(operation.CallExpression, input)
	arguments := input[1 : len(input)-1]
	if operation.optionsRequired {
		if arguments != "" {
			arguments += ", "
		}
		arguments += "options"
	}
	buffered, err := operationHasBufferedSuccess(plan.document, operation.compiled)
	if err != nil {
		return "", err
	}
	if !buffered {
		if len(operation.streamMediaTypes) > 0 {
			callee += ".stream"
		} else {
			callee += ".raw"
		}
	}
	return callee + "(" + arguments + ")", nil
}
