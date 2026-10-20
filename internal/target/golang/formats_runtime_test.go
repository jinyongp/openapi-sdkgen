package golang

import (
	"fmt"
	"strings"
	"testing"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
	schemaplan "openapi-sdkgen/internal/schema/plan"
)

func TestGeneratedFormatAssertions(t *testing.T) {
	cases := []struct {
		format, input string
		valid         bool
	}{
		{"date", `"0001-01-01"`, true}, {"date", `"2000-02-29"`, true}, {"date", `"1900-02-29"`, false}, {"date", `"2020-01-00"`, false},
		{"time", `"23:59:60Z"`, true}, {"time", `"22:59:60Z"`, false}, {"time", `"01:29:60+01:30"`, true}, {"time", `"23:59:60+01:00"`, false},
		{"time", `"01:02:03+24:00"`, false}, {"time", `"01:02:03+00:60"`, false}, {"time", `"08:30:06.283185z"`, true}, {"time", `"08:30:06.Z"`, false},
		{"date-time", `"2020-02-29T23:00:01Z"`, true}, {"date-time", `"2020-02-29T23:00:01ZTjunk"`, false}, {"date-time", `"2020-02-30T23:00:01Z"`, false},
		{"duration", `"P1Y2M3DT4H5M6.5S"`, true}, {"duration", `"PT"`, false}, {"duration", `"P"`, false},
		{"email", `"user@example.test"`, true}, {"email", `"user@example.test\n"`, false}, {"email", `"user@localhost"`, true},
		{"idn-email", `"사용자@예시"`, true}, {"idn-email", `"two words@example.test"`, false},
		{"hostname", `"example.test."`, false}, {"hostname", `"-bad.test"`, false}, {"hostname", `"example.test\n"`, false}, {"hostname", `"예시.test"`, false},
		{"idn-hostname", `"例え.テスト"`, true}, {"idn-hostname", `"white space.test"`, false},
		{"idn-hostname", `"é--a"`, true}, {"idn-hostname", `"-é"`, false}, {"idn-hostname", `"é-"`, false}, {"idn-hostname", `"éa--b"`, false},
		{"ipv4", `"127.0.0.1"`, true}, {"ipv4", `"127.01.0.1"`, false}, {"ipv4", `"256.0.0.1"`, false},
		{"ipv6", `"::1"`, true}, {"ipv6", `"::ffff:127.0.0.1"`, true}, {"ipv6", `"fe80::1%eth0"`, false},
		{"uri", `"https://example.test/%zz"`, false}, {"uri", `"relative/path"`, false}, {"uri", `"https://例え.テスト"`, false},
		{"uri", `"K://example.test"`, false}, {"uri-reference", `"ſ://example.test"`, false},
		{"iri", `"K://example.test"`, false}, {"iri-reference", `"ſ://example.test"`, false},
		{"uri", `"http://[v1.K]"`, false}, {"iri", `"http://[v1.ſ]"`, false},
		{"uri", `"HTTPS://example.test"`, true}, {"iri", `"http://[Vf.X:1]"`, true},
		{"uri", `"http://example.test/#fragment\n"`, false}, {"uri", `"http://example.test:80\n"`, false},
		{"iri", `"http://[::1]:80\n"`, false}, {"iri", `"http://[v1.x\n]"`, false},
		{"iri", `"http://example.test:80\u2028"`, false}, {"iri", `"http://[v1.x\u2028]"`, false},
		{"iri", `"http://example.test/#\u2028"`, true}, {"uri", `"http://example.test/#\u2028"`, false},
		{"uri-reference", `"relative/path"`, true}, {"uri-reference", `"https://[::gg]"`, false},
		{"iri", `"https://例え.テスト/道"`, true}, {"iri-reference", `"道"`, true}, {"iri", `"relative"`, false},
		{"uri-template", `"https://example.test/{id}"`, true}, {"uri-template", `"https://example.test/{id"`, false},
		{"uuid", `"123e4567-e89b-12d3-a456-426614174000"`, true}, {"uuid", `"123e4567-e89b-12d3-a456-426614174000\n"`, false},
		{"json-pointer", `"/a~1b/~0"`, true}, {"json-pointer", `"/a~2b"`, false},
		{"relative-json-pointer", `"0#"`, true}, {"relative-json-pointer", `"12/a"`, true}, {"relative-json-pointer", `"01/a"`, false},
		{"regex", `"(?<=x)y"`, true}, {"regex", `"\\a"`, false},
		{"custom-annotation", `"anything"`, true}, {"date", `12`, true},
	}
	document := &ir.Document{OpenAPIVersionLine: "3.1"}
	var source strings.Builder
	source.WriteString("package schema\nimport(\"context\";\"sync\";\"testing\";\"example.test/consumer/api/internal/wire\")\nfunc TestFormats(t *testing.T){ctx:=context.Background()\n")
	for index, test := range cases {
		schema := map[string]any{"format": test.format, "$vocabulary": map[string]any{schemaplan.FormatAssertionVocabulary: true}}
		document.Operations = append(document.Operations, ir.Operation{RouteKey: fmt.Sprintf("GET /%d", index), OperationID: fmt.Sprintf("format%d", index), Method: "GET", Path: fmt.Sprintf("/%d", index), Responses: []ir.Response{{Status: "200", Content: []ir.MediaType{{ContentType: "application/json", Schema: schema}}}}})
		fmt.Fprintf(&source, "{value,err:=wire.Parse([]byte(%q));if err!=nil{t.Fatal(err)};_,err=ValidateHTTP__Format%dOutput__(ctx,value,\"\");if (err==nil)!=%t{t.Errorf(\"case %d (%s): %%v\",err)}}\n", test.input, index, test.valid, index, test.format)
	}
	source.WriteString(`
 var workers sync.WaitGroup
 for index:=0;index<16;index++{workers.Go(func(){for sample:=0;sample<20;sample++{if !FormatURL("https://example.test/path",true)||!FormatURL("relative/path",false){t.Error("concurrent URL format state")}}})};workers.Wait()
}
`)
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("prepare: %v %v", err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"api/internal/schema/formats_test.go": source.String()})
}
