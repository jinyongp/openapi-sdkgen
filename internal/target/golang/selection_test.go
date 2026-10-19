package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"strings"
	"testing"
)

const selectionFixture = `{"openapi":"3.1.0","info":{"title":"Selection","version":"1"},"paths":{
"/source":{"get":{"operationId":"source","responses":{"200":{"description":"OK","content":{"application/json":{"schema":{"type":"object","required":["id"],"properties":{"id":{"type":"string"}}}}},"links":{"Next":{"operationId":"target","parameters":{"id":"$response.body#/id"}}}}}}},
"/target/{id}":{"get":{"operationId":"target","x-sdk-visibility":"internal","parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"OK","content":{"text/plain":{"schema":{"type":"string"}}}}}}},
"/unused":{"get":{"operationId":"unused","parameters":[{"name":"pattern","in":"query","schema":{"type":"string","pattern":"unused-only"}}],"responses":{"204":{"description":"OK"}}}},
"/hidden":{"get":{"operationId":"hidden","x-sdk-visibility":"hidden","responses":{"204":{"description":"OK"}}}}
}}`

func TestGeneratedSelectedLinkClosure(t *testing.T) {
	document, err := compiler.Compile([]byte(selectionFixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, selection := range []*generator.Selection{{Operations: []string{"source"}}, {Routes: []string{"GET /source"}}} {
		plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}, Selection: selection})
		if err != nil || diagnostic.HasErrors(diagnostics) {
			t.Fatalf("prepare %v %v", err, diagnostics)
		}
		testGeneratedModule(t, plan, map[string]string{"selection_test.go": `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"reflect";"testing";sdk "example.test/consumer/api")
func TestSelectedClient(t *testing.T){s:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){if r.URL.Path=="/source"{w.Header().Set("Content-Type","application/json");io.WriteString(w,"{\"id\":\"item\"}")}else{if r.URL.Path!="/target/item"{t.Error(r.URL.Path)};w.Header().Set("Content-Type","text/plain");io.WriteString(w,"linked")}}));defer s.Close();c,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:s.URL});if err!=nil{t.Fatal(err)}
 for _,name:=range []string{"Target","Unused","Hidden"}{if _,exists:=reflect.TypeOf(c.Operations).MethodByName(name);exists{t.Fatal("nonselected direct method",name)};if _,exists:=reflect.TypeOf(*c).FieldByName(name);exists{t.Fatal("nonselected resource",name)}}
 raw,err:=c.Operations.Source().Raw(context.Background());if err!=nil{t.Fatal(err)};result,err:=c.Links.Source.Next.Follow(raw).Do(context.Background());if err!=nil||result!="linked"{t.Fatal(result,err)}
}
`})
		artifacts, err := (Generator{}).Emit(plan)
		if err != nil {
			t.Fatal(err)
		}
		for _, artifact := range artifacts {
			if artifact.Path == "internal/schema/regex.go" {
				t.Fatal("unused pattern runtime retained")
			}
		}
	}
}

func TestGoSelectionRejectsInvalidEntries(t *testing.T) {
	document, err := compiler.Compile([]byte(selectionFixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, selection := range []*generator.Selection{{}, {Operations: []string{"missing"}}, {Operations: []string{"hidden"}}, {Routes: []string{"GET /missing"}}} {
		_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}, Selection: selection})
		if err != nil || !diagnostic.HasErrors(diagnostics) || diagnostics[0].Code != "SDKGEN-E530" {
			t.Fatalf("selection %v: %v %v", selection, err, diagnostics)
		}
	}
}

func TestGoSelectedVisibilityAndEmptySurface(t *testing.T) {
	for _, test := range []struct{ source, id, code string }{
		{strings.Replace(selectionFixture, `"x-sdk-visibility":"internal"`, `"x-sdk-visibility":"hidden"`, 1), "source", "SDKGEN-E621"},
		{`{"openapi":"3.0.3","info":{"title":"Omitted selection","version":"1"},"paths":{"/unsafe":{"get":{"operationId":"unsafe","requestBody":{"content":{"application/json":{"schema":{"type":"string"}}}},"responses":{"204":{"description":"OK"}}}}}}`, "unsafe", "SDKGEN-E512"},
	} {
		document, err := compiler.Compile([]byte(test.source))
		if err != nil {
			t.Fatal(err)
		}
		_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}, Selection: &generator.Selection{Operations: []string{test.id}}})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, diagnostic := range diagnostics {
			found = found || diagnostic.Code == test.code
		}
		if !found {
			t.Fatalf("missing %s: %v", test.code, diagnostics)
		}
	}
}
