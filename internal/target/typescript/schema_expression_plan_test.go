package typescript

import (
	"bytes"
	"strings"
	"testing"

	schemaplan "openapi-sdkgen/internal/target/typescript/schema/plan"
)

func TestSchemaDescriptorAliasesAreOwnedByEachFile(t *testing.T) {
	runtime := newSchemaRuntimePlan()
	var nodes []*schemaplan.Node
	for _, value := range []any{map[string]any{"type": "string"}, map[string]any{"type": "object", "properties": map[string]any{"__proto__": map[string]any{"type": "integer", "minimum": 1}}}} {
		node, err := runtime.lower(value, projectionInput, false, false, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, node)
	}
	if err := runtime.freeze(); err != nil {
		t.Fatal(err)
	}
	before := schemaProgramsSnapshot(t, runtime)
	render := func(owner string, reserved ...string) string {
		names := newLocalIdentifierPlan(owner)
		if err := names.reserve(reserved...); err != nil {
			t.Fatal(err)
		}
		collector := newWireRenderContext(wirePropertiesConstructed)
		collector.schemaPrograms, collector.names, collector.collectOnly = runtime, names, true
		for _, node := range nodes {
			if _, err := collector.emitSchemaDescriptor(node); err != nil {
				t.Fatal(err)
			}
		}
		if err := names.freeze(); err != nil {
			t.Fatal(err)
		}
		wire := newWireRenderContext(wirePropertiesConstructed)
		wire.schemaPrograms, wire.names = runtime, names
		var result string
		for _, node := range nodes {
			expression, err := wire.emitSchemaDescriptor(node)
			if err != nil {
				t.Fatal(err)
			}
			result += expression
		}
		imports, err := wire.programImportSource(owner)
		if err != nil {
			t.Fatal(err)
		}
		return imports + result
	}
	a := render("a.ts", "__sdkgen_d_d0", "__sdkgen_p_d0")
	b := render("b.ts")
	if !strings.Contains(a, "as __sdkgen_d_d1") || !strings.Contains(a, "as __sdkgen_p_d1") || !strings.Contains(b, "as __sdkgen_d_d0") || !strings.Contains(b, "as __sdkgen_p_d0") {
		t.Fatalf("owner reservations leaked: %s\n%s", a, b)
	}
	if b != render("b.ts") || a != render("a.ts", "__sdkgen_d_d0", "__sdkgen_p_d0") || !bytes.Equal(before, schemaProgramsSnapshot(t, runtime)) {
		t.Fatal("repeated/reversed owners mutated the prepared schema cache")
	}
}
