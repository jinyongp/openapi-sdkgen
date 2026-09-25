package typescript

import (
	"regexp"
	"testing"
)

// Check the binding's exact imported target rather than its incidental spelling.
func generatedTypeImportAlias(t testing.TB, source, specifier, exportName string) string {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^import type \{ ` + regexp.QuoteMeta(exportName) + ` as ([A-Za-z_$][A-Za-z0-9_$]*) \} from ` + regexp.QuoteMeta(quoteTS(specifier)) + `$`)
	matches := pattern.FindAllStringSubmatch(source, -1)
	if len(matches) != 1 {
		t.Fatalf("expected one %s import from %q, got %d:\n%s", exportName, specifier, len(matches), source)
	}
	return matches[0][1]
}
