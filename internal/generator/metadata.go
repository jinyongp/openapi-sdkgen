package generator

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
)

// SourceMetadata preserves entry values before target selection or bundling.
// Formatting owns its bytes and rejects malformed/duplicate JSON members.
func SourceMetadata(document *ir.Document) ([]byte, error) {
	data := document.SourceMetadataJSON
	if len(data) == 0 {
		var err error
		data, err = json.Marshal(document.Raw)
		if err != nil {
			return nil, fmt.Errorf("encode OpenAPI metadata: %w", err)
		}
	}
	compact := jsontext.Value(bytes.Clone(data))
	if err := compact.Format(jsontext.ReorderRawObjects(true)); err != nil {
		return nil, fmt.Errorf("encode OpenAPI metadata: %w", err)
	}
	return []byte(compact), nil
}
