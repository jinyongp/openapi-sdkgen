package diagnostic

import (
	"encoding/json"
	"strings"
	"testing"

	"openapi-sdkgen/internal/failure"
)

func TestReportV4AssignsStableIssueIdentityAndDeduplicates(t *testing.T) {
	first := Diagnostic{
		Severity:       SeverityError,
		Code:           "SDKGEN-E900",
		Phase:          PhaseOpenAPI,
		Location:       Location{Source: "/checkout/one/openapi.yaml", Pointer: "#/paths"},
		IdentitySource: "root",
		Scope:          failure.ScopeDocument,
		Effect:         failure.EffectBlock,
		Rule:           "TEST-001",
		Message:        "first wording",
	}
	duplicate := first
	duplicate.Message = "second wording"
	duplicate.Related = []Location{{Source: "/checkout/one/schema.yaml", Pointer: "#/Thing"}}

	report := NewReport([]Diagnostic{duplicate, first}, nil)
	if report.SchemaVersion != 4 {
		t.Fatalf("schemaVersion = %d", report.SchemaVersion)
	}
	if len(report.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", report.Diagnostics)
	}
	if report.Diagnostics[0].ID == "" {
		t.Fatal("diagnostic id is empty")
	}
	if len(report.Diagnostics[0].Related) != 1 {
		t.Fatalf("related = %#v", report.Diagnostics[0].Related)
	}

	moved := first
	moved.Location.Source = "/checkout/two/openapi.yaml"
	movedReport := NewReport([]Diagnostic{moved}, nil)
	if movedReport.Diagnostics[0].ID != report.Diagnostics[0].ID {
		t.Fatalf("id changed across checkout roots: %q != %q", movedReport.Diagnostics[0].ID, report.Diagnostics[0].ID)
	}

	distinct := first
	distinct.Location.Pointer = "#/components"
	distinctReport := NewReport([]Diagnostic{first, distinct}, nil)
	if len(distinctReport.Diagnostics) != 2 || distinctReport.Diagnostics[0].ID == distinctReport.Diagnostics[1].ID {
		t.Fatalf("distinct locations were merged: %#v", distinctReport.Diagnostics)
	}
}

func TestReportV4NormalizesCoverageAndRedactsSources(t *testing.T) {
	const secret = "credential-secret"
	location := Location{
		Source:  "https://user:" + secret + "@example.test/openapi.yaml?token=alpha#fragment",
		Pointer: "#/paths",
	}
	coverage := []AnalysisCoverage{
		{
			Phase:    PhaseTarget,
			Analyzer: "security",
			Status:   CoverageSkipped,
			Reason:   "IR unavailable",
			Prerequisites: []CoveragePrerequisite{
				{Name: "ir", Available: false, Reason: "compiler blocker"},
				{Name: "effective-source", Available: true},
			},
			BlockedBy: []string{"b", "a", "a"},
		},
		{
			Phase:    PhaseOpenAPI,
			Analyzer: "version",
			Status:   CoveragePartial,
			Location: &location,
			Scope:    failure.ScopeDocument,
		},
	}
	rendered, err := RenderJSON(nil, nil, coverage...)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered, secret) || strings.Contains(rendered, "token=") || strings.Contains(rendered, "#fragment") {
		t.Fatalf("coverage leaked source credentials: %s", rendered)
	}
	var report Report
	if err := json.Unmarshal([]byte(rendered), &report); err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 4 || len(report.Coverage) != 2 {
		t.Fatalf("report = %#v", report)
	}
	if report.Coverage[0].Phase != PhaseOpenAPI || report.Coverage[1].Phase != PhaseTarget {
		t.Fatalf("coverage order = %#v", report.Coverage)
	}
	target := report.Coverage[1]
	if len(target.Prerequisites) != 2 ||
		target.Prerequisites[0].Name != "effective-source" ||
		target.Prerequisites[1].Name != "ir" {
		t.Fatalf("prerequisites = %#v", target.Prerequisites)
	}
	if len(target.BlockedBy) != 2 || target.BlockedBy[0] != "a" || target.BlockedBy[1] != "b" {
		t.Fatalf("blockedBy = %#v", target.BlockedBy)
	}

	human := RenderHuman(nil, nil, coverage...)
	for _, want := range []string{
		"Analysis coverage:",
		"- openapi/version: partial",
		"- target/security: skipped",
		"prerequisite effective-source: available",
		"prerequisite ir: unavailable",
		"blocked by: a, b",
	} {
		if !strings.Contains(human, want) {
			t.Fatalf("human coverage missing %q:\n%s", want, human)
		}
	}
}
