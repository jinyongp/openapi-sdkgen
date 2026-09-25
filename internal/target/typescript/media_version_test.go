package typescript

import (
	"testing"

	"openapi-sdkgen/internal/compiler/ir"
)

func TestBinaryMediaClassificationIsVersionAware(t *testing.T) {
	formatBinary := map[string]any{"type": "string", "format": "binary"}
	contentEncodingBinary := map[string]any{"type": "string", "contentEncoding": "binary"}
	tests := []struct {
		name      string
		document  *ir.Document
		mediaType string
		schema    map[string]any
		want      bool
	}{
		{name: "3.0 format binary", document: &ir.Document{OpenAPIVersionLine: "3.0"}, mediaType: "application/pdf", schema: formatBinary, want: true},
		{name: "3.1 format binary annotation", document: &ir.Document{OpenAPIVersionLine: "3.1"}, mediaType: "application/pdf", schema: formatBinary, want: false},
		{name: "3.2 format binary annotation", document: &ir.Document{OpenAPIVersionLine: "3.2"}, mediaType: "application/pdf", schema: formatBinary, want: false},
		{name: "3.1 contentEncoding binary", document: &ir.Document{OpenAPIVersionLine: "3.1"}, mediaType: "application/pdf", schema: contentEncodingBinary, want: true},
		{name: "3.2 contentEncoding binary", document: &ir.Document{OpenAPIVersionLine: "3.2"}, mediaType: "application/pdf", schema: contentEncodingBinary, want: true},
		{name: "3.2 octet stream", document: &ir.Document{OpenAPIVersionLine: "3.2"}, mediaType: "application/octet-stream", schema: formatBinary, want: true},
		{name: "json stays structured", document: &ir.Document{OpenAPIVersionLine: "3.0"}, mediaType: "application/json", schema: formatBinary, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isBinaryMediaForDocument(test.document, test.mediaType, test.schema); got != test.want {
				t.Fatalf("isBinaryMediaForDocument = %t, want %t", got, test.want)
			}
		})
	}
}
