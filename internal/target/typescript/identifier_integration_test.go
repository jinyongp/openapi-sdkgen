package typescript

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
)

func aliasFixtureSource(t testing.TB) []byte {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "typescript", "fixtures", "aliases.openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func TestLocalAliasesUseSharedFixtureAndRemainArtifactLocal(t *testing.T) {
	source := aliasFixtureSource(t)
	document, err := compiler.Compile(source)
	if err != nil {
		t.Fatal(err)
	}
	base, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := SourceArtifacts(document)
	if err != nil {
		t.Fatal(err)
	}
	for i, artifact := range base {
		if artifact.Path != repeated[i].Path || !bytes.Equal(artifact.Data, repeated[i].Data) {
			t.Fatalf("repeat changed %s", artifact.Path)
		}
	}
	envelope := string(artifactByPath(t, base, "internal/schemas/envelope.ts"))
	if !strings.Contains(envelope, " as __sdkgen_t_d") || !strings.Contains(envelope, "export type Input") || !strings.Contains(envelope, "export type Output") {
		t.Fatalf("fixture did not exercise schema aliases: %s", envelope)
	}
	for _, fragment := range []string{`readonly "__sdkgen_t_d0"`, `"__sdkgen_r_d0"`, `"__proto__"`} {
		if !strings.Contains(envelope, fragment) {
			t.Fatalf("opaque/authored key changed: %s", fragment)
		}
	}
	for _, artifact := range base {
		if strings.HasPrefix(artifact.Path, "internal/operations/") || strings.HasPrefix(artifact.Path, "internal/schemas/") || strings.HasPrefix(artifact.Path, "internal/resources/") {
			for _, oldRole := range []string{"_r736368656d612d74797065_", "_r747970652d696d706f7274_", "_r7265736f757263652d6275696c646572_"} {
				if bytes.Contains(artifact.Data, []byte(oldRole)) {
					t.Fatalf("%s retained legacy local role %s", artifact.Path, oldRole)
				}
			}
		}
	}
	linked := operationArtifactSource(t, base, "GET /linked-events")
	linkedAlias := generatedTypeImportAlias(t, linked, "../../schemas/event-value.js", "Output")
	if strings.Count(linked, linkedAlias) < 3 || !strings.Contains(linked, "const __sdkgen_l_d") ||
		!strings.Contains(linked, "export function bindLinks(") || !strings.Contains(linked, "export function bindStream(") {
		t.Fatalf("combined owner did not exercise repeated schema aliases, Link groups and streaming:\n%s", linked)
	}
	root := string(artifactByPath(t, base, "internal/resources/root.ts"))
	if !strings.Contains(root, "build as __sdkgen_r_d") {
		t.Fatal("resource alias fixture was not exercised")
	}

	var raw map[string]any
	if err := json.Unmarshal(source, &raw); err != nil {
		t.Fatal(err)
	}
	paths := raw["paths"].(map[string]any)
	paths["/unrelated"] = map[string]any{"get": map[string]any{"operationId": "unrelated", "responses": map[string]any{"204": map[string]any{"description": "empty"}}}}
	raw["components"].(map[string]any)["schemas"].(map[string]any)["AAA unrelated"] = map[string]any{"type": "boolean"}
	changedSource, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	changedDocument, err := compiler.Compile(changedSource)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := SourceArtifacts(changedDocument)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range base {
		if strings.HasPrefix(artifact.Path, "internal/operations/") || artifact.Path == "internal/schemas/envelope.ts" || strings.HasPrefix(artifact.Path, "internal/resources/left/") || strings.HasPrefix(artifact.Path, "internal/resources/right/") {
			if !bytes.Equal(artifact.Data, artifactByPath(t, changed, artifact.Path)) {
				t.Fatalf("unrelated entity changed owner %s", artifact.Path)
			}
		}
	}
	server, err := sourceArtifacts(document, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range base {
		if !bytes.Equal(artifact.Data, artifactByPath(t, server, artifact.Path)) {
			t.Fatalf("optional server changed client owner %s", artifact.Path)
		}
	}
}

func TestOperationAliasAllocationSharesFixedAndProtectedReservations(t *testing.T) {
	module := operationModulePlan{routeKey: "GET /source", path: "internal/operations/source/get.ts"}
	path := "internal/schemas/a.ts"
	semantic := &semanticModulePlan{schemaByName: map[string]string{"A": path}, schemaByQuotedName: map[string]string{quoteTS("A"): "A"}}
	names, err := operationLocalIdentifiers(module, ManifestOperation{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := names.reserve("__sdkgen_t_d0"); err != nil {
		t.Fatal(err)
	}
	source := "import type * as ContractSchemas from \"../../schemas/index.js\"\n" +
		"type One = ContractSchemas.ComponentInput<\"A\">\n" +
		"type Two = ContractSchemas.ComponentInput<\"A\">\n" +
		"const __sdkgen_t_d0 = \"__sdkgen_t_d1\"\n"
	got, err := localizeOperationSchemaReferences(source, module, semantic, "../../schemas/index.js", names)
	if err != nil {
		t.Fatal(err)
	}
	if generatedTypeImportAlias(t, got, "../../schemas/a.js", "Input") != "__sdkgen_t_d1" {
		t.Fatal("fixed binding not reserved")
	}
	if !strings.Contains(got, "type One = __sdkgen_t_d1") || !strings.Contains(got, "type Two = __sdkgen_t_d1") || !strings.Contains(got, "const __sdkgen_t_d0 = \"__sdkgen_t_d1\"") {
		t.Fatalf("binding or literal changed: %s", got)
	}
	if err := names.request(typeImportIdentifierKey("late.ts", "Input")); err == nil {
		t.Fatal("operation aliases were not frozen")
	}
}

func TestLinkGroupsUseTheFrozenOwnerWithoutRenamingLeaves(t *testing.T) {
	source := ir.Operation{Method: "GET", Path: "/source"}
	target := ir.Operation{Method: "GET", Path: "/target"}
	link := generatedLink{SourceOperation: source, TargetOperation: target, Status: "200", Name: "follow", Definition: "{}"}
	module := operationModulePlan{routeKey: operationRouteKey(source), path: "internal/operations/source/get.ts"}
	names, err := operationLocalIdentifiers(module, ManifestOperation{}, []generatedLink{link})
	if err != nil {
		t.Fatal(err)
	}
	groups := linkGroupsForSource([]generatedLink{link}, module.routeKey)
	if len(groups) != 1 {
		t.Fatalf("groups=%d", len(groups))
	}
	if _, err := routeLinkGroupsValue(groups, names); err == nil {
		t.Fatal("group resolved before freeze")
	}
	if err := names.reserve("__sdkgen_l_d0"); err != nil {
		t.Fatal(err)
	}
	if err := names.freeze(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := emitLinkValuesForGroups(&output, &ir.Document{}, []generatedLink{link}, groups, func(string) (string, error) { return "target", nil }, names); err != nil {
		t.Fatal(err)
	}
	leaf, err := generatedLinkVariableName(link)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "const "+leaf+":") || !strings.Contains(output.String(), "const __sdkgen_l_d1:") {
		t.Fatalf("group/leaf ownership changed: %s", output.String())
	}
	unplanned := newLocalIdentifierPlan(module.path)
	if err := unplanned.freeze(); err != nil {
		t.Fatal(err)
	}
	if _, err := routeLinkGroupsValue(groups, unplanned); err == nil {
		t.Fatal("missing group owner accepted")
	}
}
