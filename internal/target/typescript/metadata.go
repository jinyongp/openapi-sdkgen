package typescript

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"

	"openapi-sdkgen/internal/compiler/ir"
)

// emitMetadata publishes the lossless entry document independently of client
// execution. One JSON string avoids a TypeScript syntax tree and nested runtime
// construction calls for every source object. JSON.parse retains ordinary own
// properties such as __proto__, just like the previous Object.fromEntries form.
func emitMetadata(document *ir.Document, typescript bool) ([]byte, error) {
	data := document.SourceMetadataJSON
	if len(data) == 0 {
		var err error
		data, err = json.Marshal(document.Raw)
		if err != nil {
			return nil, fmt.Errorf("encode OpenAPI metadata: %w", err)
		}
	}
	// Format owns its buffer: never rewrite the compiler's source snapshot.
	compact := jsontext.Value(bytes.Clone(data))
	if err := compact.Format(jsontext.ReorderRawObjects(true)); err != nil {
		return nil, fmt.Errorf("encode OpenAPI metadata: %w", err)
	}
	raw := "/* @__PURE__ */ JSON.parse(" + quoteTS(string(compact)) + ")"
	var output bytes.Buffer
	output.WriteString("/** Lossless OpenAPI metadata, kept separate from the client call surface. */\n")
	if typescript {
		fmt.Fprintf(&output, "export const openapi = { document: %s, version: %s, versionLine: %s } as const\n", raw, quoteTS(document.OpenAPIVersion), quoteTS(document.OpenAPIVersionLine))
	} else {
		fmt.Fprintf(&output, "export const openapi = Object.freeze({ document: %s, version: %s, versionLine: %s })\n", raw, quoteTS(document.OpenAPIVersion), quoteTS(document.OpenAPIVersionLine))
	}
	return output.Bytes(), nil
}
