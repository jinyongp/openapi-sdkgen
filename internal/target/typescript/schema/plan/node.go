// Package plan owns normalized schema semantics independently of TypeScript output.
package plan

type Projection string

const (
	Input  Projection = "input"
	Output Projection = "output"
)

// Node preserves the canonical lowering order without storing source expressions.
type Node struct{ Fields []Field }
type Field struct {
	Name  string
	Value Value
}
type Value interface{ schemaValue() }
type Literal struct{ Data any }
type Child struct{ Node *Node }
type Children struct{ Nodes []*Node }
type Property struct {
	Name   string
	Schema *Node
}
type Properties struct{ Entries []Property }
type SchemaMap struct{ Entries []Property }
type DynamicReference struct {
	Anchor   string
	Fallback *Node
}
type Discriminator struct {
	Property string
	Mapping  []Property
	Default  *Node
}

func (Literal) schemaValue()          {}
func (Child) schemaValue()            {}
func (Children) schemaValue()         {}
func (Properties) schemaValue()       {}
func (SchemaMap) schemaValue()        {}
func (DynamicReference) schemaValue() {}
func (Discriminator) schemaValue()    {}

func (node *Node) Get(name string) Value {
	for _, field := range node.Fields {
		if field.Name == name {
			return field.Value
		}
	}
	return nil
}

// Observer connects semantic lowering to the target's reference and feature plan.
type Observer interface {
	Field(string)
	Value(string, string)
	Reference(string, Projection)
	Dynamic()
	ContentMedia(string)
}
type Options struct {
	Projection      Projection
	FormatAssertion bool
	LegacyNullable  bool
	ReferenceName   func(string) (string, error)
	Observer        Observer
}
