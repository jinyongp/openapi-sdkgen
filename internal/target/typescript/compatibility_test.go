package typescript

import (
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
)

func TestCompatibilityIgnoredConsumerSemanticsDoNotReachTypeScriptContracts(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Compatibility target","version":"1"},
  "paths":{"/items":{"post":{
    "operationId":"createItem",
    "parameters":[
      {"name":"Accept","in":"header","schema":{"type":"string"}},
      {"name":"content-type","in":"header","schema":{"type":"string"}},
      {"name":"AUTHORIZATION","in":"header","schema":{"type":"string"}},
      {"name":"X-Trace","in":"header","required":true,"schema":{"type":"string"}}
    ],
    "requestBody":{"content":{"application/json":{
      "schema":{"type":"object","properties":{"name":{"type":"string"}}},
      "encoding":{"name":{"style":"form","headers":{"X-Ignored":{"schema":{"type":"string"}}}}}
    }}},
    "responses":{"200":{
      "description":"OK",
      "headers":{
        "Content-Type":{"schema":{"type":"string"}},
        "X-Rate":{"schema":{"type":"integer"}}
      },
      "content":{"application/json":{"schema":{"type":"string"}}}
    }}
  }}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := sourceArtifactsWithMetadata(document)
	if err != nil {
		t.Fatal(err)
	}
	client := clientSemanticSource(artifacts)
	for _, unexpected := range []string{`name: "Accept"`, `name: "content-type"`, `name: "AUTHORIZATION"`, `readonly "Content-Type"`} {
		if strings.Contains(client, unexpected) {
			t.Fatalf("ignored consumer semantic %q leaked into generated client:\n%s", unexpected, client)
		}
	}
	for _, expected := range []string{`name: "X-Trace"`, `readonly "X-Rate"?: number`} {
		if !strings.Contains(client, expected) {
			t.Fatalf("generated client missing control semantic %q:\n%s", expected, client)
		}
	}
	metadata := metadataJSON(t, artifactByPath(t, artifacts, "metadata.ts"))
	for _, expected := range []string{`"Accept"`, `"content-type"`, `"AUTHORIZATION"`, `"Content-Type"`, `"encoding"`} {
		if !strings.Contains(metadata, expected) {
			t.Fatalf("source metadata lost %q:\n%s", expected, metadata)
		}
	}
	compileTypeScriptArtifacts(t, document)
}
