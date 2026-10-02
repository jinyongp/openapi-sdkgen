package typescript

import (
	"strings"
	"testing"
)

func TestGeneratedTypeImportsUseLocalizedTypes(t *testing.T) {
	source := `import type { Used, Unused, Original as Alias } from "./types.js"
import { bind, type Needed, type Absent } from "./runtime.js"
import type * as Ignored from "./schemas.js"
// Unused and Absent appear in documentation only.
export type Output = Used & Alias & Needed
export const literal = "Unused Ignored Absent"
export const operation = bind()
`
	actual := generatedTypeImports(source)
	for _, expected := range []string{`import type { Used, Original as Alias }`, `import { bind, type Needed }`, `export const literal = "Unused Ignored Absent"`} {
		if !strings.Contains(actual, expected) {
			t.Fatalf("missing %q in %s", expected, actual)
		}
	}
	if strings.Contains(actual, "type * as Ignored") || strings.Contains(actual, "Used, Unused") || strings.Contains(actual, "type Absent") {
		t.Fatalf("unused type import retained: %s", actual)
	}
	if repeated := generatedTypeImports(actual); repeated != actual {
		t.Fatal("import plan is not stable")
	}
}
