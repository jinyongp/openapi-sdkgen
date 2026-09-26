package typescript

import (
	"strings"
	"testing"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/generator"
)

func TestOperationOmissionRemovesUnsupportedCallablesAndKeepsSupportedSiblings(t *testing.T) {
	for _, test := range []struct {
		name    string
		version string
		method  string
	}{
		{name: "compiler restriction", version: "3.0.3", method: "get"},
		{name: "target restriction", version: "3.1.1", method: "head"},
	} {
		t.Run(test.name, func(t *testing.T) {
			document, err := sdkgen.Compile([]byte(`{
  "openapi":"` + test.version + `",
  "info":{"title":"Operation omission","version":"1"},
  "paths":{
    "/items":{"` + test.method + `":{
      "operationId":"fetchBody",
      "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"string"}}}},
      "responses":{"204":{"description":"OK"}}
    }},
    "/ok":{"post":{"operationId":"createItem","responses":{"204":{"description":"OK"}}}}
  }
}`))
			if err != nil {
				t.Fatal(err)
			}
			route := strings.ToUpper(test.method) + " /items"
			probe := `import { createClient } from "./index.js"
declare const api: ReturnType<typeof createClient>
api.$operations.createItem()
// @ts-expect-error omitted operation ID is not generated
api.$operations.fetchBody()
// @ts-expect-error omitted exact route is not generated
api.$routes["` + route + `"]
// @ts-expect-error omitted operation leaves no resource namespace
api.items
`
			compileTypeScriptArtifactsWithProbe(t, document, "operation-omission.probe.ts", probe)
		})
	}
}

func TestOmittedOperationKeepsReservationCollisionsStable(t *testing.T) {
	compile := func(omitLower bool) *sourcePlan {
		body := ""
		if omitLower {
			body = `"requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"string"}}}},`
		}
		document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Reservation stability","version":"1"},
  "paths":{
    "/Foo":{"get":{"operationId":"getUpper","responses":{"204":{"description":"OK"}}}},
    "/foo":{"get":{"operationId":"getLower",` + body + `"responses":{"204":{"description":"OK"}}}}
  }
}`))
		if err != nil {
			t.Fatal(err)
		}
		prepared, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if diagnostic.HasErrors(diagnostics) {
			t.Fatalf("diagnostics = %#v", diagnostics)
		}
		value, err := prepared.Value("typescript")
		if err != nil {
			t.Fatal(err)
		}
		plan, ok := value.(*sourcePlan)
		if !ok {
			t.Fatalf("plan = %T", value)
		}
		return plan
	}

	baseline := compile(false)
	omitted := compile(true)
	baselinePath := baseline.modules.operationByRoute["GET /Foo"]
	omittedPath := omitted.modules.operationByRoute["GET /Foo"]
	if baselinePath == "" || omittedPath != baselinePath {
		t.Fatalf("supported artifact path changed after omission: %q -> %q", baselinePath, omittedPath)
	}
	if _, exists := omitted.modules.operationByRoute["GET /foo"]; exists {
		t.Fatalf("omitted operation received a module: %#v", omitted.modules.operationByRoute)
	}
	baselineCalls := manifestCalls(*baseline.manifest)
	omittedCalls := manifestCalls(*omitted.manifest)
	if baselineCalls["getUpper"] == "" || omittedCalls["getUpper"] != baselineCalls["getUpper"] {
		t.Fatalf("supported public call changed after omission: %q -> %q", baselineCalls["getUpper"], omittedCalls["getUpper"])
	}
	if omitted.resourceTree.children["foo"] != nil {
		t.Fatalf("omitted collision left an empty resource namespace: %#v", omitted.resourceTree.children["foo"])
	}
}

func TestOperationOmissionRemovesFetchForbiddenMethodWithoutChangingSupportedSibling(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Forbidden method omission","version":"1"},
  "paths":{
    "/trace":{"trace":{"operationId":"traceItems","responses":{"204":{"description":"OK"}}}},
    "/ok":{"post":{"operationId":"createItem","responses":{"204":{"description":"OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "SDKGEN-W511" ||
		diagnostics[0].Severity != diagnostic.SeverityWarning ||
		diagnostics[0].Scope != failure.ScopeOperation ||
		diagnostics[0].Effect != failure.EffectOmitOperation ||
		diagnostics[0].Operation != "traceItems" {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if _, err := (Generator{}).Emit(plan); err != nil {
		t.Fatal(err)
	}
	probe := `import { createClient } from "./index.js"
declare const api: ReturnType<typeof createClient>
api.$operations.createItem()
// @ts-expect-error TRACE operation is omitted
api.$operations.traceItems()
// @ts-expect-error TRACE route is omitted
api.$routes["TRACE /trace"]
// @ts-expect-error omitted TRACE operation leaves no resource namespace
api.trace
`
	compileTypeScriptArtifactsWithProbe(t, document, "trace-omission.probe.ts", probe)
}

func TestOperationOmissionPreservesAuthorVisibility(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Visibility preservation","version":"1"},
  "paths":{
    "/internal":{"get":{
      "operationId":"getInternal",
      "x-sdk-visibility":"internal",
      "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"string"}}}},
      "responses":{"204":{"description":"OK"}}
    }},
    "/public":{"post":{"operationId":"createPublic","responses":{"204":{"description":"OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Operations) != 2 || document.Operations[0].Visibility != "internal" {
		t.Fatalf("compiled visibility = %#v", document.Operations)
	}
	prepared, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.HasErrors(diagnostics) {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	value, err := prepared.Value("typescript")
	if err != nil {
		t.Fatal(err)
	}
	plan := value.(*sourcePlan)
	if document.Operations[0].Visibility != "internal" || plan.document.Operations[0].Visibility != "internal" {
		t.Fatalf("target planning mutated visibility: source=%q prepared=%q", document.Operations[0].Visibility, plan.document.Operations[0].Visibility)
	}
	foundReservation := false
	for _, operation := range plan.reservationManifest.Operations {
		if operation.OperationID == "getInternal" {
			foundReservation = true
			if operation.Visibility != "internal" {
				t.Fatalf("reservation visibility = %q", operation.Visibility)
			}
		}
	}
	if !foundReservation {
		t.Fatal("omitted internal operation did not participate in portable reservation")
	}
	for _, operation := range plan.manifest.Operations {
		if operation.OperationID == "getInternal" {
			t.Fatal("omitted internal operation remained in emission manifest")
		}
	}
}

func TestOperationOmissionTransitivelyRemovesOwnedAndTargetingLinks(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Link omission","version":"1"},
  "paths":{
    "/source":{"get":{"operationId":"getSource","responses":{"200":{
      "description":"OK",
      "links":{"toOmitted":{"operationId":"getOmitted"}}
    }}}},
    "/omitted":{"get":{
      "operationId":"getOmitted",
      "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"string"}}}},
      "responses":{"200":{
        "description":"OK",
        "links":{"toSupported":{"operationId":"getTarget"}}
      }}
    }},
    "/target":{"get":{"operationId":"getTarget","responses":{"204":{"description":"OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	plan, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.HasErrors(diagnostics) {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if !diagnosticsContainCode(diagnostics, "SDKGEN-W511") || !diagnosticsContainCode(diagnostics, "SDKGEN-W509") {
		t.Fatalf("omission diagnostics = %#v", diagnostics)
	}
	foundUnavailableLink := false
	for _, value := range diagnostics {
		if value.Code != "SDKGEN-W509" {
			continue
		}
		if strings.Contains(value.Message, "targets unavailable operation") && !strings.Contains(value.Message, "targets hidden operation") {
			foundUnavailableLink = true
			break
		}
	}
	if !foundUnavailableLink {
		t.Fatalf("omitted target Link diagnostic misclassified visibility: %#v", diagnostics)
	}
	if _, err := (Generator{}).Emit(plan); err != nil {
		t.Fatal(err)
	}
	probe := `import { createClient } from "./index.js"
declare const api: ReturnType<typeof createClient>
api.$operations.getSource()
api.$operations.getTarget()
// @ts-expect-error target operation is omitted
api.$operations.getOmitted()
// @ts-expect-error link to omitted target is omitted
api.$links.getSource.toOmitted
// @ts-expect-error omitted source owns no link helpers
api.$links.getOmitted.toSupported
`
	compileTypeScriptArtifactsWithProbe(t, document, "link-operation-omission.probe.ts", probe)
}

func TestOmittedSourceOperationDoesNotBlockOnOwnedLinkVisibility(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Omitted source Link visibility","version":"1"},
  "paths":{
    "/source":{"get":{
      "operationId":"getSource",
      "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"string"}}}},
      "responses":{"200":{
        "description":"OK",
        "links":{"toHidden":{"operationId":"getHidden"}}
      }}
    }},
    "/hidden":{"post":{
      "operationId":"getHidden",
      "x-sdk-visibility":"hidden",
      "responses":{"204":{"description":"OK"}}
    }},
    "/ok":{"post":{"operationId":"createItem","responses":{"204":{"description":"OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.HasErrors(diagnostics) || diagnosticsContainCode(diagnostics, "SDKGEN-E621") {
		t.Fatalf("omitted source Link visibility blocked generation: %#v", diagnostics)
	}
	if !diagnosticsContainCode(diagnostics, "SDKGEN-W511") {
		t.Fatalf("missing source operation omission diagnostic: %#v", diagnostics)
	}
}

func TestOperationOmissionRemovesOwnedServerCallbackContract(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Callback omission","version":"1"},
  "paths":{
    "/source":{"get":{
      "operationId":"getSource",
      "requestBody":{"required":true,"content":{"application/json":{"schema":{
        "type":"object","properties":{"callbackURL":{"type":"string","format":"uri"}}
      }}}},
      "responses":{"202":{"description":"Accepted"}},
      "callbacks":{"completed":{"{$request.body#/callbackURL}":{"post":{
        "operationId":"completedCallback",
        "responses":{"204":{"description":"OK"}}
      }}}}
    }},
    "/ok":{"post":{"operationId":"createItem","responses":{"204":{"description":"OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	registry, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	options, err := registry.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := (Generator{}).Generate(document, options)
	if err != nil {
		t.Fatal(err)
	}
	callbacks := string(artifactByPath(t, artifacts, "server/callbacks.ts"))
	if strings.Contains(callbacks, "getSource") || strings.Contains(callbacks, "completedCallback") ||
		strings.Contains(callbacks, "completed") || strings.Contains(callbacks, "GET /source") {
		t.Fatalf("omitted operation leaked callback server contract:\n%s", callbacks)
	}
}

func TestOperationRestrictionWithAmbiguousSharedSourceOwnershipStaysBlocking(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Ambiguous restriction ownership","version":"1"},
  "paths":{
    "/a":{"get":{"operationId":"getA","responses":{"204":{"description":"OK"}}}},
    "/b":{"get":{"operationId":"getB","responses":{"204":{"description":"OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	shared := ir.SourceLocation{Source: "shared.yaml", Pointer: "#/Shared/get"}
	document.Provenance = map[string]ir.Provenance{
		document.Operations[0].Pointer: {Primary: shared},
		document.Operations[1].Pointer: {Primary: shared},
	}
	document.SemanticRestrictions = []ir.SemanticRestriction{{
		RuleID: "shared",
		Scope:  failure.ScopeOperation,
		Effect: failure.EffectOmitOperation,
		Location: ir.SourceLocation{
			Source:  shared.Source,
			Pointer: shared.Pointer + "/requestBody",
		},
	}}
	_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "SDKGEN-E507" ||
		diagnostics[0].Severity != diagnostic.SeverityError ||
		diagnostics[0].Scope != failure.ScopeDocument ||
		diagnostics[0].Effect != failure.EffectBlock {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestOperationRestrictionWithUnprovenSourceOwnershipStaysBlocking(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Restriction ownership","version":"1"},
  "paths":{
    "/items":{"get":{"operationId":"getItems","responses":{"204":{"description":"OK"}}}}
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	document.SemanticRestrictions = append(document.SemanticRestrictions, ir.SemanticRestriction{
		RuleID: "foreign",
		Scope:  failure.ScopeOperation,
		Effect: failure.EffectOmitOperation,
		Location: ir.SourceLocation{
			Source:  "foreign.yaml",
			Pointer: "#/paths/~1items/get/requestBody",
		},
	})
	_, diagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Code != "SDKGEN-E507" ||
		diagnostics[0].Severity != diagnostic.SeverityError ||
		diagnostics[0].Scope != failure.ScopeDocument ||
		diagnostics[0].Effect != failure.EffectBlock {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}
