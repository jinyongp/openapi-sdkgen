package golang

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"testing"
)

func TestPartitionASTTransformPreservesParserSyntax(t *testing.T) {
	const source = `package sample
import "fmt"
type Store[T any] struct { Obj T; Scope string; Unresolved bool }
type Reader[T ~int | ~int64] interface { Read(T) (T, error) }
func Convert[T ~int](input []T) (output T) {
 defer func() { _ = fmt.Sprint(output) }()
 for index, value := range input {
  switch { case index == 0: output = value; default: output += value }
 }
 return
}
func Branch(input <-chan int) {
 select { case value := <-input: _ = value; default: }
 label: for { break label }
}
`
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "sample.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	expected := make(map[*ast.Ident]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		if identifier, ok := node.(*ast.Ident); ok {
			expected[identifier] = true
		}
		return true
	})
	var before, after bytes.Buffer
	if err := format.Node(&before, set, file); err != nil {
		t.Fatal(err)
	}
	actual := make(map[*ast.Ident]bool)
	if err := transformPartitionAST(file, func(node ast.Node) ast.Node {
		if identifier, ok := node.(*ast.Ident); ok {
			actual[identifier] = true
		}
		return node
	}); err != nil {
		t.Fatal(err)
	}
	if len(actual) != len(expected) {
		t.Fatalf("visited %d distinct identifiers, standard AST walk visited %d", len(actual), len(expected))
	}
	for identifier := range expected {
		if !actual[identifier] {
			t.Fatalf("missed syntax identifier %q", identifier.Name)
		}
	}
	if err := format.Node(&after, set, file); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before.Bytes(), after.Bytes()) {
		t.Fatal("identity transform modified parser syntax")
	}
}
