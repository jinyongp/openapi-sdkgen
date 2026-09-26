package typescript

import (
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/generator"
)

func TestCallbackPreparationFailureRemainsDocumentBlockingForServerAddon(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Callback failure scope","version":"1"},
  "paths":{
    "/jobs":{"post":{"operationId":"createJob","responses":{"202":{"description":"Accepted"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	callbacks := map[string]any{"broken": "not-a-callback-object"}
	document.Operations[0].Raw["callbacks"] = callbacks
	paths := document.Raw["paths"].(map[string]any)
	pathItem := paths["/jobs"].(map[string]any)
	pathItem["post"].(map[string]any)["callbacks"] = callbacks

	_, diagnostics, err := (Generator{}).Prepare(document, serverFailureScopeOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	value := diagnostics[0]
	if value.Code != "SDKGEN-E506" || value.Severity != diagnostic.SeverityError ||
		value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock ||
		!strings.Contains(value.Message, "callback contracts") {
		t.Fatalf("callback failure diagnostic = %#v", value)
	}
}

func TestWebhookPreparationFailureRemainsDocumentBlockingForServerAddon(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Webhook failure scope","version":"1"},
  "paths":{
    "/health":{"get":{"operationId":"health","responses":{"204":{"description":"OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	document.Raw["webhooks"] = map[string]any{"broken": "not-a-path-item"}

	_, diagnostics, err := (Generator{}).Prepare(document, serverFailureScopeOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	value := diagnostics[0]
	if value.Code != "SDKGEN-E506" || value.Severity != diagnostic.SeverityError ||
		value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock ||
		!strings.Contains(value.Message, "webhook contracts") {
		t.Fatalf("webhook failure diagnostic = %#v", value)
	}
}

func TestInboundContractsWithoutServerAddonRemainDocumentBlocking(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Inbound add-on scope","version":"1"},
  "paths":{
    "/jobs":{"post":{
      "operationId":"createJob",
      "responses":{"202":{"description":"Accepted"}},
      "callbacks":{"completed":{"{$request.body#/callbackURL}":{"post":{"responses":{"204":{"description":"OK"}}}}}}
    }}
  },
  "webhooks":{"events":{"post":{"responses":{"204":{"description":"OK"}}}}}
}`))
	if err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	for _, value := range diagnostics {
		if value.Code != "SDKGEN-E505" || value.Severity != diagnostic.SeverityError ||
			value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock {
			t.Fatalf("inbound add-on diagnostic = %#v", value)
		}
	}
}

func serverFailureScopeOptions(t *testing.T) generator.Options {
	t.Helper()
	registry, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	options, err := registry.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	return options
}
