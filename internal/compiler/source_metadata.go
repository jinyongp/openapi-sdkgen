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

// sourceMetadataFromOwnedInput reuses valid JSON bytes loaded by the compiler.
// Generated metadata decodes this buffer before deterministic emission, so JSON
// whitespace/key order is not part of the public contract. YAML still needs a
// JSON representation of the decoded entry source.
func sourceMetadataFromOwnedInput(data []byte, value any) ([]byte, error) {
	if json.Valid(data) {
		return data, nil
	}
	return encodeSourceMetadata(value)
}

func attachSourceMetadata(document *ir.Document, data []byte) {
	if document == nil {
		return
	}
	// encodeSourceMetadata returns a fresh compiler-owned buffer. Transfer that
	// ownership to the IR instead of copying the full entry document again.
	document.SourceMetadataJSON = data
}
