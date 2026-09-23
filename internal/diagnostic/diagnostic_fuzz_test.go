package diagnostic

import (
	"net/url"
	"strings"
	"testing"
)

func FuzzSafeSourceDisplay(f *testing.F) {
	for _, seed := range []string{
		"https://user:secret@example.test/openapi.yaml?token=secret#fragment",
		"http://example.test/openapi.json?signature=value",
		"HTTPS://name:password@example.test/path#secret",
		"https://user:secret@example.test/%zz?token=secret#fragment",
		"not-a-url?token=preserved",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, source string) {
		got := SafeSourceDisplay(source)
		value, err := url.Parse(source)
		if err == nil && value.Scheme != "" && value.Host != "" {
			sanitized := *value
			sanitized.User = nil
			sanitized.RawQuery = ""
			sanitized.ForceQuery = false
			sanitized.Fragment = ""
			if want := sanitized.String(); got != want {
				t.Fatalf("SafeSourceDisplay(%q) = %q, want %q", source, got, want)
			}
			return
		}

		lower := strings.ToLower(source)
		if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
			return
		}
		if strings.ContainsAny(got, "?#") {
			t.Fatalf("malformed HTTP(S) display retained query/fragment delimiter: %q -> %q", source, got)
		}
		schemeEnd := strings.Index(got, "://") + 3
		authority := got[schemeEnd:]
		if slash := strings.IndexByte(authority, '/'); slash >= 0 {
			authority = authority[:slash]
		}
		if strings.Contains(authority, "@") {
			t.Fatalf("malformed HTTP(S) display retained credentials: %q -> %q", source, got)
		}
	})
}
