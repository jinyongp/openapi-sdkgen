package typescript

import (
	"reflect"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
)

func TestExecutionPlansArePreparedBeforeEmission(t *testing.T) {
	document, err := sdkgen.Compile([]byte(emitterFixture))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := prepareSourcePlan(document, false)
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("prepare: %v; %v", err, diagnostics)
	}
	if len(plan.executions) != len(plan.modules.operations) || len(plan.executions) == 0 {
		t.Fatalf("prepared executions = %d, modules = %d", len(plan.executions), len(plan.modules.operations))
	}
	for _, module := range plan.modules.operations {
		execution, exists := plan.executions[module.routeKey]
		if !exists || execution.profile == "" {
			t.Fatalf("missing prepared profile for %s", module.routeKey)
		}
	}
	first, err := emitSourcePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	second, err := emitSourcePlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("reusing a prepared plan changed emitted artifacts")
	}
}

func TestMissingExecutionPlanFailsBeforeWritingAnyArtifact(t *testing.T) {
	document, err := sdkgen.Compile([]byte(emitterFixture))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := prepareSourcePlan(document, false)
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("prepare: %v; %v", err, diagnostics)
	}
	delete(plan.executions, plan.modules.operations[len(plan.modules.operations)-1].routeKey)
	writes := 0
	err = emitSourcePlanTo(plan, func(Artifact) error { writes++; return nil })
	if err == nil || !strings.Contains(err.Error(), "missing prepared execution") {
		t.Fatalf("expected missing execution error, got %v", err)
	}
	if writes != 0 {
		t.Fatalf("invalid plan wrote %d artifacts", writes)
	}
}
