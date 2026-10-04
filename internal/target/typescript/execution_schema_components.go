package typescript

import (
	"fmt"
	"strings"
)

func (planner *executionPlanner) schemaClosureIdentity(roots map[executionSchemaReference]bool) string {
	canonical := make(map[executionSchemaReference]bool, len(roots))
	for root := range roots {
		if component, exists := planner.components[root]; exists {
			root = component
		}
		canonical[root] = true
	}
	var identity strings.Builder
	for _, root := range sortedExecutionReferences(canonical) {
		fmt.Fprintf(&identity, "%d:%s:%s;", len(root.name), root.name, root.direction)
	}
	return identity.String()
}

// Every discovered node already has its complete reachable graph loaded. Its
// strongly connected component cannot later acquire new members from another
// root. All members therefore share one closure identity, without repeated
// traversal of recursive models for each operation that enters the same cycle.
func (planner *executionPlanner) indexSchemaComponents(reachable map[executionSchemaReference]bool) {
	remaining := false
	for key := range reachable {
		if _, complete := planner.components[key]; complete {
			continue
		}
		if len(planner.schemas[key].facts.references) == 0 {
			planner.components[key] = key
		} else {
			remaining = true
		}
	}
	if !remaining {
		return
	}
	indexes := make(map[executionSchemaReference]int)
	low := make(map[executionSchemaReference]int)
	onStack := make(map[executionSchemaReference]bool)
	var stack []executionSchemaReference
	var visit func(executionSchemaReference)
	visit = func(key executionSchemaReference) {
		indexes[key] = len(indexes) + 1
		low[key] = indexes[key]
		stack = append(stack, key)
		onStack[key] = true
		for child := range planner.schemas[key].facts.references {
			if _, complete := planner.components[child]; complete {
				continue
			}
			if indexes[child] == 0 {
				visit(child)
				if low[child] < low[key] {
					low[key] = low[child]
				}
			} else if onStack[child] && indexes[child] < low[key] {
				low[key] = indexes[child]
			}
		}
		if low[key] != indexes[key] {
			return
		}
		var members []executionSchemaReference
		canonical := key
		for {
			member := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[member] = false
			members = append(members, member)
			if member.name < canonical.name || member.name == canonical.name && member.direction < canonical.direction {
				canonical = member
			}
			if member == key {
				break
			}
		}
		for _, member := range members {
			planner.components[member] = canonical
		}
	}
	for key := range reachable {
		if _, complete := planner.components[key]; !complete && indexes[key] == 0 {
			visit(key)
		}
	}
}
