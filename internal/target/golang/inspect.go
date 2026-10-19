package golang

import (
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
)

func (Generator) InspectPlan(plan generator.Plan) (map[string]generator.OperationInspection, error) {
	value, err := plan.Value("golang")
	if err != nil {
		return nil, err
	}
	prepared, ok := value.(sourcePlan)
	if !ok {
		return nil, fmt.Errorf("Go analysis plan is unavailable")
	}
	result := make(map[string]generator.OperationInspection, len(prepared.Inspection))
	for route, inspection := range prepared.Inspection {
		if inspection.ResourceCall != nil {
			call := *inspection.ResourceCall
			inspection.ResourceCall = &call
		}
		result[route] = inspection
	}
	return result, nil
}

func prepareInspection(document *ir.Document, operations []operationPlan, resources []resourceNodePlan, omitted map[string]bool) map[string]generator.OperationInspection {
	reasons := resourceOmissionReasons(operations, resources)
	result := make(map[string]generator.OperationInspection, len(document.Operations))
	for _, operation := range document.Operations {
		status := "omitted"
		if operation.Visibility == "hidden" {
			status = "hidden"
		}
		result[operation.RouteKey] = generator.OperationInspection{Status: status}
	}
	for _, operation := range operations {
		if operation.DependencyOnly {
			result[operation.Route] = generator.OperationInspection{Status: "hidden"}
			continue
		}
		result[operation.Route] = generator.OperationInspection{Status: "exposed", Routes: true, Operations: true, ResourceOmission: reasons[operation.Route]}
	}
	byName := make(map[string]resourceNodePlan, len(resources))
	for _, resource := range resources {
		byName[resource.Name] = resource
	}
	var visit func(string, string)
	visit = func(name, prefix string) {
		node := byName[name]
		for _, call := range node.Calls {
			input := ""
			if call.Operation.ResourceHasInput {
				input = "input"
			}
			expression := prefix + "." + call.Method + "(" + input + ")"
			if len(call.Operation.Security) > 1 {
				expression += ".WithOptions(options)"
			}
			terminal := "Do"
			if !call.Operation.CanDo {
				terminal = "Raw"
				if call.Operation.HasTypedStream {
					terminal = "Stream"
				}
			}
			expression += "." + terminal + "(ctx)"
			inspection := result[call.Operation.Route]
			inspection.ResourceCall = &expression
			inspection.ResourceOmission = ""
			result[call.Operation.Route] = inspection
		}
		for _, child := range node.Children {
			visit(child.Target, prefix+"."+child.Name)
		}
		if node.Selector != nil {
			visit(node.Selector.Child, prefix+"."+node.Selector.Name+"(argument)")
		}
	}
	for _, resource := range resources {
		if resource.Identity == "/" {
			visit(resource.Name, "api")
			break
		}
	}
	for route := range omitted {
		if _, present := result[route]; present {
			result[route] = generator.OperationInspection{Status: "omitted"}
		}
	}
	return result
}
