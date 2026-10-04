package typescript

import "fmt"

type resourceCallableBinding struct {
	name       string
	template   string
	streamName string
	links      bool
	pagination bool
}

// resourceBinding consumes the same prepared input, execution and Link facts
// for resource imports, invocation and runtime closure. No generated source is read.
func resourceBinding(item ManifestOperation, plan *semanticModulePlan) resourceCallableBinding {
	binding := resourceCallableBinding{name: "bindResourceNoInput", template: "resource-bind-none.ts"}
	if item.InputSections.hasInput(true) {
		binding.name, binding.template = "bindResourceOptionalInput", "resource-bind-optional.ts"
		if item.prepared.resourceInputRequired {
			binding.name, binding.template = "bindResourceInput", "resource-bind-input.ts"
		}
	}
	route := manifestRouteKey(item)
	if plan.resourceExecutions[route].hasStream {
		binding.streamName = binding.name + "Stream"
	}
	for _, link := range plan.resourceLinks {
		if operationRouteKey(link.SourceOperation) == route {
			binding.links = true
			break
		}
	}
	binding.pagination = item.compiled.PaginationPlan != nil
	return binding
}

func (binding resourceCallableBinding) imports() map[string][]string {
	imports := map[string][]string{binding.template: {binding.name}}
	if binding.streamName != "" {
		delete(imports, binding.template)
		imports["resource-stream-binding.ts"] = []string{binding.streamName}
	}
	if binding.links {
		imports["resource-helper-binding.ts"] = append(imports["resource-helper-binding.ts"], "bindResourceLinks")
	}
	if binding.pagination {
		imports["resource-helper-binding.ts"] = append(imports["resource-helper-binding.ts"], "bindResourcePagination")
	}
	return imports
}

func (binding resourceCallableBinding) expression(call, path string) string {
	expression := fmt.Sprintf("%s(%s, %s)", binding.name, call, path)
	if binding.streamName != "" {
		expression = fmt.Sprintf("%s(%s, %s)", binding.streamName, call, path)
	}
	if binding.links {
		expression = fmt.Sprintf("bindResourceLinks(%s as object, %s)", expression, call)
	}
	if binding.pagination {
		expression = fmt.Sprintf("bindResourcePagination(%s as object, %s)", expression, call)
	}
	return expression
}
