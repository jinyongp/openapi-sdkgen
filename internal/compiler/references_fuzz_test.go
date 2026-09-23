package sdkgen

import (
	"net/url"
	"strings"
	"testing"
)

func FuzzJSONPointerTokenRoundTrip(f *testing.F) {
	for _, seed := range []string{"name", "a/b", "a~b", "~", "/", "~0", "~1", ""} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, token string) {
		pointer := appendSchemaPointer("", token)
		encoded := strings.TrimPrefix(pointer, "/")
		decoded, err := decodeJSONPointerToken(encoded)
		if err != nil {
			t.Fatalf("encoded token %q from %q was rejected: %v", encoded, token, err)
		}
		if decoded != token {
			t.Fatalf("JSON Pointer round trip = %q, want %q (encoded %q)", decoded, token, encoded)
		}

		// Arbitrary tokens may be malformed JSON Pointer escapes; decoding must
		// terminate cleanly regardless of whether the input is accepted.
		_, _ = decodeJSONPointerToken(token)
	})
}

func FuzzRemoteReferenceURLSyntax(f *testing.F) {
	for _, seed := range []string{
		"https://example.test/schema.json",
		"https://EXAMPLE.test:443/schema.json",
		"http://trusted.test/schema.json",
		"https://user:secret@example.test/schema.json",
		"ftp://example.test/schema.json",
		"not-a-url",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		resolver := &remoteReferenceResolver{
			origins:       map[string]struct{}{"https://example.test": {}},
			trustedOrigin: "http://trusted.test",
		}

		value, err := resolver.validateURLSyntax(raw)
		if err != nil {
			return
		}
		if value == nil || value.User != nil || value.Host == "" {
			t.Fatalf("accepted remote reference has unsafe URL shape: %q -> %#v", raw, value)
		}
		if !resolver.isTrustedURL(value) && value.Scheme != "https" {
			t.Fatalf("untrusted remote reference accepted non-HTTPS scheme: %q", raw)
		}

		origin, originErr := canonicalRemoteOrigin(value.Scheme + "://" + value.Host)
		if originErr == nil {
			parsed, parseErr := url.Parse(origin)
			if parseErr != nil || parsed.Scheme != "https" || parsed.User != nil ||
				parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
				t.Fatalf("canonical remote origin is not an exact HTTPS origin: %q", origin)
			}
		}
	})
}
