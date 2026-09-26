package generator

import (
	"fmt"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
)

// Preparation is the complete compiler and target preflight outcome.
type Preparation struct {
	Plan           Plan
	Diagnostics    []diagnostic.Diagnostic
	SkippedPhases  []diagnostic.SkippedPhase
	Coverage       []diagnostic.AnalysisCoverage
	DiagnosticMode diagnostic.Mode
}

// PrepareCompilation owns the compiler result, preserves its diagnostics and
// skipped phases, and invokes the target only when a safe document exists.
func PrepareCompilation(target Target, compiled compiler.Result, options Options) (Preparation, error) {
	if target == nil {
		return Preparation{}, fmt.Errorf("internal generation pipeline: target is nil")
	}
	compileMode, err := compiled.DiagnosticMode.Resolve()
	if err != nil {
		return Preparation{}, fmt.Errorf("internal generation pipeline: compiler diagnostic mode: %w", err)
	}
	targetMode, err := options.DiagnosticMode.Resolve()
	if err != nil {
		return Preparation{}, fmt.Errorf("internal generation pipeline: target diagnostic mode: %w", err)
	}
	if compileMode != targetMode {
		return Preparation{}, fmt.Errorf("internal generation pipeline: compiler diagnostic mode %q does not match target mode %q", compileMode, targetMode)
	}
	orchestrator, err := diagnostic.NewOrchestrator(targetMode)
	if err != nil {
		return Preparation{}, fmt.Errorf("internal generation pipeline: diagnostic orchestration: %w", err)
	}
	result := Preparation{
		Diagnostics:    append([]diagnostic.Diagnostic(nil), compiled.Diagnostics...),
		SkippedPhases:  append([]diagnostic.SkippedPhase(nil), compiled.SkippedPhases...),
		Coverage:       append([]diagnostic.AnalysisCoverage(nil), compiled.Coverage...),
		DiagnosticMode: targetMode,
	}
	targetAnalyzer := diagnostic.AnalyzerSpec{
		Phase:    diagnostic.PhaseTarget,
		Name:     "target.prepare",
		Requires: []string{"compiler-document"},
		Target:   target.Name(),
	}
	if compiled.Document == nil || diagnostic.HasErrors(compiled.Diagnostics) {
		if compiled.Document == nil && !diagnostic.HasErrors(compiled.Diagnostics) {
			return Preparation{}, fmt.Errorf("internal generation pipeline: compiler returned neither a document nor an error diagnostic")
		}
		orchestrator.Ready(targetAnalyzer)
		result.Coverage = append(result.Coverage, orchestrator.Coverage()...)
		result.SkippedPhases = append(result.SkippedPhases,
			diagnostic.SkippedPhase{Phase: diagnostic.PhaseTarget, Reason: "compiler preflight did not produce a safe document"},
			diagnostic.SkippedPhase{Phase: diagnostic.PhaseEmit, Reason: "compiler preflight did not produce a safe document"},
			diagnostic.SkippedPhase{Phase: diagnostic.PhasePublish, Reason: "compiler preflight did not produce a safe document"},
		)
		return result, nil
	}
	orchestrator.Provide("compiler-document")
	if !orchestrator.Ready(targetAnalyzer) {
		return Preparation{}, fmt.Errorf("internal generation pipeline: target analyzer prerequisites unexpectedly unavailable")
	}
	plan, targetDiagnostics, err := target.Prepare(compiled.Document, options)
	result.Diagnostics = diagnostic.Sort(append(result.Diagnostics, targetDiagnostics...))
	if err != nil {
		return result, err
	}
	orchestrator.Record(targetAnalyzer, diagnostic.CoverageOutcome{Status: diagnostic.CoverageComplete})
	result.Coverage = append(result.Coverage, orchestrator.Coverage()...)
	result.Plan = plan
	if diagnostic.HasErrors(targetDiagnostics) {
		result.SkippedPhases = append(result.SkippedPhases,
			diagnostic.SkippedPhase{Phase: diagnostic.PhaseEmit, Reason: "target preflight reported errors"},
			diagnostic.SkippedPhase{Phase: diagnostic.PhasePublish, Reason: "target preflight reported errors"},
		)
	}
	return result, nil
}
