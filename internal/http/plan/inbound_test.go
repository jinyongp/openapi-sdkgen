package plan

import (
	"bytes"
	"encoding/json"
	"openapi-sdkgen/internal/compiler/ir"
	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"strings"
	"testing"
)

func TestInboundCollectionPreservesContractsAndOwners(t *testing.T) {
	response := map[string]any{"204": map[string]any{"description": "OK"}}
	callback := map[string]any{"{$request.body#/callbackURL}": map[string]any{"post": map[string]any{"responses": response}}}
	raw := map[string]any{
		"openapi": "3.2.0", "info": map[string]any{"title": "Inbound", "version": "1"},
		"security":   []any{map[string]any{"Key": []any{}}},
		"components": map[string]any{"callbacks": map[string]any{"shared": callback}, "securitySchemes": map[string]any{"Key": map[string]any{"type": "apiKey", "in": "header", "name": "X-Key"}}},
		"paths":      map[string]any{"/jobs": map[string]any{"post": map[string]any{"operationId": "createJob", "responses": response, "callbacks": map[string]any{"done": map[string]any{"$ref": "#/components/callbacks/shared"}}}}},
		"webhooks": map[string]any{"events": map[string]any{
			"parameters": []any{map[string]any{"name": "X-Trace", "in": "header", "schema": map[string]any{"type": "string"}}},
			"post":       map[string]any{"parameters": []any{map[string]any{"name": "old", "in": "query", "deprecated": true, "schema": map[string]any{"type": "boolean"}}}, "responses": response, "requestBody": map[string]any{"required": true, "content": map[string]any{"application/x-ndjson": map[string]any{"itemSchema": map[string]any{"type": "number"}}}}},
		}},
	}
	document, err := ir.Build(&openapidoc.Document{Raw: raw, Version: openapidoc.Version32})
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(raw)
	definitions, failures := CollectInbound(document, InboundScopes(document, nil, nil))
	if len(failures) != 0 || len(definitions) != 3 {
		t.Fatal(definitions, failures)
	}
	var webhook, sourceCallback bool
	for _, definition := range definitions {
		operation := definition.Operation
		if definition.Kind == "webhook" {
			webhook = true
			if operation.OperationID != "events" || operation.Pointer != "#/webhooks/events/post" || operation.Method != "POST" || len(operation.Parameters) != 2 || !operation.RequestBody.Required || len(operation.Security) != 1 || operation.Security[0].Schemes[0].Name != "Key" || operation.RequestBody.Content[0].Stream.Framing != ir.StreamFramingLineDelimitedJSON {
				t.Fatal("inbound IR contract", operation)
			}
			for _, parameter := range operation.Parameters {
				if parameter.Name == "X-Trace" && !strings.HasPrefix(parameter.Pointer, "#/webhooks/events/parameters/") {
					t.Fatal("path-item owner lost", parameter.Pointer)
				}
				if parameter.Name == "old" && !parameter.Deprecated {
					t.Fatal("parameter metadata lost")
				}
			}
		}
		if definition.SourceRoute != "" {
			sourceCallback = true
			if definition.SourceRoute != "POST /jobs" || definition.SourceOperationID != "createJob" || definition.Expression != "{$request.body#/callbackURL}" || definition.CallbackName != "done" {
				t.Fatal("callback identity lost", definition)
			}
		}
	}
	if !webhook || !sourceCallback {
		t.Fatal("missing inbound group")
	}
	for _, scopes := range [][]InboundScope{InboundScopes(document, nil, map[string]bool{}), InboundScopes(document, map[string]bool{"POST /jobs": true}, nil)} {
		definitions, failures := CollectInbound(document, scopes)
		if len(failures) != 0 || len(definitions) != 2 {
			t.Fatal("selection retained source callback", definitions, failures)
		}
	}
	after, _ := json.Marshal(raw)
	if !bytes.Equal(before, after) {
		t.Fatal("collection mutated source")
	}
}
