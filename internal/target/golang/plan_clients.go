package golang

import (
	"fmt"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

type clientSelections struct {
	Root, Union *generationSelection
	Named       map[string]*generationSelection
}

func clientPackageName(name string) string {
	name = strings.ReplaceAll(name, "-", "_")
	if !validPackage(name) || name == "main" {
		name += "_client"
	}
	return name
}

func prepareClientSelections(document *ir.Document, options generator.Options) (clientSelections, []diagnostic.Diagnostic, error) {
	root, diagnostics := selectOperations(document, options.Selection)
	result := clientSelections{Root: root, Union: root}
	if diagnostic.HasErrors(diagnostics) || options.Clients == nil {
		return result, diagnostics, nil
	}
	clients, err := generator.CanonicalClients(options.Clients)
	if err != nil {
		return result, nil, err
	}
	result.Named = make(map[string]*generationSelection, len(clients))
	direct := make(map[string]bool)
	if root != nil {
		for route := range root.Direct {
			direct[route] = true
		}
	}
	for _, name := range sortedKeys(clients) {
		selected, findings := selectOperations(document, clients[name].Selection)
		for index := range findings {
			findings[index].Message = fmt.Sprintf("Client %q: %s", name, findings[index].Message)
		}
		diagnostics = append(diagnostics, findings...)
		if selected == nil {
			continue
		}
		result.Named[name] = selected
		for route := range selected.Direct {
			direct[route] = true
		}
	}
	if diagnostic.HasErrors(diagnostics) {
		return result, diagnostics, nil
	}
	union, findings := selectOperations(document, &generator.Selection{Routes: sortedKeys(direct)})
	diagnostics = append(diagnostics, findings...)
	result.Union = union
	if result.Root == nil {
		result.Root = union
	}
	return result, diagnostics, nil
}

func clientOperations(operations []operationPlan, selection *generationSelection) []operationPlan {
	var result []operationPlan
	for _, operation := range operations {
		if !operation.DependencyOnly && (selection == nil || selection.Direct[operation.Route]) {
			result = append(result, operation)
		}
	}
	return result
}

func clientLinks(sources []linkSourcePlan, selection *generationSelection) []linkSourcePlan {
	var result []linkSourcePlan
	for _, source := range sources {
		if !source.Operation.DependencyOnly && (selection == nil || selection.Direct[source.Operation.Route]) {
			result = append(result, source)
		}
	}
	return result
}

func clientLinkTargets(sources []linkSourcePlan, operations []operationPlan) []operationPlan {
	needed := make(map[string]bool)
	for _, source := range sources {
		for _, group := range source.Groups {
			for _, leaf := range group.Links {
				needed[leaf.Target.Route] = true
			}
		}
	}
	var result []operationPlan
	for _, operation := range operations {
		if needed[operation.Route] {
			result = append(result, operation)
		}
	}
	return result
}

// Scoped namespaces reuse the already reconciled selector contracts and names.
// Only methods and child paths change; another client's members cannot leak.
func clientResources(resources []resourceNodePlan, selection *generationSelection) []resourceNodePlan {
	if selection == nil {
		return resources
	}
	byName := make(map[string]resourceNodePlan, len(resources))
	for _, resource := range resources {
		byName[resource.Name] = resource
	}
	var result []resourceNodePlan
	var visit func(string) bool
	visit = func(name string) bool {
		source := byName[name]
		node := source
		node.Calls = nil
		node.Children = nil
		node.Selector = nil
		for _, call := range source.Calls {
			if selection.Direct[call.Operation.Route] {
				node.Calls = append(node.Calls, call)
			}
		}
		for _, child := range source.Children {
			if visit(child.Target) {
				node.Children = append(node.Children, child)
			}
		}
		if source.Selector != nil && visit(source.Selector.Child) {
			selector := *source.Selector
			node.Selector = &selector
		}
		if len(node.Calls) == 0 && len(node.Children) == 0 && node.Selector == nil {
			return false
		}
		result = append(result, node)
		return true
	}
	for _, resource := range resources {
		if resource.Identity == "/" {
			visit(resource.Name)
			break
		}
	}
	return result
}

func clientFile(config generator.GoOptions, filePath, packageName, resourcePath string, operations []operationPlan, resources []resourceNodePlan, links []linkSourcePlan, allOperations []operationPlan) filePlan {
	aliases := append([]operationPlan(nil), operations...)
	seen := make(map[string]bool, len(aliases))
	for _, operation := range aliases {
		seen[operation.Route] = true
	}
	for _, operation := range clientLinkTargets(links, allOperations) {
		if !seen[operation.Route] {
			aliases = append(aliases, operation)
			seen[operation.Route] = true
		}
	}
	imports := []importPlan{{Path: config.ImportPath + "/internal/core"}, {Path: config.ImportPath + "/internal/operations"}, {Path: config.ImportPath + "/types"}}
	models := false
	for _, operation := range aliases {
		models = models || operation.HasInput || operation.HasHeaders || operation.HasTypedStream && !operation.StreamUnion || !operation.OutputUnion && strings.Contains(operation.OutputType, "models.")
	}
	if models {
		imports = append(imports, importPlan{Path: config.ImportPath + "/models"})
	}
	if len(resources) != 0 {
		imports = append(imports, importPlan{Path: config.ImportPath + "/" + resourcePath})
	}
	return filePlan{Path: filePath, Package: packageName, Template: "templates/client.go.tmpl", Imports: imports, Operations: aliases, ClientOperations: operations, Resources: resources, LinkSources: links}
}

func appendClientEntries(config generator.GoOptions, files []filePlan, operations []operationPlan, resources clientResourcePlans, links []linkSourcePlan, selections clientSelections) ([]filePlan, error) {
	rootOperations := clientOperations(operations, selections.Root)
	rootResources := resources.Root
	rootLinks := clientLinks(links, selections.Root)
	for index := range files {
		if files[index].Path == "client.go" {
			files[index] = clientFile(config, "client.go", config.Package, "internal/resources", rootOperations, rootResources, rootLinks, operations)
		}
	}
	// Replace the union root resource files with the root client's view.
	kept := files[:0]
	for _, file := range files {
		if !strings.HasPrefix(file.Path, "internal/resources/") && file.Path != "links.go" {
			kept = append(kept, file)
		}
	}
	files = appendResourceFiles(config, kept, rootResources)
	for _, file := range appendLinkFiles(config, nil, rootLinks, clientLinkTargets(rootLinks, operations)) {
		if file.Path == "links.go" {
			files = append(files, file)
		}
	}
	// appendResourceFiles also wires the root. clientFile already owns its import.
	for index := range files {
		if files[index].Path == "client.go" {
			files[index] = clientFile(config, "client.go", config.Package, "internal/resources", rootOperations, rootResources, rootLinks, operations)
		}
	}
	for _, name := range sortedKeys(selections.Named) {
		selected := selections.Named[name]
		entryOperations := clientOperations(operations, selected)
		if len(entryOperations) == 0 {
			return nil, fmt.Errorf("client %q has no executable selected operations", name)
		}
		entryResources := resources.Named[name]
		entryLinks := clientLinks(links, selected)
		resourcePath := "internal/clients/" + name + "/resources"
		packageName := clientPackageName(name)
		files = append(files, clientFile(config, "clients/"+name+"/client.go", packageName, resourcePath, entryOperations, entryResources, entryLinks, operations))
		for _, file := range appendResourceFiles(config, nil, entryResources) {
			file.Path = resourcePath + strings.TrimPrefix(file.Path, "internal/resources")
			files = append(files, file)
		}
		for _, file := range appendLinkFiles(config, nil, entryLinks, clientLinkTargets(entryLinks, operations)) {
			if file.Path == "links.go" {
				file.Path = "clients/" + name + "/links.go"
				file.Package = packageName
				files = append(files, file)
			}
		}
	}
	return files, nil
}

func appendClientErrorFiles(config generator.GoOptions, files []filePlan, document *ir.Document, selections clientSelections, contracts []errorContractPlan, names namePlan) []filePlan {
	filtered := func(selection *generationSelection) []errorContractPlan {
		codes := goErrorContracts(selection.view(document))
		var result []errorContractPlan
		for _, contract := range contracts {
			if _, present := codes[contract.Code]; present {
				result = append(result, contract)
			}
		}
		return result
	}
	entryNames := func(contracts []errorContractPlan) namePlan {
		result := names
		result.errorCategories = make(map[string]string)
		for _, contract := range contracts {
			if contract.Category != "" {
				result.errorCategories[contract.Category] = names.errorCategories[contract.Category]
			}
		}
		return result
	}
	root := filtered(selections.Root)
	files = appendErrorFiles(config, files, root, entryNames(root))
	for _, name := range sortedKeys(selections.Named) {
		contracts := filtered(selections.Named[name])
		for _, file := range appendErrorFiles(config, nil, contracts, entryNames(contracts)) {
			file.Path = "clients/" + name + "/errors.go"
			file.Package = clientPackageName(name)
			files = append(files, file)
		}
	}
	return files
}
