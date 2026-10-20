// Package golang prepares and emits consumer-owned Go SDK source.
package golang

import (
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

// Generator implements the Go target. Preparation owns schema semantics;
// emission renders the frozen plan and partitions large implementation packages.
type Generator struct{}

func (Generator) Name() string { return "golang" }

func (Generator) SupportsAddon(addon generator.Addon) bool {
	return addon == generator.AddonMetadata || addon == generator.AddonServer
}

func (Generator) SupportsClients() bool { return true }

func (Generator) Prepare(document *ir.Document, options generator.Options) (generator.Plan, []diagnostic.Diagnostic, error) {
	return prepare(document, options)
}

func (target Generator) Emit(plan generator.Plan) ([]generator.Artifact, error) {
	sink := &artifactCollector{}
	if err := target.EmitTo(plan, sink); err != nil {
		return nil, err
	}
	return sink.artifacts, nil
}

type artifactCollector struct{ artifacts []generator.Artifact }

func (sink *artifactCollector) WriteArtifact(artifact generator.Artifact) error {
	sink.artifacts = append(sink.artifacts, artifact)
	return nil
}
