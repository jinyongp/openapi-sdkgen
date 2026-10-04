package typescript

import (
	"fmt"
	"reflect"
	"testing"

	"openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
)

func executionPlanFixture(t *testing.T, operation, components string, stream bool) operationExecutionPlan {
	t.Helper()
	if components == "" {
		components = "{}"
	}
	document, err := sdkgen.Compile([]byte(fmt.Sprintf(`{"openapi":"3.1.1","info":{"title":"Execution plan","version":"1"},"paths":{"/items":{"post":%s}},"components":{"schemas":%s}}`, operation, components)))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := buildManifest(document)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Operations) != 1 {
		t.Fatalf("operations = %d", len(manifest.Operations))
	}
	item := manifest.Operations[0]
	result, err := newExecutionPlanner(document).operation(item.compiled, item, stream)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestExecutionPlanSelectsProvenServiceCompositions(t *testing.T) {
	for _, test := range []struct {
		name, operation string
		stream          bool
		want            executionProfile
	}{
		{"json", `{"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object"}}}}}}`, false, executionJSON},
		{"json-body", `{"requestBody":{"content":{"application/json":{"schema":{"type":"object"}}}},"responses":{"204":{"description":"ok"}}}`, false, executionJSON},
		{"json-case", `{"requestBody":{"content":{"Application/JSON":{"schema":{"type":"object"}}}},"responses":{"204":{"description":"ok"}}}`, false, executionJSON},
		{"xml-response", `{"responses":{"200":{"description":"ok","content":{"application/xml":{"schema":{"type":"object","xml":{"name":"item"}}}}}}}`, false, executionBufferedXML},
		{"xml-error-response", `{"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{}}}},"400":{"description":"bad","content":{"application/xml":{"schema":{"type":"object","xml":{"name":"error"}}}}}}}`, false, executionBufferedXML},
		{"xml-body", `{"requestBody":{"content":{"application/xml":{"schema":{"type":"object","xml":{"name":"item"}}}}},"responses":{"204":{"description":"ok"}}}`, false, executionBufferedXML},
		{"xml-header", `{"responses":{"200":{"description":"ok","headers":{"X-Item":{"content":{"application/xml":{"schema":{"type":"object","xml":{"name":"item"}}}}}},"content":{"application/json":{"schema":{}}}}}}`, false, executionBufferedXML},
		{"xml-parameter", `{"parameters":[{"in":"query","name":"q","content":{"application/xml":{"schema":{"type":"object","xml":{"name":"q"}}}}}],"responses":{"204":{"description":"ok"}}}`, false, executionBufferedXML},
		{"form", `{"requestBody":{"content":{"application/x-www-form-urlencoded":{"schema":{"type":"object","properties":{"name":{"type":"string"}}}}}},"responses":{"204":{"description":"ok"}}}`, false, executionGeneral},
		{"multipart", `{"requestBody":{"content":{"multipart/form-data":{"schema":{"type":"object","properties":{"name":{"type":"string"}}}}}},"responses":{"204":{"description":"ok"}}}`, false, executionGeneral},
		{"text-body", `{"requestBody":{"content":{"text/plain":{"schema":{"type":"string"}}}},"responses":{"204":{"description":"ok"}}}`, false, executionGeneral},
		{"wildcard-response", `{"responses":{"default":{"description":"ok","content":{"*/*":{"schema":{}}}}}}`, false, executionGeneral},
		{"stream-service", `{"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object"}}}}}}`, true, executionJSONStream},
		{"stream-xml-fallback", `{"responses":{"200":{"description":"ok","content":{"application/xml":{"schema":{"type":"object","xml":{"name":"item"}}}}}}}`, true, executionGeneral},
		{"empty", `{"responses":{"204":{"description":"ok"}}}`, false, executionJSON},
		{"custom-parameter", `{"parameters":[{"in":"query","name":"q","content":{"application/x-record":{"schema":{"type":"object"}}}}],"responses":{"204":{"description":"ok"}}}`, false, executionJSON},
		{"request-media-range", `{"requestBody":{"content":{"application/*":{"schema":{"type":"object"}}}},"responses":{"204":{"description":"ok"}}}`, false, executionGeneral},
		{"binary-response", `{"responses":{"200":{"description":"ok","content":{"application/octet-stream":{"schema":{"type":"string","format":"binary"}}}}}}`, false, executionJSON},
		{"buffered-ndjson", `{"responses":{"200":{"description":"ok","content":{"application/x-ndjson":{"schema":{"type":"array","items":{"type":"object"}}}}}}}`, false, executionJSONStream},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan := executionPlanFixture(t, test.operation, "", test.stream)
			if plan.profile != test.want {
				t.Fatalf("profile = %s, want %s; reasons %v", plan.profile, test.want, plan.reasons)
			}
			if plan.hasStream != test.stream {
				t.Fatal("stream callable contract changed")
			}
		})
	}
}

func TestExecutionPlanUsesSemanticSchemaSlotsAndDirection(t *testing.T) {
	components := `{"Input":{"type":"object","properties":{"onlyOutput":{"readOnly":true,"type":"string","contentMediaType":"application/xml","contentSchema":{"type":"object"}},"actual":{"$ref":"#/components/schemas/Shared"}}},"Shared":{"type":"string","enum":["contentMediaType","application/xml"]},"Unused":{"type":"string","contentMediaType":"application/xml","contentSchema":{"type":"object"}}}`
	operation := `{"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Input"}}}},"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Shared"}}}}}}`
	plan := executionPlanFixture(t, operation, components, false)
	if plan.profile != executionJSON {
		t.Fatalf("opaque data/unused/readOnly schema retained XML: %v", plan)
	}
	if !reflect.DeepEqual(plan.inputSchemas, []string{"Input", "Shared"}) || !reflect.DeepEqual(plan.outputSchemas, []string{"Shared"}) {
		t.Fatalf("projection closure = %#v", plan)
	}
	// The same component in the output projection must include its readOnly XML field.
	plan = executionPlanFixture(t, `{"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Input"}}}}}}`, components, false)
	if plan.profile != executionBufferedXML {
		t.Fatalf("output projection lost XML: %v", plan)
	}
}

func TestExecutionSchemaSummaryCyclePropagatesLateCapability(t *testing.T) {
	document := &ir.Document{ComponentSchemas: map[string]map[string]any{
		"A":   {"type": "object", "properties": map[string]any{"b": map[string]any{"$ref": "#/components/schemas/B"}, "xml": map[string]any{"$ref": "#/components/schemas/XML"}}},
		"B":   {"type": "object", "properties": map[string]any{"a": map[string]any{"$ref": "#/components/schemas/A"}}},
		"XML": {"type": "string", "contentMediaType": "application/xml", "contentSchema": map[string]any{"type": "object"}},
	}}
	for _, roots := range [][]string{{"A", "B", "A", "B"}, {"B", "A", "B", "A"}} {
		planner := newExecutionPlanner(document)
		for _, name := range roots {
			key := executionSchemaReference{name: name, direction: projectionOutput}
			caps, _, output, err := planner.schemaClosure(executionSchemaFacts{references: map[executionSchemaReference]bool{key: true}})
			if err != nil {
				t.Fatal(err)
			}
			if caps&executionSchemaXML == 0 || planner.schemas[key].capabilities&executionSchemaXML == 0 {
				t.Fatalf("%s lost XML after root order %v", name, roots)
			}
			if !reflect.DeepEqual(output, []string{"A", "B", "XML"}) {
				t.Fatal(output)
			}
		}
		if len(planner.schemas) != 3 {
			t.Fatal("schema analysis cache duplicated an existing projection")
		}
	}
}

func TestExecutionSchemaClosureRejectsMissingComponent(t *testing.T) {
	_, _, _, err := newExecutionPlanner(&ir.Document{}).schemaClosure(executionSchemaFacts{references: map[executionSchemaReference]bool{{name: "Missing", direction: projectionInput}: true}})
	if err == nil {
		t.Fatal("missing schema was silently treated as a general profile")
	}
}

func TestExecutionDynamicScopeIsConservativeAndKeepsFallback(t *testing.T) {
	document := &ir.Document{ComponentSchemas: map[string]map[string]any{"Base": {"type": "object"}}}
	wire := newWireRenderContext(wirePropertiesConstructed)
	facts := executionSchemaFacts{}
	wire.execution = &facts
	_, err := wire.wireSchemaDescriptor(map[string]any{"x-sdkgen-dynamic-reference": map[string]any{"anchor": "node", "reference": "#/components/schemas/Base"}}, projectionInput)
	if err != nil {
		t.Fatal(err)
	}
	caps, input, _, err := newExecutionPlanner(document).schemaClosure(facts)
	if err != nil {
		t.Fatal(err)
	}
	if caps&executionSchemaDynamic == 0 || !reflect.DeepEqual(input, []string{"Base"}) {
		t.Fatalf("dynamic facts = %v, %v", caps, input)
	}
}
