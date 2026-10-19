package golang

import (
	"bytes"
	"fmt"
	"openapi-sdkgen/internal/generator"
)

func appendStaticSelectionFiles(config generator.GoOptions, files []filePlan, operations []operationPlan) []filePlan {
	if len(operations) == 0 {
		return files
	}
	files = append(files, filePlan{Path: "operations/reference.go", Package: "operations", Template: "runtime/operation-reference.go.tmpl", Imports: []importPlan{{Path: config.ImportPath + "/internal/core"}, {Alias: "internalops", Path: config.ImportPath + "/internal/operations"}, {Path: config.ImportPath + "/types"}}})
	aliases := clientFile(config, "operations/types.go", "operations", "", operations, nil, nil, operations)
	aliases.ExportsOnly = true
	files = append(files, aliases)
	for _, operation := range operations {
		if operation.DependencyOnly {
			continue
		}
		var body bytes.Buffer
		argument := ""
		forward := ""
		if operation.HasInput {
			argument = "input models." + operation.Name + "Input"
			forward = "input"
			if !operation.RequiredInput {
				argument = "inputs ...models." + operation.Name + "Input"
				forward = "inputs..."
			}
		}
		binding := operation.Name + "Operation"
		fmt.Fprintf(&body, "type %s struct{client *core.Client", binding)
		if operation.LinkSource {
			fmt.Fprintf(&body, ";Links internalops.%sLinks", operation.Name)
		}
		fmt.Fprintln(&body, "}")
		fmt.Fprintf(&body, "func(operation %s)Call(%s)internalops.%sCall{return internalops.New(operation.client).%s(%s)}\n", binding, argument, operation.Name, operation.Name, forward)
		fmt.Fprintf(&body, "func(operation %s)Dynamic()DynamicOperation{return internalops.NewDynamic%s(operation.client)}\n", binding, operation.Name)
		fmt.Fprint(&body, operation.Documentation)
		fmt.Fprintf(&body, "func %s()Ref[%s]{return Ref[%s]{info:types.OperationInfo{OperationID:%q,Route:%q,Method:%q,Path:%q},bind:func(client *core.Client)%s{return %s{client:client", operation.Name, binding, binding, operation.OperationID, operation.Route, operation.Method, operation.Path, binding, binding)
		if operation.LinkSource {
			fmt.Fprintf(&body, ",Links:internalops.NewLinks(client).%s", operation.Name)
		}
		fmt.Fprintln(&body, "}}}}")
		imports := []importPlan{{Path: config.ImportPath + "/internal/core"}, {Alias: "internalops", Path: config.ImportPath + "/internal/operations"}, {Path: config.ImportPath + "/types"}}
		if operation.HasInput {
			imports = append(imports, importPlan{Path: config.ImportPath + "/models"})
		}
		files = append(files, filePlan{Path: namedSourceFilePath("operations", "reference", operation.Name), Package: "operations", Template: "templates/prepared.go.tmpl", Imports: imports, Prepared: body.String()})
	}
	files = append(files, filePlan{Path: "selective/client.go", Package: "selective", Template: "runtime/selective.go.tmpl", Imports: []importPlan{{Path: "sort"}, {Path: config.ImportPath + "/internal/core"}, {Path: config.ImportPath + "/operations"}, {Path: config.ImportPath + "/types"}}})
	files = appendRegistryFiles(config, files, operations)
	return files
}

func appendRegistryFiles(config generator.GoOptions, files []filePlan, operations []operationPlan) []filePlan {
	var body bytes.Buffer
	fmt.Fprintln(&body, "var state struct{once sync.Once;ids,routes map[string]operations.Ref[operations.DynamicOperation]}")
	fmt.Fprintln(&body, "func load(){state.once.Do(func(){state.ids=make(map[string]operations.Ref[operations.DynamicOperation]);state.routes=make(map[string]operations.Ref[operations.DynamicOperation]);for _,ref:=range []operations.Ref[operations.DynamicOperation]{")
	for _, operation := range operations {
		if !operation.DependencyOnly {
			fmt.Fprintf(&body, "operations.%s().Dynamic(),\n", operation.Name)
		}
	}
	fmt.Fprintln(&body, "}{info:=ref.Info();if info.OperationID!=\"\"{state.ids[info.OperationID]=ref};state.routes[info.Route]=ref}})}")
	fmt.Fprintln(&body, "func LookupOperation(id string)(operations.Ref[operations.DynamicOperation],bool){load();ref,present:=state.ids[id];return ref,present}\nfunc LookupRoute(route string)(operations.Ref[operations.DynamicOperation],bool){load();ref,present:=state.routes[route];return ref,present}")
	files = append(files, filePlan{Path: "internal/registry/registry.go", Package: "registry", Template: "templates/prepared.go.tmpl", Imports: []importPlan{{Path: "sync"}, {Path: config.ImportPath + "/operations"}}, Prepared: body.String()})
	lookup := "func LookupOperation(id string)(operations.Ref[operations.DynamicOperation],bool){return registry.LookupOperation(id)}\nfunc LookupRoute(route string)(operations.Ref[operations.DynamicOperation],bool){return registry.LookupRoute(route)}\n"
	return append(files, filePlan{Path: "selective/lookup.go", Package: "selective", Template: "templates/prepared.go.tmpl", Imports: []importPlan{{Path: config.ImportPath + "/internal/registry"}, {Path: config.ImportPath + "/operations"}}, Prepared: lookup})
}
