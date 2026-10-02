package typescript

import (
	"fmt"
	"maps"
	"sort"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

type namedClientPlan struct {
	name string
	view *sourcePlan
}

// Resolve independent public selections before preparing their shared execution
// union. Semantic lowering, resource reconciliation and execution planning run
// once; client views reuse those decisions and collision-resolved module names.
func prepareClientSourcePlanWithCoverage(document *ir.Document, options generator.Options, mode diagnostic.Mode) (*sourcePlan, []diagnostic.Diagnostic, []diagnostic.AnalysisCoverage, error) {
	server, metadata := options.HasAddon(generator.AddonServer), options.HasAddon(generator.AddonMetadata)
	if options.Clients == nil {
		return prepareSelectedSourcePlanWithCoverage(document, server, mode, options.FailOnResourceOmission, options.Selection, metadata)
	}
	clients, err := generator.CanonicalClients(options.Clients)
	if err != nil {
		return &sourcePlan{document: document}, []diagnostic.Diagnostic{selectionDiagnostic("", err.Error())}, nil, nil
	}
	_, root, values, err := selectGenerationDocument(document, options.Selection, metadata)
	if err != nil || diagnostic.HasErrors(values) {
		return &sourcePlan{document: document}, values, nil, err
	}
	selections := make(map[string]*generationSelection, len(clients))
	names := make([]string, 0, len(clients))
	for name := range clients {
		names = append(names, name)
	}
	sort.Strings(names)
	direct := make(map[string]bool)
	if root != nil {
		for route := range root.direct {
			direct[route] = true
		}
	}
	for _, name := range names {
		_, selected, diagnostics, err := selectGenerationDocument(document, clients[name].Selection, metadata)
		if err != nil {
			return nil, values, nil, fmt.Errorf("client %q selection: %w", name, err)
		}
		for _, value := range diagnostics {
			value.Message = fmt.Sprintf("Client %q: %s", name, value.Message)
			values = append(values, value)
		}
		selections[name] = selected
		if selected != nil {
			for route := range selected.direct {
				direct[route] = true
			}
		}
	}
	if diagnostic.HasErrors(values) {
		return &sourcePlan{document: document}, diagnostic.Sort(values), nil, nil
	}
	// The ordinary root SDK keeps its existing full-document default.
	var union *generator.Selection
	if root != nil {
		union = &generator.Selection{Routes: sortedStringKeys(direct)}
	}
	plan, diagnostics, coverage, err := prepareSelectedSourcePlanWithCoverage(document, server, mode, options.FailOnResourceOmission, union, metadata)
	if err != nil || diagnostic.HasErrors(diagnostics) {
		return plan, diagnostics, coverage, err
	}
	if err := plan.modules.planSchemaProjections(); err != nil {
		return nil, diagnostics, coverage, err
	}
	plan.root = scopedSourcePlan(plan, root)
	for _, name := range names {
		plan.clients = append(plan.clients, namedClientPlan{name: name, view: scopedSourcePlan(plan, selections[name])})
	}
	return plan, diagnostics, coverage, nil
}

func scopedSourcePlan(shared *sourcePlan, selection *generationSelection) *sourcePlan {
	view := *shared
	view.root, view.clients, view.selection = nil, nil, selection
	view.ownership = nil
	if selection == nil {
		return &view
	}
	retained := func(route string) bool { return selection.direct[route] || selection.dependencies[route] }
	document := *shared.document
	document.Operations = nil
	for _, operation := range shared.document.Operations {
		if retained(operationRouteKey(operation)) {
			document.Operations = append(document.Operations, operation)
		}
	}
	view.document = &document
	manifest := Manifest{}
	for _, item := range shared.manifest.Operations {
		route := manifestRouteKey(item)
		if retained(route) {
			item.dependencyOnly = !selection.direct[route]
			manifest.Operations = append(manifest.Operations, item)
		}
	}
	view.manifest = &manifest
	view.resourceTree = cloneSelectedResourceTree(shared.resourceTree, selection.direct)
	view.resourceReachable = make(map[string]bool)
	resourceOperationIDs(view.resourceTree, view.resourceReachable)
	view.links = nil
	for _, link := range shared.links {
		if retained(operationRouteKey(link.SourceOperation)) {
			view.links = append(view.links, link)
		}
	}
	view.streams = nil
	for _, stream := range shared.streams {
		if retained(operationRouteKey(stream.Operation)) {
			view.streams = append(view.streams, stream)
		}
	}
	modules := *shared.modules
	modules.selection = selection
	modules.relativeSpecifiers = make(map[string]string)
	modules.relativeSpecifierFrom = ""
	modules.operations = nil
	for _, module := range shared.modules.operations {
		if retained(module.routeKey) {
			modules.operations = append(modules.operations, module)
		}
	}
	input, output := reachableComponentSchemaProjections(view.document, false)
	public := publicReachableComponentSchemas(view.document, input, output)
	modules.schemas = nil
	for _, schema := range shared.modules.schemas {
		if public[schema.name] {
			schema.inputWire, schema.outputWire = input[schema.name], output[schema.name]
			modules.schemas = append(modules.schemas, schema)
		}
	}
	view.modules = &modules
	return &view
}

func cloneSelectedResourceTree(node *resourceNode, direct map[string]bool) *resourceNode {
	clone := *node
	clone.childSources = maps.Clone(node.childSources)
	clone.operations = make(map[string]ManifestOperation)
	for name, item := range node.operations {
		if direct[manifestRouteKey(item)] {
			clone.operations[name] = item
		}
	}
	if node.pagination != nil && !direct[manifestRouteKey(*node.pagination)] {
		clone.pagination = nil
	}
	clone.children = make(map[string]*resourceNode)
	for name, child := range node.children {
		clone.children[name] = cloneSelectedResourceTree(child, direct)
	}
	if node.parameterChild != nil {
		clone.parameterChild = cloneSelectedResourceTree(node.parameterChild, direct)
	}
	pruneEmptyResourceNodes(&clone)
	return &clone
}
