package sdkgen

import (
	"encoding/json"
	"fmt"

	"openapi-sdkgen/internal/compiler/ir"
)

// encodeSourceMetadata snapshots one decoded entry document before compiler
// normalization. The deterministic JSON representation is retained instead of
// a second long-lived decoded tree.
func encodeSourceMetadata(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode decoded OpenAPI source metadata: %w", err)
	}
	return data, nil
}

func attachSourceMetadata(document *ir.Document, data []byte) {
	if document == nil {
		return
	}
	document.SourceMetadataJSON = append(document.SourceMetadataJSON[:0], data...)
}
