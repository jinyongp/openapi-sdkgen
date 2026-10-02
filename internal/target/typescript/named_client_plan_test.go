package typescript

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

func TestNamedClientPlanKeepsIndependentRootsAndPrivateLinkClosure(t *testing.T) {
	document := selectedFixtureDocument(t)
	before, _ := json.Marshal(document)
	options := generator.Options{
		Selection: &generator.Selection{Routes: []string{"GET /idless/{task-id}"}},
		Clients: map[string]generator.Client{
			"a": {Selection: &generator.Selection{Operations: []string{"a"}}},
			"b": {Selection: &generator.Selection{Operations: []string{"b"}}},
		},
	}
	plan, values, err := (Generator{}).Prepare(document, options)
	if err != nil || diagnostic.HasErrors(values) {
		t.Fatalf("prepare: %v, %#v", err, values)
	}
	value, _ := plan.Value("typescript")
	shared := value.(*sourcePlan)
	if len(shared.executions) != 4 || len(shared.modules.operations) != 4 || len(shared.clients) != 2 {
		t.Fatalf("shared plan = %#v", shared)
	}
	if shared.selection.dependencies["GET /b"] || !shared.selection.dependencies["GET /c"] {
		t.Fatalf("union dependencies = %v", shared.selection.dependencies)
	}
	if actual := shared.root.selection.publicManifest(*shared.root.manifest); len(actual.Operations) != 1 || actual.Operations[0].RouteKey != "GET /idless/{task-id}" {
		t.Fatalf("root surface = %#v", actual)
	}
	a := shared.clients[0].view
	if !a.selection.dependencies["GET /b"] || !a.selection.dependencies["GET /c"] || a.selection.direct["GET /b"] {
		t.Fatalf("client a dependencies = %#v", a.selection)
	}
	if len(a.manifest.Operations) != 3 || len(selectiveResourcePlacements(a.resourceTree)) != 1 {
		t.Fatalf("client a public and private scope = %#v", a.manifest)
	}
	for _, client := range shared.clients {
		for _, module := range client.view.modules.operations {
			if module.path != shared.modules.operationByRoute[module.routeKey] {
				t.Fatal("client changed shared operation path")
			}
		}
	}
	after, _ := json.Marshal(document)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("named client planning mutated compiler IR")
	}
}

func TestNamedClientsPreserveFullRootDefault(t *testing.T) {
	document, err := sdkgen.Compile([]byte(strings.ReplaceAll(generationSelectionFixture, `,"x-envelope":false`, "")))
	if err != nil {
		t.Fatal(err)
	}
	plan, values, err := (Generator{}).Prepare(document, generator.Options{Clients: map[string]generator.Client{"c": {Selection: &generator.Selection{Operations: []string{"c"}}}}})
	if err != nil || diagnostic.HasErrors(values) {
		t.Fatalf("prepare: %v, %v", err, values)
	}
	value, _ := plan.Value("typescript")
	shared := value.(*sourcePlan)
	if shared.root.selection != nil || len(shared.root.manifest.Operations) != len(shared.manifest.Operations) {
		t.Fatal("clients restricted the ordinary SDK default")
	}
	if len(shared.clients[0].view.manifest.Operations) != 1 {
		t.Fatal("full root leaked into named selection")
	}
}

func TestNamedClientSelectorErrorsIdentifyClient(t *testing.T) {
	for _, id := range []string{"unknown", "hidden"} {
		_, values, err := (Generator{}).Prepare(selectedFixtureDocument(t), generator.Options{Clients: map[string]generator.Client{"orders": {Selection: &generator.Selection{Operations: []string{id}}}}})
		if err != nil || !diagnostic.HasErrors(values) || !strings.Contains(values[0].Message, `Client "orders"`) {
			t.Fatalf("selector %s: %v, %#v", id, err, values)
		}
	}
}
