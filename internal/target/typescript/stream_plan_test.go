package typescript

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler/ir"
)

func TestOperationResponseMediaSetsPartitionNormalAndStreaming(t *testing.T) {
	operation := ir.Operation{
		Method:      "GET",
		Path:        "/events",
		OperationID: "watchEvents",
		Raw: map[string]any{"responses": map[string]any{
			"200": map[string]any{"description": "OK", "content": map[string]any{
				"application/json":     map[string]any{"schema": map[string]any{"type": "string"}},
				"text/plain":           map[string]any{"schema": map[string]any{"type": "string"}},
				"application/x-ndjson": map[string]any{"itemSchema": map[string]any{"type": "string"}},
				"text/event-stream": map[string]any{
					"schema":     map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
					"itemSchema": map[string]any{"type": "object"},
				},
			}},
		}},
	}
	document := &ir.Document{}
	sets, err := operationResponseMediaSets(document, operation)
	if err != nil {
		t.Fatal(err)
	}
	if got := responseMediaTypes(sets.normal); !reflect.DeepEqual(got, []string{"application/json", "text/event-stream", "text/plain"}) {
		t.Fatalf("normal media = %#v", got)
	}
	if got := responseMediaTypes(sets.streaming); !reflect.DeepEqual(got, []string{"application/x-ndjson", "text/event-stream"}) {
		t.Fatalf("stream media = %#v", got)
	}
	output, err := operationOutputType(document, operation)
	if err != nil {
		t.Fatal(err)
	}
	if output != "string | readonly (Readonly<Record<string, unknown>>)[]" {
		t.Fatalf("normal output = %q, want dual-mode complete output", output)
	}
	media, err := operationMediaOutputTypes(document, operation)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(media, map[string]string{
		"application/json":  "string",
		"text/event-stream": "readonly (Readonly<Record<string, unknown>>)[]",
		"text/plain":        "string",
	}) {
		t.Fatalf("normal media outputs = %#v", media)
	}
}

func TestNormalAndStreamOptionsUseSeparateAcceptMedia(t *testing.T) {
	operation := ir.Operation{Method: "GET", Path: "/events", OperationID: "watchEvents"}
	plan := ManifestOperation{
		mediaTypes:       []string{"application/json", "text/plain"},
		streamMediaTypes: []string{"application/x-ndjson", "text/event-stream"},
	}
	var normal bytes.Buffer
	if err := emitOperationOptions(&normal, "WatchEvents", operation, plan); err != nil {
		t.Fatal(err)
	}
	normalType := normal.String()
	for _, want := range []string{`"application/json"`, `"text/plain"`} {
		if !strings.Contains(normalType, want) {
			t.Fatalf("normal options = %q, missing %s", normalType, want)
		}
	}
	for _, excluded := range []string{`"application/x-ndjson"`, `"text/event-stream"`} {
		if strings.Contains(normalType, excluded) {
			t.Fatalf("normal options = %q, contains stream media %s", normalType, excluded)
		}
	}

	stream := generatedStream{Operation: operation, ItemType: "string", Plan: plan}
	streamType, err := streamFunctionType(&ir.Document{}, stream)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"application/x-ndjson"`, `"text/event-stream"`} {
		if !strings.Contains(streamType, want) {
			t.Fatalf("stream type = %q, missing %s", streamType, want)
		}
	}
	for _, excluded := range []string{`"application/json"`, `"text/plain"`} {
		if strings.Contains(streamType, excluded) {
			t.Fatalf("stream type = %q, contains normal media %s", streamType, excluded)
		}
	}
}
