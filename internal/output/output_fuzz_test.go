package output

import (
	"path/filepath"
	"strings"
	"testing"
)

func FuzzSafeArtifactPath(f *testing.F) {
	for _, seed := range []string{
		"client/index.ts",
		"./client/../types.ts",
		"../escape.ts",
		"../../escape.ts",
		"/absolute.ts",
		".",
		"",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, value string) {
		clean := filepath.Clean(filepath.FromSlash(value))
		unsafe := clean == "." ||
			filepath.IsAbs(clean) ||
			clean == ".." ||
			strings.HasPrefix(clean, ".."+string(filepath.Separator))

		got, err := SafeArtifactPath(value)
		if unsafe {
			if err == nil {
				t.Fatalf("unsafe artifact path %q was accepted as %q", value, got)
			}
			return
		}
		if err != nil {
			t.Fatalf("safe artifact path %q was rejected: %v", value, err)
		}
		if got != clean {
			t.Fatalf("SafeArtifactPath(%q) = %q, want normalized %q", value, got, clean)
		}
	})
}
