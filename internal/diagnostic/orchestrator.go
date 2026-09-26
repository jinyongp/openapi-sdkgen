package diagnostic

import (
	"fmt"
	"sort"
	"strings"

	"openapi-sdkgen/internal/failure"
)

// AnalyzerSpec declares one analysis unit and the prerequisite capabilities it
// needs before it can execute.
type AnalyzerSpec struct {
	Phase      Phase
	Name       string
	Requires   []string
	Location   *Location
	Target     string
	Route      string
	Operation  string
	Capability string
	Scope      failure.Scope
}

// CoverageOutcome records the result of a scheduled analyzer.
type CoverageOutcome struct {
	Status    CoverageStatus
	Reason    string
	BlockedBy []string
}

// Orchestrator owns one invocation's diagnostic mode and prerequisite state.
// It decides whether a registered analyzer is eligible to run and records why
// unavailable analyzers were skipped.
type Orchestrator struct {
	mode        Mode
	available   map[string]bool
	blocked     bool
	blockReason string
	blockedBy   []string
	coverage    []AnalysisCoverage
}

// NewOrchestrator validates mode and creates an empty prerequisite scheduler.
func NewOrchestrator(mode Mode) (*Orchestrator, error) {
	resolved, err := mode.Resolve()
	if err != nil {
		return nil, err
	}
	return &Orchestrator{
		mode:      resolved,
		available: make(map[string]bool),
	}, nil
}

// Mode returns the canonical invocation mode.
func (orchestrator *Orchestrator) Mode() Mode {
	return orchestrator.mode
}

// Provide marks one prerequisite capability as available.
func (orchestrator *Orchestrator) Provide(name string) {
	if name != "" {
		orchestrator.available[name] = true
	}
}

// Block records an expected blocking diagnostic boundary. Fail-fast mode stops
// scheduling otherwise-independent analyzers after this point; collect mode may
// continue analyzers whose explicit prerequisites remain available.
func (orchestrator *Orchestrator) Block(reason string, blockedBy []string) {
	orchestrator.blocked = true
	orchestrator.blockReason = reason
	orchestrator.blockedBy = append([]string(nil), blockedBy...)
	sort.Strings(orchestrator.blockedBy)
	orchestrator.blockedBy = deduplicateStrings(orchestrator.blockedBy)
}

// Ready reports whether an analyzer may execute. A false result always records
// a skipped coverage entry with the unavailable prerequisite or fail-fast stop.
func (orchestrator *Orchestrator) Ready(spec AnalyzerSpec) bool {
	prerequisites, missing := orchestrator.prerequisites(spec)
	if len(missing) != 0 {
		orchestrator.coverage = append(orchestrator.coverage, analysisCoverage(spec, prerequisites, CoverageOutcome{
			Status: CoverageSkipped,
			Reason: fmt.Sprintf("prerequisite unavailable: %s", strings.Join(missing, ", ")),
		}))
		return false
	}
	if orchestrator.mode == ModeFailFast && orchestrator.blocked {
		orchestrator.coverage = append(orchestrator.coverage, analysisCoverage(spec, prerequisites, CoverageOutcome{
			Status:    CoverageSkipped,
			Reason:    orchestrator.blockReason,
			BlockedBy: orchestrator.blockedBy,
		}))
		return false
	}
	return true
}

// Record appends one analyzer outcome after it was eligible to run.
func (orchestrator *Orchestrator) Record(spec AnalyzerSpec, outcome CoverageOutcome) {
	if outcome.Status == "" {
		outcome.Status = CoverageComplete
	}
	prerequisites, _ := orchestrator.prerequisites(spec)
	orchestrator.coverage = append(orchestrator.coverage, analysisCoverage(spec, prerequisites, outcome))
}

// Coverage returns a deterministic copy of all recorded analyzer coverage.
func (orchestrator *Orchestrator) Coverage() []AnalysisCoverage {
	return normalizeCoverage(orchestrator.coverage)
}

func (orchestrator *Orchestrator) prerequisites(spec AnalyzerSpec) ([]CoveragePrerequisite, []string) {
	names := append([]string(nil), spec.Requires...)
	sort.Strings(names)
	names = deduplicateStrings(names)
	prerequisites := make([]CoveragePrerequisite, 0, len(names))
	var missing []string
	for _, name := range names {
		available := orchestrator.available[name]
		prerequisites = append(prerequisites, CoveragePrerequisite{Name: name, Available: available})
		if !available {
			missing = append(missing, name)
		}
	}
	return prerequisites, missing
}

func analysisCoverage(spec AnalyzerSpec, prerequisites []CoveragePrerequisite, outcome CoverageOutcome) AnalysisCoverage {
	var location *Location
	if spec.Location != nil {
		copy := *spec.Location
		location = &copy
	}
	return AnalysisCoverage{
		Phase:         spec.Phase,
		Analyzer:      spec.Name,
		Status:        outcome.Status,
		Location:      location,
		Target:        spec.Target,
		Route:         spec.Route,
		Operation:     spec.Operation,
		Capability:    spec.Capability,
		Scope:         spec.Scope,
		Prerequisites: append([]CoveragePrerequisite(nil), prerequisites...),
		Reason:        outcome.Reason,
		BlockedBy:     append([]string(nil), outcome.BlockedBy...),
	}
}
