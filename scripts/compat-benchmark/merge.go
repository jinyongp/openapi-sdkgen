package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"
)

func mergeBenchmarkReports(manifestPath string, paths []string, outputPath string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var manifest benchmarkManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}
	if err := validateManifest(manifest); err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	manifestSHA := hex.EncodeToString(digest[:])
	expected := make(map[string]corpusSpec, len(manifest.Corpora))
	for _, corpus := range manifest.Corpora {
		if typecheckCandidate(corpus) {
			expected[corpus.ID] = corpus
		}
	}
	merged := benchmarkReport{
		SchemaVersion: benchmarkReportSchemaVersion, ManifestSHA256: manifestSHA,
		FeatureCatalog:       featureCatalog(),
		DocumentMeasurements: map[string]*benchmarkMeasurement{},
	}
	seen := map[string]bool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var shard benchmarkReport
		if err := json.Unmarshal(data, &shard); err != nil {
			return fmt.Errorf("decode shard %s: %w", path, err)
		}
		if shard.SchemaVersion != benchmarkReportSchemaVersion || shard.ManifestSHA256 != manifestSHA ||
			!slices.Equal(shard.FeatureCatalog, merged.FeatureCatalog) || len(shard.Documents) == 0 {
			return fmt.Errorf("shard %s does not match the original manifest and report contract", path)
		}
		if err := mergeMeasurement(&merged, shard.Measurement); err != nil {
			return fmt.Errorf("shard %s: %w", path, err)
		}
		for _, document := range shard.Documents {
			if err := validateGenerationAddons(document); err != nil {
				return err
			}
			corpus, exists := expected[document.ID]
			if !exists || seen[document.ID] {
				return fmt.Errorf("unknown or repeated shard document %q", document.ID)
			}
			if document.Input != filepath.ToSlash(corpus.Input) || document.Cohort != corpus.Cohort ||
				(corpus.SHA256 != "" && document.InputSHA256 != corpus.SHA256) ||
				(corpus.GitBlob != "" && document.GitBlob != corpus.GitBlob) ||
				(corpus.OpenAPIVersion != "" && document.OpenAPIVersion != corpus.OpenAPIVersion) {
				return fmt.Errorf("shard document %q provenance differs from the original input", document.ID)
			}
			var selection *benchmarkSelection
			if document.GenerationScope == "selected" {
				selection, err = loadBenchmarkSelection(filepath.Join(filepath.Dir(manifestPath), "selections", corpus.ID+".toml"))
				if err != nil {
					return fmt.Errorf("selected scope policy for %s: %w", corpus.ID, err)
				}
			}
			if err := validateSelectionEvidence(document, corpus, selection); err != nil {
				return err
			}
			success := documentVerificationSuccess(document)
			if document.DocumentSuccess != success {
				return fmt.Errorf("shard document %q has inconsistent verification status", document.ID)
			}
			adjusted := success
			for _, profile := range document.SupportProfiles {
				profileSuccess := profile.DiscoveryComplete && profile.Diagnostics.Errors == 0 &&
					profile.Generation.Status == "pass" && profile.Typecheck.Status == "pass"
				if document.GenerationScope == "selected" {
					profileSuccess = profileSuccess && document.Selection != nil && document.Selection.Runtime.Status == "pass"
				}
				if profile.Success != profileSuccess {
					return fmt.Errorf("shard document %q has inconsistent profile verification", document.ID)
				}
				adjusted = adjusted || profile.Success
			}
			if document.CapabilityAdjustedSuccess != adjusted {
				return fmt.Errorf("shard document %q has inconsistent adjusted verification", document.ID)
			}
			seen[document.ID] = true
			merged.Documents = append(merged.Documents, document)
			measurement := *shard.Measurement
			if individual := shard.DocumentMeasurements[document.ID]; individual != nil {
				if individual.SourceCommit != measurement.SourceCommit || individual.SourceDirty != measurement.SourceDirty {
					return fmt.Errorf("document %q measurement uses a different source revision or worktree state", document.ID)
				}
				measurement = *individual
			}
			merged.DocumentMeasurements[document.ID] = &measurement
		}
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("incomplete shard results: got %d documents, want %d", len(seen), len(expected))
	}
	sort.Slice(merged.Documents, func(i, j int) bool {
		if merged.Documents[i].Cohort != merged.Documents[j].Cohort {
			return merged.Documents[i].Cohort < merged.Documents[j].Cohort
		}
		return merged.Documents[i].ID < merged.Documents[j].ID
	})
	merged.Cohorts = summarizeCohorts(merged.Documents)
	merged.Overall = summarizeDocuments("", merged.Documents)
	return writeBenchmarkReport(merged, outputPath)
}

func mergeMeasurement(report *benchmarkReport, measurement *benchmarkMeasurement) error {
	if measurement == nil || measurement.GenerationScope != "compile-prepare-write" || measurement.Samples != 1 {
		return fmt.Errorf("missing or incompatible measurement metadata")
	}
	measuredAt, err := time.Parse(time.RFC3339, measurement.MeasuredAt)
	if err != nil {
		return fmt.Errorf("invalid measurement date: %w", err)
	}
	if report.Measurement == nil {
		copy := *measurement
		report.Measurement = &copy
		return nil
	}
	combined := report.Measurement
	if combined.OS != measurement.OS || combined.Architecture != measurement.Architecture || combined.GoVersion != measurement.GoVersion || combined.TypeScriptVersion != measurement.TypeScriptVersion {
		return fmt.Errorf("shards use different platforms or compiler versions")
	}
	if combined.SourceCommit != measurement.SourceCommit || combined.SourceDirty != measurement.SourceDirty {
		return fmt.Errorf("shards use different source revisions or worktree states")
	}
	if combined.CPU != measurement.CPU {
		combined.CPU = ""
	}
	previous, _ := time.Parse(time.RFC3339, combined.MeasuredAt)
	if measuredAt.After(previous) {
		combined.MeasuredAt = measurement.MeasuredAt
	}
	return nil
}

func validateGenerationAddons(document documentResult) error {
	// Historical evidence has no recorded add-on setting. Keep it unknown.
	if document.GenerationAddons == nil {
		return nil
	}
	addons := *document.GenerationAddons
	if !slices.Equal(addons, []string{}) && !slices.Equal(addons, []string{"metadata"}) {
		return fmt.Errorf("invalid generation add-ons for %s", document.ID)
	}
	for _, profile := range document.SupportProfiles {
		if profile.Name != "server-addon" {
			continue
		}
		expected := append(slices.Clone(addons), "server")
		if profile.GenerationAddons == nil || !slices.Equal(*profile.GenerationAddons, expected) {
			return fmt.Errorf("server add-on settings differ for %s", document.ID)
		}
	}
	return nil
}
