package golang

import "openapi-sdkgen/internal/generator"

func appendResourceFiles(config generator.GoOptions, files []filePlan, resources []resourceNodePlan) []filePlan {
	if len(resources) == 0 {
		return files
	}
	for index := range files {
		if files[index].Path == "client.go" {
			files[index].Imports = append(files[index].Imports, importPlan{Path: config.ImportPath + "/internal/resources"})
			files[index].Resources = resources
		}
	}
	for _, resource := range resources {
		imports := []importPlan{{Path: config.ImportPath + "/internal/core"}}
		models := false
		if resource.Bound || resource.Selector != nil {
			imports = append(imports, importPlan{Path: config.ImportPath + "/types"})
		}
		if resource.Selector != nil {
			imports = append(imports, importPlan{Path: "errors"})
			if resource.Selector.Model != "" {
				models = true
				imports = append(imports, importPlan{Path: "context"})
			}
		}
		if len(resource.Calls) != 0 {
			imports = append(imports, importPlan{Path: config.ImportPath + "/internal/operations"})
			for _, call := range resource.Calls {
				models = models || call.Operation.ResourceHasInput
			}
		}
		if models {
			imports = append(imports, importPlan{Path: config.ImportPath + "/models"})
		}
		files = append(files, filePlan{Path: sourceFilePath("internal/resources", resource.Name), Package: "resources", Template: "templates/resources.go.tmpl", Imports: imports, Resources: []resourceNodePlan{resource}})
	}
	return files
}
