package sdkgen

import "openapi-sdkgen/internal/diagnostic"

type sourceScanBoundary int

const (
	sourceScanNone sourceScanBoundary = iota
	sourceScanCompatibility
	sourceScanReserved
	sourceScanReferences
	sourceScanVersion
)

type sourceAnalysis struct {
	mode          diagnostic.Mode
	options       *CompileOptions
	collector     *diagnostic.Collector
	orchestrator  *diagnostic.Orchestrator
	extraCoverage []diagnostic.AnalysisCoverage
	blocked       sourceScanBoundary
}

func newSourceAnalysis(mode diagnostic.Mode, options *CompileOptions, collector *diagnostic.Collector) (*sourceAnalysis, error) {
	orchestrator, err := diagnostic.NewOrchestrator(mode)
	if err != nil {
		return nil, err
	}
	orchestrator.Provide("decoded-source")
	return &sourceAnalysis{
		mode:         mode,
		options:      options,
		collector:    collector,
		orchestrator: orchestrator,
	}, nil
}

func (analysis *sourceAnalysis) compatibility(source string, run func() error) (bool, error) {
	spec := sourceAnalyzerSpec(diagnostic.PhaseOpenAPI, "source.compatibility", source, "decoded-source")
	ran, err := analysis.run(spec, sourceScanCompatibility, "compatibility policy rejected source before reference traversal", run)
	if ran && err == nil {
		analysis.orchestrator.Provide("effective-source")
	}
	return ran, err
}

func (analysis *sourceAnalysis) reserved(source string, run func() error) (bool, error) {
	return analysis.run(
		sourceAnalyzerSpec(diagnostic.PhaseOpenAPI, "source.reserved-extensions", source, "effective-source"),
		sourceScanReserved,
		"reserved extension keywords were found before reference bundling",
		run,
	)
}

func (analysis *sourceAnalysis) references(source string, run func() error) (bool, error) {
	return analysis.run(
		sourceAnalyzerSpec(diagnostic.PhaseReferences, "source.local-references", source, "effective-source"),
		sourceScanReferences,
		"reference resolution reported errors",
		run,
	)
}

func (analysis *sourceAnalysis) run(
	spec diagnostic.AnalyzerSpec,
	boundary sourceScanBoundary,
	blockReason string,
	run func() error,
) (bool, error) {
	if !analysis.orchestrator.Ready(spec) {
		return false, nil
	}
	if err := run(); err != nil {
		return true, err
	}
	analysis.orchestrator.Record(spec, diagnostic.CoverageOutcome{Status: diagnostic.CoverageComplete})
	if analysis.blocked == sourceScanNone {
		blocked, blockedBy := compilationBlockingDiagnostics(analysis.collector, analysis.options.compatibilitySession)
		if blocked {
			analysis.blocked = boundary
			analysis.orchestrator.Block(blockReason, blockedBy)
		}
	}
	return true, nil
}

func (analysis *sourceAnalysis) blockingResult() (Result, bool) {
	if analysis.blocked == sourceScanNone {
		return Result{}, false
	}
	var result Result
	if analysis.mode == diagnostic.ModeFailFast {
		switch analysis.blocked {
		case sourceScanCompatibility:
			result = compatibilitySourceScanResult(analysis.collector)
		case sourceScanReserved:
			result = reservedSourceScanResult(analysis.collector)
		case sourceScanReferences:
			result = referenceSourceScanResult(analysis.collector)
		}
	} else {
		const reason = "source analysis reported blocking diagnostics before canonical normalization and IR construction"
		result = Result{
			Diagnostics: diagnostic.Sort(analysis.collector.Diagnostics()),
			SkippedPhases: []diagnostic.SkippedPhase{
				{Phase: diagnostic.PhaseNormalize, Reason: reason},
				{Phase: diagnostic.PhaseIR, Reason: reason},
			},
		}
	}
	return analysis.attach(result), true
}

func (analysis *sourceAnalysis) addCoverage(values ...diagnostic.AnalysisCoverage) {
	analysis.extraCoverage = append(analysis.extraCoverage, values...)
}

func (analysis *sourceAnalysis) attach(result Result) Result {
	result.DiagnosticMode = analysis.mode
	result.Coverage = append(result.Coverage, analysis.orchestrator.Coverage()...)
	result.Coverage = append(result.Coverage, analysis.extraCoverage...)
	return result
}

func sourceAnalyzerSpec(phase diagnostic.Phase, name, source string, prerequisites ...string) diagnostic.AnalyzerSpec {
	location := &diagnostic.Location{Source: source, Pointer: "#"}
	return diagnostic.AnalyzerSpec{
		Phase:    phase,
		Name:     name,
		Requires: append([]string(nil), prerequisites...),
		Location: location,
	}
}
