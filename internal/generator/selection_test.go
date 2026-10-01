package generator

import (
	"reflect"
	"testing"
)

func TestSelectionCanonicalPresenceAndIdentity(t *testing.T) {
	if value, err := (*Selection)(nil).Canonical(); err != nil || value != nil {
		t.Fatalf("omitted = %v, %v", value, err)
	}
	for _, value := range []*Selection{{}, {Operations: []string{" "}}, {Routes: []string{"GET"}}, {Routes: []string{"GET  /tasks"}}, {Routes: []string{"GET\t/tasks"}}, {Routes: []string{"G(ET /tasks"}}} {
		if _, err := value.Canonical(); err == nil {
			t.Fatalf("invalid selection accepted: %#v", value)
		}
	}
	input := &Selection{Operations: []string{"get_pet", "get-pet", "get_pet"}, Routes: []string{"GET /tasks/{task-id}", "QUERY /search", "GET /tasks/{task-id}"}}
	want := &Selection{Operations: []string{"get-pet", "get_pet"}, Routes: []string{"GET /tasks/{task-id}", "QUERY /search"}}
	value, err := input.Canonical()
	if err != nil || !reflect.DeepEqual(value, want) {
		t.Fatalf("canonical = %#v, %v", value, err)
	}
	value.Operations[0] = "changed"
	if input.Operations[0] != "get_pet" {
		t.Fatal("canonicalization mutated input")
	}
}
