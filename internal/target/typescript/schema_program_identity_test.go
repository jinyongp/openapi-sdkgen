package typescript

import (
	"bytes"
	"strings"
	"testing"

	schemaemit "openapi-sdkgen/internal/target/typescript/schema/emit"
	schemaplan "openapi-sdkgen/internal/target/typescript/schema/plan"
)

func TestSchemaProgramsShareDispatchAndPreserveReferenceTargets(t *testing.T) {
	runtime := newSchemaRuntimePlan()
	for _, target := range []string{"Number", "String"} {
		value := map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"$ref": "#/components/schemas/" + target}}}
		if _, err := runtime.lower(value, projectionOutput, false, false, false, nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(runtime.modules) != 2 {
		t.Fatal("identical dispatch algorithms were duplicated")
	}
	if err := runtime.freeze(); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"Number", "String"} {
		wire := newWireRenderContext(wirePropertiesConstructed)
		wire.schemaPrograms = runtime
		value := map[string]any{"type": "object", "properties": map[string]any{"value": map[string]any{"$ref": "#/components/schemas/" + target}}}
		descriptor, err := renderOwnedSchemaForTest(t, wire, func() (string, error) { return wire.wireSchemaDescriptor(value, projectionOutput) })
		if err != nil {
			t.Fatal(err)
		}
		descriptor = sharedDescriptorSource(wire, runtime, descriptor)
		if !strings.Contains(descriptor, "reference: "+quoteTS(target)) {
			t.Fatalf("shared algorithm changed the %s contract: %s", target, descriptor)
		}
	}
}

func TestSchemaProgramsShareAlgorithmsAcrossDifferentContractData(t *testing.T) {
	runtime := newSchemaRuntimePlan()
	var roots []*schemaplan.Node
	for index, name := range []string{"first", "second"} {
		value := map[string]any{"type": "object", "required": []any{name}, "properties": map[string]any{name: map[string]any{"type": "integer", "minimum": index + 1}}, "additionalProperties": false}
		root, err := runtime.lower(value, projectionOutput, false, false, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		roots = append(roots, root)
	}
	if len(runtime.modules) != 2 {
		t.Fatalf("different property names and bounds duplicated algorithms: %d", len(runtime.modules))
	}
	if runtime.owners[roots[0]].schemaRuntimeModule != runtime.owners[roots[1]].schemaRuntimeModule {
		t.Fatal("equivalent object algorithms have different owners")
	}
	if runtime.descriptorKeys[roots[0]] == runtime.descriptorKeys[roots[1]] {
		t.Fatal("different required names or bounds were erased from descriptor identity")
	}
	if err := runtime.freeze(); err != nil {
		t.Fatal(err)
	}
	for index, root := range roots {
		wire := newWireRenderContext(wirePropertiesLiteral)
		wire.schemaPrograms = runtime
		descriptor, err := renderOwnedSchemaForTest(t, wire, func() (string, error) { return wire.emitSchemaDescriptor(root) })
		if err != nil {
			t.Fatal(err)
		}
		descriptor = sharedDescriptorSource(wire, runtime, descriptor)
		name := []string{"first", "second"}[index]
		if !strings.Contains(descriptor, "required: ["+quoteTS(name)+"]") || !strings.Contains(descriptor, "minimum: "+[]string{"1", "2"}[index]) {
			t.Fatalf("contract-specific data changed: %s", descriptor)
		}
	}
}

func TestSchemaProgramsShareMediaViewsAndFreezeDeterministically(t *testing.T) {
	generate := func(reverse bool) map[string][]byte {
		runtime := newSchemaRuntimePlan()
		runtime.views = true
		values := []any{map[string]any{"$ref": "#/components/schemas/First", "type": "string"}, map[string]any{"$ref": "#/components/schemas/Second", "type": "string"}}
		if reverse {
			values[0], values[1] = values[1], values[0]
		}
		for _, value := range values {
			root, err := runtime.lower(value, projectionOutput, false, false, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(schemaemit.ProgramViews(root)) != 3 {
				t.Fatal("fixture must exercise all media views")
			}
		}
		if err := runtime.freeze(); err != nil {
			t.Fatal(err)
		}
		result := make(map[string][]byte)
		for _, module := range runtime.modules {
			result[module.path] = module.source
		}
		for _, value := range values {
			wire := newWireRenderContext(wirePropertiesLiteral)
			wire.schemaPrograms = runtime
			if _, err := renderOwnedSchemaForTest(t, wire, func() (string, error) { return wire.wireSchemaDescriptor(value, projectionOutput) }); err != nil {
				t.Fatal(err)
			}
		}
		if len(result) != len(runtime.modules) {
			t.Fatal("emission created an unprepared module")
		}
		return result
	}
	before, after := generate(false), generate(true)
	programs := 0
	for path := range before {
		if strings.HasPrefix(path, "internal/schema-programs/") {
			programs++
		}
	}
	if programs != 1 || len(after) != len(before) {
		t.Fatalf("equivalent media views duplicated algorithms: %d / %d", len(before), len(after))
	}
	for path, source := range before {
		if !bytes.Equal(source, after[path]) {
			t.Fatalf("module depends on preparation order: %s", path)
		}
	}
}

func sharedDescriptorSource(wire *wireRenderContext, runtime *schemaRuntimePlan, expression string) string {
	for _, dependency := range wire.programImports {
		if dependency.name != "schema" {
			continue
		}
		expected, err := wire.schemaImportName(dependency)
		if err != nil || expected != expression {
			continue
		}
		for _, module := range runtime.modules {
			if module.path == dependency.path {
				return string(module.source)
			}
		}
	}
	return expression
}

func renderOwnedSchemaForTest(t *testing.T, wire *wireRenderContext, render func() (string, error)) (string, error) {
	t.Helper()
	wire.names, wire.collectOnly = newLocalIdentifierPlan("test-owner.ts"), true
	if _, err := render(); err != nil {
		return "", err
	}
	if err := wire.names.freeze(); err != nil {
		return "", err
	}
	wire.collectOnly = false
	return render()
}
