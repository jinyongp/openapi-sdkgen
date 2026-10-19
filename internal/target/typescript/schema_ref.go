package typescript

import schemaplan "openapi-sdkgen/internal/schema/plan"

func componentSchemaReferenceName(reference string) (string, error) {
	return schemaplan.ComponentReferenceName(reference)
}
