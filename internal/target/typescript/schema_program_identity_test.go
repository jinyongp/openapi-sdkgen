package typescript

import (
	"strings"
	"testing"
)

func TestSchemaProgramsShareDispatchAndPreserveReferenceTargets(t *testing.T) {
	runtime := newSchemaRuntimePlan()
	for _, target := range []string{"Number", "String"} {
		value := map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"$ref": "#/components/schemas/" + target}}}
		if _, err := runtime.lower(value, projectionOutput, false, false, false, nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(runtime.modules) != 1 {
		t.Fatal("identical dispatch algorithms were duplicated")
	}
	if err := runtime.freeze(); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"Number", "String"} {
		wire := newWireRenderContext(wirePropertiesConstructed)
		wire.schemaPrograms = runtime
		value := map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"$ref": "#/components/schemas/" + target}}}
		descriptor, err := wire.wireSchemaDescriptor(value, projectionOutput)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(descriptor, "reference: "+quoteTS(target)) {
			t.Fatalf("shared algorithm changed the %s contract: %s", target, descriptor)
		}
	}
}
