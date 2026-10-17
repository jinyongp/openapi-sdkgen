package typescript

import (
	"bytes"
	"testing"
)

func TestWriteJSDoc(t *testing.T) {
	for _, test := range []struct {
		name   string
		indent string
		layout jsDocLayout
		lines  []string
		want   string
	}{
		{
			name: "inline wildcard", indent: "  ", layout: jsDocInline,
			lines: []string{`Type: { readonly contentType: "*/*" }`},
			want:  "  /** Type: { readonly contentType: \"* /*\" } */\n",
		},
		{
			name: "single line block", layout: jsDocBlock,
			lines: []string{"Description\nwith  spacing */"},
			want:  "/**\n * Description with spacing * /\n */\n",
		},
		{
			name: "nested list and empty line", indent: "  ", layout: jsDocBlock,
			lines: []string{"Response", "", "  - Header `X-Value` — text */"},
			want:  "  /**\n   * Response\n   *\n   *   - Header `X-Value` — text * /\n   */\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			writeJSDoc(&output, test.indent, test.layout, test.lines...)
			if got := output.String(); got != test.want {
				t.Fatalf("JSDoc = %q, want %q", got, test.want)
			}
		})
	}
}
