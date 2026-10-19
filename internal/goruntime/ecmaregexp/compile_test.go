package ecmaregexp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCompileSyntaxContract(t *testing.T) {
	cases := []struct {
		source string
		valid  bool
	}{
		{"", true}, {"a|", true}, {"|a", true}, {"[]", true}, {"[^]", true}, {"[a-b-c]", true}, {"[\\d-a]", false}, {"[z-a]", false},
		{"a{", false}, {"a}", false}, {"a]", false}, {"{1}", false}, {"a{,2}", false}, {"a{2,1}", false}, {"a{01,2}", true},
		{"a{999999999999999999999999999999999,100000000000000000000}", false}, {"a{999999999999999999999999999999999,}", true},
		{"\\p{=Letter}", false}, {"(?i-:a)", true}, {"a**", false}, {"a+?", true}, {"^+", false}, {"(?=a)+", false}, {"(?<=a){2}", false}, {"(?:a+){2}", true}, {"(?:){2}", true},
		{"(?<a>x)|(?<a>y)", true}, {"(?<a>x)(?<a>y)", false}, {"(?:(?<a>x)|y)(?:(?<a>y)|z)", false},
		{"(?<a>x)|(?:(?<a>y)|(?<a>z))", true}, {"(?<a>x)|(?<a>y)\\k<a>", true}, {"(?<a>x)|\\k<missing>", false},
		{"\\1(a)", true}, {"\\2(a)", false}, {"\\0", true}, {"\\01", false}, {"\\09", false}, {"[\\1]", false},
		{"\\a", false}, {"\\-", false}, {"[\\-]", true}, {"\\cA", true}, {"\\c1", false}, {"\\u{1f600}", true},
		{"\\u{}", false}, {"\\u{110000}", false}, {"\\p{Script=Greek}", true}, {"\\p{Script=greek}", false}, {"\\p{Hrkt}", false},
		{"(?i:a)", true}, {"(?i-m:a)", true}, {"(?-i:a)", true}, {"(?i-i:a)", false}, {"(?ii:a)", false}, {"(?i)", false}, {"(?-:a)", false},
		{"(?<\\u0061>x)\\k<a>", true}, {"(?<1>x)", false}, {"(?<가>x)", true}, {"(?<😀>x)", false},
	}
	for _, c := range cases {
		_, err := Compile(c.source)
		if (err == nil) != c.valid {
			t.Errorf("Compile(%q): %v, valid=%t", c.source, err, c.valid)
		}
	}
}
func TestCompileAgainstNodeSyntaxOracle(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("supplementary Node syntax oracle unavailable")
	}
	var existing []struct{ Name, Pattern, Input string }
	data, err := os.ReadFile("../../target/golang/testdata/regex-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &existing); err != nil {
		t.Fatal(err)
	}
	sources := make([]string, 0, len(existing)+1000)
	for _, c := range existing {
		sources = append(sources, c.Pattern)
	}
	atoms := []string{"a", ".", "[]", "[^]", "[a-z]", "[\\W]", "\\p{Script=Greek}", "\\P{Lowercase_Letter}", "(?i:a)", "(?<a>a)", "(?=a)", "(?<=a)", "(?:a+)", "\\uD83D\\uDE00"}
	suffixes := []string{"", "*", "+", "?", "??", "{0}", "{2,}", "{2,1}", "{1,3}?", "+*", "|b", "}", ")"}
	for _, a := range atoms {
		for _, b := range suffixes {
			sources = append(sources, a+b)
		}
	}
	data, err = json.Marshal(sources)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(node, "-e", `const fs=require('node:fs');console.log(JSON.stringify(JSON.parse(fs.readFileSync(0,'utf8')).map(p=>{try{new RegExp(p,'u');return true}catch{return false}})))`)
	command.Stdin = strings.NewReader(string(data))
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Node: %v %s", err, output)
	}
	var expected []bool
	if err := json.Unmarshal(output, &expected); err != nil {
		t.Fatal(err)
	}
	if len(expected) != len(sources) {
		t.Fatal("incomplete oracle")
	}
	for i, s := range sources {
		_, err := Compile(s)
		if (err == nil) != expected[i] {
			t.Errorf("Compile(%q): %v, Node valid=%t", s, err, expected[i])
		}
	}
}
func TestCompilerDepthAndCancellation(t *testing.T) {
	if _, err := Compile(strings.Repeat("(?:", MaxNesting) + "a" + strings.Repeat(")", MaxNesting)); err != nil {
		t.Fatal("valid deep pattern", err)
	}
	if _, err := Compile(strings.Repeat("(", MaxNesting+1)); !errors.Is(err, ErrComplexity) {
		t.Fatal("lost compile complexity limit", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CompileContext(ctx, "a"); !errors.Is(err, context.Canceled) {
		t.Fatal("lost compile cancellation", err)
	}
}
func FuzzCompile(f *testing.F) {
	for _, s := range []string{"", "a", "[a-z]", "(?<=a)b", "(?<x>a)|(?<x>b)", "\\p{Letter}", "(?i:a{2})"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 32768 {
			t.Skip()
		}
		_, _ = Compile(s)
	})
}

func TestPinnedTest262ConstructorSyntax(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/goruntime/regexp-syntax.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Pattern string
			Units   []uint16
			Valid   bool
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) == 0 {
		t.Fatal("empty syntax corpus")
	}
	for _, c := range corpus.Cases {
		var encoded []byte
		for _, unit := range c.Units {
			r := rune(unit)
			if r >= 0xd800 && r <= 0xdfff {
				encoded = append(encoded, byte(0xe0|r>>12), byte(0x80|r>>6&63), byte(0x80|r&63))
			} else {
				encoded = utf8.AppendRune(encoded, r)
			}
		}
		_, err := Compile(string(encoded))
		if (err == nil) != c.Valid {
			t.Errorf("Compile(%q): %v, expected valid=%t", c.Pattern, err, c.Valid)
		}
	}
}

var compiledBenchmark *Program

func BenchmarkCompileRepeatedProperties(b *testing.B) {
	source := strings.Repeat(`\p{Letter}`, 1000)
	b.ReportAllocs()
	for b.Loop() {
		program, err := Compile(source)
		if err != nil {
			b.Fatal(err)
		}
		compiledBenchmark = program
	}
	b.ReportMetric(float64(len(compiledBenchmark.Ranges)), "ranges")
}
