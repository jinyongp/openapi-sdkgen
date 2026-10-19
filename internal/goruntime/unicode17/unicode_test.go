package unicode17

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func source(t *testing.T, name string) string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name+".gz"))
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
	return string(data)
}
func sourceRows(t *testing.T, name string) [][]string {
	t.Helper()
	var rows [][]string
	for line := range strings.SplitSeq(source(t, name), "\n") {
		line, _, _ = strings.Cut(line, "#")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "@") {
			continue
		}
		parts := strings.Split(line, ";")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		rows = append(rows, parts)
	}
	return rows
}
func point(t *testing.T, text string) rune {
	t.Helper()
	n, err := strconv.ParseInt(text, 16, 32)
	if err != nil {
		t.Fatal(err)
	}
	return rune(n)
}
func points(t *testing.T, text string) string {
	t.Helper()
	var r []rune
	for item := range strings.FieldsSeq(text) {
		r = append(r, point(t, item))
	}
	return string(r)
}
func bounds(t *testing.T, text string) (rune, rune) {
	t.Helper()
	first, last, ok := strings.Cut(text, "..")
	if !ok {
		last = first
	}
	return point(t, first), point(t, last)
}

func TestOfficialInputHashes(t *testing.T) {
	data, err := os.ReadFile("testdata/inputs.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Unicode string `json:"unicode"`
		Inputs  []struct {
			URL, SHA256 string
			Bytes       int
		}
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Unicode != Version || len(manifest.Inputs) != 15 {
		t.Fatal("incomplete input manifest")
	}
	for _, row := range manifest.Inputs {
		_, name, ok := strings.Cut(row.URL, "/17.0.0/")
		if !ok {
			t.Fatal(row.URL)
		}
		text := source(t, name)
		hash := sha256.Sum256([]byte(text))
		if len(text) != row.Bytes || hex.EncodeToString(hash[:]) != row.SHA256 {
			t.Fatal("fixture hash mismatch", name)
		}
	}
}
func TestOfficialNormalizationConformance(t *testing.T) {
	rows := sourceRows(t, "ucd/NormalizationTest.txt")
	if len(rows) < 19000 {
		t.Fatal("incomplete normalization corpus", len(rows))
	}
	for i, row := range rows {
		if len(row) < 5 {
			t.Fatal(i, row)
		}
		c := make([]string, 5)
		for j := range c {
			c[j] = points(t, row[j])
		}
		for j, value := range c {
			expected := c[1]
			if j >= 3 {
				expected = c[3]
			}
			if got := NFC(value); got != expected {
				t.Fatalf("row %d column %d NFC(%U)=%U want %U", i, j, []rune(value), []rune(got), []rune(expected))
			}
		}
	}
}
func TestOfficialSimpleCaseFolding(t *testing.T) {
	count := 0
	for _, row := range sourceRows(t, "ucd/CaseFolding.txt") {
		if row[1] != "C" && row[1] != "S" {
			continue
		}
		from, to := point(t, row[0]), point(t, row[2])
		if Fold(from) != to || Fold(to) != to {
			t.Fatalf("fold U+%04X", from)
		}
		count++
	}
	if count < 1400 {
		t.Fatal("incomplete folding corpus")
	}
	if Fold('İ') != 'İ' {
		t.Fatal("full or Turkic folding leaked into simple fold")
	}
}
func TestOfficialIDNAAndJoiningRanges(t *testing.T) {
	statuses := map[string]IDNAStatus{"valid": IDNAValid, "ignored": IDNAIgnored, "mapped": IDNAMapped, "deviation": IDNADeviation, "disallowed": IDNADisallowed, "disallowed_STD3_valid": IDNASTD3Valid, "disallowed_STD3_mapped": IDNASTD3Mapped}
	for _, row := range sourceRows(t, "idna/IdnaMappingTable.txt") {
		lo, hi := bounds(t, row[0])
		mapping := ""
		if len(row) > 2 {
			mapping = points(t, row[2])
		}
		expected, ok := statuses[row[1]]
		if !ok {
			t.Fatal(row[1])
		}
		for _, r := range []rune{lo, hi} {
			status, text := LookupIDNA(r)
			if status != expected || text != mapping {
				t.Fatalf("IDNA U+%04X: %d %q want %d %q", r, status, text, expected, mapping)
			}
		}
	}
	for _, row := range sourceRows(t, "ucd/extracted/DerivedJoiningType.txt") {
		lo, hi := bounds(t, row[0])
		if JoiningType(lo) != row[1] || JoiningType(hi) != row[1] {
			t.Fatal("joining type", row)
		}
	}
	if BidiClass('א') != "R" || BidiClass('ا') != "AL" || BidiClass('1') != "EN" || BidiClass('١') != "AN" {
		t.Fatal("bidi classes")
	}
}
func TestPropertyBoundariesAndAliases(t *testing.T) {
	cases := []struct {
		name, value string
		r           rune
		want        bool
	}{
		{"", "Letter", '가', true}, {"gc", "L", '가', true}, {"General_Category", "Letter", '1', false},
		{"Script", "Greek", 'α', true}, {"sc", "Grek", 'α', true}, {"Script_Extensions", "Hiragana", 'ー', true}, {"Script", "Hiragana", 'ー', false},
		{"", "Emoji", '😀', true}, {"", "ASCII", 127, true}, {"", "ASCII", 128, false}, {"", "White_Space", '\u00a0', true},
		{"", "Assigned", 0x0378, false}, {"", "Unassigned", 0x0378, true}, {"", "Surrogate", 0xd800, true},
		{"Script", "Tai_Yo", 0x1e6c0, true}, {"", "Any", 0x10ffff, true}, {"", "Any", 0x110000, false},
	}
	for _, c := range cases {
		p, ok := LookupProperty(c.name, c.value)
		if !ok || p.Contains(c.r) != c.want {
			t.Errorf("%s=%s U+%04X: known=%t match=%t want=%t", c.name, c.value, c.r, ok, p.Contains(c.r), c.want)
		}
	}
	for _, c := range [][2]string{{"Script", "Hrkt"}, {"Script_Extensions", "Katakana_Or_Hiragana"}, {"script", "Greek"}, {"Script", "greek"}, {"", "IsGreek"}, {"", "Basic_Emoji"}, {"Emoji", "Yes"}, {"", "lowercase_letter"}} {
		if _, ok := LookupProperty(c[0], c[1]); ok {
			t.Error("accepted non-ECMAScript alias", c)
		}
	}
	var zero Property
	if zero.Contains('a') || len(zero.AppendRanges(nil)) != 0 {
		t.Fatal("nonempty zero property")
	}
	p, _ := LookupProperty("", "ASCII")
	ranges := p.AppendRanges(nil)
	ranges[0].Lo = 100
	if !p.Contains(0) {
		t.Fatal("caller modified immutable property")
	}
}
func TestOfficialBinaryPropertyRangeEdges(t *testing.T) {
	for _, name := range []string{"ucd/PropList.txt", "ucd/DerivedCoreProperties.txt", "ucd/DerivedNormalizationProps.txt", "ucd/emoji/emoji-data.txt"} {
		for _, row := range sourceRows(t, name) {
			p, ok := LookupProperty("", row[1])
			if !ok {
				continue
			}
			lo, hi := bounds(t, row[0])
			if !p.Contains(lo) || !p.Contains(hi) {
				t.Fatalf("%s %s: missing boundary", name, row[1])
			}
		}
	}
}
func FuzzNFCIdempotence(f *testing.F) {
	for _, s := range []string{"", "가", "e\u0301", "\u1100\u1161\u11a8", "a\u0315\u0300"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		first := NFC(s)
		if NFC(first) != first {
			t.Fatal("NFC is not idempotent")
		}
	})
}
