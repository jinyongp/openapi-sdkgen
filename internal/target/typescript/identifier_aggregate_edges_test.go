package typescript

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler/ir"
)

func TestRegistryIdentifiersRequireExactCompiledOwner(t *testing.T) {
	manifest := Manifest{Operations: []ManifestOperation{{RouteKey: "GET /a"}}}
	for _, operations := range []map[string]ir.Operation{
		nil,
		{"GET /a": {Method: "POST", Path: "/b"}},
	} {
		_, err := planRegistryIdentifiers("internal/client/registry.ts", manifest, operations, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "no matching compiled operation") {
			t.Fatalf("missing/mismatched compiled owner accepted: %v", err)
		}
	}
}

func TestAggregateInvalidRequestsDoNotPartiallyRegister(t *testing.T) {
	var absent *aggregateIdentifierPlan
	key := newAggregateEntity("route", "GET /a")
	if absent.reserve("fixed") == nil || absent.request(key, aggregateBaseFactory) == nil || absent.freeze() == nil {
		t.Fatal("nil owner accepted")
	}
	if _, err := absent.resolve(key, aggregateBaseFactory); err == nil {
		t.Fatal("nil owner resolved")
	}
	plan := newAggregateIdentifierPlan("owner.ts")
	if err := plan.request(key, aggregateBaseFactory, aggregateInputWire); err == nil {
		t.Fatal("cross-domain role accepted")
	}
	if len(plan.requests) != 0 {
		t.Fatal("failed request partially registered roles")
	}
	if err := plan.reserve("valid", ""); err == nil || len(plan.reserved) != 0 {
		t.Fatal("failed reservation was accepted or partially registered")
	}
	if plan.request(aggregateEntityKey{}, aggregateBaseFactory) == nil || plan.request(key) == nil {
		t.Fatal("incomplete request accepted")
	}
	plan.digest = nil
	if plan.freeze() == nil || plan.frozen {
		t.Fatal("invalid freeze published a plan")
	}
}

func TestAggregateFullCollisionIsPermutationAndOwnerIndependent(t *testing.T) {
	keys := []aggregateEntityKey{
		newAggregateEntity("webhook", "a\x00b", "POST"),
		newAggregateEntity("webhook", "a", "b\x00POST"),
		newAggregateEntity("webhook", "é", "POST"),
		newAggregateEntity("webhook", "e\u0301", "POST"),
	}
	build := func(owner string, reverse bool, extra bool) *aggregateIdentifierPlan {
		p := newAggregateIdentifierPlan(owner)
		p.digest = func([]byte) [sha256.Size]byte { return [sha256.Size]byte{} }
		// A full-digest collision must also preserve derived-type reservation.
		fallback := aggregateIdentifierStem(aggregateWebhookType, "x"+aggregateBase32.EncodeToString([]byte(keys[0].payload)))
		if err := p.reserve(fallback+"Context", fallback+"_r0Response"); err != nil {
			t.Fatal(err)
		}
		for i := range keys {
			at := i
			if reverse {
				at = len(keys) - 1 - i
			}
			if err := p.request(keys[at], aggregateWebhookDefinition, aggregateWebhookType); err != nil {
				t.Fatal(err)
			}
		}
		if extra {
			if err := p.request(newAggregateEntity("webhook", "unrelated", "POST"), aggregateWebhookType); err != nil {
				t.Fatal(err)
			}
		}
		if err := p.freeze(); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a, b, c := build("server/webhooks.ts", false, false), build("another-artifact.ts", true, false), build("server/webhooks.ts", true, true)
	if !reflect.DeepEqual(a.names, b.names) {
		t.Fatal("fallback depends on request order or artifact path")
	}
	for binding, name := range a.names {
		if c.names[binding] != name {
			t.Fatal("unrelated full-collision entity renamed an existing exact fallback")
		}
	}
}

func TestAggregateLargeRegistryHasUniqueFinalBindings(t *testing.T) {
	if testing.Short() {
		t.Skip("large identifier uniqueness")
	}
	const count = 100000
	plan := newAggregateIdentifierPlan("internal/client/registry.ts")
	for i := 0; i < count; i++ {
		key := newAggregateEntity("route", fmt.Sprintf("GET /scale/%d", i))
		if err := plan.request(key, aggregateBaseFactory, aggregateBaseValue, aggregateOperationValue); err != nil {
			t.Fatal(err)
		}
	}
	if err := plan.freeze(); err != nil {
		t.Fatal(err)
	}
	if len(plan.names) != 3*count {
		t.Fatalf("bindings=%d", len(plan.names))
	}
	seen := make(map[string]bool, len(plan.names))
	for _, name := range plan.names {
		if seen[name] {
			t.Fatalf("duplicate final binding %q", name)
		}
		seen[name] = true
	}
}
