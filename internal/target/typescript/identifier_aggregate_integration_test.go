package typescript

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
)

func generatedEnumBindings(t testing.TB, source string) (record, values string) {
	t.Helper()
	matches := regexp.MustCompile(`(?m)^const ([A-Za-z_$][A-Za-z0-9_$]*) = /\* @__PURE__ \*/ __sdkgen_createEnumValues\(([A-Za-z_$][A-Za-z0-9_$]*)\)$`).FindAllStringSubmatch(source, -1)
	if len(matches) != 1 {
		t.Fatalf("expected one enum record constructor, got %d", len(matches))
	}
	return matches[0][1], matches[0][2]
}

func TestAggregateEmittersPreserveExactOwnershipAndLocality(t *testing.T) {
	source := aliasFixtureSource(t)
	document, err := compiler.Compile(source)
	if err != nil {
		t.Fatal(err)
	}
	base, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	registry := string(artifactByPath(t, base, "internal/client/registry.ts"))
	wire := string(artifactByPath(t, base, "internal/schemas/wire.ts"))
	if !strings.Contains(registry, "bindBase as __sdkgen_bf_h") || !strings.Contains(registry, "const __sdkgen_ov_h") {
		t.Fatal("aggregate route bindings were not exercised")
	}
	if !strings.Contains(wire, "WireSchema as __sdkgen_w") {
		t.Fatal("aggregate schema aliases were not exercised")
	}
	for _, text := range []string{registry, wire} {
		for _, oldRole := range []string{"_r626173652d666163746f7279_", "_r6f7065726174696f6e2d76616c7565_", "_r736368656d612d696e7075742d77697265_", "_r736368656d612d6f75747075742d77697265_"} {
			if strings.Contains(text, oldRole) {
				t.Fatalf("aggregate retained legacy role %s", oldRole)
			}
		}
	}
	// Imports identify declarations independently of compact spelling. Adding an
	// unrelated operation must not rename imports for existing owner modules.
	imports := func(source string) map[string]string {
		result := map[string]string{}
		for _, match := range regexp.MustCompile(`(?m)^import \{ (.+) \} from ("[^"]+")$`).FindAllStringSubmatch(source, -1) {
			result[match[2]] = match[1]
		}
		return result
	}
	var raw map[string]any
	if err := json.Unmarshal(source, &raw); err != nil {
		t.Fatal(err)
	}
	raw["paths"].(map[string]any)["/unrelated-aggregate"] = map[string]any{"get": map[string]any{"operationId": "unrelatedAggregate", "responses": map[string]any{"204": map[string]any{"description": "empty"}}}}
	changedSource, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	changedDoc, err := compiler.Compile(changedSource)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := SourceArtifacts(changedDoc)
	if err != nil {
		t.Fatal(err)
	}
	newImports := imports(string(artifactByPath(t, changed, "internal/client/registry.ts")))
	for specifier, names := range imports(registry) {
		if newImports[specifier] != names {
			t.Fatalf("unrelated operation renamed %s", specifier)
		}
	}
	if !bytes.Equal(artifactByPath(t, base, "internal/schemas/wire.ts"), artifactByPath(t, changed, "internal/schemas/wire.ts")) {
		t.Fatal("unrelated route changed schema wire registry")
	}
	withServer, err := sourceArtifacts(document, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range base {
		if !bytes.Equal(artifact.Data, artifactByPath(t, withServer, artifact.Path)) {
			t.Fatalf("server addon changed client artifact %s", artifact.Path)
		}
	}
}

func TestServerAliasPlansPreservePreparedDefinitionsAndExactFields(t *testing.T) {
	callbacks := []callbackDefinition{
		{name: "first", sourceRouteKey: "GET /source", callbackName: "a\x00b", expression: "c", method: "POST"},
		{name: "second", sourceRouteKey: "GET /source", callbackName: "a", expression: "b\x00c", method: "POST"},
	}
	if callbackIdentity(callbacks[0]) != callbackIdentity(callbacks[1]) {
		t.Fatal("fixture does not exercise legacy joined-key collision")
	}
	before := append([]callbackDefinition(nil), callbacks...)
	planned, err := planCallbackIdentifiers(callbacks)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(callbacks, before) {
		t.Fatal("emission mutated prepared callback definitions")
	}
	if planned[0].typeName == planned[1].typeName || planned[0].endpointSymbol == planned[1].endpointSymbol || planned[0].definitionSymbol == planned[1].definitionSymbol {
		t.Fatal("exact callback fields collapsed")
	}
	if _, err := planCallbackIdentifiers([]callbackDefinition{callbacks[0], callbacks[0]}); err == nil {
		t.Fatal("duplicate callback declaration accepted")
	}
	webhooks := []webhookDefinition{{name: "a\x00POST", method: "GET"}, {name: "a", method: "POST\x00GET"}}
	plannedWebhooks, err := planWebhookIdentifiers(webhooks)
	if err != nil {
		t.Fatal(err)
	}
	if plannedWebhooks[0].typeName == plannedWebhooks[1].typeName || plannedWebhooks[0].definitionSymbol == plannedWebhooks[1].definitionSymbol {
		t.Fatal("webhook field boundary collapsed")
	}
	if webhooks[0].typeName != "" || webhooks[0].definitionSymbol != "" {
		t.Fatal("emission mutated prepared webhook definitions")
	}
	if _, err := planWebhookIdentifiers([]webhookDefinition{webhooks[0], webhooks[0]}); err == nil {
		t.Fatal("duplicate webhook declaration accepted")
	}
}

func TestServerAndEnumAggregateBindingsUseSharedCollisionFixture(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "typescript", "fixtures", "collisions.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	document, err := compiler.Compile(source)
	if err != nil {
		t.Fatal(err)
	}
	callbacks, err := collectCallbacks(document)
	if err != nil {
		t.Fatal(err)
	}
	webhooks, err := collectWebhooks(document)
	if err != nil {
		t.Fatal(err)
	}
	for _, callback := range callbacks {
		if callback.typeName != "" || callback.definitionSymbol != "" || callback.endpointSymbol != "" {
			t.Fatal("collector assigned lexical callback spellings")
		}
	}
	for _, webhook := range webhooks {
		if webhook.typeName != "" || webhook.definitionSymbol != "" {
			t.Fatal("collector assigned lexical webhook spellings")
		}
	}
	artifacts, err := sourceArtifacts(document, true)
	if err != nil {
		t.Fatal(err)
	}
	for path, fragments := range map[string][]string{
		"server/callbacks.ts": {"interface __sdkgen_ct_h", "const __sdkgen_cd_h", "const __sdkgen_ce_h"},
		"server/webhooks.ts":  {"interface __sdkgen_wt_h", "const __sdkgen_wd_h", "async fetch(request: Request)"},
		"internal/enums.ts":   {"const __sdkgen_ev_h", "const __sdkgen_e_h", "export const Enums"},
	} {
		text := string(artifactByPath(t, artifacts, path))
		for _, fragment := range fragments {
			if !strings.Contains(text, fragment) {
				t.Fatalf("%s missing %s", path, fragment)
			}
		}
	}
	again, err := sourceArtifacts(document, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range artifacts {
		if !bytes.Equal(artifact.Data, artifactByPath(t, again, artifact.Path)) {
			t.Fatalf("repeat changed %s", artifact.Path)
		}
	}
}

func TestAggregateOwnerPlansRejectDuplicateDeclarations(t *testing.T) {
	manifest := Manifest{Operations: []ManifestOperation{{RouteKey: "GET /a"}, {RouteKey: "GET /a"}}}
	operations := map[string]ir.Operation{"GET /a": {Method: "GET", Path: "/a"}}
	if _, err := planRegistryIdentifiers("internal/client/registry.ts", manifest, operations, nil, nil); err == nil || !strings.Contains(err.Error(), "duplicate route") {
		t.Fatalf("duplicate registry declaration accepted: %v", err)
	}
	if err := planEnumIdentifiers([]enumValuesPlan{{name: "A"}, {name: "A"}}); err == nil {
		t.Fatal("duplicate enum declaration accepted")
	}
	if _, err := planSchemaWireIdentifiers(nil); err == nil {
		t.Fatal("nil schema module accepted")
	}
}
