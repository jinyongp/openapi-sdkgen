package golang

import (
	"bytes"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
	"testing"
)

func TestGeneratedMetadataPreservesOriginalValues(t *testing.T) {
	original := []byte(`{"openapi":"3.2.0","info":{"title":"한글 😀","version":"1","summary":"Original","description":"quote \" backslash \\ newline\n separators\u2028\u2029 </script>"},"paths":{"/widgets":{"get":{"responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"./schema.yaml#/Thing"}}}}}}}},"components":{"examples":{"widget":{"dataValue":{"id":"1"},"serializedValue":"{\"id\":\"1\"}"}}},"tags":[{"name":"widgets","parent":"api","kind":"navigation"}],"servers":[{"url":"https://example.test","name":"production"}],"x-root":{"__proto__":{"polluted":true},"constructor":"data","prototype":"ordinary","array":[null,true,false,-0,9007199254740993,1e400,1e-400]}}`)
	document := &ir.Document{OpenAPIVersion: "3.2.0", OpenAPIVersionLine: "3.2", SourceMetadataJSON: bytes.Clone(original), Raw: map[string]any{"openapi": "3.2.0", "info": map[string]any{"title": "Effective", "version": "1"}, "paths": map[string]any{}}}
	registry, err := generator.NewAddonRegistry(generator.AddonMetadata)
	if err != nil {
		t.Fatal(err)
	}
	options, err := registry.Resolve([]string{"metadata"})
	if err != nil {
		t.Fatal(err)
	}
	options.Golang = &generator.GoOptions{ImportPath: "example.test/consumer/api"}
	plan, diagnostics, err := (Generator{}).Prepare(document, options)
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	if !bytes.Equal(original, document.SourceMetadataJSON) {
		t.Fatal("source snapshot mutated")
	}
	document.SourceMetadataJSON[0] = '!'
	testGeneratedModule(t, plan, map[string]string{"original.json": string(original), "original_test.go": `package consumer_test
import("bytes";"encoding/json";"os";"reflect";"testing";"example.test/consumer/api/metadata")
func TestOriginalDocument(t *testing.T){expected,err:=os.ReadFile("original.json");if err!=nil{t.Fatal(err)};decode:=func(data []byte)any{decoder:=json.NewDecoder(bytes.NewReader(data));decoder.UseNumber();var value any;if err:=decoder.Decode(&value);err!=nil{t.Fatal(err)};return value};if !reflect.DeepEqual(decode(metadata.Document()),decode(expected)){t.Fatal("original values changed",string(metadata.Document()))};if metadata.Version!="3.2.0"||metadata.VersionLine!="3.2"{t.Fatal("wrong declared version")}}
`})
}

func TestGoMetadataRejectsMalformedAndCanonicalizesFallback(t *testing.T) {
	for _, value := range []string{`{"x":`, `{} {}`, `{"x":NaN}`, `{"x":1,"x":2}`} {
		if _, err := appendMetadataFile(nil, &ir.Document{SourceMetadataJSON: []byte(value)}); err == nil {
			t.Fatalf("accepted malformed source metadata %s", value)
		}
	}
	document := &ir.Document{OpenAPIVersion: "3.2.0", OpenAPIVersionLine: "3.2", Raw: map[string]any{"openapi": "3.2.0", "info": map[string]any{"title": "Synthetic", "version": "1"}, "paths": map[string]any{}}}
	first, err := appendMetadataFile(nil, document)
	if err != nil {
		t.Fatal(err)
	}
	document.SourceMetadataJSON = []byte(` {"paths":{},"info":{"version":"1","title":"Synthetic"},"openapi":"3.2.0"} `)
	second, err := appendMetadataFile(nil, document)
	if err != nil || first[0].Prepared != second[0].Prepared {
		t.Fatal("metadata depends on whitespace or key order", err)
	}
}
