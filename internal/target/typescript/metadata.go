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
			return []byte(fmt.Sprintf("interface OpenAPIMetadata { readonly version: %s; readonly versionLine: %s }\n/** Declared OpenAPI version. */\nexport const openapi: OpenAPIMetadata = { version: %s, versionLine: %s } as const\n", quoteTS(document.OpenAPIVersion), quoteTS(document.OpenAPIVersionLine), quoteTS(document.OpenAPIVersion), quoteTS(document.OpenAPIVersionLine))), nil
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
		raw += " as OpenAPIDocument"
	}
	var output bytes.Buffer
	if typescript {
		output.WriteString("type OpenAPIDocument = Record<string, any>\n")
		fmt.Fprintf(&output, "interface OpenAPIMetadata { readonly document: OpenAPIDocument; readonly version: %s; readonly versionLine: %s }\n", quoteTS(document.OpenAPIVersion), quoteTS(document.OpenAPIVersionLine))
	}
	output.WriteString("/** Lossless OpenAPI metadata, kept separate from the client call surface. */\n")
	if typescript {
		fmt.Fprintf(&output, "export const openapi: OpenAPIMetadata = { document: %s, version: %s, versionLine: %s } as const\n", raw, quoteTS(document.OpenAPIVersion), quoteTS(document.OpenAPIVersionLine))
	} else {
		fmt.Fprintf(&output, "export const openapi = Object.freeze({ document: %s, version: %s, versionLine: %s })\n", raw, quoteTS(document.OpenAPIVersion), quoteTS(document.OpenAPIVersionLine))
	}
	return output.Bytes(), nil
}

// Selection metadata keeps its exact readonly tuple and literal types.
func emitLiteralMetadata(name, typeName string, data []byte) ([]byte, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	definition, err := readonlyJSONType(value)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	emitTypedConstant(&output, "export ", name, typeName, string(data)+" as const")
	fmt.Fprintf(&output, "type %s = %s\n", typeName, definition)
	return output.Bytes(), nil
}
