package golang

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"openapi-sdkgen/internal/goruntime/ecmaregexp"
)

// patternProgramSource emits versioned values, never source to be recompiled by
// the SDK. Numeric op/flag values are owned by the versioned Program contract.
func patternProgramSource(program *ecmaregexp.Program) string {
	var out strings.Builder
	fmt.Fprintf(&out, "&ecmaregexp.Program{Version:%d,Root:%d,Groups:%d,Nodes:[]ecmaregexp.Instruction{", program.Version, program.Root, program.Groups)
	for _, node := range program.Nodes {
		fmt.Fprintf(&out, "{Op:%d", node.Op)
		for _, field := range []struct {
			name  string
			value int64
		}{
			{"A", int64(node.A)}, {"B", int64(node.B)}, {"Value", int64(node.Value)}, {"Flags", int64(node.Flags)}, {"Group", int64(node.Group)}, {"FirstGroup", int64(node.FirstGroup)}, {"LastGroup", int64(node.LastGroup)},
		} {
			if field.value != 0 {
				fmt.Fprintf(&out, ",%s:%d", field.name, field.value)
			}
		}
		if node.Min != 0 {
			fmt.Fprintf(&out, ",Min:%d", node.Min)
		}
		if node.Max != 0 {
			fmt.Fprintf(&out, ",Max:%d", node.Max)
		}
		if node.Negative {
			out.WriteString(",Negative:true")
		}
		if node.Backward {
			out.WriteString(",Backward:true")
		}
		if node.Lazy {
			out.WriteString(",Lazy:true")
		}
		out.WriteString("},")
	}
	out.WriteString("},")
	if len(program.Edges) > 0 {
		out.WriteString("Edges:[]int{")
		for _, edge := range program.Edges {
			fmt.Fprintf(&out, "%d,", edge)
		}
		out.WriteString("},")
	}
	if len(program.Ranges) > 0 {
		out.WriteString("Ranges:[]ecmaregexp.Range{")
		for _, span := range program.Ranges {
			fmt.Fprintf(&out, "{Lo:%d,Hi:%d},", span.Lo, span.Hi)
		}
		out.WriteString("},")
	}
	if len(program.Named) > 0 {
		out.WriteString("Named:[]ecmaregexp.NamedCapture{")
		for _, capture := range program.Named {
			fmt.Fprintf(&out, "{Name:%q,Groups:[]int{", capture.Name)
			for _, group := range capture.Groups {
				fmt.Fprintf(&out, "%d,", group)
			}
			out.WriteString("}},")
		}
		out.WriteString("},")
	}
	out.WriteString("}")
	return out.String()
}

// Identical pattern source always has the same Unicode-u semantics. Keep the
// immutable program once; each schema retains the binding/source metadata used
// by header, XML and server projections. Emission collects successful programs
// so an omitted operation cannot leave references to a discarded declaration.
func (planner *schemaPlanner) preparePattern(source string) (schemaPattern, error) {
	if prepared, ok := planner.compiledPatterns[source]; ok {
		return prepared, nil
	}
	compiled, err := ecmaregexp.Compile(source)
	if err != nil {
		return schemaPattern{}, fmt.Errorf("invalid JavaScript pattern: %w", err)
	}
	hash := sha256.Sum256([]byte(source))
	prepared := schemaPattern{Name: "CompiledPattern_" + hex.EncodeToString(hash[:]), Pattern: source, Program: patternProgramSource(compiled)}
	if planner.compiledPatterns == nil {
		planner.compiledPatterns = make(map[string]schemaPattern)
	}
	planner.compiledPatterns[source] = prepared
	return prepared, nil
}
