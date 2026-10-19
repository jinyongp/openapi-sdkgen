// Package extension owns target-neutral SDK extension validation. Language
// adapters supply the existing synthetic-IR views and item representation key.
package extension

import (
	"encoding/json"
	"openapi-sdkgen/internal/compiler/ir"
	httpplan "openapi-sdkgen/internal/http/plan"
	schemaplan "openapi-sdkgen/internal/schema/plan"
	"sort"
	"strings"
)

type Planner struct {
	Target            string
	Parameters        func(*ir.Document, ir.Operation) ([]ir.Parameter, error)
	Responses         func(*ir.Document, ir.Operation) ([]ir.Response, error)
	Snapshot          func(*ir.Document) *ir.Document
	ItemTypeSignature func(*ir.Document, map[string]any) string
}

func (planner *Planner) parameters(document *ir.Document, operation ir.Operation) ([]ir.Parameter, error) {
	if planner.Parameters != nil {
		return planner.Parameters(document, operation)
	}
	return operation.Parameters, nil
}
func (planner *Planner) responses(document *ir.Document, operation ir.Operation) ([]ir.Response, error) {
	if planner.Responses != nil {
		return planner.Responses(document, operation)
	}
	return operation.Responses, nil
}
func (planner *Planner) snapshot(document *ir.Document) *ir.Document {
	if planner.Snapshot != nil {
		return planner.Snapshot(document)
	}
	result := *document
	result.Operations = append([]ir.Operation(nil), document.Operations...)
	return &result
}
func (planner *Planner) itemSignature(document *ir.Document, schema map[string]any) string {
	if planner.ItemTypeSignature != nil {
		return planner.ItemTypeSignature(document, schema)
	}
	encoded, _ := json.Marshal(planner.ResolveSchema(document, schema, make(map[string]bool)))
	return string(encoded)
}
func componentSchemaReferenceName(reference string) (string, error) {
	return schemaplan.ComponentReferenceName(reference)
}
func operationRouteKey(operation ir.Operation) string {
	if operation.RouteKey != "" {
		return operation.RouteKey
	}
	return operation.Method + " " + operation.Path
}
func reachableSuccessResponses(responses []ir.Response) []ir.Response {
	return httpplan.SuccessResponses(responses)
}
func isBinaryMediaForDocument(document *ir.Document, media string, schema map[string]any) bool {
	version := ""
	if document != nil {
		version = document.OpenAPIVersionLine
	}
	return httpplan.IsBinaryMedia(version, media, schema)
}
func isJSONMediaType(media string) bool {
	bare := strings.ToLower(strings.TrimSpace(strings.SplitN(media, ";", 2)[0]))
	return bare == "application/json" || strings.HasSuffix(bare, "+json")
}
func isTextMedia(media string) bool {
	media = strings.ToLower(media)
	return strings.HasPrefix(media, "text/") && !strings.Contains(media, "xml")
}
func boolValue(value map[string]any, key string) bool { result, _ := value[key].(bool); return result }
func schemaTypes(value any) []string {
	switch value := value.(type) {
	case string:
		return []string{value}
	case []any:
		var result []string
		for _, item := range value {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	}
	return nil
}
func sortedAnyKeys[T any](values map[string]T) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
func mapStringAny[T any](values map[string]T) map[string]any {
	result := make(map[string]any, len(values))
	for key := range values {
		result[key] = nil
	}
	return result
}
func sortedStringKeys(values map[string]bool) []string { return sortedAnyKeys(values) }
func quotedStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		data, _ := json.Marshal(value)
		result = append(result, string(data))
	}
	return result
}
