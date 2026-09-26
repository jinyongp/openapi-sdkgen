package typescript

import "openapi-sdkgen/internal/compiler/ir"

func snapshotIRDocument(document *ir.Document) *ir.Document {
	if document == nil {
		return nil
	}
	if document.OpenAPIVersion != "" && document.Raw != nil {
		result := *document
		result.Operations = append([]ir.Operation(nil), document.Operations...)
		return &result
	}
	return snapshotSyntheticIRDocument(document)
}

func snapshotSyntheticIRDocument(document *ir.Document) *ir.Document {
	result := *document
	result.Servers = cloneIRServers(document.Servers)
	result.Security = cloneIRSecurityRequirements(document.Security)
	result.SecuritySchemes = cloneIRSecuritySchemes(document.SecuritySchemes)
	result.Operations = cloneIROperations(document.Operations)
	result.ComponentSchemas = cloneIRComponentSchemas(document.ComponentSchemas)
	result.Schemas = cloneIRSchemas(document.Schemas)
	result.Raw = cloneJSONMap(document.Raw)
	result.SourceMetadataJSON = append([]byte(nil), document.SourceMetadataJSON...)
	result.Provenance = cloneIRProvenance(document.Provenance)
	result.ErrorCategories = cloneStringStringMap(document.ErrorCategories)
	result.ParameterSortPlans = cloneIRSortParameterPlans(document.ParameterSortPlans)
	result.SemanticRestrictions = append([]ir.SemanticRestriction(nil), document.SemanticRestrictions...)
	return &result
}

func cloneIROperations(values []ir.Operation) []ir.Operation {
	if values == nil {
		return nil
	}
	result := make([]ir.Operation, len(values))
	for index, value := range values {
		item := value
		item.Tags = append([]string(nil), value.Tags...)
		item.Extensions.Envelope.Raw = cloneJSONValue(value.Extensions.Envelope.Raw)
		item.Extensions.Pagination.Raw = cloneJSONValue(value.Extensions.Pagination.Raw)
		item.Extensions.Visibility.Raw = cloneJSONValue(value.Extensions.Visibility.Raw)
		item.PaginationPlan = cloneIRPaginationPlan(value.PaginationPlan)
		item.SortParameters = cloneIRSortParameterPlans(value.SortParameters)
		item.PathParameterOrder = append([]string(nil), value.PathParameterOrder...)
		item.Parameters = cloneIRParameters(value.Parameters)
		item.RequestBody = cloneIRRequestBody(value.RequestBody)
		item.Responses = cloneIRResponses(value.Responses)
		item.Servers = cloneIRServers(value.Servers)
		item.Security = cloneIRSecurityRequirements(value.Security)
		item.PathItemRaw = cloneJSONMap(value.PathItemRaw)
		item.Raw = cloneJSONMap(value.Raw)
		result[index] = item
	}
	return result
}

func cloneIRParameters(values []ir.Parameter) []ir.Parameter {
	if values == nil {
		return nil
	}
	result := make([]ir.Parameter, len(values))
	for index, value := range values {
		item := value
		item.Content = cloneIRMediaTypes(value.Content)
		item.Schema = cloneJSONValue(value.Schema)
		item.Raw = cloneJSONMap(value.Raw)
		result[index] = item
	}
	return result
}

func cloneIRRequestBody(value *ir.RequestBody) *ir.RequestBody {
	if value == nil {
		return nil
	}
	result := *value
	result.Content = cloneIRMediaTypes(value.Content)
	result.Raw = cloneJSONMap(value.Raw)
	return &result
}

func cloneIRResponses(values []ir.Response) []ir.Response {
	if values == nil {
		return nil
	}
	result := make([]ir.Response, len(values))
	for index, value := range values {
		item := value
		item.Content = cloneIRMediaTypes(value.Content)
		item.Raw = cloneJSONMap(value.Raw)
		item.SourceRaw = cloneJSONMap(value.SourceRaw)
		result[index] = item
	}
	return result
}

func cloneIRMediaTypes(values []ir.MediaType) []ir.MediaType {
	if values == nil {
		return nil
	}
	result := make([]ir.MediaType, len(values))
	for index, value := range values {
		item := value
		item.Schema = cloneJSONValue(value.Schema)
		item.ItemSchema = cloneJSONValue(value.ItemSchema)
		item.Raw = cloneJSONMap(value.Raw)
		result[index] = item
	}
	return result
}

func cloneIRServers(values []ir.Server) []ir.Server {
	if values == nil {
		return nil
	}
	result := make([]ir.Server, len(values))
	for index, value := range values {
		item := value
		item.Raw = cloneJSONMap(value.Raw)
		item.Variables = make([]ir.ServerVariable, len(value.Variables))
		for variableIndex, variable := range value.Variables {
			cloned := variable
			cloned.Enum = append([]string(nil), variable.Enum...)
			item.Variables[variableIndex] = cloned
		}
		result[index] = item
	}
	return result
}

func cloneIRSecurityRequirements(values []ir.SecurityRequirement) []ir.SecurityRequirement {
	if values == nil {
		return nil
	}
	result := make([]ir.SecurityRequirement, len(values))
	for index, value := range values {
		item := value
		item.Raw = cloneJSONMap(value.Raw)
		item.Schemes = make([]ir.SecurityRequirementScheme, len(value.Schemes))
		for schemeIndex, scheme := range value.Schemes {
			cloned := scheme
			cloned.Scopes = append([]string(nil), scheme.Scopes...)
			item.Schemes[schemeIndex] = cloned
		}
		result[index] = item
	}
	return result
}

func cloneIRSecuritySchemes(values map[string]ir.SecurityScheme) map[string]ir.SecurityScheme {
	if values == nil {
		return nil
	}
	result := make(map[string]ir.SecurityScheme, len(values))
	for name, value := range values {
		item := value
		item.Flows = cloneJSONValue(value.Flows)
		item.Raw = cloneJSONMap(value.Raw)
		result[name] = item
	}
	return result
}

func cloneIRComponentSchemas(values map[string]map[string]any) map[string]map[string]any {
	if values == nil {
		return nil
	}
	result := make(map[string]map[string]any, len(values))
	for name, value := range values {
		result[name] = cloneJSONMap(value)
	}
	return result
}

func cloneIRSchemas(values map[string]ir.Schema) map[string]ir.Schema {
	if values == nil {
		return nil
	}
	result := make(map[string]ir.Schema, len(values))
	for name, value := range values {
		item := value
		item.Value = cloneJSONValue(value.Value)
		result[name] = item
	}
	return result
}

func cloneIRProvenance(values map[string]ir.Provenance) map[string]ir.Provenance {
	if values == nil {
		return nil
	}
	result := make(map[string]ir.Provenance, len(values))
	for pointer, value := range values {
		item := value
		item.Related = append([]ir.SourceLocation(nil), value.Related...)
		result[pointer] = item
	}
	return result
}

func cloneIRPaginationPlan(value *ir.PaginationPlan) *ir.PaginationPlan {
	if value == nil {
		return nil
	}
	result := *value
	result.ItemSchema = cloneJSONMap(value.ItemSchema)
	result.Response.Items = append([]string(nil), value.Response.Items...)
	result.Response.NextCursor = append([]string(nil), value.Response.NextCursor...)
	result.Response.Offset = append([]string(nil), value.Response.Offset...)
	result.Response.Limit = append([]string(nil), value.Response.Limit...)
	result.Response.Total = append([]string(nil), value.Response.Total...)
	return &result
}

func cloneIRSortParameterPlans(values map[string]ir.SortParameterPlan) map[string]ir.SortParameterPlan {
	if values == nil {
		return nil
	}
	result := make(map[string]ir.SortParameterPlan, len(values))
	for name, value := range values {
		item := value
		item.Values = append([]ir.SortValue(nil), value.Values...)
		result[name] = item
	}
	return result
}

func cloneStringStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	result := make(map[string]string, len(values))
	for name, value := range values {
		result[name] = value
	}
	return result
}

func cloneJSONMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	result := make(map[string]any, len(values))
	for name, value := range values {
		result[name] = cloneJSONValue(value)
	}
	return result
}

func cloneJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneJSONMap(typed)
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = cloneJSONValue(item)
		}
		return result
	case []string:
		return append([]string(nil), typed...)
	case map[string]string:
		return cloneStringStringMap(typed)
	default:
		return value
	}
}
