package ir

import (
	"strings"
	"testing"

	openapidoc "openapi-sdkgen/internal/compiler/openapi"
)

func TestStreamPlanForMediaTypeNormalizesFraming(t *testing.T) {
	tests := []struct {
		name          string
		contentType   string
		hasItemSchema bool
		want          StreamFraming
	}{
		{name: "ndjson", contentType: "application/x-ndjson", want: StreamFramingLineDelimitedJSON},
		{name: "json lines alias", contentType: "APPLICATION/JSONL; charset=utf-8", want: StreamFramingLineDelimitedJSON},
		{name: "json sequence", contentType: "application/json-seq", want: StreamFramingJSONSequence},
		{name: "json sequence suffix", contentType: "application/geo+json-seq", want: StreamFramingJSONSequence},
		{name: "sse", contentType: "text/event-stream", want: StreamFramingSSE},
		{name: "multipart stream", contentType: "multipart/mixed", hasItemSchema: true, want: StreamFramingMultipart},
		{name: "custom stream", contentType: "application/vnd.example.frames", hasItemSchema: true, want: StreamFramingCustom},
		{name: "normal json", contentType: "application/json", want: StreamFramingNone},
		{name: "ndjson lookalike", contentType: "application/not-ndjson", want: StreamFramingNone},
		{name: "sse lookalike", contentType: "text/event-streaming", want: StreamFramingNone},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := StreamPlanForMediaType(test.contentType, test.hasItemSchema).Framing; got != test.want {
				t.Fatalf("framing = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBuildAttachesStreamPlansToRequestAndResponseMedia(t *testing.T) {
	document := &openapidoc.Document{Raw: map[string]any{
		"openapi": "3.2.0",
		"info":    map[string]any{"title": "Streams", "version": "1"},
		"paths": map[string]any{
			"/events": map[string]any{"post": map[string]any{
				"operationId": "events",
				"requestBody": map[string]any{"content": map[string]any{
					"application/vnd.example.frames": map[string]any{"itemSchema": map[string]any{"type": "string"}},
				}},
				"responses": map[string]any{"200": map[string]any{
					"description": "OK",
					"content": map[string]any{
						"application/json":     map[string]any{"schema": map[string]any{"type": "string"}},
						"application/x-ndjson": map[string]any{"itemSchema": map[string]any{"type": "string"}},
					},
				}},
			}},
		},
	}}
	model, err := Build(document)
	if err != nil {
		t.Fatal(err)
	}
	operation := model.Operations[0]
	if got := operation.RequestBody.Content[0].Stream.Framing; got != StreamFramingCustom {
		t.Fatalf("request framing = %q", got)
	}
	if got := operation.Responses[0].Content[0].Stream.Framing; got != StreamFramingNone {
		t.Fatalf("normal response framing = %q", got)
	}
	if got := operation.Responses[0].Content[1].Stream.Framing; got != StreamFramingLineDelimitedJSON {
		t.Fatalf("stream response framing = %q", got)
	}
}

func TestBuildRejectsBuiltInStreamWithoutItemSchema(t *testing.T) {
	document := &openapidoc.Document{Raw: map[string]any{
		"openapi": "3.2.0",
		"info":    map[string]any{"title": "Streams", "version": "1"},
		"paths": map[string]any{
			"/events": map[string]any{"get": map[string]any{
				"operationId": "events",
				"responses": map[string]any{"200": map[string]any{
					"description": "OK",
					"content": map[string]any{
						"text/event-stream": map[string]any{"schema": map[string]any{"type": "string"}},
					},
				}},
			}},
		},
	}}
	_, err := Build(document)
	if err == nil {
		t.Fatal("expected missing itemSchema error")
	}
	for _, want := range []string{"#/paths/~1events/get/responses/200/content/text~1event-stream", "requires itemSchema"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q", err, want)
		}
	}
}
