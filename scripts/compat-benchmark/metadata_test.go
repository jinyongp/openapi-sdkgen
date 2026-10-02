package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"openapi-sdkgen/internal/generator"
)

func TestBenchmarkMetadataSettingsAndMeasuredBytes(t *testing.T) {
	for _, inbound := range []bool{false, true} {
		t.Run(map[bool]string{false: "client", true: "server"}[inbound], func(t *testing.T) {
			extra := ""
			if inbound {
				extra = `,"webhooks":{"event":{"post":{"responses":{"204":{"description":"OK"}}}}}`
			}
			input := filepath.Join(t.TempDir(), "api.json")
			data := `{"openapi":"3.2.1","info":{"title":"Original sentinel","version":"1"},"paths":{"/items":{"get":{"responses":{"204":{"description":"OK"}}}}},"x-large":"` + strings.Repeat("original", 1000) + `"` + extra + `}`
			if err := os.WriteFile(input, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			registry, _ := generator.NewAddonRegistry(generator.AddonMetadata)
			var measured []verificationResult
			for _, withMetadata := range []bool{false, true} {
				var names []string
				if withMetadata {
					names = []string{"metadata"}
				}
				options, err := registry.Resolve(names)
				if err != nil {
					t.Fatal(err)
				}
				var metadataBytes int64
				result, err := benchmarkDocumentWithOptions(corpusSpec{ID: "metadata", Cohort: "fixture", Input: "api.json"}, input, func(directory string) verificationResult {
					source, err := os.ReadFile(filepath.Join(directory, "metadata.ts"))
					if err != nil {
						t.Fatal(err)
					}
					metadataBytes = int64(len(source))
					if bytes.Contains(source, []byte("Original sentinel")) != withMetadata || bytes.Contains(source, []byte("@ts-nocheck")) {
						t.Fatal("metadata setting or measured bytes changed")
					}
					return verificationResult{Status: "pass"}
				}, nil, nil, options)
				if err != nil {
					t.Fatal(err)
				}
				if result.GenerationAddons == nil || !reflect.DeepEqual(*result.GenerationAddons, addonNames(options)) {
					t.Fatal("generation settings not recorded")
				}
				if err := validateGenerationAddons(result); err != nil {
					t.Fatal(err)
				}
				generation := result.Generation
				if inbound {
					profile := result.SupportProfiles[0]
					generation = profile.Generation
					want := []string{"server"}
					if withMetadata {
						want = []string{"metadata", "server"}
					}
					if profile.GenerationAddons == nil || !reflect.DeepEqual(*profile.GenerationAddons, want) {
						t.Fatal("server dropped metadata settings")
					}
				}
				if generation.MetadataBytes != metadataBytes || generation.MetadataBytes <= 0 || generation.Status != "pass" {
					t.Fatal("metadata bytes do not match strict input")
				}
				measured = append(measured, generation)
			}
			if measured[0].ArtifactCount != measured[1].ArtifactCount || measured[0].SchemaArtifactCount != measured[1].SchemaArtifactCount || measured[1].ArtifactBytes-measured[0].ArtifactBytes != measured[1].MetadataBytes-measured[0].MetadataBytes {
				t.Fatal("metadata option changed unrelated output")
			}
		})
	}
}

func TestHistoricalBenchmarkDoesNotInventMetadataSettings(t *testing.T) {
	var document documentResult
	if err := json.Unmarshal([]byte(`{"id":"old","generation":{"status":"pass"}}`), &document); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("generationAddons")) || bytes.Contains(data, []byte("metadataBytes")) {
		t.Fatal("historical metadata settings or size invented")
	}
}

func TestBenchmarkRejectsInvalidMetadataSettingsBeforeReadingInputs(t *testing.T) {
	for _, addons := range [][]string{{"metadata", "metadata"}, {"unknown"}, {"server"}} {
		if err := runBenchmarkSelection("missing.json", "missing", "missing-report.json", "missing", time.Second, "", "", addons...); err == nil || !strings.Contains(err.Error(), "add-on") {
			t.Fatalf("invalid add-ons accepted: %v", err)
		}
	}
}

func TestMergedMeasurementsKeepTypeScriptVersionBoundary(t *testing.T) {
	measurement := benchmarkMeasurement{MeasuredAt: "2026-10-02T00:00:00Z", GenerationScope: "compile-prepare-write", Samples: 1, TypeScriptVersion: "7.0.2"}
	var report benchmarkReport
	if err := mergeMeasurement(&report, &measurement); err != nil {
		t.Fatal(err)
	}
	if err := mergeMeasurement(&report, &measurement); err != nil {
		t.Fatal(err)
	}
	measurement.TypeScriptVersion = "5.9.3"
	if err := mergeMeasurement(&report, &measurement); err == nil {
		t.Fatal("different consumer compilers merged")
	}
}
