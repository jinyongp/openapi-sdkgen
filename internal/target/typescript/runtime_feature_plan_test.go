package typescript

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
)

func runtimeFeatureFixture(t *testing.T, operation, components, webhooks string, server bool) *sourcePlan {
	t.Helper()
	if components == "" {
		components = "{}"
	}
	if webhooks == "" {
		webhooks = "{}"
	}
	version := "3.1.1"
	if strings.Contains(operation+webhooks, `"itemSchema"`) {
		version = "3.2.0"
	}
	document, err := sdkgen.Compile([]byte(fmt.Sprintf(`{"openapi":"%s","info":{"title":"Features","version":"1"},"paths":{"/items":{"post":%s}},"components":%s,"webhooks":%s}`, version, operation, components, webhooks)))
	if err != nil {
		t.Fatal(err)
	}
	options := generator.Options{}
	if server {
		registry, err := generator.NewAddonRegistry(generator.AddonServer)
		if err != nil {
			t.Fatal(err)
		}
		options, err = registry.Resolve([]string{"server"})
		if err != nil {
			t.Fatal(err)
		}
	}
	prepared, diagnostics, err := (Generator{}).Prepare(document, options)
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v, %v", err, diagnostics)
	}
	value, err := prepared.Value("typescript")
	if err != nil {
		t.Fatal(err)
	}
	return value.(*sourcePlan)
}

func TestRuntimeFeaturePlanClosureRegression(t *testing.T) {
	const noBody = `{"responses":{"204":{"description":"ok"}}}`
	t.Run("media and framing", func(t *testing.T) {
		for _, test := range []struct {
			media, shape string
			want         []runtimeFeature
		}{
			{"application/json", `"schema":{"type":"object"}`, []runtimeFeature{"request.media.json", "schema.type.object"}},
			{"application/xml", `"schema":{"type":"object"}`, []runtimeFeature{"request.media.xml"}},
			{"application/x-www-form-urlencoded", `"schema":{"type":"object"}`, []runtimeFeature{"request.media.form"}},
			{"multipart/form-data", `"schema":{"type":"object"}`, []runtimeFeature{"request.media.multipart", "request.open-part-media"}},
			{"text/plain", `"schema":{"type":"string"}`, []runtimeFeature{"request.media.text"}},
			{"application/octet-stream", `"schema":{"type":"string","format":"binary"}`, []runtimeFeature{"request.media.binary"}},
			{"application/*", `"schema":{"type":"object"}`, []runtimeFeature{"request.media.open"}},
			{"application/x-ndjson", `"schema":{"type":"array","items":{"type":"object"}}`, []runtimeFeature{"request.framing.complete.line-delimited-json", "request.protocol-override"}},
			{"application/json-seq", `"itemSchema":{"type":"object"}`, []runtimeFeature{"request.framing.incremental.json-sequence", "request.protocol-override"}},
			{"application/vnd.frames", `"itemSchema":{"type":"object"}`, []runtimeFeature{"request.framing.incremental.custom", "request.protocol-override"}},
		} {
			t.Run(test.media, func(t *testing.T) {
				plan := runtimeFeatureFixture(t, fmt.Sprintf(`{"requestBody":{"content":{"%s":{%s}}},"responses":{"204":{"description":"ok"}}}`, test.media, test.shape), "", "", false)
				features := plan.runtimeFeatures.operations["POST /items"]
				for _, feature := range test.want {
					if !slices.Contains(features, feature) {
						t.Fatalf("missing %s: %v", feature, features)
					}
				}
				if test.media == "application/json" && (slices.Contains(features, "request.media.xml") || slices.Contains(features, "schema.dynamic")) {
					t.Fatal("JSON root acquired unrelated implementations")
				}
			})
		}
	})
	t.Run("projection and opaque data", func(t *testing.T) {
		components := `{"schemas":{"Input":{"type":"object","properties":{"hidden":{"readOnly":true,"type":"string","contentMediaType":"application/xml","contentSchema":{}},"value":{"enum":[{"format":"ipv6","contentMediaType":"application/xml"}]}}}}}`
		plan := runtimeFeatureFixture(t, `{"requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Input"}}}},"responses":{"204":{"description":"ok"}}}`, components, "", false)
		features := plan.runtimeFeatures.shared
		if slices.Contains(features, "schema.content.media.xml") || slices.Contains(features, "schema.format.ipv6") {
			t.Fatal("filtered/opaque value became schema dependency", features)
		}
		plan = runtimeFeatureFixture(t, `{"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Input"}}}}}}`, components, "", false)
		if !slices.Contains(plan.runtimeFeatures.shared, "schema.content.media.xml") {
			t.Fatal("output projection lost readOnly XML")
		}
	})
	t.Run("recursive summaries and format annotation", func(t *testing.T) {
		document := &ir.Document{ComponentSchemas: map[string]map[string]any{
			"A":    {"properties": map[string]any{"b": map[string]any{"$ref": "#/components/schemas/B"}, "leaf": map[string]any{"$ref": "#/components/schemas/Leaf"}}},
			"B":    {"properties": map[string]any{"a": map[string]any{"$ref": "#/components/schemas/A"}}},
			"Leaf": {"type": "string", "format": "ipv6", "$vocabulary": map[string]any{formatAssertionVocabulary: true}},
		}}
		planner := newExecutionPlanner(document)
		for _, root := range []string{"A", "B", "A"} {
			features, err := planner.runtimeSchemaClosure(executionSchemaFacts{references: map[executionSchemaReference]bool{{name: root, direction: projectionInput}: true}})
			if err != nil || !features["schema.format.ipv6"] {
				t.Fatalf("%s lost late feature through cycle: %v, %v", root, features, err)
			}
		}
		facts := executionSchemaFacts{}
		wire := newWireRenderContext(wirePropertiesConstructed)
		wire.execution = &facts
		if _, err := wire.wireSchemaDescriptor(map[string]any{"type": "string", "format": "ipv6"}, projectionInput); err != nil {
			t.Fatal(err)
		}
		if facts.features["schema.format.ipv6"] {
			t.Fatal("annotation format became assertion code")
		}
	})
	t.Run("inbound-only roots and compatibility", func(t *testing.T) {
		webhooks := `{"notice":{"post":{"requestBody":{"content":{"application/xml":{"schema":{"type":"object","properties":{"id":{"type":"string"}}}}}},"responses":{"204":{"description":"ok"}}}}}`
		plan := runtimeFeatureFixture(t, noBody, "", webhooks, true)
		if !plan.runtimeFeatures.genericServer || len(plan.runtimeFeatures.inbound) != 1 {
			t.Fatal("missing inbound/public compatibility roots")
		}
		if !slices.Contains(plan.runtimeFeatures.shared, "inbound.request.media.xml") || slices.Contains(plan.runtimeFeatures.operations["POST /items"], "inbound.request.media.xml") {
			t.Fatal("inbound feature missing or leaked to client")
		}
		if !reflect.DeepEqual(plan.runtimeFeatures.reasons["inbound.request.media.xml"], []string{"webhook notice"}) {
			t.Fatal("feature origin not preserved", plan.runtimeFeatures.reasons)
		}
	})
	t.Run("headers, positional parts and dynamic fallback", func(t *testing.T) {
		facts := executionSchemaFacts{}
		wire := newWireRenderContext(wirePropertiesConstructed)
		wire.execution = &facts
		_, err := wire.positionalMultipartWireEncodings(nil, []any{map[string]any{"contentType": "application/xml", "headers": map[string]any{"X-Value": map[string]any{"content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "integer"}}}}}}})
		if err != nil || !facts.features["part.media.xml"] || !facts.features["part.header.media.json"] {
			t.Fatalf("part/header dependencies: %v, %v", facts.features, err)
		}
		_, err = wire.wireSchemaDescriptor(map[string]any{"x-sdkgen-dynamic-reference": map[string]any{"anchor": "node", "reference": "#/components/schemas/Fallback"}}, projectionInput)
		if err != nil {
			t.Fatal(err)
		}
		planner := newExecutionPlanner(&ir.Document{ComponentSchemas: map[string]map[string]any{"Fallback": {"type": "number", "multipleOf": 2}}})
		features, err := planner.runtimeSchemaClosure(facts)
		if err != nil || !features["schema.dynamic"] || !features["schema.multipleOf"] {
			t.Fatalf("dynamic fallback features: %v, %v", features, err)
		}
	})
	t.Run("named roots, Link closure and deterministic emission", func(t *testing.T) {
		document := registrySelectionDocument(t)
		options := generator.Options{Clients: map[string]generator.Client{
			"linked": {Selection: &generator.Selection{Operations: []string{"a"}}},
			"plain":  {Selection: &generator.Selection{Operations: []string{"unused"}}},
		}}
		prepared, _, err := (Generator{}).Prepare(document, options)
		if err != nil {
			t.Fatal(err)
		}
		value, _ := prepared.Value("typescript")
		plan := value.(*sourcePlan)
		for _, client := range plan.clients {
			features := client.view.runtimeFeatures
			if client.name == "linked" && (len(features.operations) != 3 || !slices.Contains(features.shared, "callable.links")) {
				t.Fatal("private Link dependency lost")
			}
			if client.name == "plain" && (len(features.operations) != 1 || slices.Contains(features.shared, "callable.links")) {
				t.Fatal("other client's Link features leaked into plain root")
			}
		}
		a, err := (Generator{}).Emit(prepared)
		if err != nil {
			t.Fatal(err)
		}
		b, err := (Generator{}).Emit(prepared)
		if err != nil || !reflect.DeepEqual(a, b) {
			t.Fatal("frozen plan emission is not repeatable", err)
		}
	})
}
