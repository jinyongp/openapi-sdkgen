package typescript

import (
	"bytes"
	"fmt"
)

// emitTypedConstant keeps repeated declaration emission explicit at its call sites.
// prefix contains indentation and, when needed, the export modifier.
func emitTypedConstant(output *bytes.Buffer, prefix, name, typeName, value string) {
	fmt.Fprintf(output, "%sconst %s: %s = %s\n", prefix, name, typeName, value)
}
