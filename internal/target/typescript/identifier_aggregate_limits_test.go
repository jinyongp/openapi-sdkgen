package typescript

import (
	"strings"
	"testing"
)

func TestAggregatePlanKeepsOrdinaryLongIdentitiesCompact(t *testing.T) {
	key := newAggregateEntity("schema", strings.Repeat("長い-schema-\x00", 10000))
	plan := newAggregateIdentifierPlan("internal/schemas/wire.ts")
	if err := plan.request(key, aggregateInputWire, aggregateOutputWire); err != nil {
		t.Fatal(err)
	}
	if err := plan.freeze(); err != nil {
		t.Fatal(err)
	}
	for _, role := range []aggregateIdentifierRole{aggregateInputWire, aggregateOutputWire} {
		name := aggregateNameForTest(t, plan, key, role)
		if len(name) != len("__sdkgen_")+len(role)+1+1+aggregateMinimumPrefix {
			t.Fatalf("ordinary long identity leaked into emitted name: length %d", len(name))
		}
	}
}
