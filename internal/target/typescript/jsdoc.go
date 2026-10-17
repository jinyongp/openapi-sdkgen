package typescript

import (
	"bytes"
	"fmt"
	"strings"
)

type jsDocLayout uint8

const (
	jsDocInline jsDocLayout = iota
	jsDocBlock
)

// writeJSDoc owns comment escaping and delimiters. Each body entry is one logical
// line; leading whitespace preserves nested lists and example indentation.
func writeJSDoc(output *bytes.Buffer, indent string, layout jsDocLayout, lines ...string) {
	body := make([]string, len(lines))
	for index, line := range lines {
		leading := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		text := leading + strings.Join(strings.Fields(line), " ")
		body[index] = strings.ReplaceAll(text, "*/", "* /")
	}
	if layout == jsDocInline {
		fmt.Fprintf(output, "%s/** %s */\n", indent, strings.Join(body, " "))
		return
	}
	fmt.Fprintf(output, "%s/**\n", indent)
	for _, line := range body {
		if line == "" {
			fmt.Fprintf(output, "%s *\n", indent)
		} else {
			fmt.Fprintf(output, "%s * %s\n", indent, line)
		}
	}
	fmt.Fprintf(output, "%s */\n", indent)
}
