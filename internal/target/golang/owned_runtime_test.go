package golang

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func TestOwnedRuntimeSelectionAndImports(t *testing.T) {
	for _, test := range []struct {
		name              string
		schema            any
		compiler, matcher bool
	}{
		{name: "plain", schema: true},
		{name: "static", schema: map[string]any{"pattern": `(?i:^(?<word>\p{Script=Greek}+)-\k<word>$)`}, matcher: true},
		{name: "dynamic-format", schema: map[string]any{"format": "regex", "$vocabulary": map[string]any{schemaplan.FormatAssertionVocabulary: true}}, matcher: true, compiler: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/output/nested", Package: "api"}}
			document := &ir.Document{OpenAPIVersionLine: "3.1", Operations: []ir.Operation{{OperationID: "check", RouteKey: "GET /check", Method: "GET", Path: "/check", Responses: []ir.Response{{Status: "200", Content: []ir.MediaType{{ContentType: "application/json", Schema: test.schema}}}}}}}
			plan, diagnostics, err := (Generator{}).Prepare(document, config)
			if err != nil || len(diagnostics) > 0 {
				t.Fatal(err, diagnostics)
			}
			first, err := (Generator{}).Emit(plan)
			if err != nil {
				t.Fatal(err)
			}
			second, err := (Generator{}).Emit(plan)
			if err != nil {
				t.Fatal(err)
			}
			paths := make(map[string]bool)
			for i, file := range first {
				paths[file.Path] = true
				if file.Path != second[i].Path || string(file.Data) != string(second[i].Data) {
					t.Fatal("nondeterministic emission", file.Path)
				}
				if !strings.HasSuffix(file.Path, ".go") {
					continue
				}
				node, err := parser.ParseFile(token.NewFileSet(), file.Path, file.Data, parser.ImportsOnly)
				if err != nil {
					t.Fatal(err)
				}
				for _, imported := range node.Imports {
					if strings.Contains(imported.Path.Value, "openapi-sdkgen/") || strings.Contains(imported.Path.Value, "github.com/") || strings.Contains(imported.Path.Value, "golang.org/") {
						t.Fatal("non-owned SDK import", file.Path, imported.Path.Value)
					}
				}
			}
			if paths["internal/runtime/ecmaregexp/compile.go"] != test.compiler || paths["internal/runtime/ecmaregexp/match.go"] != test.matcher || paths["internal/runtime/unicode17/properties_data.go"] != test.compiler {
				t.Fatal("unexpected runtime/parser/property reachability", paths)
			}
			if !paths["internal/runtime/urlstd/url.go"] || !paths["internal/runtime/unicode17/LICENSE.txt"] {
				t.Fatal("missing URL runtime or Unicode notice")
			}
		})
	}
}

func utf16PatternSource(units []uint16) string {
	var out []byte
	for _, unit := range units {
		r := rune(unit)
		if r >= 0xd800 && r <= 0xdfff {
			out = append(out, byte(0xe0|r>>12), byte(0x80|r>>6&63), byte(0x80|r&63))
		} else {
			out = utf8.AppendRune(out, r)
		}
	}
	return string(out)
}
func TestGeneratedOwnedRegexConformance(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/goruntime/regexp-matches.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			PatternUnits, InputUnits []uint16
			Matches                  bool
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	doc := &ir.Document{OpenAPIVersionLine: "3.1"}
	var test strings.Builder
	test.WriteString("package schema\nimport(\"context\";\"testing\";\"os\";\"encoding/json\";\"unicode/utf8\";\"example.test/consumer/api/internal/wire\")\nfunc TestPinnedMatches(t *testing.T){ctx:=context.Background()\n")
	for i, c := range corpus.Cases {
		pattern, input := utf16PatternSource(c.PatternUnits), utf16PatternSource(c.InputUnits)
		name := fmt.Sprintf("case%d", i)
		doc.Operations = append(doc.Operations, ir.Operation{OperationID: name, RouteKey: "GET /" + name, Method: "GET", Path: "/" + name, Responses: []ir.Response{{Status: "200", Content: []ir.MediaType{{ContentType: "application/json", Schema: map[string]any{"pattern": pattern}}}}}})
		fmt.Fprintf(&test, "{_,err:=ValidateHTTP__Case%dOutput__(ctx,wire.Text(%q),\"\");if (err==nil)!=%t{t.Errorf(\"static case %d: %%v\",err)};dynamic,err:=PreparePattern(%q);if err!=nil{t.Fatal(err)};got,err:=dynamic.Match(ctx,%q);if err!=nil||got!=%t{t.Errorf(\"dynamic case %d: %%t %%v\",got,err)}}\n", i, input, c.Matches, i, pattern, input, c.Matches, i)
	}
	test.WriteString("}\n")
	test.WriteString(`
func sourceUnits(units []uint16)string{var out []byte;for _,u:=range units{r:=rune(u);if r>=0xd800&&r<=0xdfff{out=append(out,byte(0xe0|r>>12),byte(0x80|r>>6&63),byte(0x80|r&63))}else{out=utf8.AppendRune(out,r)}};return string(out)}
func TestPinnedSyntax(t *testing.T){data,err:=os.ReadFile("corpus-syntax.json");if err!=nil{t.Fatal(err)};var corpus struct{Cases []struct{Units []uint16;Valid bool}};if err:=json.Unmarshal(data,&corpus);err!=nil{t.Fatal(err)};for i,c:=range corpus.Cases{_,err:=PreparePattern(sourceUnits(c.Units));if (err==nil)!=c.Valid{t.Errorf("syntax case %d: %v",i,err)}}}
`)
	// A regex-format assertion needs dynamic compilation; ordinary static patterns
	// above alone do not cause the runtime compiler/property table to be emitted.
	doc.Operations = append(doc.Operations, ir.Operation{OperationID: "syntax", RouteKey: "GET /syntax", Method: "GET", Path: "/syntax", Responses: []ir.Response{{Status: "200", Content: []ir.MediaType{{ContentType: "application/json", Schema: map[string]any{"format": "regex", "$vocabulary": map[string]any{schemaplan.FormatAssertionVocabulary: true}}}}}}})
	plan, diagnostics, err := (Generator{}).Prepare(doc, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) > 0 {
		t.Fatal(err, diagnostics)
	}
	syntax, err := os.ReadFile("../../../testdata/goruntime/regexp-syntax.json")
	if err != nil {
		t.Fatal(err)
	}
	testGeneratedModule(t, plan, map[string]string{"api/internal/schema/conformance_test.go": test.String(), "api/internal/schema/corpus-syntax.json": string(syntax)})
}
