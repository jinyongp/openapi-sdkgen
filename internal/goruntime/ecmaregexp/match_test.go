package ecmaregexp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

type matchSample struct{ Pattern, Input string }
type matchExpected struct{ Valid, Matches bool }

func compareMatches(t *testing.T, cases []matchSample) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("supplementary Node matcher oracle unavailable")
	}
	data, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, node, "-e", `const fs=require('node:fs');console.log(JSON.stringify(JSON.parse(fs.readFileSync(0,'utf8')).map(c=>{try{return {Valid:true,Matches:new RegExp(c.Pattern,'u').test(c.Input)}}catch{return {Valid:false,Matches:false}}})))`)
	command.Stdin = strings.NewReader(string(data))
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Node oracle: %v %s", err, output)
	}
	var expected []matchExpected
	if err := json.Unmarshal(output, &expected); err != nil {
		t.Fatal(err)
	}
	if len(cases) != len(expected) {
		t.Fatal("incomplete oracle")
	}
	for i, c := range cases {
		program, err := Compile(c.Pattern)
		want := expected[i]
		// ECMA-262 Canonicalize + UpdateModifiers and the previous engine agree
		// that scoped Unicode folding maps Kelvin to k. Node 24.22 disagrees
		// for this literal; preserve the normative expectation explicitly.
		if c.Pattern == `(?i:\bK\b)` && (c.Input == "k" || c.Input == "K") {
			if !want.Matches {
				t.Log("recorded Node scoped Kelvin folding discrepancy", c.Input)
			}
			want.Matches = true
		}
		if (err == nil) != want.Valid {
			t.Errorf("compile %q: %v valid=%t", c.Pattern, err, want.Valid)
			continue
		}
		if !want.Valid {
			continue
		}
		got, err := program.Match(context.Background(), c.Input)
		if err != nil || got != want.Matches {
			t.Errorf("%q on %q: %t %v want %t", c.Pattern, c.Input, got, err, want.Matches)
		}
	}
}
func TestExistingMatcherOracle(t *testing.T) {
	data, err := os.ReadFile("../../target/golang/testdata/regex-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []matchSample
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	compareMatches(t, cases)
}
func TestMatcherCapturesAssertionsAndFlags(t *testing.T) {
	patterns := []string{
		``, `a|ab`, `^a$`, `(?m:^a$)`, `(?s:^.$)`, `(?i:^k$)`, `(?i:^s$)`, `(?i:^\w$)`, `(?i:^\W$)`, `(?i:^[^\W]$)`,
		`(?i:^\p{Lowercase_Letter}$)`, `(?i:^\P{Lowercase_Letter}$)`, `(?i:^[^\p{Lowercase_Letter}]$)`, `(?i:k(?-i:K))`,
		`a*ab`, `a*?ab`, `a+ab`, `a+?ab`, `a{2,3}a`, `a{2,3}?a`, `(?:a+){2}`, `(?:(?:a))*a`,
		`(|a){2}\1$`, `(a|ab)+b\1`, `(a|(b))*\2c`, `^(a|(b))\2c$`, `^(?:(a)|(b))*\1$`,
		`^(?<first>a)(b)\2$`, `^\1(a)$`, `^(a)?\1$`, `(?<x>a)|(?<x>b)\k<x>`,
		`(?<=(a)\1)b`, `(?<=\1(a))b`, `(?<=([ab]+)([bc]+))$`, `(?<=(\w)+)\1`,
		`(?=(a+))a*b\1`, `(?!(a))b\1`, `a(?!(b))\1c`, `(?=(a|ab))\1c`,
		`(?<=(?=a)a)b`, `(?<=(a|b)+)c`, `(?<!a)b`, `(?<!(a))b\1`,
		`(?:)*`, `()+`, `(?:|a)*b`, `(a*)*b`, `(a?){2}a\1`, `(a*){2}\1$`,
		`\b`, `\B`, `(?i:\bK\b)`, `^\p{Cs}$`, `^\uD83D\uDE00$`, `^\u{D83D}\u{DE00}$`,
	}
	inputs := []string{"", "a", "aa", "aaa", "aaaa", "ab", "aab", "aaba", "abb", "abc", "ac", "b", "bb", "bc", "c", "go-go", "k", "K", "K", "ſ", "s", "S", "a\n", "a\r\n", "\u00a0", "\u2028", "😀", "α", "한글"}
	cases := make([]matchSample, 0, len(patterns)*len(inputs))
	for _, pattern := range patterns {
		for _, input := range inputs {
			cases = append(cases, matchSample{Pattern: pattern, Input: input})
		}
	}
	compareMatches(t, cases)
}
func TestMatcherIndependentContracts(t *testing.T) {
	cases := []struct {
		pattern, input string
		want           bool
	}{
		{`(|a){2}\1$`, "aa", true}, {`(?=(a|ab))\1c`, "abc", false}, {`(?<=([ab]+)([bc]+))$`, "abc", true},
		{`(?i:^\W$)`, "K", false}, {`(?i:^\P{Lowercase_Letter}$)`, "a", true}, {`(?i:^[^\p{Lowercase_Letter}]$)`, "a", false},
		{`(?i:\bK\b)`, "k", true}, {`(?i:ſ)`, "s", true}, {`a$`, "a\n", false}, {`(?m:a$)`, "a\n", true}, {`^.$`, "😀", true}, {`^..$`, "😀", false},
		{`^\uD800$`, string([]byte{0xed, 0xa0, 0x80}), true}, {`^\p{Cs}$`, string([]byte{0xed, 0xa0, 0x80}), true},
		{`^😀$`, string([]byte{0xed, 0xa0, 0xbd, 0xed, 0xb8, 0x80}), true},
	}
	for _, c := range cases {
		p, err := Compile(c.pattern)
		if err != nil {
			t.Fatal(err)
		}
		got, err := p.Match(context.Background(), c.input)
		if err != nil || got != c.want {
			t.Errorf("%q on %q: %t %v want %t", c.pattern, c.input, got, err, c.want)
		}
	}
}
func TestMatcherBudgetsCancellationAndReuse(t *testing.T) {
	p, err := Compile(`^(a+)+$`)
	if err != nil {
		t.Fatal(err)
	}
	if matched, err := p.match(context.Background(), strings.Repeat("a", 24)+"!", 10000); matched || !errors.Is(err, ErrComplexity) {
		t.Fatal("lost execution limit", matched, err)
	}
	if ok, err := p.Match(context.Background(), "aaa"); err != nil || !ok {
		t.Fatal("program poisoned after budget failure", ok, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Match(ctx, "aaa"); !errors.Is(err, context.Canceled) {
		t.Fatal("lost cancellation", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if _, err := p.Match(ctx, strings.Repeat("a", 32)+"!"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("long match ignored cancellation", err)
	}
	if ok, err := p.Match(context.Background(), "aaa"); err != nil || !ok {
		t.Fatal("program poisoned after canceled match", ok, err)
	}
}
func TestConcurrentMatcherAndLargeSimpleRepeat(t *testing.T) {
	p, err := Compile(`(?i:^(?<word>\p{L}+)-\k<word>$)`)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for range 16 {
		group.Go(func() {
			for range 100 {
				if ok, err := p.Match(context.Background(), "Go-go"); err != nil || !ok {
					t.Error(ok, err)
				}
				if ok, err := p.Match(context.Background(), "Go-no"); err != nil || ok {
					t.Error(ok, err)
				}
			}
		})
	}
	group.Wait()
	p, err = Compile(`^[a-z]+$`)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := p.Match(context.Background(), strings.Repeat("a", 1<<20)); err != nil || !ok {
		t.Fatal("large homogeneous repeat", ok, err)
	}
}
func FuzzMatcher(f *testing.F) {
	for _, c := range []matchSample{{"a", "a"}, {"a*ab", "aaab"}, {`(|a){2}\1$`, "aa"}, {"(?<=a)b", "ab"}, {"(?<x>a)|(?<x>b)", "b"}, {`\p{Letter}`, "α"}} {
		f.Add(c.Pattern, c.Input)
	}
	f.Fuzz(func(t *testing.T, pattern, input string) {
		if len(pattern) > 512 || len(input) > 256 {
			t.Skip()
		}
		p, err := Compile(pattern)
		if err != nil {
			return
		}
		_, _ = p.match(context.Background(), input, 10000)
	})
}

var matchedBenchmark bool

func BenchmarkMatchASCII(b *testing.B) {
	p, err := Compile(`^[a-z]+$`)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		matchedBenchmark, err = p.Match(context.Background(), "sdkgen")
		if err != nil || !matchedBenchmark {
			b.Fatal(err)
		}
	}
}

func TestScratchReleasesRequestReferencesAndGrowth(t *testing.T) {
	p, err := Compile("a")
	if err != nil {
		t.Fatal(err)
	}
	scratch := new(matchScratch)
	scratch.state = machine{program: p, input: strings.Repeat("private", 1000), context: context.WithValue(context.Background(), struct{}{}, "private"), tasks: make([]task, 2000), choices: make([]choice, 2000), journal: make([]change, 2000), captures: make([]int, 2000)}
	resetScratch(scratch)
	state := scratch.state
	if state.program != nil || state.input != "" || state.context != nil || state.tasks != nil || state.choices != nil || state.journal != nil || state.captures != nil {
		t.Fatal("scratch retained request references or grown buffers")
	}
}

func wireUnits(units []uint16) string {
	var encoded []byte
	for _, unit := range units {
		r := rune(unit)
		if r >= 0xd800 && r <= 0xdfff {
			encoded = append(encoded, byte(0xe0|r>>12), byte(0x80|r>>6&63), byte(0x80|r&63))
		} else {
			encoded = utf8.AppendRune(encoded, r)
		}
	}
	return string(encoded)
}

func TestPinnedTest262BooleanMatches(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/goruntime/regexp-matches.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Pattern, Input           string
			PatternUnits, InputUnits []uint16
			Matches                  bool
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) == 0 {
		t.Fatal("empty matching corpus")
	}
	for _, c := range corpus.Cases {
		p, err := Compile(wireUnits(c.PatternUnits))
		if err != nil {
			t.Fatalf("%q: %v", c.Pattern, err)
		}
		got, err := p.Match(context.Background(), wireUnits(c.InputUnits))
		if err != nil || got != c.Matches {
			t.Errorf("%q on %q: %t %v want %t", c.Pattern, c.Input, got, err, c.Matches)
		}
	}
}

func TestMatcherComposedOracle(t *testing.T) {
	atoms := []string{`a`, `b`, ``, `[ab]`, `(a)`, `(b?)`, `(?:a|ab)`, `(?:|a)`, `(?=a)`, `(?!a)`, `(?<=a)`, `(?<!b)`}
	suffixes := []string{``, `?`, `*`, `+`, `{2}`, `{0,3}?`}
	inputs := []string{""}
	for length := 1; length <= 4; length++ {
		for bits := 0; bits < 1<<length; bits++ {
			var s strings.Builder
			for bit := 0; bit < length; bit++ {
				if bits&(1<<bit) == 0 {
					s.WriteByte('a')
				} else {
					s.WriteByte('b')
				}
			}
			inputs = append(inputs, s.String())
		}
	}
	var cases []matchSample
	for _, left := range atoms {
		for _, right := range atoms {
			for _, suffix := range suffixes {
				for _, input := range inputs {
					cases = append(cases, matchSample{Pattern: `^(?:` + left + right + `)` + suffix + `\1?$`, Input: input})
				}
			}
		}
	}
	compareMatches(t, cases)
}

func TestMatcherExplicitStacks(t *testing.T) {
	p, err := Compile(strings.Repeat("(?:", MaxNesting) + "a" + strings.Repeat(")", MaxNesting))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := p.Match(context.Background(), "a"); err != nil || !ok {
		t.Fatal(ok, err)
	}
	p, err = Compile(strings.Repeat("(?=", 20000) + "a" + strings.Repeat(")", 20000))
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := p.Match(context.Background(), "a"); err != nil || !ok {
		t.Fatal(ok, err)
	}
}
