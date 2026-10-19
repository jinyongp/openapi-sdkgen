package golang

import (
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/generator"
	"strings"
	"testing"
)

const inboundScopeFixture = `{"openapi":"3.2.0","info":{"title":"Inbound ownership","version":"1"},"paths":{"/health":{"get":{"operationId":"health","responses":{"204":{"description":"OK"}}}},"/jobs":{"post":{"operationId":"createJob","responses":{"202":{"description":"Accepted"}},"callbacks":{"completed":{"{$request.body#/callbackURL}":{"post":{"responses":{"204":{"description":"OK"}}}}}}}}},"webhooks":{"events":{"post":{"responses":{"204":{"description":"OK"}}}}}}`

func TestGoInboundRequiresServerAddonWithDocumentScope(t *testing.T) {
	document, err := compiler.Compile([]byte(inboundScopeFixture))
	if err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}})
	if err != nil || len(diagnostics) != 2 {
		t.Fatal(err, diagnostics)
	}
	for _, value := range diagnostics {
		if value.Code != "SDKGEN-E505" || value.Severity != diagnostic.SeverityError || value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock {
			t.Fatal(value)
		}
	}
}

func TestGoInboundCallbackErrorsFollowGenerationSelection(t *testing.T) {
	source := strings.Replace(inboundScopeFixture, `,"webhooks":{"events":{"post":{"responses":{"204":{"description":"OK"}}}}}`, "", 1)
	document, err := compiler.Compile([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	options := generator.Options{Golang: &generator.GoOptions{ImportPath: "example.test/consumer/api"}, Selection: &generator.Selection{Operations: []string{"health"}}}
	if _, diagnostics, err := (Generator{}).Prepare(document, options); err != nil || diagnostic.HasErrors(diagnostics) {
		t.Fatal("unselected callback blocked client", err, diagnostics)
	}
	options.Selection.Operations = []string{"createJob"}
	if _, diagnostics, err := (Generator{}).Prepare(document, options); err != nil || len(diagnostics) != 1 || diagnostics[0].Code != "SDKGEN-E505" {
		t.Fatal(err, diagnostics)
	}
}
