package typescript

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestAggregateCanonicalIdentityUsesExactFieldBoundaries(t *testing.T) {
	entity := newAggregateEntity("route", "GET /x")
	want := "73646b67656e2e6c65786963616c2e7631" + "0000000500000001" + "726f757465" + "0000000000000006" + "474554202f78"
	if got := hex.EncodeToString([]byte(entity.payload)); got != want {
		t.Fatalf("canonical bytes %s, want %s", got, want)
	}
	cases := []aggregateEntityKey{
		newAggregateEntity("callback", "a\x00b", "c"), newAggregateEntity("callback", "a", "b\x00c"),
		newAggregateEntity("callback"), newAggregateEntity("callback", ""), newAggregateEntity("callback", "", ""),
		newAggregateEntity("schema", "Foo"), newAggregateEntity("schema", "foo"),
		newAggregateEntity("schema", "é"), newAggregateEntity("schema", "e\u0301"),
		newAggregateEntity("route", "foo"), newAggregateEntity("schema", "__sdkgen_ov_hx_r0"),
	}
	seen := map[string]bool{}
	for _, key := range cases {
		if seen[key.payload] {
			t.Fatalf("identity collapsed: %q", key.payload)
		}
		seen[key.payload] = true
	}
}

func aggregateNameForTest(t testing.TB, plan *aggregateIdentifierPlan, key aggregateEntityKey, role aggregateIdentifierRole) string {
	t.Helper()
	name, err := plan.resolve(key, role)
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func TestAggregateIdentifierReservationsCoverDerivedBindings(t *testing.T) {
	key := newAggregateEntity("webhook", "event", "POST")
	base := newAggregateIdentifierPlan("server/webhooks.ts")
	if err := base.request(key, aggregateWebhookType, aggregateWebhookDefinition); err != nil {
		t.Fatal(err)
	}
	if err := base.freeze(); err != nil {
		t.Fatal(err)
	}
	oldType := aggregateNameForTest(t, base, key, aggregateWebhookType)
	oldDef := aggregateNameForTest(t, base, key, aggregateWebhookDefinition)
	for _, reserved := range []string{oldType + "Context", oldType + "Response", oldDef + "Handlers", oldDef + "PathParameters", oldDef} {
		p := newAggregateIdentifierPlan("server/webhooks.ts")
		if err := p.reserve(reserved); err != nil {
			t.Fatal(err)
		}
		if err := p.request(key, aggregateWebhookDefinition, aggregateWebhookType); err != nil {
			t.Fatal(err)
		}
		if err := p.freeze(); err != nil {
			t.Fatal(err)
		}
		typeName := aggregateNameForTest(t, p, key, aggregateWebhookType)
		defName := aggregateNameForTest(t, p, key, aggregateWebhookDefinition)
		if typeName == oldType || defName == oldDef {
			t.Fatalf("reservation %q did not extend shared entity token", reserved)
		}
		if strings.TrimPrefix(typeName, "__sdkgen_wt_") != strings.TrimPrefix(defName, "__sdkgen_wd_") {
			t.Fatal("related roles did not share token")
		}
	}
}

func TestAggregateForcedPrefixAndFullDigestCollisions(t *testing.T) {
	keys := []aggregateEntityKey{newAggregateEntity("schema", "a"), newAggregateEntity("schema", "b"), newAggregateEntity("schema", "c")}
	for _, full := range []bool{false, true} {
		t.Run(fmt.Sprint("full=", full), func(t *testing.T) {
			p := newAggregateIdentifierPlan("internal/schemas/wire.ts")
			p.digest = func(value []byte) [sha256.Size]byte {
				var d [sha256.Size]byte
				if !full {
					d[31] = value[len(value)-1]
				}
				return d
			}
			for _, key := range keys {
				if err := p.request(key, aggregateInputWire, aggregateOutputWire); err != nil {
					t.Fatal(err)
				}
			}
			if err := p.freeze(); err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			for _, key := range keys {
				for _, role := range []aggregateIdentifierRole{aggregateInputWire, aggregateOutputWire} {
					name := aggregateNameForTest(t, p, key, role)
					if seen[name] || !regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*$`).MatchString(name) {
						t.Fatalf("unsafe name %q", name)
					}
					seen[name] = true
					token := strings.TrimPrefix(name, "__sdkgen_"+string(role)+"_")
					if full && !strings.HasPrefix(token, "x") {
						t.Fatalf("full collision needs lossless fallback: %s", name)
					}
					if !full && (!strings.HasPrefix(token, "h") || len(token) <= 1+aggregateMinimumPrefix) {
						t.Fatalf("prefix collision not extended: %s", name)
					}
				}
			}
		})
	}
}

func TestAggregateReservedFullHashAndLosslessFallback(t *testing.T) {
	key := newAggregateEntity("schema", "a\x00b")
	d := sha256.Sum256([]byte(key.payload))
	full := aggregateBase32.EncodeToString(d[:])
	p := newAggregateIdentifierPlan("internal/schemas/wire.ts")
	for n := aggregateMinimumPrefix; n <= len(full); n++ {
		if err := p.reserve(aggregateIdentifierStem(aggregateInputWire, "h"+full[:n])); err != nil {
			t.Fatal(err)
		}
	}
	fallback := aggregateIdentifierStem(aggregateInputWire, "x"+aggregateBase32.EncodeToString([]byte(key.payload)))
	if err := p.reserve(fallback, fallback+"_r0"); err != nil {
		t.Fatal(err)
	}
	if err := p.request(key, aggregateInputWire); err != nil {
		t.Fatal(err)
	}
	if err := p.freeze(); err != nil {
		t.Fatal(err)
	}
	if got := aggregateNameForTest(t, p, key, aggregateInputWire); got != fallback+"_r1" {
		t.Fatalf("fallback %q", got)
	}
}

func TestAggregatePlanIsDeterministicLocalAndFrozen(t *testing.T) {
	keys := make([]aggregateEntityKey, 200)
	for i := range keys {
		keys[i] = newAggregateEntity("route", fmt.Sprintf("GET /resources/%d", i))
	}
	build := func(reverse bool, extra bool) *aggregateIdentifierPlan {
		p := newAggregateIdentifierPlan("internal/client/registry.ts")
		for i := range keys {
			at := i
			if reverse {
				at = len(keys) - 1 - i
			}
			if err := p.request(keys[at], aggregateBaseFactory, aggregateBaseValue, aggregateOperationValue); err != nil {
				t.Fatal(err)
			}
		}
		if err := p.request(keys[0], aggregateBaseFactory); err != nil {
			t.Fatal(err)
		}
		if extra {
			if err := p.request(newAggregateEntity("route", "POST /independent"), aggregateOperationValue); err != nil {
				t.Fatal(err)
			}
		}
		if err := p.freeze(); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a, b, c := build(false, false), build(true, false), build(false, true)
	if !reflect.DeepEqual(a.names, b.names) {
		t.Fatal("request permutation changed spellings")
	}
	for key, name := range a.names {
		if c.names[key] != name {
			t.Fatal("unrelated entity renamed an existing binding")
		}
	}
	if err := a.request(keys[0], aggregateBaseValue); err == nil {
		t.Fatal("late request accepted")
	}
	if err := a.reserve("late"); err == nil {
		t.Fatal("late reservation accepted")
	}
	if err := a.freeze(); err == nil {
		t.Fatal("second freeze accepted")
	}
	if _, err := a.resolve(keys[0], aggregateLinksValue); err == nil {
		t.Fatal("unrequested role resolved")
	}
	if _, err := newAggregateIdentifierPlan("owner").resolve(keys[0], aggregateBaseValue); err == nil {
		t.Fatal("unfrozen reference resolved")
	}
	for _, owner := range []string{"", "owner"} {
		p := newAggregateIdentifierPlan(owner)
		if err := p.request(keys[0], aggregateInputWire); err == nil {
			t.Fatal("cross-domain role accepted")
		}
		if err := p.request(keys[0], aggregateIdentifierRole("unknown")); err == nil {
			t.Fatal("unknown role accepted")
		}
	}
}

func TestAggregatePlansAreIsolated(t *testing.T) {
	for worker := 0; worker < 24; worker++ {
		t.Run(fmt.Sprint(worker), func(t *testing.T) {
			t.Parallel()
			p := newAggregateIdentifierPlan(fmt.Sprintf("owner-%d", worker))
			key := newAggregateEntity("route", fmt.Sprintf("GET /%d", worker))
			if err := p.request(key, aggregateOperationValue); err != nil {
				t.Fatal(err)
			}
			if err := p.freeze(); err != nil {
				t.Fatal(err)
			}
			if len(p.names) != 1 {
				t.Fatal("state leaked from another plan")
			}
		})
	}
}

func BenchmarkAggregateIdentifierPlan(b *testing.B) {
	for _, size := range []int{1000, 10000, 100000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			keys := make([]aggregateEntityKey, size)
			for i := range keys {
				keys[i] = newAggregateEntity("route", fmt.Sprintf("GET /benchmark/%d", i))
			}
			b.ReportAllocs()
			b.ResetTimer()
			for run := 0; run < b.N; run++ {
				p := newAggregateIdentifierPlan("internal/client/registry.ts")
				for _, key := range keys {
					if err := p.request(key, aggregateBaseFactory, aggregateBaseValue, aggregateOperationValue); err != nil {
						b.Fatal(err)
					}
				}
				if err := p.freeze(); err != nil {
					b.Fatal(err)
				}
				for _, key := range keys {
					if _, err := p.resolve(key, aggregateOperationValue); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
