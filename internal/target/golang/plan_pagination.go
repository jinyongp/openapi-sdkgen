package golang

import (
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/http/extension"
	schemaplan "openapi-sdkgen/internal/schema/plan"
	"strconv"
	"strings"
)

type paginationPlan struct {
	ItemModel, Validation, Configuration, Mode string
	Query                                      inputFieldPlan
}

func preparePagination(planned *operationPlan, operation ir.Operation, document *ir.Document, schemas *schemaPlanner) error {
	source := operation.PaginationPlan
	if source == nil {
		return nil
	}
	page := &paginationPlan{ItemModel: planned.Name + "PaginationItem", Mode: source.Mode}
	for _, field := range planned.InputFields {
		if field.Name == "Query" {
			page.Query = field
		}
	}
	if page.Query.Type == "" {
		return fmt.Errorf("pagination requires query contract for %s", operation.RouteKey)
	}
	itemSchema := source.ItemSchema
	variants, err := (&extension.Planner{}).PaginationItemSchemas(document, operation, source.Response.Items)
	if err != nil {
		return err
	}
	if len(variants) > 1 {
		branches := make([]any, len(variants))
		for index, variant := range variants {
			branches[index] = variant
		}
		itemSchema = map[string]any{"anyOf": branches}
	}
	validation, err := schemas.prepare(httpValidationName(page.ItemModel), itemSchema, schemaplan.Output, document.OpenAPIVersionLine == "3.0", schemaOccurrence{Source: operation.Pointer + "/x-pagination/item"})
	if err != nil {
		return err
	}
	page.Validation = validation
	pointer := func(tokens []string) string {
		if tokens == nil {
			return "nil"
		}
		var values []string
		for _, token := range tokens {
			values = append(values, strconv.Quote(token))
		}
		return "[]string{" + strings.Join(values, ",") + "}"
	}
	page.Configuration = fmt.Sprintf("types.PaginationConfig{Mode:%q,Cursor:%q,Offset:%q,Limit:%q,Items:%s,NextCursor:%s,ResponseOffset:%s,ResponseLimit:%s,Total:%s}", source.Mode, source.Request.Cursor, source.Request.Offset, source.Request.Limit, pointer(source.Response.Items), pointer(source.Response.NextCursor), pointer(source.Response.Offset), pointer(source.Response.Limit), pointer(source.Response.Total))
	planned.Pagination = page
	return nil
}
