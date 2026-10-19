package golang

import "openapi-sdkgen/internal/generator"

func operationFiles(config generator.GoOptions, operation operationPlan) []filePlan {
	imports := []importPlan{{Path: "context"}, {Path: "net/http"}, {Path: config.ImportPath + "/internal/core"}, {Path: config.ImportPath + "/types"}}
	if operation.HasSequential {
		imports = append(imports, importPlan{Path: "io"})
	}
	needsModels, needsSchema := operation.HasInput || operation.HasHeaders, operation.ResourceBound || len(operation.MultipartHeaders) != 0 || len(operation.MultipartParts) != 0 || len(operation.MultipartResponses) != 0
	for _, group := range operation.Headers {
		for _, field := range group.Fields {
			needsSchema = needsSchema || field.ContentType == "" || field.Text || field.XMLDecoder != ""
		}
	}
	for _, parameter := range operation.Parameters {
		needsSchema = needsSchema || parameter.Encoder != ""
	}
	for _, field := range operation.InputFields {
		needsSchema = needsSchema || field.Encoder != ""
	}
	for _, body := range operation.RequestBodies {
		needsSchema = needsSchema || body.Encoder != ""
	}
	for _, response := range operation.Responses {
		needsModels = needsModels || response.Validation != ""
		needsSchema = needsSchema || response.Validation != ""
	}
	if needsModels {
		imports = append(imports, importPlan{Path: config.ImportPath + "/models"})
	}
	if needsSchema {
		imports = append(imports, importPlan{Path: config.ImportPath + "/internal/schema"})
	}
	files := []filePlan{{Path: sourceFilePath("internal/operations", operation.Name), Template: "templates/operations.go.tmpl", Package: "operations", Imports: imports, Operations: []operationPlan{operation}}}
	if operation.Pagination != nil {
		files = append(files, filePlan{Path: namedSourceFilePath("internal/operations", "pagination", operation.Name), Template: "templates/operation-pagination.go.tmpl", Package: "operations", Imports: []importPlan{{Path: "context"}, {Path: config.ImportPath + "/models"}, {Path: config.ImportPath + "/types"}}, Operations: []operationPlan{operation}})
	}
	streaming := false
	for _, response := range operation.Responses {
		streaming = streaming || response.ItemValidation != ""
	}
	if streaming {
		streamImports := []importPlan{{Path: "context"}, {Path: config.ImportPath + "/internal/core"}, {Path: config.ImportPath + "/internal/stream"}, {Path: config.ImportPath + "/internal/schema"}, {Path: config.ImportPath + "/models"}, {Path: config.ImportPath + "/types"}}
		if operation.HasTypedStream {
			streamImports = append(streamImports, importPlan{Path: "net/http"})
		}
		files = append(files, filePlan{Path: namedSourceFilePath("internal/operations", "stream", operation.Name), Template: "templates/operation-stream.go.tmpl", Package: "operations", Imports: streamImports, Operations: []operationPlan{operation}})
	}
	if len(operation.RequestStreams) != 0 {
		requestImports := []importPlan{{Path: "context"}, {Path: config.ImportPath + "/internal/core"}, {Path: config.ImportPath + "/types"}}
		needsRequestModels, needsNative := false, false
		for _, request := range operation.RequestStreams {
			needsRequestModels = needsRequestModels || request.ItemValidation != ""
			needsNative = needsNative || request.Framing != "custom"
		}
		if needsRequestModels {
			requestImports = append(requestImports, importPlan{Path: config.ImportPath + "/models"})
		}
		if needsNative {
			requestImports = append(requestImports, importPlan{Path: config.ImportPath + "/internal/stream"})
		}
		files = append(files, filePlan{Path: namedSourceFilePath("internal/operations", "stream_request", operation.Name), Template: "templates/operation-stream-request.go.tmpl", Package: "operations", Imports: requestImports, Operations: []operationPlan{operation}})
	}
	if operation.HasInput {
		var modelImports []importPlan
		needsTypes := operation.BodyUnion
		for _, field := range operation.InputFields {
			needsTypes = needsTypes || !field.Required || field.Binary || field.Incremental
		}
		if needsTypes {
			modelImports = append(modelImports, importPlan{Path: config.ImportPath + "/types"})
		}
		if operation.BodyUnion {
			modelImports = append(modelImports, importPlan{Path: "context"})
		}
		files = append(files, filePlan{Path: namedSourceFilePath("models", "input", operation.Name), Template: "templates/models.go.tmpl", Package: "models", Imports: modelImports, Operations: []operationPlan{operation}})
	}
	return files
}
