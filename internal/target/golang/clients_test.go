package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGeneratedIndependentClients(t *testing.T) {
	document, err := compiler.Compile([]byte(selectionFixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, explicitRoot := range []bool{false, true} {
		options := generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}, Clients: map[string]generator.Client{"source-only": {Selection: &generator.Selection{Operations: []string{"source"}}}, "other": {Selection: &generator.Selection{Operations: []string{"unused"}}}}}
		if explicitRoot {
			options.Selection = &generator.Selection{Operations: []string{"source"}}
		}
		plan, diagnostics, err := (Generator{}).Prepare(document, options)
		if err != nil || len(diagnostics) != 0 {
			t.Fatalf("prepare %v %v", err, diagnostics)
		}
		consumer := strings.ReplaceAll(independentClientConsumer, "ROOT_HAS_UNUSED", map[bool]string{true: "false", false: "true"}[explicitRoot])
		root := testGeneratedModule(t, plan, map[string]string{"clients_test.go": consumer})
		command := exec.Command("go", "list", "-mod=mod", "-deps", "example.test/consumer/api/clients/source-only")
		command.Dir = root
		command.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("dependencies: %v\n%s", err, output)
		}
		for _, dependency := range strings.Fields(string(output)) {
			if dependency == "example.test/consumer/api" || strings.HasPrefix(dependency, "example.test/consumer/api/clients/other") || strings.Contains(dependency, "/metadata") || strings.Contains(dependency, "/selective") {
				t.Fatal("independent entry pulls unrelated entry", dependency)
			}
		}
	}
}

func TestGeneratedKeywordClientEntries(t *testing.T) {
	document, err := compiler.Compile([]byte(selectionFixture))
	if err != nil {
		t.Fatal(err)
	}
	clients := map[string]generator.Client{}
	for _, name := range []string{"for", "main"} {
		clients[name] = generator.Client{Selection: &generator.Selection{Operations: []string{"source"}}}
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}, Clients: clients})
	if err != nil || len(diagnostics) != 0 {
		t.Fatal(err, diagnostics)
	}
	testGeneratedModule(t, plan, map[string]string{"keyword_test.go": `package consumer_test
import("testing";forClient "example.test/consumer/api/clients/for";mainClient "example.test/consumer/api/clients/main")
func TestPortableClientNames(t *testing.T){var left forClient.Client;var right mainClient.Client;_=left.Operations.Source();_=right.Operations.Source()}
`})
}

const independentClientConsumer = `package consumer_test
import("context";"io";"net/http";"net/http/httptest";"reflect";"testing";sdk "example.test/consumer/api";source "example.test/consumer/api/clients/source-only";"example.test/consumer/api/clients/other")
func TestIndependentMethodSets(t *testing.T){server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){switch r.URL.Path{case "/source":w.Header().Set("Content-Type","application/json");io.WriteString(w,"{\"id\":\"item\"}");case "/target/item":w.Header().Set("Content-Type","text/plain");io.WriteString(w,"linked");case "/unused":w.WriteHeader(204);default:t.Error(r.URL.Path)}}));defer server.Close();left,err:=source.NewClient(source.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};right,err:=other.NewClient(other.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)};root,err:=sdk.NewClient(sdk.ClientOptions{BaseURL:server.URL});if err!=nil{t.Fatal(err)}
 if _,exists:=reflect.TypeOf(left.Operations).MethodByName("Unused");exists{t.Fatal("other client method leaked")};if _,exists:=reflect.TypeOf(right.Operations).MethodByName("Source");exists{t.Fatal("source client method leaked")};if _,exists:=reflect.TypeOf(root.Operations).MethodByName("Unused");exists!=ROOT_HAS_UNUSED{t.Fatal("root selection union",exists)}
 if _,exists:=reflect.TypeOf(*right).FieldByName("Source");exists{t.Fatal("resource leaked")};if _,exists:=reflect.TypeOf(right).Elem().FieldByName("Links");exists{t.Fatal("Link source leaked")}
 raw,err:=left.Source.Get().Raw(context.Background());if err!=nil{t.Fatal(err)};linked,err:=left.Links.Source.Next.Follow(raw).Do(context.Background());if err!=nil||linked!="linked"{t.Fatal(linked,err)};if _,err:=right.Unused.Get().Do(context.Background());err!=nil{t.Fatal(err)}
}
`
