package typescript

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"

	"openapi-sdkgen/internal/compiler/ir"
)

// emitMetadata always publishes the declared version. The optional document is
// independent of client execution; its JSON string retains the source values
// without constructing a TypeScript syntax tree for every source object.
func emitMetadata(document *ir.Document, typescript, includeDocument bool) ([]byte, error) {
	if !includeDocument {
		if typescript {
			return []byte(fmt.Sprintf("/** Declared OpenAPI version. */\nexport const openapi = { version: %s, versionLine: %s } as const\n", quoteTS(document.OpenAPIVersion), quoteTS(document.OpenAPIVersionLine))), nil
		}
		return []byte(fmt.Sprintf("/** Declared OpenAPI version. */\nexport const openapi = Object.freeze({ version: %s, versionLine: %s })\n", quoteTS(document.OpenAPIVersion), quoteTS(document.OpenAPIVersionLine))), nil
	}
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
	if typescript {
		// Object.fromEntries inferred an indexable object rather than any at
		// the document root. Keep that boundary while leaving fields dynamic.
		raw += " as { [key: string]: any }"
	}
	var output bytes.Buffer
	output.WriteString("/** Lossless OpenAPI metadata, kept separate from the client call surface. */\n")
	if typescript {
		fmt.Fprintf(&output, "export const openapi = { document: %s, version: %s, versionLine: %s } as const\n", raw, quoteTS(document.OpenAPIVersion), quoteTS(document.OpenAPIVersionLine))
	} else {
		fmt.Fprintf(&output, "export const openapi = Object.freeze({ document: %s, version: %s, versionLine: %s })\n", raw, quoteTS(document.OpenAPIVersion), quoteTS(document.OpenAPIVersionLine))
	}
	return output.Bytes(), nil
}
