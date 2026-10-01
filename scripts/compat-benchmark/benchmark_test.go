package main

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
)

func TestStrictTypecheckRecordsElapsedTimeAndKeepsOutcome(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node is required to verify benchmark compiler process timing")
	}
	for _, test := range []struct {
		name, script, status string
		timeout              time.Duration
	}{
		{"success", "process.exit(0)", "pass", time.Minute},
		{"failure", "console.error('compiler failed'); process.exit(1)", "fail", time.Minute},
		{"timeout", "setInterval(() => {}, 1000)", "timeout", time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			toolchain := t.TempDir()
			compiler := filepath.Join(toolchain, "node_modules", "typescript", "lib", "tsc.js")
			if err := os.MkdirAll(filepath.Dir(compiler), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(compiler, []byte(test.script), 0o644); err != nil {
				t.Fatal(err)
			}
			result := strictTypecheck(t.TempDir(), toolchain, test.timeout)
			if result.Status != test.status || result.DurationMillis == nil || *result.DurationMillis <= 0 {
				t.Fatalf("typecheck result = %#v", result)
			}
		})
	}
}

func TestOperationRetentionDeduplicatesOneOmittedOperation(t *testing.T) {
	document := &ir.Document{
		Operations: []ir.Operation{
			{Pointer: "#/paths/~1bad/get", Method: "get", Path: "/bad", OperationID: "bad"},
			{Pointer: "#/paths/~1ok/get", Method: "get", Path: "/ok", OperationID: "ok"},
		},
		SemanticRestrictions: []ir.SemanticRestriction{{
			Scope:        failure.ScopeOperation,
			Effect:       failure.EffectOmitOperation,
			OwnerPointer: "#/paths/~1bad/get",
		}},
	}
	diagnostics := []diagnostic.Diagnostic{
		{Scope: failure.ScopeOperation, Effect: failure.EffectOmitOperation, Route: "GET /bad", Operation: "bad"},
		{Scope: failure.ScopeOperation, Effect: failure.EffectOmitOperation, Route: "GET /bad", Operation: "bad"},
	}
	metric := operationRetention(document, diagnostics)
	if !metric.Available || metric.Total != 2 || metric.Omitted != 1 || metric.Retained != 1 {
		t.Fatalf("metric = %#v", metric)
	}
	if metric.Rate == nil || math.Abs(*metric.Rate-0.5) > 0.000001 {
		t.Fatalf("rate = %#v", metric.Rate)
	}
}

func TestCompatibilityMetricKeepsZeroDenominatorUnavailable(t *testing.T) {
	empty := compatibilityMetrics(nil)
	if empty.Total != 0 || empty.Rate != nil {
		t.Fatalf("empty compatibility = %#v", empty)
	}
	values := []diagnostic.Diagnostic{
		{Rule: "COMP-A", Action: "preserve-extension"},
		{Rule: "COMP-B", Action: "normalize"},
		{Rule: "COMP-C", Action: "reject"},
		{Code: "SDKGEN-W999"},
	}
	metric := compatibilityMetrics(values)
	if metric.Preserved != 2 || metric.Rejected != 1 || metric.Total != 3 {
		t.Fatalf("metric = %#v", metric)
	}
	if metric.Rate == nil || math.Abs(*metric.Rate-(2.0/3.0)) > 0.000001 {
		t.Fatalf("rate = %#v", metric.Rate)
	}
}

func TestDetectFeaturesCoversVersionReferencesMediaSecurityAndSchemas(t *testing.T) {
	value, err := decodeFeatureInput([]byte(`openapi: 3.1.0
components:
  securitySchemes:
    token:
      type: apiKey
      in: header
      name: X-Token
  schemas:
    A:
      type: [string, "null"]
      allOf:
        - $ref: '#/components/schemas/B'
      additionalProperties: true
      prefixItems: []
webhooks:
  changed:
    post:
      responses:
        "204":
          description: ok
paths:
  /items:
    post:
      servers:
        - url: https://example.test
      requestBody:
        required: true
        content:
          multipart/form-data:
            schema:
              type: object
            encoding:
              value: {}
      responses:
        "200":
          description: ok
          links:
            next:
              operationId: next
`))
	if err != nil {
		t.Fatal(err)
	}
	features, version := detectFeatures(value)
	if version != "3.1.0" {
		t.Fatalf("version = %q", version)
	}
	want := []string{
		"oas.version.3.1",
		"reference.local",
		"operation.request-body",
		"operation.required-request-body",
		"operation.servers",
		"document.webhooks",
		"response.links",
		"media.multipart",
		"media.encoding",
		"security.api-key",
		"schema.allOf",
		"schema.additional-properties.boolean",
		"schema.type-array",
		"schema.null-type",
		"schema.prefix-items",
	}
	if !reflect.DeepEqual(features, want) {
		t.Fatalf("features = %#v\nwant = %#v", features, want)
	}
}

func TestDetectFeaturesIgnoresOpaqueAndNamedMapKeywordCollisions(t *testing.T) {
	value, err := decodeFeatureInput([]byte(`openapi: 3.1.0
info:
  title: Opaque
  version: "1"
  x-sample:
    $ref: https://example.test/external.yaml
    allOf:
      - type: ["string", "null"]
    format: binary
components:
  schemas:
    properties:
      type: string
  callbacks:
    callbacks:
      '{$request.body#/url}':
        post:
          responses:
            "204":
              description: ok
paths:
  /items:
    get:
      operationId: getItems
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                type: object
                example:
                  $ref: https://example.test/example.yaml
                  allOf:
                    - type: ["string", "null"]
                  format: binary
`))
	if err != nil {
		t.Fatal(err)
	}
	features, _ := detectFeatures(value)
	for _, forbidden := range []string{
		"reference.external",
		"operation.callbacks",
		"schema.allOf",
		"schema.type-array",
		"schema.null-type",
		"media.binary",
	} {
		if slices.Contains(features, forbidden) {
			t.Fatalf("opaque or named-map collision produced feature %q: %#v", forbidden, features)
		}
	}
}

func TestSafePathsRejectTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := safeCorpusPath(root, "../escape.yaml"); err == nil {
		t.Fatal("expected corpus traversal rejection")
	}
	if _, err := safeArtifactPath(root, "../escape.ts"); err == nil {
		t.Fatal("expected artifact traversal rejection")
	}
	if _, err := safeCorpusPath(root, "nested/openapi.yaml"); err != nil {
		t.Fatal(err)
	}
	if _, err := safeArtifactPath(root, "nested/index.ts"); err != nil {
		t.Fatal(err)
	}
}

func TestBenchmarkDocumentIsDeterministicAcrossCheckoutRoots(t *testing.T) {
	makeInput := func(root string) string {
		if err := os.MkdirAll(filepath.Join(root, "specs"), 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "specs", "openapi.json")
		if err := os.WriteFile(path, []byte(`{
  "openapi":"3.1.1",
  "info":{"title":"deterministic","version":"1"},
  "paths":{}
}`), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	corpus := corpusSpec{ID: "same", Cohort: "fixture", Input: "specs/openapi.json"}
	first, err := benchmarkDocument(corpus, makeInput(filepath.Join(t.TempDir(), "checkout-a")), func(string) verificationResult {
		t.Fatal("typecheck must not run for blocked input")
		return verificationResult{}
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := benchmarkDocument(corpus, makeInput(filepath.Join(t.TempDir(), "checkout-b")), func(string) verificationResult {
		t.Fatal("typecheck must not run for blocked input")
		return verificationResult{}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("cross-root benchmark differs:\nfirst=%#v\nsecond=%#v", first, second)
	}
	for _, finding := range first.Findings {
		if finding.Location.Source != "$root" {
			t.Fatalf("finding source = %q, want $root", finding.Location.Source)
		}
	}
	for _, coverage := range first.Coverage {
		if coverage.Location != nil && coverage.Location.Source != "$root" {
			t.Fatalf("coverage source = %q, want $root", coverage.Location.Source)
		}
	}
}

func TestBenchmarkDocumentRecordsBlockingAndSuccessfulDocuments(t *testing.T) {
	root := t.TempDir()
	blocked := filepath.Join(root, "blocked.json")
	if err := os.WriteFile(blocked, []byte(`{
  "openapi":"3.1.1",
  "info":{"title":"blocked","version":"1"},
  "paths":{}
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	blockedResult, err := benchmarkDocument(corpusSpec{ID: "blocked", Cohort: "fixture", Input: "blocked.json"}, blocked, func(string) verificationResult {
		t.Fatal("typecheck must not run for blocked input")
		return verificationResult{}
	})
	if err != nil {
		t.Fatal(err)
	}
	if blockedResult.Diagnostics.Errors == 0 || blockedResult.Generation.Status != "blocked" || blockedResult.Typecheck.Status != "not-run" || blockedResult.DocumentSuccess {
		t.Fatalf("blocked result = %#v", blockedResult)
	}

	success := filepath.Join(root, "success.json")
	if err := os.WriteFile(success, []byte(`{
  "openapi":"3.1.1",
  "info":{"title":"success","version":"1"},
  "paths":{
    "/items":{"get":{"operationId":"listItems","responses":{"204":{"description":"OK"}}}}
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	successResult, err := benchmarkDocument(corpusSpec{ID: "success", Cohort: "fixture", Input: "success.json"}, success, func(directory string) verificationResult {
		if _, err := os.Stat(filepath.Join(directory, "index.ts")); err != nil {
			t.Fatalf("missing generated index.ts: %v", err)
		}
		return verificationResult{Status: "pass"}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !successResult.DiscoveryComplete || successResult.Diagnostics.Errors != 0 || successResult.Generation.Status != "pass" || successResult.Typecheck.Status != "pass" || !successResult.DocumentSuccess {
		t.Fatalf("success result = %#v", successResult)
	}
	if !successResult.OperationRetention.Available || successResult.OperationRetention.Total != 1 || successResult.OperationRetention.Retained != 1 {
		t.Fatalf("operation retention = %#v", successResult.OperationRetention)
	}
}

func TestBenchmarkDocumentSeparatesDefaultFailureFromServerCapabilitySupport(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "webhook.json")
	if err := os.WriteFile(input, []byte(`{
  "openapi":"3.1.1",
  "info":{"title":"server capability","version":"1"},
  "paths":{
    "/events":{"get":{"operationId":"listEvents","responses":{"204":{"description":"OK"}}}}
  },
  "webhooks":{
    "event":{
      "post":{
        "operationId":"receiveEvent",
        "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","properties":{"id":{"type":"string"}}}}}},
        "responses":{"204":{"description":"OK"}}
      }
    }
  }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	typechecks := 0
	result, err := benchmarkDocument(corpusSpec{ID: "webhook", Cohort: "fixture", Input: "webhook.json"}, input, func(directory string) verificationResult {
		typechecks++
		if _, err := os.Stat(filepath.Join(directory, "index.ts")); err != nil {
			t.Fatalf("missing generated index.ts: %v", err)
		}
		return verificationResult{Status: "pass"}
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.DocumentSuccess {
		t.Fatalf("default client-only result unexpectedly succeeded: %#v", result)
	}
	if !result.CapabilityAdjustedSuccess {
		t.Fatalf("server-capability result did not recover document support: %#v", result)
	}
	if len(result.SupportProfiles) != 1 || result.SupportProfiles[0].Name != "server-addon" || !result.SupportProfiles[0].Success {
		t.Fatalf("support profiles = %#v", result.SupportProfiles)
	}
	if typechecks != 1 {
		t.Fatalf("typechecks = %d, want server profile only", typechecks)
	}
}

func TestSummarizeDocumentsDoesNotCreateCompositeScore(t *testing.T) {
	documents := []documentResult{
		{
			Cohort: "holdout", DocumentSuccess: true, CapabilityAdjustedSuccess: true, DiscoveryComplete: true,
			Generation: verificationResult{Status: "pass"}, Typecheck: verificationResult{Status: "pass"},
			OperationRetention: operationMetric{Available: true, Total: 2, Retained: 1, Omitted: 1},
			Compatibility:      compatibilityMetric{Preserved: 1, Total: 1, Actions: map[string]int{"preserve": 1}, Rules: map[string]int{"A": 1}},
			Features:           []string{"oas.version.3.1"},
		},
		{
			Cohort: "holdout", DocumentSuccess: false, CapabilityAdjustedSuccess: true,
			Generation: verificationResult{Status: "blocked"}, Typecheck: verificationResult{Status: "not-run"},
			Compatibility: compatibilityMetric{Actions: map[string]int{}, Rules: map[string]int{}},
		},
	}
	summary := summarizeDocuments("holdout", documents)
	if summary.Documents != 2 || summary.SuccessfulDocuments != 1 || summary.CapabilityAdjustedDocuments != 2 || summary.GeneratedDocuments != 1 || summary.TypecheckedDocuments != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	if summary.DocumentSuccessRate == nil || math.Abs(*summary.DocumentSuccessRate-0.5) > 0.000001 {
		t.Fatalf("success rate = %#v", summary.DocumentSuccessRate)
	}
	if summary.CapabilityAdjustedSuccessRate == nil || *summary.CapabilityAdjustedSuccessRate != 1 {
		t.Fatalf("capability-adjusted rate = %#v", summary.CapabilityAdjustedSuccessRate)
	}
	if summary.GeneratedVerificationRate == nil || *summary.GeneratedVerificationRate != 1 {
		t.Fatalf("verification rate = %#v", summary.GeneratedVerificationRate)
	}
}

func TestWriteTypecheckFilesCreatesESMPackageMetadata(t *testing.T) {
	directory := t.TempDir()
	if err := writeTypecheckFiles(directory); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(string(data)), `{"type":"module","private":true}`; got != want {
		t.Fatalf("package.json = %q, want %q", got, want)
	}
}

func TestValidateTypecheckToolchainRejectsMissingCompiler(t *testing.T) {
	err := validateTypecheckToolchain(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "typescript compiler unavailable") {
		t.Fatalf("validateTypecheckToolchain error = %v, want missing compiler", err)
	}
}
