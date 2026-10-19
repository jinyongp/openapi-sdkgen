package probes

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/dlclark/regexp2/v2"
	"github.com/dop251/goja"
	"github.com/go-quickjs/go-quickjs/jsregexp"
)

type regexCase struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
	Input   string `json:"input"`
}

func TestRegexConcurrentUseAndComplexity(t *testing.T) {
	compiled, err := jsregexp.Compile(`^(?<word>\p{Letter}+)-\k<word>$`, "u")
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	for index := 0; index < 16; index++ {
		workers.Go(func() {
			for sample := 0; sample < 20; sample++ {
				matched, err := compiled.MatchString("한글-한글")
				if err != nil || !matched {
					t.Errorf("concurrent match: %v, %v", matched, err)
				}
			}
		})
	}
	workers.Wait()
	pathological, err := jsregexp.Compile(`^(a+)+$`, "u")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pathological.MatchString(strings.Repeat("a", 32) + "!"); !errors.Is(err, jsregexp.ErrComplexity) {
		t.Fatalf("expected bounded complexity error, got %v", err)
	}
	matched, err := pathological.MatchString("aaa")
	if err != nil || !matched {
		t.Fatalf("compiled pattern was not reusable after budget exhaustion: %v, %v", matched, err)
	}
	t.Log("16 concurrent consumers; bounded backtracking; compiled pattern reusable after budget exhaustion")
}

func TestUnicodeDataBaseline(t *testing.T) {
	const pattern = `^\p{Script=Tai_Yo}$`
	const input = "\U0001e6c0"
	compiled, err := jsregexp.Compile(pattern, "u")
	if err != nil {
		t.Fatal(err)
	}
	matched, err := compiled.MatchString(input)
	if err != nil || !matched {
		t.Fatalf("Unicode 17 script: %v, %v", matched, err)
	}
	output, err := exec.Command("node", "-e", `console.log(JSON.stringify({unicode:process.versions.unicode,icu:process.versions.icu,matches:/^\p{Script=Tai_Yo}$/u.test("\u{1E6C0}")}))`).CombinedOutput()
	if err != nil {
		t.Fatalf("Node Unicode data: %v: %s", err, output)
	}
	var baseline struct {
		Unicode string `json:"unicode"`
		ICU     string `json:"icu"`
		Matches bool   `json:"matches"`
	}
	if err := json.Unmarshal(output, &baseline); err != nil {
		t.Fatal(err)
	}
	if baseline.Unicode != "17.0" || !baseline.Matches {
		t.Fatalf("baseline Unicode data changed: %+v", baseline)
	}
	t.Logf("Node Unicode %s / ICU %s and selected regex data agree on a Unicode 17 script", baseline.Unicode, baseline.ICU)
}

func BenchmarkRegexEngines(b *testing.B) {
	const pattern = `^[a-z]+-[0-9]+$`
	const input = "todo-123"
	standard := regexp.MustCompile(pattern)
	compatible := jsregexp.MustCompile(pattern, "u")
	utf8Compatible := jsregexp.MustCompileUTF8(pattern, "u")
	b.Run("standard", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if !standard.MatchString(input) {
				b.Fatal("unexpected mismatch")
			}
		}
	})
	for _, sample := range []struct {
		name string
		re   *jsregexp.Regexp
	}{{"ecmascript", compatible}, {"ecmascript-utf8", utf8Compatible}} {
		b.Run(sample.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if matched, err := sample.re.MatchString(input); err != nil || !matched {
					b.Fatalf("unexpected result %v %v", matched, err)
				}
			}
		})
	}
}

type regexResult struct {
	Name    string `json:"name"`
	Valid   bool   `json:"valid"`
	Matches bool   `json:"matches"`
}

func TestECMAScriptUnicodeRegex(t *testing.T) {
	data, err := os.ReadFile("regex-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []regexCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	const script = `const fs = require("node:fs");
const cases = JSON.parse(fs.readFileSync("regex-cases.json", "utf8"));
console.log(JSON.stringify(cases.map(c => {
  try { return { name: c.name, valid: true, matches: new RegExp(c.pattern, "u").test(c.input) }; }
  catch { return { name: c.name, valid: false, matches: false }; }
})));`
	output, err := exec.Command("node", "-e", script).CombinedOutput()
	if err != nil {
		t.Fatalf("Node oracle: %v: %s", err, output)
	}
	var oracle []regexResult
	if err := json.Unmarshal(output, &oracle); err != nil {
		t.Fatal(err)
	}
	stdlibDifferences := 0
	rawDifferences := 0
	gojaDifferences := 0
	selectedDifferences := 0
	vm := goja.New()
	value, err := vm.RunString(`(function(pattern, input) {
  try { return JSON.stringify({valid: true, matches: new RegExp(pattern, "u").test(input)}); }
  catch { return JSON.stringify({valid: false, matches: false}); }
})`)
	if err != nil {
		t.Fatal(err)
	}
	check, ok := goja.AssertFunction(value)
	if !ok {
		t.Fatal("regex checker is not callable")
	}
	for index, sample := range cases {
		t.Run(sample.Name, func(t *testing.T) {
			want := oracle[index]
			compiled, err := regexp2.Compile(sample.Pattern, regexp2.ECMAScript|regexp2.Unicode, regexp2.OptionMaintainCaptureOrder())
			got := regexResult{Name: sample.Name, Valid: err == nil}
			if err == nil {
				got.Matches, err = compiled.MatchString(sample.Input)
				if err != nil {
					t.Fatal(err)
				}
			}
			if got != want {
				rawDifferences++
				t.Logf("raw regexp2 differs: %+v; Node result %+v", got, want)
			}
			result, err := check(goja.Undefined(), vm.ToValue(sample.Pattern), vm.ToValue(sample.Input))
			if err != nil {
				t.Fatal(err)
			}
			var interpreted regexResult
			if err := json.Unmarshal([]byte(result.String()), &interpreted); err != nil {
				t.Fatal(err)
			}
			interpreted.Name = sample.Name
			if interpreted != want {
				gojaDifferences++
				t.Logf("goja differs: %+v; Node result %+v", interpreted, want)
			}
			quick, err := jsregexp.Compile(sample.Pattern, "u")
			actual := regexResult{Name: sample.Name, Valid: err == nil}
			if err == nil {
				actual.Matches, err = quick.MatchString(sample.Input)
				if err != nil {
					t.Fatal(err)
				}
			}
			if actual != want {
				selectedDifferences++
				t.Errorf("jsregexp result %+v; Node result %+v", actual, want)
			}
		})
		standard, err := regexp.Compile(sample.Pattern)
		got := regexResult{Name: sample.Name, Valid: err == nil}
		if err == nil {
			got.Matches = standard.MatchString(sample.Input)
		}
		if got != oracle[index] {
			stdlibDifferences++
		}
	}
	t.Logf("%d ECMAScript Unicode samples; standard regexp differences: %d; raw regexp2 differences: %d; goja differences: %d; selected differences: %d", len(cases), stdlibDifferences, rawDifferences, gojaDifferences, selectedDifferences)
}
