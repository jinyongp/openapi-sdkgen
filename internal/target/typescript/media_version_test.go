package typescript

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
)

func TestDeclaredBinaryResponsePlansDecodePDFAndZIP(t *testing.T) {
	input, err := os.ReadFile("../../../test/typescript/fixtures/runtime-quality-3-0-3.openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	document, err := compiler.Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range document.Operations {
		if operation.OperationID != "pdf" && operation.OperationID != "zip" {
			continue
		}
		plan, _, err := newWireRenderContext(wirePropertiesLiteral).operationResponseWireBodies(document, operation)
		if err != nil || !strings.Contains(plan, "binary: true") {
			t.Fatalf("binary response plan: %s, %v", plan, err)
		}
	}
	output := compileTypeScriptArtifacts(t, document)
	script := `
import { pathToFileURL } from "node:url";
const { createClient } = await import(pathToFileURL(process.argv[1]).href);
for (const [operation, contentType] of Object.entries({ pdf: "application/pdf", zip: "application/zip" })) {
  const api = createClient({ baseURL: "https://binary.test", fetch: async () => new Response(new Uint8Array([1,2,3]), { headers: { "content-type": contentType } }) });
  const stream = await api.$operations[operation]();
  if (!(stream instanceof ReadableStream)) throw new Error("binary result was not a stream");
  if (JSON.stringify([...new Uint8Array(await new Response(stream).arrayBuffer())]) !== "[1,2,3]") throw new Error("binary bytes changed");
  const raw = await api.$operations[operation].raw();
  if (!(raw.data instanceof ReadableStream) || raw.response.status !== 200) throw new Error("raw binary result changed");
  let decoded = 0;
  const custom = createClient({ baseURL: "https://binary.test", codecs: { [contentType]: { decode: async response => { decoded++; return response.body; } } }, fetch: async () => new Response(new Uint8Array([4]), { headers: { "content-type": contentType } }) });
  const customStream = await custom.$operations[operation]();
  if (decoded !== 1 || new Uint8Array(await new Response(customStream).arrayBuffer())[0] !== 4) throw new Error("custom binary codec was bypassed");
}
`
	if result, err := exec.Command("node", "--input-type=module", "--eval", script, filepath.Join(output, "index.js")).CombinedOutput(); err != nil {
		t.Fatalf("binary SDK execution: %v\n%s", err, result)
	}
}

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
