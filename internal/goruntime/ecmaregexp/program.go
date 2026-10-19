// Package ecmaregexp compiles the ECMAScript Unicode regular expressions used by
// schema validation. It has no JavaScript VM or external module dependency.
package ecmaregexp

import (
	"errors"
	"fmt"
	"openapi-sdkgen/internal/goruntime/unicode17"
)

const Version uint16 = 1
const MaxSteps uint64 = 100_000_000
const MaxNesting = 100_000

var ErrComplexity = errors.New("regular expression is too complex")

type SyntaxError struct {
	Offset  int
	Message string
}

func (failure *SyntaxError) Error() string {
	return fmt.Sprintf("regular expression at %d: %s", failure.Offset, failure.Message)
}

type Flags uint8

const (
	IgnoreCase Flags = 1 << iota
	Multiline
	DotAll
)

type Op uint8

const (
	Empty Op = iota
	Literal
	CharacterClass
	Any
	Sequence
	Alternative
	Capture
	Reference
	Look
	Start
	End
	WordBoundary
	Repeat
)

type Range = unicode17.Range

// Instruction indices and range/edge spans form a flat, serializable program.
// Group intervals identify captures reset by a repeated subexpression.
type Instruction struct {
	Op                       Op
	A, B                     int
	Value                    rune
	Flags                    Flags
	Group                    int
	FirstGroup, LastGroup    int
	Min, Max                 uint64
	Negative, Backward, Lazy bool
}
type NamedCapture struct {
	Name   string
	Groups []int
}

// Program is immutable after construction. Each match owns its execution state.
// Fields are exported for generation-time serialization into owned SDK source.
type Program struct {
	Version      uint16
	Nodes        []Instruction
	Edges        []int
	Ranges       []Range
	Named        []NamedCapture
	Root, Groups int
}
