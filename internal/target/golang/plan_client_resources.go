package golang

import (
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
)

type clientResourcePlans struct {
	Root  []resourceNodePlan
	Named map[string][]resourceNodePlan
	All   []resourceNodePlan
}

// Entry-local selector contracts must not be reconciled against another client's
// methods. Bound operation implementations remain shared by every entry.
func prepareClientResources(document *ir.Document, operations []operationPlan, names namePlan, schemas *schemaPlanner, selections clientSelections, strict bool) (clientResourcePlans, []diagnostic.Diagnostic, error) {
	result := clientResourcePlans{Named: make(map[string][]resourceNodePlan)}
	byRoute := make(map[string]int, len(operations))
	for index, operation := range operations {
		byRoute[operation.Route] = index
	}
	prepare := func(selection *generationSelection, scopedNames namePlan) ([]resourceNodePlan, []diagnostic.Diagnostic, error) {
		scoped := clientOperations(operations, selection)
		for index := range scoped {
			operation := &scoped[index]
			operation.ResourceBound = false
			operation.ResourceBindings = nil
			operation.ResourceInputFields = nil
			operation.ResourcePathField = nil
		}
		resources, err := prepareResources(document, scoped, scopedNames, schemas)
		if err != nil {
			return nil, nil, err
		}
		for _, operation := range scoped {
			shared := &operations[byRoute[operation.Route]]
			if operation.ResourceBound || !shared.ResourceBound {
				shared.ResourceBound = operation.ResourceBound
				shared.ResourceBindings = operation.ResourceBindings
				shared.ResourceInputName = operation.ResourceInputName
				shared.ResourceInputFields = operation.ResourceInputFields
				shared.ResourcePathField = operation.ResourcePathField
				shared.ResourceHasInput = operation.ResourceHasInput
				shared.ResourceRequiredInput = operation.ResourceRequiredInput
			}
		}
		return resources, resourceDiagnostics(document, scoped, resources, strict), nil
	}
	var diagnostics []diagnostic.Diagnostic
	root, findings, err := prepare(selections.Root, names)
	if err != nil {
		return result, nil, err
	}
	result.Root = root
	result.All = append(result.All, root...)
	diagnostics = append(diagnostics, findings...)
	for _, client := range sortedKeys(selections.Named) {
		scopedNames := names
		scopedNames.resourceParameters = make(map[string]string, len(names.resourceParameters))
		for _, identity := range sortedKeys(names.resourceParameters) {
			model, err := reserveDerivedName(names.reserved, "Client "+client+" "+names.resourceParameters[identity], "client/"+client+"/resource"+identity, modelNames)
			if err != nil {
				return result, nil, err
			}
			scopedNames.resourceParameters[identity] = model
		}
		resources, findings, err := prepare(selections.Named[client], scopedNames)
		if err != nil {
			return result, nil, err
		}
		for index := range findings {
			findings[index].Message = "Client " + client + ": " + findings[index].Message
		}
		result.Named[client] = resources
		result.All = append(result.All, resources...)
		diagnostics = append(diagnostics, findings...)
	}
	return result, diagnostic.Sort(diagnostics), nil
}
