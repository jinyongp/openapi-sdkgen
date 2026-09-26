package sdkgen

import (
	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"openapi-sdkgen/internal/diagnostic"
)

func collectVersionDiagnostics(
	analysis *sourceAnalysis,
	effective any,
	source string,
) error {
	document, _ := effective.(map[string]any)
	var version openapidoc.VersionLine
	var versionAvailable bool

	identitySpec := sourceAnalyzerSpec(
		diagnostic.PhaseOpenAPI,
		"source.version-identity",
		source,
		"effective-source",
	)
	if _, err := analysis.run(
		identitySpec,
		sourceScanVersion,
		"OpenAPI version identity is unavailable",
		func() error {
			declared, _ := document["openapi"].(string)
			value, err := openapidoc.DetectVersionLine(declared)
			if err != nil {
				analysis.collector.Add(compileErrorDiagnostic(
					phaseError(diagnostic.PhaseOpenAPI, err),
					source,
				))
				return nil
			}
			version = value
			versionAvailable = true
			return nil
		},
	); err != nil {
		return err
	}
	if versionAvailable {
		analysis.orchestrator.Provide("version-identity")
	}

	featureSpec := sourceAnalyzerSpec(
		diagnostic.PhaseOpenAPI,
		"source.version-features",
		source,
		"effective-source",
		"version-identity",
	)
	_, err := analysis.run(
		featureSpec,
		sourceScanVersion,
		"version-specific OpenAPI validation reported errors",
		func() error {
			for _, value := range openapidoc.CollectVersionFeatureErrors(document, version) {
				analysis.collector.Add(compileErrorDiagnostic(
					phaseError(diagnostic.PhaseOpenAPI, value),
					source,
				))
			}
			return nil
		},
	)
	return err
}
