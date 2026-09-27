package sdkgen

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"openapi-sdkgen/internal/compiler/compatibility"
	"openapi-sdkgen/internal/diagnostic"
)

func TestCollectModePreservesSchemaContextAcrossNestedExternalSources(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	schema := filepath.Join(directory, "schema.yaml")
	child := filepath.Join(directory, "child.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.0.3
info: {title: Nested external schema context, version: "1"}
paths: {}
components:
  schemas:
    Root:
      $ref: schema.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(schema, []byte(`type: object
properties:
  child:
    $ref: child.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, []byte("false\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := CompileFileResultWithOptions(root, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, value := range result.Diagnostics {
		if value.Rule == compatibility.RuleSchemaBoolean30 && strings.HasSuffix(filepath.ToSlash(value.Location.Source), "/child.yaml") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v, want nested external boolean Schema compatibility finding", result.Diagnostics)
	}
}

func TestNestedExternalSchemaBlockerStopsStructuredAndDirectCompilation(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	schema := filepath.Join(directory, "schema.yaml")
	child := filepath.Join(directory, "child.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.0.3
info: {title: Nested external schema blocker, version: "1"}
paths: {}
components:
  schemas:
    Root:
      $ref: schema.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(schema, []byte(`type: object
properties:
  child:
    $ref: child.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, []byte("type: [string, number]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := CompileFileResultWithOptions(root, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil {
		t.Fatalf("collect mode built IR past nested schema blocker: %#v", result.Document)
	}
	var found bool
	for _, value := range result.Diagnostics {
		if value.Code == "SDKGEN-E140" &&
			value.Rule == compatibility.RuleSchemaNullableTypes30 &&
			strings.HasSuffix(filepath.ToSlash(value.Location.Source), "/child.yaml") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v, want nested external schema blocker", result.Diagnostics)
	}

	if _, err := CompileFile(root); err == nil || !strings.Contains(err.Error(), "type array") {
		t.Fatalf("direct compile error = %v, want nested schema compatibility blocker", err)
	}
}

func TestDirectCompileBlocksNestedExternalSchemaCompatibilityReject(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	schema := filepath.Join(directory, "schema.yaml")
	child := filepath.Join(directory, "child.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.0.3
info: {title: Nested external schema reject, version: "1"}
paths: {}
components:
  schemas:
    Root:
      $ref: schema.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(schema, []byte(`type: object
properties:
  child:
    $ref: child.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, []byte(`contains:
  type: string
`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := CompileFile(root); err == nil ||
		!strings.Contains(err.Error(), compatibility.RuleSchemaKeywords30) &&
			!strings.Contains(err.Error(), "contains") {
		t.Fatalf("direct compile error = %v, want nested external schema compatibility rejection", err)
	}

	result, err := CompileFileResultWithOptions(root, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil {
		t.Fatalf("collect mode built document despite nested blocking schema finding: %#v", result.Document)
	}
	var found bool
	for _, value := range result.Diagnostics {
		if value.Rule == compatibility.RuleSchemaKeywords30 &&
			value.Code == "SDKGEN-E140" &&
			strings.HasSuffix(filepath.ToSlash(value.Location.Source), "/child.yaml") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v, want nested external COMP-SCHEMA-005 blocker", result.Diagnostics)
	}
}

func TestAmbiguousReferencedContextsRemainFailClosed(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	shared := filepath.Join(directory, "shared.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.0.3
info: {title: Ambiguous referenced contexts, version: "1"}
paths:
  /items:
    get:
      operationId: getItems
      parameters:
        - $ref: shared.yaml
      responses:
        "200":
          $ref: shared.yaml
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shared, []byte(`name: Content-Type
in: header
description: shared
schema:
  type: string
`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := CompileFileResultWithOptions(root, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil || !diagnostic.HasErrors(result.Diagnostics) {
		t.Fatalf("ambiguous multi-context source was guessed instead of blocked: %#v", result)
	}
}

func TestCollectModeReportsIndependentMissingReferenceSources(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.0
info: {title: Missing sources, version: "1"}
paths: {}
components:
  schemas:
    One: {$ref: one.yaml#/Thing}
    Two: {$ref: two.yaml#/Thing}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	failFast, err := CompileFileResultWithOptions(root, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if failFast.Document != nil || len(failFast.Diagnostics) != 1 || failFast.Diagnostics[0].Code != "SDKGEN-E120" {
		t.Fatalf("fail-fast result = %#v", failFast)
	}

	collected, err := CompileFileResultWithOptions(root, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	if collected.Document != nil {
		t.Fatalf("collect mode built poisoned IR: %#v", collected.Document)
	}
	var referenceErrors int
	for _, value := range collected.Diagnostics {
		if value.Code == "SDKGEN-E120" {
			referenceErrors++
		}
	}
	if referenceErrors != 2 {
		t.Fatalf("diagnostics = %#v, want two independent E120 findings", collected.Diagnostics)
	}
	var unavailable int
	for _, value := range collected.Coverage {
		if value.Analyzer == "reference.source" && value.Status == diagnostic.CoverageSkipped {
			unavailable++
		}
	}
	if unavailable != 2 {
		t.Fatalf("coverage = %#v, want two unavailable reference branches", collected.Coverage)
	}
}

func TestCollectModeReportsEveryOccurrenceOfSameMissingReferenceSource(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.0
info: {title: Repeated missing source, version: "1"}
paths: {}
components:
  schemas:
    One: {$ref: missing.yaml#/One}
    Two: {$ref: missing.yaml#/Two}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := CompileFileResultWithOptions(root, CompileOptions{DiagnosticMode: diagnostic.ModeCollect})
	if err != nil {
		t.Fatal(err)
	}
	var referenceErrors, unavailable int
	for _, value := range result.Diagnostics {
		if value.Code == "SDKGEN-E120" {
			referenceErrors++
		}
	}
	for _, value := range result.Coverage {
		if value.Analyzer == "reference.source" && value.Status == diagnostic.CoverageSkipped {
			unavailable++
		}
	}
	if referenceErrors != 2 || unavailable != 2 {
		t.Fatalf("diagnostics=%#v coverage=%#v, want one failure record per reference occurrence", result.Diagnostics, result.Coverage)
	}
}

func TestCollectModeContinuesHealthyReferenceBranchAfterBrokenSibling(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.0
info: {title: Reference branches, version: "1"}
paths: {}
components:
  schemas:
    Healthy: {$ref: healthy.yaml#/Thing}
    Missing: {$ref: missing.yaml#/Thing}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "healthy.yaml"), []byte(`Thing:
  type: object
  properties:
    nested: {$ref: nested.yaml#/Nested}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "nested.yaml"), []byte(`Nested:
  type: string
`), 0o600); err != nil {
		t.Fatal(err)
	}

	metrics := &compilationMetrics{}
	result, err := CompileFileResultWithOptions(root, CompileOptions{
		DiagnosticMode: diagnostic.ModeCollect,
		metrics:        metrics,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil {
		t.Fatalf("collect mode built poisoned IR: %#v", result.Document)
	}
	if metrics.SourceDecodes != 1 || metrics.ReferenceSourceDecodes != 2 {
		t.Fatalf("decode metrics = entry:%d reference:%d, want 1 + 2 unique physical sources", metrics.SourceDecodes, metrics.ReferenceSourceDecodes)
	}
	var missing int
	for _, value := range result.Diagnostics {
		if value.Code == "SDKGEN-E120" {
			missing++
		}
	}
	if missing != 1 {
		t.Fatalf("diagnostics = %#v, want only missing sibling failure", result.Diagnostics)
	}
}

func TestCollectModeDecodesBrokenLocalPhysicalSourceOnceAcrossContexts(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	shared := filepath.Join(directory, "shared.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.0
info: {title: Broken local contexts, version: "1"}
paths:
  /items:
    get:
      parameters:
        - {$ref: shared.yaml}
      responses:
        "200": {$ref: shared.yaml}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shared, []byte("description: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	metrics := &compilationMetrics{}
	result, err := CompileFileResultWithOptions(root, CompileOptions{
		DiagnosticMode: diagnostic.ModeCollect,
		metrics:        metrics,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil {
		t.Fatalf("collect mode built poisoned IR: %#v", result.Document)
	}
	if metrics.ReferenceSourceDecodes != 1 {
		t.Fatalf("reference source decode attempts = %d, want one per physical source", metrics.ReferenceSourceDecodes)
	}
}

func TestReferenceDiscoveryEvaluatesOnePhysicalSourceInEveryRegisteredContext(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	shared := filepath.Join(directory, "shared.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.0
info: {title: Contexts, version: "1"}
paths:
  /items:
    get:
      parameters:
        - {$ref: shared.yaml}
      responses:
        "200": {$ref: shared.yaml}
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shared, []byte(`description: shared
`), 0o600); err != nil {
		t.Fatal(err)
	}

	source, err := loadFileInput(root)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeInputValue(source.data, nil)
	if err != nil {
		t.Fatal(err)
	}
	collector := &diagnostic.Collector{}
	options := CompileOptions{
		DiagnosticMode: diagnostic.ModeCollect,
		diagnostics:    collector,
		sourceCache:    newDecodedSourceCache(),
	}
	if err := options.sourceCache.remember(source.filePath, decodedSource{data: source.data, value: decoded}); err != nil {
		t.Fatal(err)
	}
	effective, _, err := prepareCompatibilityValue(source.display, decoded, &options)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := collectReferenceGraphDiagnostics(source, effective, &options); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(shared)
	if err != nil {
		t.Fatal(err)
	}
	contexts := options.compatibilitySession.sourceContexts(resolved)
	if len(contexts) != 2 {
		t.Fatalf("source contexts = %#v, want two distinct OpenAPI object contexts", contexts)
	}
	var effectiveViews int
	for key := range options.compatibilitySession.effectiveSources {
		if key.source == resolved {
			effectiveViews++
		}
	}
	if effectiveViews != 2 {
		t.Fatalf("effective views = %d, want one per source/context", effectiveViews)
	}
}

func TestCollectModeDoesNotFetchUnallowlistedRemoteReferences(t *testing.T) {
	var requests atomic.Int32
	remote := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		_, _ = response.Write([]byte("Thing: {type: string}\n"))
	}))
	defer remote.Close()

	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.0
info: {title: Remote policy, version: "1"}
paths: {}
components:
  schemas:
    Remote: {$ref: "`+remote.URL+`/schema.yaml#/Thing"}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := CompileFileResultWithOptions(root, CompileOptions{
		DiagnosticMode:        diagnostic.ModeCollect,
		remoteReferenceClient: remote.Client(),
		remoteReferenceLookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil || !diagnostic.HasErrors(result.Diagnostics) {
		t.Fatalf("result = %#v", result)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("remote requests = %d, want 0 without allowlist", got)
	}
}

func TestCollectModeReportsRemoteFailureAfterBrokenLocalSibling(t *testing.T) {
	var requests atomic.Int32
	remote := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		http.Error(response, "unavailable", http.StatusBadGateway)
	}))
	defer remote.Close()

	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.0
info: {title: Mixed broken references, version: "1"}
paths: {}
components:
  schemas:
    Local: {$ref: missing.yaml#/Thing}
    Remote: {$ref: "`+remote.URL+`/schema.yaml#/Thing"}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := CompileFileResultWithOptions(root, CompileOptions{
		DiagnosticMode:        diagnostic.ModeCollect,
		RemoteRefAllowlist:    []string{remote.URL},
		UpdateRefLock:         true,
		remoteReferenceClient: remote.Client(),
		remoteReferenceLookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil {
		t.Fatalf("collect mode built poisoned IR: %#v", result.Document)
	}
	var references int
	for _, value := range result.Diagnostics {
		if value.Code == "SDKGEN-E120" {
			references++
		}
	}
	if references != 2 {
		t.Fatalf("diagnostics = %#v, want local and remote E120 findings", result.Diagnostics)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("remote requests = %d, want one discovery request", got)
	}
}

func TestCollectModeReusesSuccessfulRemoteSourceAcrossDiscoveryAndBundling(t *testing.T) {
	var requests atomic.Int32
	remote := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		_, _ = response.Write([]byte("Thing:\n  type: string\n"))
	}))
	defer remote.Close()

	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.0
info: {title: Remote reuse, version: "1"}
paths: {}
components:
  schemas:
    One: {$ref: "`+remote.URL+`/schema.yaml#/Thing"}
    Two: {$ref: "`+remote.URL+`/schema.yaml#/Thing"}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	metrics := &compilationMetrics{}
	result, err := CompileFileResultWithOptions(root, CompileOptions{
		DiagnosticMode:        diagnostic.ModeCollect,
		RemoteRefAllowlist:    []string{remote.URL},
		UpdateRefLock:         true,
		remoteReferenceClient: remote.Client(),
		remoteReferenceLookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
		metrics: metrics,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil || diagnostic.HasErrors(result.Diagnostics) {
		t.Fatalf("collect result = %#v", result)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("remote requests = %d, want one physical fetch", got)
	}
	if metrics.RemoteReferenceSourceDecodes != 1 {
		t.Fatalf("remote source decodes = %d, want 1", metrics.RemoteReferenceSourceDecodes)
	}
}

func TestDirectRemoteCompatibilityBlockerStopsBeforeNestedFetch(t *testing.T) {
	var schemaRequests atomic.Int32
	var laterRequests atomic.Int32
	remote := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/schema.yaml":
			schemaRequests.Add(1)
			_, _ = response.Write([]byte("type: [string, number]\nproperties:\n  later:\n    $ref: later.yaml\n"))
		case "/later.yaml":
			laterRequests.Add(1)
			_, _ = response.Write([]byte("type: string\n"))
		default:
			http.NotFound(response, request)
		}
	}))
	defer remote.Close()

	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.0.3
info: {title: Remote fail-fast compatibility, version: "1"}
paths: {}
components:
  schemas:
    Root:
      $ref: "`+remote.URL+`/schema.yaml"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := CompileFileWithOptions(root, CompileOptions{
		RemoteRefAllowlist:    []string{remote.URL},
		UpdateRefLock:         true,
		remoteReferenceClient: remote.Client(),
		remoteReferenceLookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "type array") {
		t.Fatalf("direct compile error = %v, want remote compatibility blocker", err)
	}
	if got := schemaRequests.Load(); got != 1 {
		t.Fatalf("schema requests = %d, want 1", got)
	}
	if got := laterRequests.Load(); got != 0 {
		t.Fatalf("nested requests after fail-fast blocker = %d, want 0", got)
	}
	if _, statErr := os.Stat(defaultReferenceLockPath(root)); !os.IsNotExist(statErr) {
		t.Fatalf("blocked direct compile published reference lock: %v", statErr)
	}
}

func TestCollectModePreservesSchemaContextAcrossNestedRemoteSources(t *testing.T) {
	var requests atomic.Int32
	remote := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		switch request.URL.Path {
		case "/schema.yaml":
			_, _ = response.Write([]byte("type: object\nproperties:\n  child:\n    $ref: child.yaml\n"))
		case "/child.yaml":
			_, _ = response.Write([]byte("false\n"))
		default:
			http.NotFound(response, request)
		}
	}))
	defer remote.Close()

	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.0.3
info: {title: Nested remote schema context, version: "1"}
paths: {}
components:
  schemas:
    Root:
      $ref: "`+remote.URL+`/schema.yaml"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := CompileFileResultWithOptions(root, CompileOptions{
		DiagnosticMode:        diagnostic.ModeCollect,
		RemoteRefAllowlist:    []string{remote.URL},
		UpdateRefLock:         true,
		remoteReferenceClient: remote.Client(),
		remoteReferenceLookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document == nil || diagnostic.HasErrors(result.Diagnostics) {
		t.Fatalf("collect result = %#v", result)
	}
	var found bool
	for _, value := range result.Diagnostics {
		if value.Rule == compatibility.RuleSchemaBoolean30 &&
			strings.HasSuffix(value.Location.Source, "/child.yaml") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("diagnostics = %#v, want nested remote boolean Schema compatibility finding", result.Diagnostics)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("remote requests = %d, want one request per physical source", got)
	}
}

func TestFailFastRemoteCompatibilityBlockerReportsSkippedIR(t *testing.T) {
	remote := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/schema.yaml":
			_, _ = response.Write([]byte("type: object\nproperties:\n  child:\n    $ref: child.yaml\n"))
		case "/child.yaml":
			_, _ = response.Write([]byte("type: [string, number]\n"))
		default:
			http.NotFound(response, request)
		}
	}))
	defer remote.Close()

	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.0.3
info: {title: Remote nested blocker, version: "1"}
paths: {}
components:
  schemas:
    Root:
      $ref: "`+remote.URL+`/schema.yaml"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := CompileFileResultWithOptions(root, CompileOptions{
		RemoteRefAllowlist:    []string{remote.URL},
		UpdateRefLock:         true,
		remoteReferenceClient: remote.Client(),
		remoteReferenceLookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil {
		t.Fatalf("fail-fast result built IR past remote compatibility blocker: %#v", result)
	}
	var foundDiagnostic, skippedNormalize, skippedIR bool
	for _, value := range result.Diagnostics {
		if value.Code == "SDKGEN-E140" && value.Rule == compatibility.RuleSchemaNullableTypes30 {
			foundDiagnostic = true
		}
	}
	for _, skipped := range result.SkippedPhases {
		switch skipped.Phase {
		case diagnostic.PhaseNormalize:
			skippedNormalize = true
		case diagnostic.PhaseIR:
			skippedIR = true
		}
	}
	if !foundDiagnostic || !skippedNormalize || !skippedIR {
		t.Fatalf("result = %#v, want E140 plus skipped normalize and IR", result)
	}
}

func TestCollectModeAccumulatesIndependentAllowedRemoteSourceFailures(t *testing.T) {
	var requests atomic.Int32
	remote := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		http.NotFound(response, request)
	}))
	defer remote.Close()

	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.0
info: {title: Remote failures, version: "1"}
paths: {}
components:
  schemas:
    One: {$ref: "`+remote.URL+`/one.yaml#/Thing"}
    Two: {$ref: "`+remote.URL+`/two.yaml#/Thing"}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := CompileFileResultWithOptions(root, CompileOptions{
		DiagnosticMode:        diagnostic.ModeCollect,
		RemoteRefAllowlist:    []string{remote.URL},
		UpdateRefLock:         true,
		remoteReferenceClient: remote.Client(),
		remoteReferenceLookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil {
		t.Fatalf("collect mode built poisoned IR: %#v", result.Document)
	}
	var referenceErrors int
	for _, value := range result.Diagnostics {
		if value.Code == "SDKGEN-E120" {
			referenceErrors++
		}
	}
	if referenceErrors != 2 {
		t.Fatalf("diagnostics = %#v, want two independent remote E120 findings", result.Diagnostics)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("remote requests = %d, want one per independent physical source", got)
	}
}

func TestCollectModeDecodesBrokenRemotePhysicalSourceOnceAcrossContexts(t *testing.T) {
	var requests atomic.Int32
	remote := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		_, _ = response.Write([]byte("Thing: [\\n"))
	}))
	defer remote.Close()

	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.0
info: {title: Broken remote contexts, version: "1"}
paths:
  /items:
    get:
      parameters:
        - {$ref: "`+remote.URL+`/shared.yaml"}
      responses:
        "200": {$ref: "`+remote.URL+`/shared.yaml"}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	metrics := &compilationMetrics{}
	result, err := CompileFileResultWithOptions(root, CompileOptions{
		DiagnosticMode:        diagnostic.ModeCollect,
		RemoteRefAllowlist:    []string{remote.URL},
		UpdateRefLock:         true,
		remoteReferenceClient: remote.Client(),
		remoteReferenceLookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
		metrics: metrics,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil {
		t.Fatalf("collect mode built poisoned IR: %#v", result.Document)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("remote requests = %d, want one fetch for the physical source", got)
	}
	if metrics.RemoteReferenceSourceDecodes != 1 {
		t.Fatalf("remote source decode attempts = %d, want one per physical source", metrics.RemoteReferenceSourceDecodes)
	}
}

func TestCollectReferenceDiagnosticRedactsRemoteSecrets(t *testing.T) {
	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.0
info: {title: Secret reference, version: "1"}
paths: {}
components:
  schemas:
    Remote: {$ref: "https://user:secret@example.test/schema.yaml?token=alpha#/Thing"}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := CompileFileResultWithOptions(root, CompileOptions{
		DiagnosticMode:     diagnostic.ModeCollect,
		RemoteRefAllowlist: []string{"https://example.test"},
		UpdateRefLock:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	rendered := diagnostic.RenderHuman(result.Diagnostics, result.SkippedPhases, result.Coverage...)
	for _, secret := range []string{"user", "secret", "token=alpha"} {
		if strings.Contains(rendered, secret) {
			t.Fatalf("diagnostics leaked %q: %s", secret, rendered)
		}
	}
	if !strings.Contains(rendered, "https://example.test/schema.yaml") {
		t.Fatalf("sanitized reference missing from diagnostics: %s", rendered)
	}
}

func TestCollectModeAttemptsFailingRemotePhysicalSourceOnceAcrossContexts(t *testing.T) {
	var requests atomic.Int32
	remote := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		http.NotFound(response, request)
	}))
	defer remote.Close()

	directory := t.TempDir()
	root := filepath.Join(directory, "openapi.yaml")
	if err := os.WriteFile(root, []byte(`openapi: 3.1.0
info: {title: Shared remote failure, version: "1"}
paths:
  /items:
    get:
      parameters:
        - {$ref: "`+remote.URL+`/shared.yaml"}
      responses:
        "200": {$ref: "`+remote.URL+`/shared.yaml"}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := CompileFileResultWithOptions(root, CompileOptions{
		DiagnosticMode:        diagnostic.ModeCollect,
		RemoteRefAllowlist:    []string{remote.URL},
		UpdateRefLock:         true,
		remoteReferenceClient: remote.Client(),
		remoteReferenceLookup: func(context.Context, string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document != nil {
		t.Fatalf("collect mode built poisoned IR: %#v", result.Document)
	}
	var referenceErrors int
	for _, value := range result.Diagnostics {
		if value.Code == "SDKGEN-E120" {
			referenceErrors++
		}
	}
	if referenceErrors != 2 {
		t.Fatalf("diagnostics = %#v, want one finding per failed reference occurrence", result.Diagnostics)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("remote requests = %d, want one attempt for the shared physical source", got)
	}
}
