package diagnostic

import (
	"encoding/json"
	"strings"
	"testing"

	"openapi-sdkgen/internal/failure"
)

func TestCollectorSortCountsAndRender(t *testing.T) {
	collector := &Collector{}
	collector.Add(Diagnostic{
		Severity: SeverityWarning,
		Code:     "SDKGEN-W002",
		Phase:    PhaseTarget,
		Location: Location{Source: "contract.yaml", Pointer: "#/paths/~1pets/get"},
		Message:  "redundant declaration",
		Related: []Location{
			{Source: "schema.yaml", Pointer: "#/B"},
			{Source: "schema.yaml", Pointer: "#/A"},
			{Source: "schema.yaml", Pointer: "#/A"},
		},
	})
	collector.Add(Diagnostic{
		Severity: SeverityError,
		Code:     "SDKGEN-E001",
		Phase:    PhaseOpenAPI,
		Location: Location{Source: "contract.yaml", Pointer: "#/paths"},
		Message:  "paths must be an object",
		Hint:     "replace paths with an object",
	})

	values := collector.Diagnostics()
	if len(values) != 2 || values[0].Code != "SDKGEN-E001" {
		t.Fatalf("diagnostics = %#v", values)
	}
	if got := len(values[1].Related); got != 2 {
		t.Fatalf("related locations = %d, want 2", got)
	}
	if counts := Count(values); counts.Errors != 1 || counts.Warnings != 1 {
		t.Fatalf("counts = %#v", counts)
	}
	report := RenderHuman(values, []SkippedPhase{{Phase: PhaseIR, Reason: "OpenAPI model unavailable"}})
	for _, want := range []string{
		"OpenAPI SDK generation: 1 error(s), 1 warning(s)",
		"Phase: openapi",
		"Phase: target",
		"Source: contract.yaml",
		"error [SDKGEN-E001] paths must be an object",
		"hint: replace paths with an object",
		"warning [SDKGEN-W002] redundant declaration",
		"related: schema.yaml#/A",
		"Skipped phases:",
		"- ir: OpenAPI model unavailable",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("report missing %q:\n%s", want, report)
		}
	}
}

func TestRenderJSONIsDeterministicVersionedAndSanitized(t *testing.T) {
	const secret = "credential-secret"
	values := []Diagnostic{
		{
			Severity: SeverityWarning,
			Code:     "SDKGEN-W002",
			Phase:    PhaseTarget,
			Location: Location{Source: "https://other:" + secret + "@example.test/openapi.yaml?token=two#fragment", Pointer: "#/b"},
			Message:  "warning",
		},
		{
			Severity: SeverityError,
			Code:     "SDKGEN-E001",
			Phase:    PhaseOpenAPI,
			Location: Location{Source: "https://user:" + secret + "@example.test/openapi.yaml?token=one#fragment", Pointer: "#/a"},
			Related:  []Location{{Source: "https://related:" + secret + "@example.test/schema.yaml?token=three", Pointer: "#/Thing"}},
			Message:  "error",
			Hint:     "fix it",
		},
	}
	skipped := []SkippedPhase{{Phase: PhaseEmit, Reason: "target errors"}, {Phase: PhaseIR, Reason: "compile errors"}}
	first, err := RenderJSON(values, skipped)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderJSON(values, skipped)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("JSON report is not deterministic:\n%s\n%s", first, second)
	}
	if strings.Contains(first, secret) || strings.Contains(first, "token=") || strings.Contains(first, "#fragment") {
		t.Fatalf("JSON report leaked source credentials: %s", first)
	}
	var report Report
	if err := json.Unmarshal([]byte(first), &report); err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 4 || report.Counts.Errors != 1 || report.Counts.Warnings != 1 {
		t.Fatalf("report header = %#v", report)
	}
	if len(report.Diagnostics) != 2 || report.Diagnostics[0].Code != "SDKGEN-E001" || report.Diagnostics[1].Code != "SDKGEN-W002" {
		t.Fatalf("diagnostics = %#v", report.Diagnostics)
	}
	if len(report.SkippedPhases) != 2 || report.SkippedPhases[0].Phase != PhaseIR || report.SkippedPhases[1].Phase != PhaseEmit {
		t.Fatalf("skipped phases = %#v", report.SkippedPhases)
	}
}

func TestDiagnosticV3RendersCompatibilityAndFailureContracts(t *testing.T) {
	values := []Diagnostic{{
		Severity:   SeverityWarning,
		Code:       "SDKGEN-W140",
		Phase:      PhaseOpenAPI,
		Location:   Location{Source: "openapi.yaml", Pointer: "#/paths/~1items/get/requestBody"},
		Route:      "GET /items",
		Operation:  "getItems",
		Capability: "request-body",
		Scope:      failure.ScopeOperation,
		Effect:     failure.EffectOmitOperation,
		Rule:       "COMP-BODY-001",
		Action:     "reject",
		Message:    "request body is unavailable for this target",
	}}
	rendered, err := RenderJSON(values, nil)
	if err != nil {
		t.Fatal(err)
	}
	var report Report
	if err := json.Unmarshal([]byte(rendered), &report); err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 4 || len(report.Diagnostics) != 1 {
		t.Fatalf("report = %#v", report)
	}
	value := report.Diagnostics[0]
	if value.Rule != "COMP-BODY-001" || value.Action != "reject" ||
		value.Scope != failure.ScopeOperation || value.Effect != failure.EffectOmitOperation ||
		value.Capability != "request-body" {
		t.Fatalf("diagnostic contracts = %#v", value)
	}
	human := RenderHuman(values, nil)
	for _, want := range []string{
		"operation: getItems",
		"capability: request-body",
		"scope: operation",
		"effect: omit-operation",
		"rule: COMP-BODY-001",
		"action: reject",
	} {
		if !strings.Contains(human, want) {
			t.Fatalf("human report missing %q:\n%s", want, human)
		}
	}
}

func TestDiagnosticV3KeepsSeverityAsTheBlockingContract(t *testing.T) {
	recoverable := []Diagnostic{{
		Severity: SeverityWarning,
		Scope:    failure.ScopeOperation,
		Effect:   failure.EffectOmitOperation,
	}}
	if HasErrors(recoverable) {
		t.Fatal("recoverable omission warning unexpectedly blocked generation")
	}
	blocking := append(recoverable, Diagnostic{
		Severity: SeverityError,
		Scope:    failure.ScopeDocument,
		Effect:   failure.EffectBlock,
	})
	if !HasErrors(blocking) {
		t.Fatal("blocking error unexpectedly allowed generation")
	}
}

func TestSourceRegistryRedactsAndAssignsStableOpaqueOrdinals(t *testing.T) {
	first := "https://user:secret@example.test/openapi.json?token=alpha#one"
	second := "https://other:credential@example.test/openapi.json?token=beta#two"
	forward := NewSourceRegistry([]string{first, second})
	reverse := NewSourceRegistry([]string{second, first})
	for _, source := range []string{first, second} {
		if forward.Display(source) != reverse.Display(source) {
			t.Fatalf("display changed with discovery order: %q != %q", forward.Display(source), reverse.Display(source))
		}
		display := forward.Display(source)
		for _, secret := range []string{"user", "secret", "other", "credential", "alpha", "beta", "#one", "#two"} {
			if strings.Contains(display, secret) {
				t.Fatalf("display leaked %q: %s", secret, display)
			}
		}
		if !strings.Contains(display, "[source ") {
			t.Fatalf("collision display lacks opaque ordinal: %s", display)
		}
	}
}

func TestSafeSourceDisplayRedactsMalformedHTTPURL(t *testing.T) {
	source := "https://user:secret@example.test/%zz?token=alpha#fragment"
	display := SafeSourceDisplay(source)
	if display != "https://example.test/%zz" {
		t.Fatalf("display = %q", display)
	}
	for _, secret := range []string{"user", "secret", "token", "alpha", "fragment"} {
		if strings.Contains(display, secret) {
			t.Fatalf("display leaked %q: %s", secret, display)
		}
	}
}

func TestSanitizeSourcesCoversPrimaryAndRelatedLocations(t *testing.T) {
	values := SanitizeSources([]Diagnostic{{
		Location: Location{Source: "https://user:secret@example.test/%zz?token=alpha#fragment"},
		Related:  []Location{{Source: "https://other:secret@example.test/%zz?token=beta#fragment"}},
	}})
	if len(values) != 1 || len(values[0].Related) != 1 {
		t.Fatalf("diagnostics = %#v", values)
	}
	for _, source := range []string{values[0].Location.Source, values[0].Related[0].Source} {
		for _, secret := range []string{"user", "other", "secret", "token", "alpha", "beta", "fragment"} {
			if strings.Contains(source, secret) {
				t.Fatalf("source leaked %q: %s", secret, source)
			}
		}
	}
}
