package golang

import (
	"bytes"
	"encoding/json"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"testing"
)

func TestPreparedInboundNamesAndStreamingMode(t *testing.T) {
	document, err := compiler.Compile([]byte(inboundScopeFixture))
	if err != nil {
		t.Fatal(err)
	}
	webhooks := document.Raw["webhooks"].(map[string]any)
	post := webhooks["events"].(map[string]any)["post"].(map[string]any)
	post["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/x-ndjson": map[string]any{"schema": map[string]any{"type": "array", "minItems": 3}, "itemSchema": map[string]any{"type": "number"}}}}
	before, _ := json.Marshal(document.Raw)
	prepare := func(selection *generationSelection) []inboundOperationPlan {
		names, err := planNames(document)
		if err != nil {
			t.Fatal(err)
		}
		schemas := new(schemaPlanner)
		plans, diagnostics := prepareInboundOperations(document, inboundScopes(document, selection, nil), names, schemas)
		if diagnostic.HasErrors(diagnostics) {
			t.Fatal(diagnostics)
		}
		return plans
	}
	all := prepare(nil)
	selected := prepare(&generationSelection{Direct: map[string]bool{"GET /health": true}})
	if len(all) != 2 || len(selected) != 1 {
		t.Fatal("inbound selection", len(all), len(selected))
	}
	webhook := selected[0]
	if webhook.Definition.Kind != "webhook" || !webhook.BodyDeclared || !webhook.BodyRequired || len(webhook.Operation.InputFields) != 1 || !webhook.Operation.InputFields[0].Incremental || webhook.Operation.BodyUnion || webhook.Operation.InputFields[0].Stream.ItemValidation == "" {
		t.Fatal("inbound itemSchema must select the source representation", webhook)
	}
	for _, entry := range all {
		if entry.Definition.Identity == webhook.Definition.Identity && (entry.PublicName != webhook.PublicName || entry.Operation.Name != webhook.Operation.Name) {
			t.Fatal("selection changed inbound identity")
		}
	}
	paths := document.Raw["paths"].(map[string]any)
	paths["/jobs"].(map[string]any)["post"].(map[string]any)["callbacks"].(map[string]any)["broken"] = "not a callback"
	if plans := prepare(&generationSelection{Direct: map[string]bool{"GET /health": true}}); len(plans) != 1 || plans[0].PublicName != webhook.PublicName || plans[0].Operation.Name != webhook.Operation.Name {
		t.Fatal("unselected malformed callback affected a valid entry")
	}
	delete(paths["/jobs"].(map[string]any)["post"].(map[string]any)["callbacks"].(map[string]any), "broken")
	after, _ := json.Marshal(document.Raw)
	if !bytes.Equal(before, after) {
		t.Fatal("inbound preparation mutated original schemas")
	}
	if cloned := inboundInputOperation(ir.Operation{}); cloned.RequestBody != nil {
		t.Fatal("absent body changed")
	}
}
