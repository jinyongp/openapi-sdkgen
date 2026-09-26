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

func TestPrepareAccumulatesIndependentTargetSupportDiagnostics(t *testing.T) {
	document := &ir.Document{
		Raw: map[string]any{
			"paths": map[string]any{
				"/things/{id}":   map[string]any{},
				"/things/{name}": map[string]any{},
			},
			"webhooks": map[string]any{"event": map[string]any{}},
		},
		ComponentSchemas: map[string]map[string]any{
			"Dynamic": {"$dynamicRef": "#node"},
		},
		Operations: []ir.Operation{
			{
				OperationID: "same",
				Method:      "GET",
				Path:        "/things/{id}",
				Raw: map[string]any{
					"security": "invalid",
					"responses": map[string]any{
						"200": map[string]any{"content": map[string]any{"text/event-stream": map[string]any{}}},
					},
				},
			},
			{OperationID: "same", Method: "GET", Path: "/things/{name}"},
		},
	}
	_, values, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	report := diagnostic.RenderHuman(values, nil)
	for _, code := range []string{"SDKGEN-E501", "SDKGEN-E502", "SDKGEN-E503", "SDKGEN-E505", "SDKGEN-E508"} {
		if !strings.Contains(report, code) {
			t.Fatalf("target preflight missing %s:\n%s", code, report)
		}
	}
	for _, value := range values {
		if value.Severity == diagnostic.SeverityError &&
			(value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock) {
			t.Fatalf("target blocking diagnostic lacks document scope: %#v", value)
		}
	}
	if strings.Contains(report, "SDKGEN-E504") {
		t.Fatalf("ordinary path resource collision remained a target diagnostic:\n%s", report)
	}
}

func TestCollectPrepareReturnsNoEmitCapablePlanAndReportsAnalyzerCoverage(t *testing.T) {
	document := &ir.Document{
		Raw: map[string]any{
			"paths": map[string]any{
				"/things/{id}":   map[string]any{},
				"/things/{name}": map[string]any{},
			},
		},
		ComponentSchemas: map[string]map[string]any{
			"Dynamic": {"$dynamicRef": "#node"},
		},
		Operations: []ir.Operation{
			{
				OperationID: "same",
				Method:      "GET",
				Path:        "/things/{id}",
				Raw: map[string]any{
					"security": "invalid",
				},
			},
			{OperationID: "same", Method: "GET", Path: "/things/{name}"},
		},
	}

	failFastPlan, failFastDiagnostics, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !diagnostic.HasErrors(failFastDiagnostics) {
		t.Fatalf("fail-fast diagnostics = %#v, want blocking target findings", failFastDiagnostics)
	}
	if _, err := failFastPlan.Value("typescript"); err != nil {
		t.Fatalf("fail-fast direct Prepare lost compatibility plan: %v", err)
	}

	collectOptions := generator.Options{DiagnosticMode: diagnostic.ModeCollect}
	collectPlan, values, coverage, err := (Generator{}).PrepareWithCoverage(document, collectOptions)
	if err != nil {
		t.Fatal(err)
	}
	report := diagnostic.RenderHuman(values, nil)
	for _, code := range []string{"SDKGEN-E501", "SDKGEN-E503", "SDKGEN-E508"} {
		if !strings.Contains(report, code) {
			t.Fatalf("collect target preflight missing %s:\n%s", code, report)
		}
	}
	if _, err := collectPlan.Value("typescript"); err == nil {
		t.Fatal("collect mode returned an emit-capable plan despite blocking target diagnostics")
	}

	statuses := map[string]diagnostic.CoverageStatus{}
	for _, item := range coverage {
		statuses[item.Analyzer] = item.Status
	}
	for _, analyzer := range []string{
		"target.extensions",
		"target.support",
		"target.visibility",
		"target.lowering",
		"target.links",
		"target.streams",
	} {
		if statuses[analyzer] != diagnostic.CoverageComplete {
			t.Fatalf("coverage[%s] = %q, all coverage = %#v", analyzer, statuses[analyzer], coverage)
		}
	}
	if statuses["target.modules"] != diagnostic.CoverageSkipped ||
		statuses["target.entry-surface"] != diagnostic.CoverageSkipped {
		t.Fatalf("blocking-plan coverage = %#v", coverage)
	}
}

func TestCollectPipelineKeepsBlockingTargetPlanNonEmitCapable(t *testing.T) {
	compiled, err := sdkgen.CompileResultWithOptions([]byte(`{
  "openapi":"3.1.1",
  "info":{"title":"Collect target pipeline","version":"1"},
  "paths":{
    "/items":{"get":{
      "operationId":"same",
      "security":[{"missing":[]}],
      "responses":{"204":{"description":"OK"}}
    }},
    "/other":{"get":{
      "operationId":"same",
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`), sdkgen.CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Document == nil || diagnostic.HasErrors(compiled.Diagnostics) {
		t.Fatalf("compiler result = %#v", compiled)
	}

	options := generator.Options{DiagnosticMode: diagnostic.ModeCollect}
	prepared, err := generator.PrepareCompilation(Generator{}, compiled, options)
	if err != nil {
		t.Fatal(err)
	}
	if !diagnostic.HasErrors(prepared.Diagnostics) {
		t.Fatalf("target diagnostics = %#v, want blocking findings", prepared.Diagnostics)
	}
	if _, err := prepared.Plan.Value("typescript"); err == nil {
		t.Fatal("pipeline retained an emit-capable plan after collect-mode target blockers")
	}
	var detailedCoverage bool
	for _, item := range prepared.Coverage {
		if item.Analyzer == "target.support" && item.Status == diagnostic.CoverageComplete {
			detailedCoverage = true
			break
		}
	}
	if !detailedCoverage {
		t.Fatalf("pipeline coverage = %#v", prepared.Coverage)
	}
}

func TestPrepareAcceptsOrdinaryTemplatedPathCollisionWithServerAddon(t *testing.T) {
	document := &ir.Document{Operations: []ir.Operation{
		pathOperation("deleteByID", "DELETE", "/users/{id}", "id", map[string]any{"type": "integer"}),
		pathOperation("getByName", "GET", "/users/{name}", "name", map[string]any{"type": "string"}),
	}}
	if _, err := (Generator{}).Generate(document, generator.Options{}); err != nil {
		t.Fatalf("client generation rejected ordinary path collision: %v", err)
	}
	registry, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	serverOptions, err := registry.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (Generator{}).Generate(document, serverOptions); err != nil {
		t.Fatalf("server add-on rejected ordinary path collision: %v", err)
	}
}

func TestPrepareReplacesBaseInboundHintWithServerSemanticDiagnostics(t *testing.T) {
	document := &ir.Document{
		Raw: map[string]any{
			"webhooks": map[string]any{
				"bad-a": "not-a-path-item",
				"bad-b": "not-a-path-item",
				"multi": map[string]any{
					"get": map[string]any{"parameters": []any{
						map[string]any{"$ref": "#/components/parameters/MissingGet"},
					}, "requestBody": map[string]any{"$ref": "#/components/requestBodies/MissingGet"},
						"responses": map[string]any{"200": map[string]any{"$ref": "#/components/responses/MissingGet"}}},
					"post": map[string]any{"parameters": []any{
						map[string]any{"$ref": "#/components/parameters/MissingPost"},
					}},
					"additionalOperations": map[string]any{
						"BAD-A": "not-an-operation",
						"BAD-B": 42,
					},
				},
			},
		},
		Operations: []ir.Operation{{
			OperationID: "source",
			Method:      "POST",
			Path:        "/source",
			Raw: map[string]any{
				"callbacks": map[string]any{
					"bad-a": "not-a-callback",
					"bad-b": "not-a-callback",
					"multi": map[string]any{
						"{$request.body#/a}": map[string]any{
							"get": map[string]any{"parameters": []any{
								map[string]any{"$ref": "#/components/parameters/MissingCallbackGet"},
							}, "requestBody": map[string]any{"$ref": "#/components/requestBodies/MissingCallbackGet"},
								"responses": map[string]any{"200": map[string]any{"$ref": "#/components/responses/MissingCallbackGet"}}},
						},
						"{$request.body#/b}": map[string]any{
							"post": map[string]any{"parameters": []any{
								map[string]any{"$ref": "#/components/parameters/MissingCallbackPost"},
							}},
						},
					},
				},
			},
		}},
	}
	_, baseValues, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	baseReport := diagnostic.RenderHuman(baseValues, nil)
	if !strings.Contains(baseReport, "SDKGEN-E505") || !strings.Contains(baseReport, "--with server") {
		t.Fatalf("base inbound diagnostic =\n%s", baseReport)
	}
	for _, value := range baseValues {
		if value.Code == "SDKGEN-E505" && (value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock) {
			t.Fatalf("base inbound scope = %#v", value)
		}
	}

	options, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		t.Fatal(err)
	}
	serverOptions, err := options.Resolve([]string{"server"})
	if err != nil {
		t.Fatal(err)
	}
	_, serverValues, err := (Generator{}).Prepare(document, serverOptions)
	if err != nil {
		t.Fatal(err)
	}
	serverReport := diagnostic.RenderHuman(serverValues, nil)
	if strings.Contains(serverReport, "SDKGEN-E505") || strings.Count(serverReport, "SDKGEN-E506") < 14 {
		t.Fatalf("server inbound diagnostics =\n%s", serverReport)
	}
	hasWebhookFailure := false
	hasCallbackFailure := false
	for _, value := range serverValues {
		if value.Code != "SDKGEN-E506" {
			continue
		}
		if value.Scope != failure.ScopeDocument || value.Effect != failure.EffectBlock {
			t.Fatalf("server inbound scope = %#v", value)
		}
		hasWebhookFailure = hasWebhookFailure || strings.Contains(value.Message, "webhook contracts")
		hasCallbackFailure = hasCallbackFailure || strings.Contains(value.Message, "callback contracts")
	}
	if !hasWebhookFailure || !hasCallbackFailure {
		t.Fatalf("server family coverage = webhook:%v callback:%v\n%s", hasWebhookFailure, hasCallbackFailure, serverReport)
	}
}

func TestPrepareAcceptsValidInboundContractsWithServerAddon(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi": "3.1.0",
  "info": {"title": "Inbound", "version": "1"},
  "paths": {},
  "webhooks": {
    "event": {
      "post": {
        "operationId": "receiveEvent",
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema": {"type": "object"}}}
        },
        "responses": {"204": {"description": "Accepted"}}
      }
    }
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
	_, values, err := (Generator{}).Prepare(document, options)
	if err != nil {
		t.Fatal(err)
	}
	if diagnostic.HasErrors(values) {
		t.Fatal(diagnostic.RenderHuman(values, nil))
	}
}

func TestPrepareRejectsCookieParameterSecurityOwnershipOverlap(t *testing.T) {
	document, err := sdkgen.Compile([]byte(`{
  "openapi": "3.2.0",
  "info": {"title": "Cookie ownership", "version": "1"},
  "components": {
    "securitySchemes": {
      "Session": {"type": "apiKey", "in": "cookie", "name": "csrf-local"}
    }
  },
  "security": [{"Session": []}],
  "paths": {
    "/conflict": {
      "get": {
        "operationId": "getConflict",
        "parameters": [
          {"name": "csrf-local", "in": "cookie", "required": true, "schema": {"type": "string"}}
        ],
        "responses": {"204": {"description": "OK"}}
      }
    },
    "/override": {
      "get": {
        "operationId": "getOverride",
        "parameters": [
          {"name": "csrf-local", "in": "cookie", "required": true, "schema": {"type": "string"}}
        ],
        "security": [],
        "responses": {"204": {"description": "OK"}}
      }
    }
  }
}`))
	if err != nil {
		t.Fatal(err)
	}
	_, values, err := (Generator{}).Prepare(document, generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	report := diagnostic.RenderHuman(values, nil)
	if strings.Count(report, "SDKGEN-E509") != 1 ||
		!strings.Contains(report, `Cookie "csrf-local" is declared as both an operation parameter and security credential`) ||
		!strings.Contains(report, "GET /conflict") {
		t.Fatalf("cookie ownership diagnostic =\n%s", report)
	}
}
