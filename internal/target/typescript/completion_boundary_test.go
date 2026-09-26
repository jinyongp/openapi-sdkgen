package typescript

import (
	"bytes"
	"errors"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/generator"
)

func TestPrepareBlocksClientOnlyTargetWithNoMeaningfulEntrySurface(t *testing.T) {
	for _, input := range []string{
		`{
  "openapi":"3.1.1",
  "info":{"title":"All omitted","version":"1"},
  "paths":{
    "/items":{"get":{
      "operationId":"getItems",
      "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"string"}}}},
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`,
		`{
  "openapi":"3.1.1",
  "info":{"title":"Empty client","version":"1"},
  "paths":{}
}`,
	} {
		document, err := sdkgen.Compile([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		prepared, values, err := (Generator{}).Prepare(document, generator.Options{})
		if err != nil {
			t.Fatal(err)
		}
		var boundary *diagnostic.Diagnostic
		for index := range values {
			if values[index].Code == "SDKGEN-E512" {
				boundary = &values[index]
				break
			}
		}
		if boundary == nil || boundary.Severity != diagnostic.SeverityError ||
			boundary.Scope != failure.ScopeDocument || boundary.Effect != failure.EffectBlock {
			t.Fatalf("meaningful-entry diagnostic = %#v", values)
		}
		value, err := prepared.Value("typescript")
		if err != nil {
			t.Fatal(err)
		}
		if plan := value.(*sourcePlan); plan.modules != nil {
			t.Fatal("zero-entry target unexpectedly produced an emission module plan")
		}
	}
}

func TestPrepareAllowsIndependentServerWebhookWithNoClientCallables(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Server-only entry","version":"1"},
  "paths":{
    "/items":{"get":{
      "operationId":"getItems",
      "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"string"}}}},
      "responses":{"204":{"description":"OK"}}
    }}
  },
  "webhooks":{
    "event":{"post":{
      "operationId":"receiveEvent",
      "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object"}}}},
      "responses":{"204":{"description":"Accepted"}}
    }}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	options := serverOnlyBoundaryOptions(t)
	prepared, values, err := (Generator{}).Prepare(document, options)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.HasErrors(values) {
		t.Fatalf("server-only preparation diagnostics = %#v", values)
	}
	if diagnosticsContainCode(values, "SDKGEN-E512") {
		t.Fatalf("independent server webhook was rejected: %#v", values)
	}
	artifacts, err := (Generator{}).Emit(prepared)
	if err != nil {
		t.Fatal(err)
	}
	foundWebhook := false
	for _, artifact := range artifacts {
		if artifact.Path == "server/webhooks.ts" && bytes.Contains(artifact.Data, []byte("receiveEvent")) {
			foundWebhook = true
			break
		}
	}
	if !foundWebhook {
		t.Fatal("server-only webhook entry surface was not emitted")
	}
}

func TestPreparedOmissionPlanSurvivesEmitFailureWithoutSemanticMutation(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Immutable plan","version":"1"},
  "paths":{
    "/skip":{"get":{
      "operationId":"skipBody",
      "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"string"}}}},
      "responses":{"204":{"description":"OK"}}
    }},
    "/ok":{"post":{"operationId":"createItem","responses":{"204":{"description":"OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	prepared, values, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.HasErrors(values) {
		t.Fatalf("prepare diagnostics = %#v", values)
	}
	value, err := prepared.Value("typescript")
	if err != nil {
		t.Fatal(err)
	}
	plan := value.(*sourcePlan)
	omittedBefore := cloneBoolMap(plan.omittedOperations)
	routesBefore := manifestRouteKeys(*plan.manifest)
	modulesBefore := cloneStringMap(plan.modules.operationByRoute)
	reachableBefore := cloneBoolMap(plan.resourceReachable)
	metadataBefore := append([]byte(nil), plan.document.SourceMetadataJSON...)

	injected := errors.New("injected artifact sink failure")
	writes := 0
	err = (Generator{}).EmitTo(prepared, generator.ArtifactSinkFunc(func(generator.Artifact) error {
		writes++
		if writes == 3 {
			return injected
		}
		return nil
	}))
	if !errors.Is(err, injected) {
		t.Fatalf("emit failure = %v", err)
	}
	if !equalBoolMap(plan.omittedOperations, omittedBefore) ||
		!equalStrings(manifestRouteKeys(*plan.manifest), routesBefore) ||
		!equalStringMap(plan.modules.operationByRoute, modulesBefore) ||
		!equalBoolMap(plan.resourceReachable, reachableBefore) ||
		!bytes.Equal(plan.document.SourceMetadataJSON, metadataBefore) {
		t.Fatal("emit failure mutated prepared semantic decisions")
	}

	first, err := (Generator{}).Emit(prepared)
	if err != nil {
		t.Fatal(err)
	}
	second, err := (Generator{}).Emit(prepared)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != len(second) {
		t.Fatalf("repeat emit artifact count = %d, %d", len(first), len(second))
	}
	for index := range first {
		if first[index].Path != second[index].Path || !bytes.Equal(first[index].Data, second[index].Data) {
			t.Fatalf("repeat emit changed artifact %d: %q / %q", index, first[index].Path, second[index].Path)
		}
	}
}

func TestPreparedPlanDoesNotObserveNestedSourceMutation(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Immutable nested plan","version":"1"},
  "paths":{
    "/thing":{"get":{
      "operationId":"getThing",
      "responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Thing"}}}}}
    }}
  },
  "components":{"schemas":{"Thing":{"type":"object","properties":{"value":{"type":"string"}}}}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	prepared, values, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.HasErrors(values) {
		t.Fatalf("prepare diagnostics = %#v", values)
	}
	before, err := (Generator{}).Emit(prepared)
	if err != nil {
		t.Fatal(err)
	}

	document.ComponentSchemas["Thing"]["properties"].(map[string]any)["value"] = map[string]any{"type": "integer"}
	components := document.Raw["components"].(map[string]any)
	schemas := components["schemas"].(map[string]any)
	schemas["Thing"].(map[string]any)["properties"].(map[string]any)["value"] = map[string]any{"type": "boolean"}

	after, err := (Generator{}).Emit(prepared)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatalf("artifact count changed after source mutation: %d -> %d", len(before), len(after))
	}
	for index := range before {
		if before[index].Path != after[index].Path || !bytes.Equal(before[index].Data, after[index].Data) {
			t.Fatalf("artifact %d changed after source mutation: %q -> %q", index, before[index].Path, after[index].Path)
		}
	}
}

func serverOnlyBoundaryOptions(t *testing.T) generator.Options {
	t.Helper()
	registry, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	options, err := registry.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	return options
}

func manifestRouteKeys(manifest Manifest) []string {
	result := make([]string, 0, len(manifest.Operations))
	for _, operation := range manifest.Operations {
		result = append(result, manifestRouteKey(operation))
	}
	return result
}

func cloneBoolMap(values map[string]bool) map[string]bool {
	result := make(map[string]bool, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func cloneStringMap(values map[string]string) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func equalBoolMap(left, right map[string]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func equalStringMap(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
