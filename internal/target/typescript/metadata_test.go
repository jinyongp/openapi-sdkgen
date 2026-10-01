package typescript

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
)

// metadataJSON unwraps the generated string to inspect source values rather
// than the JavaScript escape representation.
func metadataJSON(t *testing.T, source []byte) string {
	t.Helper()
	_, argument, ok := strings.Cut(string(source), "JSON.parse(")
	if !ok {
		t.Fatal("metadata has no document JSON")
	}
	var data string
	if err := json.NewDecoder(strings.NewReader(argument)).Decode(&data); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestEmitMetadataPreservesEntryValuesAndRuntimeContracts(t *testing.T) {
	original := []byte(`{"openapi":"3.2.0","info":{"title":"한글 😀","version":"1","summary":"Document summary","description":"quote \" backslash \\ newline\n separators\u2028\u2029 </script>"},"paths":{"/widgets":{"get":{"summary":"List widgets","deprecated":true,"responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"./schema.yaml#/Thing"}}}}}}}},"components":{"examples":{"widget":{"value":{"id":"1"},"dataValue":{"id":"1"},"serializedValue":"{\"id\":\"1\"}"}}},"tags":[{"name":"widgets","parent":"api","kind":"navigation"}],"servers":[{"url":"https://api.example.test","name":"production"}],"x-root":{"__proto__":{"polluted":true},"constructor":"data","prototype":"ordinary","array":[null,true,false,-0,9007199254740993,1e400,1e-400]}}`)
	document := &ir.Document{OpenAPIVersion: "3.2.0", OpenAPIVersionLine: "3.2", SourceMetadataJSON: original,
		Raw: map[string]any{"info": map[string]any{"title": "Effective"}, "components": map[string]any{"schemas": map[string]any{"Bundled": map[string]any{"type": "string"}}}}}
	snapshot := bytes.Clone(original)
	for _, typescript := range []bool{false, true} {
		t.Run(fmt.Sprint(typescript), func(t *testing.T) {
			source, err := emitMetadata(document, typescript)
			if err != nil {
				t.Fatal(err)
			}
			compact := metadataJSON(t, source)
			decoder := json.NewDecoder(strings.NewReader(compact))
			decoder.UseNumber()
			var got any
			if err := decoder.Decode(&got); err != nil {
				t.Fatal(err)
			}
			decoder = json.NewDecoder(bytes.NewReader(original))
			decoder.UseNumber()
			var want any
			if err := decoder.Decode(&want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("entry values changed: %s", compact)
			}
			if !bytes.Equal(original, snapshot) {
				t.Fatal("source metadata was mutated")
			}
			for _, unexpected := range []string{"Effective", "Bundled"} {
				if strings.Contains(compact, unexpected) {
					t.Fatalf("effective value leaked: %s", unexpected)
				}
			}
			executable := strings.TrimSuffix(string(source), " as const\n")
			if typescript {
				executable += "\n"
			}
			directory := t.TempDir()
			module := filepath.Join(directory, "metadata.mjs")
			if err := os.WriteFile(module, []byte(executable), 0600); err != nil {
				t.Fatal(err)
			}
			expected := filepath.Join(directory, "original.json")
			if err := os.WriteFile(expected, original, 0600); err != nil {
				t.Fatal(err)
			}
			probe := `import {readFileSync} from "node:fs";
import {pathToFileURL} from "node:url";
import {isDeepStrictEqual} from "node:util";
const {openapi} = await import(pathToFileURL(process.argv[1]));
const expected = JSON.parse(readFileSync(process.argv[2], "utf8"));
if (!isDeepStrictEqual(openapi.document, expected)) throw new Error("metadata values changed");
if (openapi.version !== "3.2.0" || openapi.versionLine !== "3.2") throw new Error("version changed");
if (Object.isFrozen(openapi) !== (process.argv[3] === "false")) throw new Error("outer freeze changed");
const extension = openapi.document["x-root"];
if (!Object.hasOwn(extension, "__proto__") || Object.getPrototypeOf(extension) !== Object.prototype) throw new Error("prototype changed");
if (Object.prototype.polluted) throw new Error("source executed");
if (!Object.is(extension.array[3], -0) || extension.array[5] !== Infinity || extension.array[6] !== 0) throw new Error("numeric semantics changed");
extension.mutable = true;
if (!extension.mutable) throw new Error("document mutability changed");
`
			if output, err := exec.Command("node", "--input-type=module", "--eval", probe, module, expected, fmt.Sprint(typescript)).CombinedOutput(); err != nil {
				t.Fatalf("metadata runtime: %v\n%s", err, output)
			}
		})
	}
}

func TestEmitMetadataSyntheticFallbackAndDeterminism(t *testing.T) {
	document := &ir.Document{OpenAPIVersion: "3.2.0", OpenAPIVersionLine: "3.2", Raw: map[string]any{
		"info": map[string]any{"title": "Synthetic", "version": "1"}, "openapi": "3.2.0", "paths": map[string]any{},
	}}
	source, err := emitMetadata(document, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(metadataJSON(t, source), `"title":"Synthetic"`) {
		t.Fatal("Raw fallback lost")
	}
	document.SourceMetadataJSON = []byte(` { "paths": {}, "openapi":"3.2.0", "info": { "version": "1", "title":"Synthetic" } } `)
	second, err := emitMetadata(document, true)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(source, second) {
		t.Fatal("whitespace/key order changed emission")
	}
}

func TestEmitMetadataRejectsInvalidJSON(t *testing.T) {
	for _, data := range []string{`{"x":`, `{} {}`, `{"x":NaN}`, `{"x":1,"x":2}`} {
		if _, err := emitMetadata(&ir.Document{SourceMetadataJSON: []byte(data)}, true); err == nil {
			t.Fatalf("invalid metadata accepted: %s", data)
		}
	}
	if _, err := emitMetadata(&ir.Document{Raw: map[string]any{"invalid": func() {}}}, true); err == nil {
		t.Fatal("non-JSON Raw accepted")
	}
}

func TestMetadataConsumerKeepsDocumentAndVersionTypes(t *testing.T) {
	document := &ir.Document{OpenAPIVersion: "3.2.0", OpenAPIVersionLine: "3.2", Raw: map[string]any{"info": map[string]any{"title": "Metadata"}}}
	source, err := emitMetadata(document, true)
	if err != nil {
		t.Fatal(err)
	}
	probe := `import { openapi } from "./metadata.js";
const version: "3.2.0" = openapi.version;
const line: "3.2" = openapi.versionLine;
const title: string = openapi.document.info.title;
// @ts-expect-error public metadata wrapper is readonly
openapi.version = "3.2.0";
`
	compileTypeScriptArtifactSet(t, []generator.Artifact{{Path: "metadata.ts", Data: source}}, "consumer.ts", probe)
}
