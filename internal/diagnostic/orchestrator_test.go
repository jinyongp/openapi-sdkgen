package diagnostic

import "testing"

func TestOrchestratorFailFastStopsIndependentAnalyzerAfterBlock(t *testing.T) {
	orchestrator, err := NewOrchestrator(ModeFailFast)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator.Provide("decoded-source")
	first := AnalyzerSpec{Phase: PhaseOpenAPI, Name: "compatibility", Requires: []string{"decoded-source"}}
	if !orchestrator.Ready(first) {
		t.Fatal("first analyzer was not ready")
	}
	orchestrator.Record(first, CoverageOutcome{Status: CoverageComplete})
	orchestrator.Block("compatibility blocker", []string{"diag-b", "diag-a"})

	second := AnalyzerSpec{Phase: PhaseOpenAPI, Name: "reserved-extensions", Requires: []string{"decoded-source"}}
	if orchestrator.Ready(second) {
		t.Fatal("fail-fast scheduled analyzer after blocker")
	}
	coverage := orchestrator.Coverage()
	if len(coverage) != 2 || coverage[1].Status != CoverageSkipped ||
		coverage[1].Reason != "compatibility blocker" ||
		len(coverage[1].BlockedBy) != 2 ||
		coverage[1].BlockedBy[0] != "diag-a" {
		t.Fatalf("coverage = %#v", coverage)
	}
}

func TestOrchestratorCollectContinuesIndependentAnalyzer(t *testing.T) {
	orchestrator, err := NewOrchestrator(ModeCollect)
	if err != nil {
		t.Fatal(err)
	}
	orchestrator.Provide("decoded-source")
	first := AnalyzerSpec{Phase: PhaseOpenAPI, Name: "compatibility", Requires: []string{"decoded-source"}}
	if !orchestrator.Ready(first) {
		t.Fatal("first analyzer was not ready")
	}
	orchestrator.Record(first, CoverageOutcome{Status: CoverageComplete})
	orchestrator.Block("compatibility blocker", []string{"diag-a"})

	second := AnalyzerSpec{Phase: PhaseOpenAPI, Name: "reserved-extensions", Requires: []string{"decoded-source"}}
	if !orchestrator.Ready(second) {
		t.Fatal("collect mode suppressed independent analyzer")
	}
	orchestrator.Record(second, CoverageOutcome{Status: CoverageComplete})
	coverage := orchestrator.Coverage()
	if len(coverage) != 2 || coverage[1].Status != CoverageComplete {
		t.Fatalf("coverage = %#v", coverage)
	}
}

func TestOrchestratorSkipsMissingPrerequisiteInBothModes(t *testing.T) {
	for _, mode := range []Mode{ModeFailFast, ModeCollect} {
		t.Run(string(mode), func(t *testing.T) {
			orchestrator, err := NewOrchestrator(mode)
			if err != nil {
				t.Fatal(err)
			}
			spec := AnalyzerSpec{
				Phase:    PhaseTarget,
				Name:     "target-security",
				Requires: []string{"canonical-ir", "ownership-index"},
			}
			if orchestrator.Ready(spec) {
				t.Fatal("analyzer ran without prerequisites")
			}
			coverage := orchestrator.Coverage()
			if len(coverage) != 1 || coverage[0].Status != CoverageSkipped ||
				len(coverage[0].Prerequisites) != 2 ||
				coverage[0].Prerequisites[0].Available ||
				coverage[0].Prerequisites[1].Available {
				t.Fatalf("coverage = %#v", coverage)
			}
		})
	}
}
