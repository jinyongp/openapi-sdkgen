package golang

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
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
	walker := partitionASTWalker{}
	if err := walker.transform(file, func(node ast.Node) ast.Node {
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

func TestPartitionASTWalkerReusesOnlyFieldMetadata(t *testing.T) {
	const source = "package sample\nfunc F(){external(); _ = []int{external, external}; _ = values[external]; _ = external + external}\n"
	for _, target := range []string{"external", "other", "last"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			walker := partitionASTWalker{}
			var previous *ast.File
			var previousSet *token.FileSet
			var previousOutput []byte
			var previousTypes int
			for _, name := range []string{target, target + "Next"} {
				set := token.NewFileSet()
				file, err := parser.ParseFile(set, "sample.go", strings.ReplaceAll(source, "external", name), 0)
				if err != nil {
					t.Fatal(err)
				}
				changed, revisited := 0, false
				if err := walker.transform(file, func(node ast.Node) ast.Node {
					if identifier, ok := node.(*ast.Ident); ok {
						if identifier.Name == "remote" {
							revisited = true
						}
						if identifier.Name == name {
							changed++
							return &ast.SelectorExpr{X: ast.NewIdent("remote"), Sel: ast.NewIdent("Value")}
						}
					}
					return node
				}); err != nil {
					t.Fatal(err)
				}
				if changed != 6 || revisited {
					t.Fatal("expression slots or replacement visitation changed", changed, revisited)
				}
				selectors := 0
				ast.Inspect(file, func(node ast.Node) bool {
					if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "Value" {
						selectors++
					}
					return true
				})
				if selectors != 6 {
					t.Fatal("not all expression slots were replaced", selectors)
				}
				if previous != nil {
					var after bytes.Buffer
					if err := format.Node(&after, previousSet, previous); err != nil || !bytes.Equal(previousOutput, after.Bytes()) {
						t.Fatal("reusing metadata changed an earlier tree", err)
					}
					if previousTypes != len(walker.childFields) {
						t.Fatal("metadata grew with tree identities instead of AST types")
					}
				}
				var output bytes.Buffer
				if err := format.Node(&output, set, file); err != nil {
					t.Fatal(err)
				}
				previous, previousSet, previousOutput, previousTypes = file, set, output.Bytes(), len(walker.childFields)
			}
		})
	}
}

func TestPartitionASTWalkerPreservesSlotErrors(t *testing.T) {
	walker := partitionASTWalker{}
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "sample.go", "package sample\nvar external int\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	err = walker.transform(file, func(node ast.Node) ast.Node {
		if identifier, ok := node.(*ast.Ident); ok && identifier.Name == "external" {
			return &ast.SelectorExpr{X: ast.NewIdent("remote"), Sel: ast.NewIdent("Value")}
		}
		return node
	})
	if err == nil || !strings.Contains(err.Error(), "cannot qualify generated identifier at *ast.Ident") {
		t.Fatal("narrow identifier slot error changed", err)
	}
	if file.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names[0].Name != "external" {
		t.Fatal("failed replacement changed its declaration")
	}
	file, err = parser.ParseFile(set, "next.go", "package sample\nvar value = external\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := walker.transform(file, func(node ast.Node) ast.Node {
		if identifier, ok := node.(*ast.Ident); ok && identifier.Name == "external" {
			return &ast.SelectorExpr{X: ast.NewIdent("remote"), Sel: ast.NewIdent("Value")}
		}
		return node
	}); err != nil {
		t.Fatal("failed replacement poisoned later tree metadata", err)
	}
	if _, ok := file.Decls[0].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0].(*ast.SelectorExpr); !ok {
		t.Fatal("later expression slot was not rewritten")
	}
}
