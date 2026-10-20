package golang

import (
	"encoding/json"
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
	schemaplan "openapi-sdkgen/internal/schema/plan"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedOfficialFormatCases(t *testing.T) {
	for _, name := range []string{"hostname", "idn-hostname", "uri", "uri-reference", "iri", "iri-reference", "email", "idn-email"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "typescript", "fixtures", "json-schema-format-"+name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var groups []schemaSuiteGroup
			if err := json.Unmarshal(data, &groups); err != nil {
				t.Fatal(err)
			}
			document := &ir.Document{OpenAPIVersionLine: "3.1"}
			var source strings.Builder
			source.WriteString("package schema\nimport(\"context\";\"testing\";\"example.test/consumer/api/internal/wire\")\nfunc TestOfficialFormats(t *testing.T){\n")
			for index, group := range groups {
				schema := group.Schema.(map[string]any)
				schema["$vocabulary"] = map[string]any{schemaplan.FormatAssertionVocabulary: true}
				document.Operations = append(document.Operations, ir.Operation{RouteKey: fmt.Sprintf("GET /%d", index), OperationID: fmt.Sprintf("format%d", index), Method: "GET", Path: fmt.Sprintf("/%d", index), Responses: []ir.Response{{Status: "200", Content: []ir.MediaType{{ContentType: "application/json", Schema: schema}}}}})
				for _, test := range group.Tests {
					fmt.Fprintf(&source, "t.Run(%q,func(t *testing.T){value,err:=wire.Parse([]byte(%q));if err!=nil{t.Fatal(err)};_,err=ValidateHTTP__Format%dOutput__(context.Background(),value,\"\");if (err==nil)!=%t{t.Fatal(err)}})\n", test.Description, string(test.Data), index, test.Valid)
				}
			}
			source.WriteString("}\n")
			plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
			if err != nil || len(diagnostics) != 0 {
				t.Fatal(err, diagnostics)
			}
			testGeneratedModule(t, plan, map[string]string{"api/internal/schema/official_formats_test.go": source.String()})
		})
	}
}
