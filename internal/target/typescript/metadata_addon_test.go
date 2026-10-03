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

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

func TestMetadataAddonFullAndSelectedConsumerContracts(t *testing.T) {
	// The selection fixture intentionally has an invalid extension on an
	// excluded API. Use a valid document when comparing full generation too.
	document, err := sdkgen.Compile([]byte(strings.Replace(generationSelectionFixture, `"x-envelope":false,`, "", 1)))
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(document.SourceMetadataJSON)
	registry, err := generator.NewAddonRegistry(generator.AddonMetadata, generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	for _, selected := range []bool{false, true} {
		for _, server := range []bool{false, true} {
			for _, metadata := range []bool{false, true} {
				t.Run(fmt.Sprintf("selected=%v/server=%v/metadata=%v", selected, server, metadata), func(t *testing.T) {
					var addons []string
					if server {
						addons = append(addons, "server")
					}
					if metadata {
						addons = append(addons, "metadata")
					}
					options, err := registry.Resolve(addons)
					if err != nil {
						t.Fatal(err)
					}
					if selected {
						options.Selection = &generator.Selection{Operations: []string{"a"}}
					}
					artifacts, err := (Generator{}).Generate(document, options)
					if err != nil {
						t.Fatal(err)
					}
					source := artifactByPath(t, artifacts, "metadata.ts")
					if metadata {
						var got, want any
						if err := json.Unmarshal([]byte(metadataJSON(t, source)), &got); err != nil {
							t.Fatal(err)
						}
						if err := json.Unmarshal(before, &want); err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(got, want) {
							t.Fatal("source document narrowed or normalized")
						}
					} else if bytes.Contains(source, []byte("document:")) || bytes.Contains(source, []byte("/unused")) {
						t.Fatal("default leaked source document")
					}
					if bytes.Contains(source, []byte("generationSelection")) != (selected && metadata) {
						t.Fatal("selection metadata changed")
					}
					probe := `import {openapi} from "./metadata.js";
import {createClient} from "./index.js";
export const version: "3.2.1" = openapi.version;
export const line: "3.2" = openapi.versionLine;
declare const api: ReturnType<typeof createClient>;
api.$operations.a; api.$links.a.next;
// @ts-expect-error wrapper is readonly
openapi.version = "3.2.1";
`
					if metadata {
						probe += "export const title: string = openapi.document['info'].title;\n"
					} else {
						probe += "// @ts-expect-error document requires the metadata add-on\nopenapi.document;\n"
					}
					output := compileTypeScriptArtifactSet(t, artifacts, "consumer.ts", probe)
					script := `import {pathToFileURL} from "node:url";
const root = process.argv[1];
const {openapi} = await import(pathToFileURL(root + "/metadata.js"));
if (Object.hasOwn(openapi, "document") !== (process.argv[2] === "true")) throw Error("document availability changed");
const {createClient} = await import(pathToFileURL(root + "/index.js"));
const seen = [];
const api = createClient({baseURL: "https://example.test", fetch: async url => {seen.push(String(url)); return new Response('{"id":"one"}', {headers:{"content-type":"application/json"}});}});
await api.$links.a.next(await api.$operations.a.raw());
if (seen.length !== 2 || !seen[0].endsWith("/a") || !seen[1].endsWith("/b")) throw Error("client Link execution changed");
if (process.argv[3] === "true" && ("b" in api.$operations || "unused" in api.$operations)) throw Error("selection exposed dependency or excluded API");
`
					if result, err := exec.Command("node", "--input-type=module", "--eval", script, output, fmt.Sprint(metadata), fmt.Sprint(selected)).CombinedOutput(); err != nil {
						t.Fatalf("runtime: %v\n%s", err, result)
					}
					if !selected && !server && !metadata {
						convenience, err := SourceArtifacts(document)
						if err != nil || !reflect.DeepEqual(convenience, artifacts) {
							t.Fatalf("default entry points disagree: %v", err)
						}
					}
				})
			}
		}
	}
	if !bytes.Equal(before, document.SourceMetadataJSON) {
		t.Fatal("source snapshot changed")
	}
}

func TestMetadataAddonPreservesEntryBeforeExternalReferenceBundling(t *testing.T) {
	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			directory := t.TempDir()
			reference := filepath.Join(directory, "schema.json")
			if err := os.WriteFile(reference, []byte(`{"type":"object","properties":{"id":{"type":"string"}}}`), 0600); err != nil {
				t.Fatal(err)
			}
			input := `{"openapi":"3.2.1","info":{"title":"External source","version":"1"},"paths":{"/items":{"get":{"operationId":"items","responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"./schema.json"}}}}}}}},"x-original":"kept"}`
			if format == "yaml" {
				input = "openapi: 3.2.1\ninfo: {title: External source, version: '1'}\npaths:\n  /items:\n    get:\n      operationId: items\n      responses:\n        '200':\n          description: OK\n          content:\n            application/json:\n              schema: {$ref: './schema.json'}\nx-original: kept\n"
			}
			path := filepath.Join(directory, "entry."+format)
			if err := os.WriteFile(path, []byte(input), 0600); err != nil {
				t.Fatal(err)
			}
			document, err := sdkgen.CompileFile(path)
			if err != nil {
				t.Fatal(err)
			}
			artifacts, err := sourceArtifactsWithMetadata(document)
			if err != nil {
				t.Fatal(err)
			}
			metadata := metadataJSON(t, artifactByPath(t, artifacts, "metadata.ts"))
			if !strings.Contains(metadata, `"$ref":"./schema.json"`) || !strings.Contains(metadata, `"x-original":"kept"`) || strings.Contains(metadata, `"properties"`) {
				t.Fatalf("entry source changed: %s", metadata)
			}
		})
	}
}

func TestMetadataAddonSelectedSyntheticSourceFallbackIsOptional(t *testing.T) {
	document, err := sdkgen.Compile([]byte(strings.Replace(generationSelectionFixture, `"x-envelope":false,`, "", 1)))
	if err != nil {
		t.Fatal(err)
	}
	document.SourceMetadataJSON = nil
	components := document.Raw["components"].(map[string]any)
	components["callbacks"] = map[string]any{"Unselected": map[string]any{"{$request.query.callback}": map[string]any{"post": map[string]any{"responses": map[string]any{"204": map[string]any{"description": "OK"}}}}}}
	document.Raw["x-non-json"] = func() {}
	selection := &generator.Selection{Operations: []string{"a"}}
	registry, err := generator.NewAddonRegistry(generator.AddonServer, generator.AddonMetadata)
	if err != nil {
		t.Fatal(err)
	}
	for _, server := range []bool{false, true} {
		var addons []string
		if server {
			addons = append(addons, "server")
		}
		options, err := registry.Resolve(addons)
		if err != nil {
			t.Fatal(err)
		}
		options.Selection = selection
		if _, err := (Generator{}).Generate(document, options); err != nil {
			t.Fatalf("default selected/server=%v serialized source: %v", server, err)
		}
		options, err = registry.Resolve(append(addons, "metadata"))
		if err != nil {
			t.Fatal(err)
		}
		options.Selection = selection
		if _, err := (Generator{}).Generate(document, options); err == nil {
			t.Fatal("source add-on accepted non-JSON fallback")
		}
	}
	delete(document.Raw, "x-non-json")
	want, err := json.Marshal(document.Raw)
	if err != nil {
		t.Fatal(err)
	}
	options, err := registry.Resolve([]string{"server", "metadata"})
	if err != nil {
		t.Fatal(err)
	}
	options.Selection = selection
	artifacts, err := (Generator{}).Generate(document, options)
	if err != nil {
		t.Fatal(err)
	}
	var actual, original any
	if err := json.Unmarshal([]byte(metadataJSON(t, artifactByPath(t, artifacts, "metadata.ts"))), &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &original); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, original) || len(document.SourceMetadataJSON) != 0 {
		t.Fatal("selected fallback changed the original document")
	}
}
