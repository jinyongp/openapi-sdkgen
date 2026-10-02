package generator

import (
	"reflect"
	"strings"
	"testing"
)

func TestCanonicalClientsPreservesIndependentSelections(t *testing.T) {
	input := map[string]Client{"orders": {Selection: &Selection{Operations: []string{"b", "a", "a"}, Routes: []string{"GET /idless"}}}}
	actual, err := CanonicalClients(input)
	if err != nil {
		t.Fatal(err)
	}
	want := &Selection{Operations: []string{"a", "b"}, Routes: []string{"GET /idless"}}
	if !reflect.DeepEqual(actual["orders"].Selection, want) {
		t.Fatalf("selection = %#v", actual["orders"].Selection)
	}
	actual["orders"].Selection.Operations[0] = "changed"
	if input["orders"].Selection.Operations[0] != "b" {
		t.Fatal("canonical client mutated input")
	}
	if actual, err := CanonicalClients(nil); actual != nil || err != nil {
		t.Fatalf("nil clients = %v, %v", actual, err)
	}
}

func TestCanonicalClientsRejectsInvalidNamesAndAssignments(t *testing.T) {
	for _, name := range []string{"", "A", "../a", "a/b", `a\b`, "a.b", "a b", "1a", "con", "nul", "com1", "lpt9", strings.Repeat("a", 65)} {
		t.Run(name, func(t *testing.T) {
			if _, err := CanonicalClients(map[string]Client{name: {Selection: &Selection{Routes: []string{"GET /a"}}}}); err == nil {
				t.Fatal("invalid client name accepted")
			}
		})
	}
	for _, input := range []map[string]Client{{}, {"a": {}}, {"a": {Selection: &Selection{}}}, {"a": {Selection: &Selection{Routes: []string{"GET  /a"}}}}} {
		if _, err := CanonicalClients(input); err == nil {
			t.Fatalf("invalid assignment accepted: %#v", input)
		}
	}
	if _, err := CanonicalClients(map[string]Client{"orders-2": {Selection: &Selection{Routes: []string{"GET /a"}}}}); err != nil {
		t.Fatal(err)
	}
}

type clientsTestTarget struct{ testTarget }

func (clientsTestTarget) SupportsClients() bool { return true }

func TestValidateTargetOptionsRequiresNamedClientSupport(t *testing.T) {
	options := Options{Clients: map[string]Client{"a": {Selection: &Selection{Routes: []string{"GET /a"}}}}}
	if err := ValidateTargetOptions(testTarget("plain"), options); err == nil {
		t.Fatal("target without client support accepted clients")
	}
	if err := ValidateTargetOptions(clientsTestTarget{testTarget("clients")}, options); err != nil {
		t.Fatal(err)
	}
	options.Clients = map[string]Client{}
	if err := ValidateTargetOptions(clientsTestTarget{testTarget("clients")}, options); err == nil {
		t.Fatal("empty clients accepted")
	}
}
