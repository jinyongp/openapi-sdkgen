package typescript

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

// emitMetadata publishes the lossless OpenAPI document for fields that inform
// consumers (documentation, examples, tags, extensions) without changing a
// request's transport semantics. Executable features continue to use generated
// client code or a feature-path diagnostic.
func emitMetadata(document *ir.Document, typescript bool) ([]byte, error) {
	var raw string
	var err error
	if len(document.SourceMetadataJSON) != 0 {
		raw, err = runtimeJSONExpressionFromJSON(document.SourceMetadataJSON)
	} else {
		raw, err = runtimeJSONExpression(document.Raw)
	}
	if err != nil {
		return nil, fmt.Errorf("encode OpenAPI metadata: %w", err)
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

func runtimeJSONExpressionFromJSON(data []byte) (string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	rendered, err := decodeRuntimeJSONExpression(decoder)
	if err != nil {
		return "", fmt.Errorf("decode source JSON: %w", err)
	}
	if token, trailingErr := decoder.Token(); trailingErr != io.EOF {
		if trailingErr != nil {
			return "", fmt.Errorf("decode trailing source JSON: %w", trailingErr)
		}
		return "", fmt.Errorf("source JSON has trailing token %v", token)
	}
	return rendered, nil
}

func decodeRuntimeJSONExpression(decoder *json.Decoder) (string, error) {
	token, err := decoder.Token()
	if err != nil {
		return "", err
	}
	delimiter, structured := token.(json.Delim)
	if !structured {
		switch value := token.(type) {
		case string:
			return quoteTS(value), nil
		case json.Number:
			return value.String(), nil
		case bool:
			if value {
				return "true", nil
			}
			return "false", nil
		case nil:
			return "null", nil
		default:
			return "", fmt.Errorf("unsupported JSON token %T", token)
		}
	}
	switch delimiter {
	case '{':
		properties := make([]runtimeProperty, 0)
		for decoder.More() {
			keyToken, keyErr := decoder.Token()
			if keyErr != nil {
				return "", keyErr
			}
			key, ok := keyToken.(string)
			if !ok {
				return "", fmt.Errorf("object key token is %T", keyToken)
			}
			value, valueErr := decodeRuntimeJSONExpression(decoder)
			if valueErr != nil {
				return "", fmt.Errorf("JSON property %q: %w", key, valueErr)
			}
			properties = append(properties, runtimeProperty{key: key, value: value})
		}
		closeToken, closeErr := decoder.Token()
		if closeErr != nil {
			return "", closeErr
		}
		if closeToken != json.Delim('}') {
			return "", fmt.Errorf("object closed with token %v", closeToken)
		}
		return runtimeObjectExpression(properties), nil
	case '[':
		var output strings.Builder
		output.WriteByte('[')
		index := 0
		for decoder.More() {
			value, valueErr := decodeRuntimeJSONExpression(decoder)
			if valueErr != nil {
				return "", fmt.Errorf("JSON item %d: %w", index, valueErr)
			}
			if index > 0 {
				output.WriteString(", ")
			}
			output.WriteString(value)
			index++
		}
		closeToken, closeErr := decoder.Token()
		if closeErr != nil {
			return "", closeErr
		}
		if closeToken != json.Delim(']') {
			return "", fmt.Errorf("array closed with token %v", closeToken)
		}
		output.WriteByte(']')
		return output.String(), nil
	default:
		return "", fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
}
