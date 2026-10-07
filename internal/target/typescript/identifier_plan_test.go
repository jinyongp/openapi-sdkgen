package typescript

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

func resolveLocalTestNames(t testing.TB, owner string, keys []localIdentifierKey, reserved ...string) map[localIdentifierKey]string {
	t.Helper()
	plan := newLocalIdentifierPlan(owner)
	if err := plan.reserve(reserved...); err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		if err := plan.request(key); err != nil {
			t.Fatal(err)
		}
	}
	if err := plan.freeze(); err != nil {
		t.Fatal(err)
	}
	result := make(map[localIdentifierKey]string)
	used := make(map[string]bool)
	for _, key := range keys {
		name, err := plan.resolve(key)
		if err != nil {
			t.Fatal(err)
		}
		if previous, exists := result[key]; exists {
			if previous != name {
				t.Fatalf("repeated reference changed: %q -> %q", previous, name)
			}
			continue
		}
		if used[name] || !isTypeScriptBindingIdentifier(name) {
			t.Fatalf("invalid/duplicate alias %q", name)
		}
		used[name] = true
		result[key] = name
	}
	for _, name := range reserved {
		if used[name] {
			t.Fatalf("reserved binding %q captured", name)
		}
	}
	return result
}

func TestLocalIdentifiersPreserveExactKeysAndReservations(t *testing.T) {
	t.Parallel()
	keys := []localIdentifierKey{
		typeImportIdentifierKey("a\x00b", "c"), typeImportIdentifierKey("a", "b\x00c"),
		typeImportIdentifierKey("", ""), typeImportIdentifierKey("A", "Input"), typeImportIdentifierKey("a", "Input"),
		typeImportIdentifierKey("foo-bar", "Input"), typeImportIdentifierKey("foo_bar", "Input"),
		typeImportIdentifierKey("é", "Input"), typeImportIdentifierKey("e\u0301", "Input"),
		typeImportIdentifierKey("__proto__", "Input"), typeImportIdentifierKey("__proto__", "Output"),
		resourceBuilderIdentifierKey("a\x00b"), resourceBuilderIdentifierKey("a"),
	}
	reserved := []string{"__sdkgen_t_d0", "__sdkgen_t_d2", "__sdkgen_r_d0", "__sdkgen_Input", "__sdkgen_Properties"}
	want := resolveLocalTestNames(t, "internal/schemas/owner.ts", keys, reserved...)
	for seed := int64(0); seed < 32; seed++ {
		permuted := append([]localIdentifierKey(nil), keys...)
		rand.New(rand.NewSource(seed)).Shuffle(len(permuted), func(i, j int) { permuted[i], permuted[j] = permuted[j], permuted[i] })
		permuted = append(permuted, permuted[0], permuted[1])
		got := resolveLocalTestNames(t, "internal/schemas/owner.ts", permuted, reserved...)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("seed %d changed allocation", seed)
		}
	}
}

func TestLocalIdentifierPlanRequiresOwnerAndFrozenReferences(t *testing.T) {
	t.Parallel()
	key := typeImportIdentifierKey("target.ts", "Input")
	for _, plan := range []*localIdentifierPlan{nil, newLocalIdentifierPlan("")} {
		if err := plan.request(key); err == nil {
			t.Fatal("ownerless request accepted")
		}
		if err := plan.freeze(); err == nil {
			t.Fatal("ownerless freeze accepted")
		}
	}
	plan := newLocalIdentifierPlan("owner.ts")
	if _, err := plan.resolve(key); err == nil {
		t.Fatal("reference resolved before freeze")
	}
	if err := plan.reserve(""); err == nil {
		t.Fatal("empty reservation accepted")
	}
	if err := plan.request(localIdentifierKey{role: "unrecognized"}); err == nil {
		t.Fatal("unknown role accepted")
	}
	if err := plan.request(key); err != nil {
		t.Fatal(err)
	}
	if err := plan.request(key); err != nil {
		t.Fatal(err)
	}
	if err := plan.freeze(); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.resolve(key); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.resolve(typeImportIdentifierKey("missing.ts", "Output")); err == nil || !strings.Contains(err.Error(), "owner.ts") {
		t.Fatalf("missing lookup = %v", err)
	}
	if err := plan.request(key); err == nil {
		t.Fatal("late request accepted")
	}
	if err := plan.reserve("late"); err == nil {
		t.Fatal("late reservation accepted")
	}
	if err := plan.freeze(); err == nil {
		t.Fatal("second freeze accepted")
	}
}

func TestLocalIdentifierPlansDoNotShareState(t *testing.T) {
	for i := 0; i < 24; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			key := typeImportIdentifierKey("target.ts", "Input")
			got := resolveLocalTestNames(t, fmt.Sprintf("owner-%d.ts", i), []localIdentifierKey{key})
			if got[key] != "__sdkgen_t_d0" {
				t.Fatalf("foreign allocation leaked: %q", got[key])
			}
		})
	}
}

func TestLocalValueImportsShareReservationsAndPreserveExports(t *testing.T) {
	keys := []localIdentifierKey{
		schemaValueIdentifierKey("shared.ts", "schema"),
		schemaValueIdentifierKey("shared.ts", "program0"),
		schemaValueIdentifierKey("shared.ts", "program1"),
		typeImportIdentifierKey("shared.ts", "Input"),
		linkGroupIdentifierKey(generatedLinkGroup{Name: "link"}),
	}
	got := resolveLocalTestNames(t, "owner.ts", keys, "__sdkgen_d_d0", "__sdkgen_p_d0")
	if got[keys[0]] != "__sdkgen_d_d1" || got[keys[1]] != "__sdkgen_p_d1" || got[keys[2]] == got[keys[1]] {
		t.Fatalf("incorrect value import allocation: %#v", got)
	}
	for _, key := range []localIdentifierKey{schemaValueIdentifierKey("", "schema"), schemaValueIdentifierKey("x.ts", "")} {
		if err := newLocalIdentifierPlan("owner.ts").request(key); err == nil {
			t.Fatal("incomplete value import accepted")
		}
	}
}

func TestLocalIdentifierPlanLargeSetIsUnique(t *testing.T) {
	if testing.Short() {
		t.Skip("100k local bindings")
	}
	keys := make([]localIdentifierKey, 100_000)
	for i := range keys {
		keys[i] = typeImportIdentifierKey(fmt.Sprintf("schema-%06d.ts", i), "Input")
	}
	got := resolveLocalTestNames(t, "large.ts", keys, "__sdkgen_t_d0")
	if len(got) != len(keys) {
		t.Fatalf("bindings = %d", len(got))
	}
}

func TestTypeReferencePlanningUsesOwnerReservationsAndExactReplayTargets(t *testing.T) {
	t.Parallel()
	owner := "internal/schemas/owner.ts"
	names := newLocalIdentifierPlan(owner)
	if err := names.reserve("__sdkgen_t_d0"); err != nil {
		t.Fatal(err)
	}
	uses := []typeReferenceUse{
		{key: "0", modulePath: "internal/schemas/a.ts", exportName: "Input"},
		{key: "1", modulePath: "internal/schemas/a.ts", exportName: "Input"},
		{key: "2", modulePath: "internal/schemas/a.ts", exportName: "Output"},
	}
	refs, err := planTypeReferences(&semanticModulePlan{}, owner, uses, names)
	if err != nil {
		t.Fatal(err)
	}
	if refs[0].alias != "__sdkgen_t_d1" || refs[1].alias != refs[0].alias || !refs[2].inline {
		t.Fatalf("policy changed: %#v", refs)
	}
	if refs[0].modulePath != uses[0].modulePath || refs[0].exportName != "Input" {
		t.Fatal("planned exact target was lost")
	}
	if _, err := planTypeReferences(&semanticModulePlan{}, owner, uses, newLocalIdentifierPlan("other.ts")); err == nil {
		t.Fatal("foreign owner accepted")
	}
	for _, invalid := range [][]typeReferenceUse{
		{{key: ""}},
		{{key: "a", modulePath: "x.ts", exportName: "Input"}, {key: "a", modulePath: "y.ts", exportName: "Output"}},
	} {
		if _, err := planTypeReferences(&semanticModulePlan{}, owner, invalid, newLocalIdentifierPlan(owner)); err == nil {
			t.Fatal("invalid/conflicting reference slots accepted")
		}
	}
}

func TestSchemaReferenceReplayRejectsSameCountWrongIdentity(t *testing.T) {
	t.Parallel()
	expected := []schemaProjectionReference{{name: "self", direction: projectionInput}, {name: "target", direction: projectionOutput}}
	replay := schemaReferenceReplay{owner: "owner.ts", expected: expected}
	if err := replay.observe("self", projectionInput); err != nil {
		t.Fatal(err)
	}
	if err := replay.observe("target", projectionOutput); err != nil {
		t.Fatal(err)
	}
	if err := replay.finish(); err != nil {
		t.Fatal(err)
	}
	for _, wrong := range []schemaProjectionReference{{name: "other", direction: projectionInput}, {name: "self", direction: projectionOutput}} {
		probe := schemaReferenceReplay{owner: "owner.ts", expected: expected}
		if err := probe.observe(wrong.name, wrong.direction); err == nil || !strings.Contains(err.Error(), "owner.ts") {
			t.Fatalf("identity drift = %v", err)
		}
	}
	if err := replay.observe("extra", projectionInput); err == nil {
		t.Fatal("extra reference accepted")
	}
	if err := (&schemaReferenceReplay{owner: "owner.ts", expected: expected}).finish(); err == nil {
		t.Fatal("missing references accepted")
	}
}

func BenchmarkLocalIdentifierPlan(b *testing.B) {
	for _, count := range []int{1000, 10000, 100000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			keys := make([]localIdentifierKey, count)
			for i := range keys {
				keys[i] = typeImportIdentifierKey(fmt.Sprintf("schema-%06d.ts", i), "Input")
			}
			b.ReportAllocs()
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				plan := newLocalIdentifierPlan("benchmark.ts")
				for _, key := range keys {
					if err := plan.request(key); err != nil {
						b.Fatal(err)
					}
				}
				if err := plan.freeze(); err != nil {
					b.Fatal(err)
				}
				for _, key := range keys {
					if _, err := plan.resolve(key); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
