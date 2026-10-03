package typescript

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

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
	for name := range shared.resourceTree.children {
		if shared.resourceTree.childSources[name] == "" {
			t.Fatalf("client pruning changed shared resource identity %q", name)
		}
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

func TestNamedClientsDefaultRootMatchesSelectionUnion(t *testing.T) {
	for _, ids := range [][]string{{"c"}, {"a"}, {"a", "b"}, {"a", "c"}} {
		t.Run(strings.Join(ids, "-"), func(t *testing.T) {
			document := selectedFixtureDocument(t)
			clients := make(map[string]generator.Client)
			for _, id := range ids {
				clients[id] = generator.Client{Selection: &generator.Selection{Operations: []string{id}}}
			}
			// Overlapping assignments must not duplicate shared implementations.
			clients["overlap"] = clients[ids[0]]
			var plans []*sourcePlan
			for _, options := range []generator.Options{
				{Selection: &generator.Selection{Operations: ids}},
				{Clients: clients},
			} {
				plan, values, err := (Generator{}).Prepare(document, options)
				if err != nil || diagnostic.HasErrors(values) {
					t.Fatalf("prepare: %v, %v", err, values)
				}
				value, _ := plan.Value("typescript")
				plans = append(plans, value.(*sourcePlan))
			}
			ordinary, named := plans[0], plans[1]
			if !reflect.DeepEqual(ordinary.selection, named.root.selection) || !reflect.DeepEqual(ordinary.manifest, named.root.manifest) {
				t.Fatal("implicit root differs from the equivalent ordinary selection")
			}
			if len(named.executions) != len(ordinary.executions) || len(named.modules.operations) != len(ordinary.modules.operations) {
				t.Fatal("named assignments expanded or duplicated shared implementations")
			}
			for _, operation := range named.manifest.Operations {
				if operation.RouteKey == "GET /unused" || operation.RouteKey == "GET /idless/{task-id}" {
					t.Fatalf("unselected operation retained: %s", operation.RouteKey)
				}
			}
			for _, client := range named.clients {
				if len(client.view.selection.direct) != 1 {
					t.Fatal("root union leaked into named public scope")
				}
			}
		})
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
