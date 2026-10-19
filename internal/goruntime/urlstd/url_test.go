package urlstd

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"
)

type urlCase struct {
	Input                                                                                    string
	Base                                                                                     *string
	Failure                                                                                  bool
	Href, Origin, Protocol, Username, Password, Host, Hostname, Port, Pathname, Search, Hash string
}

func TestPinnedWPTURLs(t *testing.T) {
	for _, file := range []string{"urltestdata.json", "urltestdata-javascript-only.json"} {
		data, err := os.ReadFile("../../../testdata/goruntime/url/" + file)
		if err != nil {
			t.Fatal(err)
		}
		var entries []json.RawMessage
		if err := json.Unmarshal(data, &entries); err != nil {
			t.Fatal(err)
		}
		for i, entry := range entries {
			if len(entry) == 0 || entry[0] != '{' {
				continue
			}
			var c urlCase
			if err := json.Unmarshal(entry, &c); err != nil {
				t.Fatal(err)
			}
			var u *URL
			if c.Base == nil {
				u, err = Parse(c.Input)
			} else {
				u, err = ParseRef(*c.Base, c.Input)
			}
			if (err != nil) != c.Failure {
				t.Errorf("%s[%d] input=%q base=%v error=%v expected failure=%t", file, i, c.Input, c.Base, err, c.Failure)
				continue
			}
			if err != nil {
				continue
			}
			got := []string{u.Href(false), u.Origin(), u.Protocol(), u.Username(), u.Password(), u.Host(), u.Hostname(), u.Port(), u.Pathname(), u.Search(), u.Hash()}
			want := []string{c.Href, c.Origin, c.Protocol, c.Username, c.Password, c.Host, c.Hostname, c.Port, c.Pathname, c.Search, c.Hash}
			names := []string{"href", "origin", "protocol", "username", "password", "host", "hostname", "port", "pathname", "search", "hash"}
			for field := range got {
				if names[field] == "origin" && c.Origin == "" {
					continue
				}
				if got[field] != want[field] {
					t.Errorf("%s[%d] %q %s=%q want %q", file, i, c.Input, names[field], got[field], want[field])
				}
			}
			round, err := Parse(u.Href(false))
			if err != nil || round.Href(false) != u.Href(false) {
				t.Errorf("non-idempotent %s: %v", u.Href(false), err)
			}
		}
	}
}
func TestPinnedWPTDomainToASCII(t *testing.T) {
	overrideData, err := os.ReadFile("../../../testdata/goruntime/url/unicode17-overrides.json")
	if err != nil {
		t.Fatal(err)
	}
	var overrides struct {
		SourceSHA256 string
		Overrides    []struct {
			Index       int
			InputSHA256 string
		}
	}
	if err := json.Unmarshal(overrideData, &overrides); err != nil {
		t.Fatal(err)
	}
	used := 0

	for _, file := range []string{"toascii.json", "IdnaTestV2.json", "IdnaTestV2-removed.json"} {
		data, err := os.ReadFile("../../../testdata/goruntime/url/" + file)
		if err != nil {
			t.Fatal(err)
		}
		if file == "IdnaTestV2.json" {
			hash := sha256.Sum256(data)
			if hex.EncodeToString(hash[:]) != overrides.SourceSHA256 {
				t.Fatal("override source changed")
			}
		}
		var entries []json.RawMessage
		if err := json.Unmarshal(data, &entries); err != nil {
			t.Fatal(err)
		}
		for i, entry := range entries {
			if len(entry) == 0 || entry[0] != '{' {
				continue
			}
			var c struct {
				Input  string
				Output *string
			}
			if err := json.Unmarshal(entry, &c); err != nil {
				t.Fatal(err)
			}
			if file == "IdnaTestV2.json" {
				for _, override := range overrides.Overrides {
					if override.Index == i {
						hash := sha256.Sum256([]byte(c.Input))
						if hex.EncodeToString(hash[:]) != override.InputSHA256 {
							t.Fatal("override input changed")
						}
						c.Output = nil
						used++
					}
				}
			}
			got, err := parseHost(c.Input, false)
			if c.Output == nil {
				if err == nil {
					t.Errorf("%s[%d] %q accepted %q", file, i, c.Input, got)
				}
				continue
			}
			if err != nil || got != *c.Output {
				t.Errorf("%s[%d] %q got %q %v want %q", file, i, c.Input, got, err, *c.Output)
			}
		}
	}
	if used != len(overrides.Overrides) || used != 7 {
		t.Fatal("unapplied IDNA override", used)
	}
}

func TestOfficialUnicode17IDNA(t *testing.T) {
	f, err := os.Open("../unicode17/testdata/idna/IdnaTestV2.txt.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	data, err := io.ReadAll(z)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for lineNumber, line := range strings.Split(string(data), "\n") {
		line, _, _ = strings.Cut(line, "#")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, ";")
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		decode := func(s string) string {
			if s == `""` {
				return ""
			}
			var out string
			if err := json.Unmarshal([]byte(`"`+s+`"`), &out); err != nil {
				t.Fatal(lineNumber, err)
			}
			return out
		}
		input := decode(fields[0])
		unicode := input
		if fields[1] != "" {
			unicode = decode(fields[1])
		}
		want := unicode
		if fields[3] != "" {
			want = decode(fields[3])
		}
		statuses := fields[4]
		if statuses == "" {
			statuses = fields[2]
		}
		invalid := false
		for _, status := range strings.Split(strings.Trim(statuses, "[]"), ",") {
			switch strings.TrimSpace(status) {
			case "", "V2", "V3", "A4_1", "A4_2", "U1":
			default:
				invalid = true
			}
		}
		if ascii(input) {
			invalid = false
			want = lower(input)
		}
		got, err := DomainToASCII(input)
		count++
		if invalid {
			if err == nil {
				t.Errorf("line %d %q accepted %q statuses %s", lineNumber+1, input, got, statuses)
			}
			continue
		}
		if err != nil || got != want {
			t.Errorf("line %d %q: %q %v want %q statuses %s", lineNumber+1, input, got, err, want, statuses)
		}
	}
	if count < 6000 {
		t.Fatal("incomplete official corpus", count)
	}
	t.Log("official IDNA cases", count)
}

func TestIndependentURLContracts(t *testing.T) {
	cases := []struct {
		input, base, href, origin string
		invalid                   bool
	}{
		{"http://0x7f.1:80/a/../b", "", "http://127.0.0.1/b", "http://127.0.0.1", false},
		{"https://BÜCHER.example:443/a%2fb", "", "https://xn--bcher-kva.example/a%2fb", "https://xn--bcher-kva.example", false},
		{"../c?x#f", "https://example.test/a/b", "https://example.test/c?x#f", "https://example.test", false},
		{"http://example.test:65536", "", "", "", true},
		{"https://example.test%2fevil.test/", "", "", "", true},
		{"https://[::ffff:127.0.0.1]/", "", "https://[::ffff:7f00:1]/", "https://[::ffff:7f00:1]", false},
		{"https://[::1%25zone]/", "", "", "", true},
		{"mailto:user@example.test?subject=x", "", "mailto:user@example.test?subject=x", "null", false},
		{"blob:https://example.test/id", "", "blob:https://example.test/id", "https://example.test", false},
		{"file:///C:/a/../b", "", "file:///C:/b", "null", false},
	}
	for _, c := range cases {
		var u *URL
		var err error
		if c.base == "" {
			u, err = Parse(c.input)
		} else {
			u, err = ParseRef(c.base, c.input)
		}
		if (err != nil) != c.invalid {
			t.Fatal(c.input, err)
		}
		if err != nil {
			continue
		}
		if u.Href(false) != c.href || u.Origin() != c.origin {
			t.Errorf("%q got %s origin %s", c.input, u.Href(false), u.Origin())
		}
	}
	base, err := Parse("https://example.test/a/b?secret#fragment")
	if err != nil {
		t.Fatal(err)
	}
	before := base.Href(false)
	group := sync.WaitGroup{}
	for range 32 {
		group.Go(func() {
			for range 20 {
				u, err := base.Parse("../other")
				if err != nil || u.Href(false) != "https://example.test/other" {
					t.Error(u, err)
				}
			}
		})
	}
	group.Wait()
	if base.Href(false) != before {
		t.Fatal("relative parsing mutated base")
	}
}
func FuzzURLRoundTrip(f *testing.F) {
	for _, s := range []string{"https://example.test/a", "http://0x7f.1", "file:///C:/a", "non-special:opaque  ?query", "https://faß.de/"} {
		f.Add(s, "https://base.test/a/b")
	}
	f.Fuzz(func(t *testing.T, input, base string) {
		if len(input) > 4096 || len(base) > 4096 {
			t.Skip()
		}
		u, err := ParseRef(base, input)
		if err != nil {
			return
		}
		got, err := Parse(u.Href(false))
		if err != nil || got.Href(false) != u.Href(false) {
			t.Fatalf("not idempotent %q: %v", u.Href(false), err)
		}
	})
}
func FuzzPunycode(f *testing.F) {
	for _, s := range []string{"bücher", "한국", "faß", "a-é", "😀"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 2048 || !utf8.ValidString(value) || ascii(value) {
			t.Skip()
		}
		encoded, err := encodePunycode(value)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodePunycode(encoded)
		if err != nil || decoded != value {
			t.Fatalf("%q encoded %q decoded %q: %v", value, encoded, decoded, err)
		}
	})
}

func TestPinnedWPTUTF8PercentEncoding(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/goruntime/url/percent-encoding.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if len(entry) == 0 || entry[0] != '{' {
			continue
		}
		var c struct {
			Input  string
			Output map[string]string
		}
		if err := json.Unmarshal(entry, &c); err != nil {
			t.Fatal(err)
		}
		want, ok := c.Output["utf-8"]
		if !ok {
			t.Fatal("missing UTF8 expectation")
		}
		if got := percent(c.Input, encodeSpecialQuery); got != want {
			t.Errorf("%q => %q want %q", c.Input, got, want)
		}
	}
}
