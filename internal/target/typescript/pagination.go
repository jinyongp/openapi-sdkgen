package typescript

import (
	"encoding/json"
	"openapi-sdkgen/internal/compiler/ir"
)

func paginationSchemaSignature(document *ir.Document, schema map[string]any) string {
	resolved := resolveSchemaReference(document, schema, make(map[string]bool))
	if value, err := schemaTypeForScope(document, resolved, projectionOutput, typeRenderLocal); err == nil {
		return value
	}
	encoded, _ := json.Marshal(resolved)
	return string(encoded)
}

func paginationRuntimePlanExpression(plan ir.PaginationPlan) (string, error) {
	request := map[string]any{}
	if plan.Request.Cursor != "" {
		request["cursor"] = plan.Request.Cursor
	}
	if plan.Request.Offset != "" {
		request["offset"] = plan.Request.Offset
	}
	if plan.Request.Limit != "" {
		request["limit"] = plan.Request.Limit
	}
	response := map[string]any{"items": plan.Response.Items}
	if plan.Response.NextCursor != nil {
		response["nextCursor"] = plan.Response.NextCursor
	}
	if plan.Response.Offset != nil {
		response["offset"] = plan.Response.Offset
	}
	if plan.Response.Limit != nil {
		response["limit"] = plan.Response.Limit
	}
	if plan.Response.Total != nil {
		response["total"] = plan.Response.Total
	}
	return runtimeJSONExpression(map[string]any{
		"mode":     plan.Mode,
		"request":  request,
		"response": response,
	})
}
