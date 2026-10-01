package sdkgen

import "testing"

func TestJSONPointerTokenDecoding(t *testing.T) {
	for _, sample := range []struct{ encoded, decoded string }{
		{"", ""}, {"users", "users"}, {"이름😀", "이름😀"},
		{"a~1b", "a/b"}, {"a~0b", "a~b"}, {"~01", "~1"},
		{"~1~0~1", "/~/"}, {"prefix~0suffix", "prefix~suffix"},
	} {
		got, err := decodeJSONPointerToken(sample.encoded)
		if err != nil || got != sample.decoded {
			t.Fatalf("decode %q = %q, %v; want %q", sample.encoded, got, err, sample.decoded)
		}
	}
	for _, invalid := range []string{"~", "suffix~", "~2", "~00~x", "~1~"} {
		if _, err := decodeJSONPointerToken(invalid); err == nil {
			t.Fatalf("accepted invalid escape %q", invalid)
		}
	}
}

func TestUnescapedJSONPointerTokenNeedsNoAllocation(t *testing.T) {
	const token = "microsoft.graph.directoryObject"
	if allocations := testing.AllocsPerRun(100, func() {
		got, err := decodeJSONPointerToken(token)
		if err != nil || got != token {
			t.Fatal("unescaped token changed")
		}
	}); allocations != 0 {
		t.Fatalf("unescaped token allocated %g times", allocations)
	}
}
