package golang

import (
	"bytes"
	"go/format"
	"go/parser"
	"go/token"
	"testing"
)

func TestPartitionIntermediateFormattingRemainsCanonical(t *testing.T) {
	for name, source := range map[string]string{
		"columns": `package sample
// Record has fields of different widths.
type Record struct {
X int ` + "`json:\"x\"`" + ` // Short name.
LongFieldName string ` + "`json:\"long\"`" + ` // Long name.
}
const (
X=1 // First value.
LongConstant=2 // Second value.
)
var Values=map[string]int{"x":X,"long":LongConstant}
`,
		"imports-and-comments": `// Package sample preserves its comments.
package sample
import (
// Keep the formatting dependency.
"fmt"
_ "net/http/pprof"
)
/* Preserve this block's layout:
   first   second
   a       longer
*/
func Value(input int) string { return fmt.Sprint(input) /* Inline body comment. */ }
`,
		"literal-bytes": "package sample\nconst Raw = `first\tcolumn\v\f\nsecond line`\n" +
			"const Escaped = \"\\t\\v\\f\\n// literal /* data */\"\n" +
			"const Hex = 0X_AB\nconst Float = 0x1.FP+2\nconst Decimal = 1E+3\n",
		"signatures": `package sample
type Name string
func Names(values ...Name) (result []Name) {return values}
func Consume(input interface{ Value() int }) int {return input.Value()}
`,
		"directives": `//go:build linux || darwin

// Package sample is available on selected platforms.
package sample

//go:noinline
func Value() int {return 1}
`,
	} {
		t.Run(name, func(t *testing.T) {
			expected, err := format.Source([]byte(source))
			if err != nil {
				t.Fatal(err)
			}
			set := token.NewFileSet()
			file, err := parser.ParseFile(set, "sample.go", source, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := formatPartitionFile(set, file)
			if err != nil || !bytes.Equal(expected, actual) {
				t.Fatalf("intermediate serialization changed canonical source: %v\nexpected:\n%s\nactual:\n%s", err, expected, actual)
			}
		})
	}
}
