package typescript

import (
	"openapi-sdkgen/internal/compiler/ir"
	httpplan "openapi-sdkgen/internal/http/plan"
)

func multipartInputSchema(document *ir.Document, contentType string, value any, media map[string]any) any {
	return httpplan.MultipartInputSchema(document, contentType, value, media)
}
func multipartItemInputSchema(document *ir.Document, contentType string, value any, encoding any) any {
	return httpplan.MultipartItemInputSchema(document, contentType, value, encoding)
}
