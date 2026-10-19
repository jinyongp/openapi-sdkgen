package typescript

import (
	httpplan "openapi-sdkgen/internal/http/plan"

	"openapi-sdkgen/internal/compiler/ir"
)

func resolveComponentObject(document *ir.Document, object map[string]any, component string) (map[string]any, error) {
	return httpplan.ResolveComponentObject(document, object, component)
}

// resolveMediaTypeObject resolves an OpenAPI 3.2 Media Type Object reference.
// It shares the Component Object merge semantics used by other reusable
// OpenAPI surfaces.
func resolveMediaTypeObject(document *ir.Document, object map[string]any) (map[string]any, error) {
	return resolveComponentObject(document, object, "mediaTypes")
}

func componentObjectFieldPointer(document *ir.Document, object map[string]any, component, pointer, field string) (string, error) {
	return httpplan.ComponentObjectFieldPointer(document, object, component, pointer, field)
}

func componentReferenceName(reference, component string) (string, error) {
	return httpplan.ComponentReferenceName(reference, component)
}
