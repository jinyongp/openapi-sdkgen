package golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// Isolate traversal overhead; parsing and SDK generation are outside the timer.
func BenchmarkPartitionASTTransform(b *testing.B) {
	var source strings.Builder
	source.WriteString("package sample\n")
	for index := 0; index < 100; index++ {
		fmt.Fprintf(&source, "func Value%d(input int) int { values := []int{input, 2}; for _, value := range values { input += value }; return input }\n", index)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "sample.go", source.String(), 0)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	var visited int
	for b.Loop() {
		visited = 0
		if err := transformPartitionAST(file, func(node ast.Node) ast.Node {
			visited++
			return node
		}); err != nil {
			b.Fatal(err)
		}
	}
	if visited == 0 {
		b.Fatal("AST traversal did not execute")
	}
	b.ReportMetric(float64(visited), "node-visits/op")
}
