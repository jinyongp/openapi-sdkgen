package main

import (
	"errors"

	"openapi-sdkgen/internal/generator"
	sdkoutput "openapi-sdkgen/internal/output"
)

const artifactManifestName = sdkoutput.ManifestName

type artifactGeneration = sdkoutput.Generation
type artifactManifest = sdkoutput.Manifest
type artifactPublisher = sdkoutput.Publisher

func isOutputFailure(err error) bool {
	return sdkoutput.IsPublicationError(err)
}

func preflightOutput(path string, incremental bool) error {
	return sdkoutput.Preflight(path, incremental)
}

func incrementalGenerationMatches(path string, expected *artifactGeneration) (bool, error) {
	return sdkoutput.GenerationMatches(path, expected)
}

func artifactGenerationEqual(left, right *artifactGeneration) bool {
	return sdkoutput.GenerationEqual(left, right)
}

func writeArtifacts(path string, artifacts []generator.Artifact) error {
	return sdkoutput.PublishArtifacts(path, artifacts, false, nil)
}

func writeArtifactsForGeneration(path string, artifacts []generator.Artifact, generation *artifactGeneration) error {
	return sdkoutput.PublishArtifacts(path, artifacts, false, generation)
}

func writeArtifactsIncremental(path string, artifacts []generator.Artifact) error {
	return sdkoutput.PublishArtifacts(path, artifacts, true, nil)
}

func writeArtifactsIncrementalForGeneration(path string, artifacts []generator.Artifact, generation *artifactGeneration) error {
	return sdkoutput.PublishArtifacts(path, artifacts, true, generation)
}

func streamArtifacts(target generator.Target, plan generator.Plan, path string) error {
	return streamArtifactsWithMode(target, plan, path, false, nil)
}

func streamArtifactsForGeneration(target generator.Target, plan generator.Plan, path string, generation *artifactGeneration) error {
	return streamArtifactsWithMode(target, plan, path, false, generation)
}

func streamArtifactsIncremental(target generator.Target, plan generator.Plan, path string) error {
	return streamArtifactsWithMode(target, plan, path, true, nil)
}

func streamArtifactsIncrementalForGeneration(target generator.Target, plan generator.Plan, path string, generation *artifactGeneration) error {
	return streamArtifactsWithMode(target, plan, path, true, generation)
}

func checkArtifactsForGeneration(target generator.Target, plan generator.Plan, path string, generation *artifactGeneration) error {
	return sdkoutput.CheckManaged(path, generation, func(sink generator.ArtifactSink) error {
		return generator.EmitTo(target, plan, sink)
	})
}

func streamArtifactsWithMode(target generator.Target, plan generator.Plan, path string, incremental bool, generation *artifactGeneration) error {
	err := sdkoutput.StreamArtifacts(path, incremental, generation, func(sink generator.ArtifactSink) error {
		return generator.EmitTo(target, plan, sink)
	})
	if err == nil {
		return nil
	}
	var staged *sdkoutput.StageError
	if errors.As(err, &staged) {
		return &generationStageError{stage: string(staged.Stage), err: staged.Err}
	}
	return err
}

func newArtifactPublisher(path string) (*artifactPublisher, error) {
	return sdkoutput.NewPublisher(path, false)
}

func newArtifactPublisherWithMode(path string, incremental bool) (*artifactPublisher, error) {
	return sdkoutput.NewPublisher(path, incremental)
}

func newIncrementalArtifactPublisher(path string) (*artifactPublisher, error) {
	return sdkoutput.NewPublisher(path, true)
}

func safeArtifactPath(value string) (string, error) {
	return sdkoutput.SafeArtifactPath(value)
}

func readArtifactManifest(path string) (map[string]string, error) {
	return sdkoutput.ReadManifestFiles(path)
}

func readArtifactManifestRecord(path string) (artifactManifest, error) {
	return sdkoutput.ReadManifest(path)
}
