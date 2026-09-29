package typescript

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
)

// The oracle traverses explicit fixture edges and projection flags, not the
// production wire lowerer or its reverse-edge capability propagation.
func TestExecutionSchemaSummaryMatchesIndependentCyclicGraphOracle(t *testing.T) {
	const count = 48
	const width = 16
	names := make([]string, count)
	edges := make([][]int, count)
	document := &ir.Document{ComponentSchemas: make(map[string]map[string]any)}
	for index := range names {
		names[index] = fmt.Sprintf("Node%02d", index)
	}
	for index, name := range names {
		group := index / width * width
		edges[index] = []int{group + (index+1)%width, group + (index+5)%width}
		properties := make(map[string]any)
		for slot, target := range edges[index] {
			properties[fmt.Sprintf("edge%d", slot)] = map[string]any{"$ref": "#/components/schemas/" + names[target]}
		}
		if index == 7 || index == 23 {
			content := map[string]any{"type": "string", "contentMediaType": "application/xml", "contentSchema": map[string]any{"type": "object"}}
			if index == 7 {
				content["readOnly"] = true
			} else {
				content["writeOnly"] = true
			}
			properties["payload"] = content
		}
		document.ComponentSchemas[name] = map[string]any{"type": "object", "properties": properties}
	}
	planner := newExecutionPlanner(document)
	for pass := 0; pass < 2; pass++ {
		for _, direction := range []projection{projectionOutput, projectionInput} {
			for position := range names {
				root := position
				if pass != 0 {
					root = count - position - 1
				}
				seen := map[int]bool{}
				pending := []int{root}
				want := executionSchemaCapabilities(0)
				var closure []string
				for len(pending) != 0 {
					index := pending[len(pending)-1]
					pending = pending[:len(pending)-1]
					if seen[index] {
						continue
					}
					seen[index] = true
					closure = append(closure, names[index])
					if (index == 7 && direction == projectionOutput) || (index == 23 && direction == projectionInput) {
						want |= executionSchemaXML
					}
					pending = append(pending, edges[index]...)
				}
				sort.Strings(closure)
				key := executionSchemaReference{name: names[root], direction: direction}
				actual, input, output, err := planner.schemaClosure(executionSchemaFacts{references: map[executionSchemaReference]bool{key: true}})
				if err != nil {
					t.Fatal(err)
				}
				got, opposite := output, input
				if direction == projectionInput {
					got, opposite = input, output
				}
				if actual != want || !reflect.DeepEqual(got, closure) || len(opposite) != 0 {
					t.Fatalf("pass=%d root=%s direction=%v: capabilities=%v want=%v, closure=%v want=%v, opposite=%v", pass, names[root], direction, actual, want, got, closure, opposite)
				}
			}
		}
	}
	if len(planner.schemas) != 2*count {
		t.Fatalf("projection cache nodes=%d, want=%d", len(planner.schemas), 2*count)
	}
}

func TestExecutionPlanDoesNotEagerlyIncludeLinkTargetCapabilities(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1", "info":{"title":"Lazy Link execution boundary","version":"1"},
  "paths":{
    "/source":{"get":{"operationId":"readSource","responses":{"200":{"description":"source","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Source"}}},"links":{"next":{"operationId":"readTarget"}}}}}},
    "/target":{"get":{"operationId":"readTarget","responses":{"200":{"description":"target","content":{"application/xml":{"schema":{"$ref":"#/components/schemas/Target"}}}}}}}
  },
  "components":{"schemas":{
    "Source":{"type":"object","properties":{"id":{"type":"string"}}},
    "Target":{"type":"object","xml":{"name":"item"},"properties":{"id":{"type":"string"}}}
  }}
}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := prepareSourcePlan(document, false)
	if err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatalf("prepare: %v; %v", err, diagnostics)
	}
	if len(plan.links) != 1 {
		t.Fatalf("resolved links=%d, want=1", len(plan.links))
	}
	executions, err := prepareOperationExecutions(plan.document, *plan.manifest, plan.modules, plan.streams)
	if err != nil {
		t.Fatal(err)
	}
	source, target := executions["GET /source"], executions["GET /target"]
	if source.profile != executionJSON || !reflect.DeepEqual(source.outputSchemas, []string{"Source"}) {
		t.Fatalf("source eagerly retained its Link target: %#v", source)
	}
	if target.profile != executionBufferedXML || !reflect.DeepEqual(target.outputSchemas, []string{"Target"}) {
		t.Fatalf("Link target lost its own execution requirements: %#v", target)
	}
}
