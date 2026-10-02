package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSelectBenchmarkCorpora(t *testing.T) {
	corpora := []corpusSpec{{ID: "first"}, {ID: "second"}}
	all, err := selectBenchmarkCorpora(corpora, "")
	if err != nil || !reflect.DeepEqual(all, corpora) {
		t.Fatalf("default selection = %v, %v", all, err)
	}
	one, err := selectBenchmarkCorpora(corpora, "second")
	if err != nil || !reflect.DeepEqual(one, corpora[1:]) {
		t.Fatalf("provider selection = %v, %v", one, err)
	}
	if _, err := selectBenchmarkCorpora(corpora, "missing"); err == nil {
		t.Fatal("unknown provider accepted")
	}
}

func mergeFixture(t *testing.T) (string, benchmarkReport, []benchmarkReport) {
	t.Helper()
	manifest := filepath.Join("..", "..", "test", "compatibility", "regression.json")
	data, err := os.ReadFile(filepath.Join("..", "..", "test", "compatibility", "regression-results.json"))
	if err != nil {
		t.Fatal(err)
	}
	var original benchmarkReport
	if err := json.Unmarshal(data, &original); err != nil {
		t.Fatal(err)
	}
	shards := make([]benchmarkReport, len(original.Documents))
	for i, document := range original.Documents {
		shards[i] = benchmarkReport{
			SchemaVersion: original.SchemaVersion, ManifestSHA256: original.ManifestSHA256,
			FeatureCatalog: original.FeatureCatalog, Documents: []documentResult{document},
			Measurement: original.Measurement,
			// Aggregate fields are deliberately absent: merge derives them from documents.
		}
	}
	return manifest, original, shards
}

func writeMergeShards(t *testing.T, shards []benchmarkReport) []string {
	t.Helper()
	root := t.TempDir()
	paths := make([]string, len(shards))
	for i, shard := range shards {
		// Separate repeated IDs so rejection tests exercise merge, not fixture overwrites.
		paths[i] = filepath.Join(root, string(rune('a'+i)), "report.json")
		if err := writeBenchmarkReport(shard, paths[i]); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

func TestMergeBenchmarkReportsPreservesMeasuredResults(t *testing.T) {
	manifest, original, shards := mergeFixture(t)
	// Runners may have different CPU models, and the latest date is chronological.
	measuredAt, err := time.Parse(time.RFC3339, original.Measurement.MeasuredAt)
	if err != nil {
		t.Fatal(err)
	}
	first := *original.Measurement
	first.CPU = "first runner"
	first.MeasuredAt = measuredAt.Add(time.Hour).In(time.FixedZone("UTC+9", 9*60*60)).Format(time.RFC3339)
	shards[0].Measurement = &first
	last := *original.Measurement
	last.CPU = "second runner"
	last.MeasuredAt = measuredAt.Add(2 * time.Hour).UTC().Format(time.RFC3339)
	shards[len(shards)-1].Measurement = &last
	paths := writeMergeShards(t, shards)
	// Artifact enumeration order must not affect document ordering or totals.
	for i, j := 0, len(paths)-1; i < j; i, j = i+1, j-1 {
		paths[i], paths[j] = paths[j], paths[i]
	}
	output := filepath.Join(t.TempDir(), "combined.json")
	if err := mergeBenchmarkReports(manifest, paths, output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var merged benchmarkReport
	if err := json.Unmarshal(data, &merged); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(merged.Documents, original.Documents) ||
		!reflect.DeepEqual(merged.Overall, original.Overall) || !reflect.DeepEqual(merged.Cohorts, original.Cohorts) {
		t.Fatal("merge changed measured document results or aggregate outcomes")
	}
	if merged.Measurement.CPU != "" || merged.Measurement.MeasuredAt != last.MeasuredAt {
		t.Fatalf("combined measurement = %#v", merged.Measurement)
	}
	for _, shard := range shards {
		id := shard.Documents[0].ID
		if !reflect.DeepEqual(merged.DocumentMeasurements[id], shard.Measurement) {
			t.Fatalf("runner measurement lost for %s", id)
		}
	}
}

func TestMergeBenchmarkReportsRejectsIncompleteOrInconsistentResults(t *testing.T) {
	cases := map[string]func([]benchmarkReport) []benchmarkReport{
		"missing":   func(shards []benchmarkReport) []benchmarkReport { return shards[1:] },
		"duplicate": func(shards []benchmarkReport) []benchmarkReport { return append(shards, shards[0]) },
		"manifest": func(shards []benchmarkReport) []benchmarkReport {
			shards[0].ManifestSHA256 = "wrong"
			return shards
		},
		"input": func(shards []benchmarkReport) []benchmarkReport {
			shards[0].Documents[0].InputSHA256 = "wrong"
			return shards
		},
		"success": func(shards []benchmarkReport) []benchmarkReport {
			shards[0].Documents[0].DocumentSuccess = !shards[0].Documents[0].DocumentSuccess
			return shards
		},
		"adjusted success": func(shards []benchmarkReport) []benchmarkReport {
			shards[0].Documents[0].CapabilityAdjustedSuccess = !shards[0].Documents[0].CapabilityAdjustedSuccess
			return shards
		},
		"measurement": func(shards []benchmarkReport) []benchmarkReport {
			shards[0].Measurement = nil
			return shards
		},
		"platform": func(shards []benchmarkReport) []benchmarkReport {
			measurement := *shards[0].Measurement
			measurement.GoVersion = "different"
			shards[0].Measurement = &measurement
			return shards
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			manifest, _, shards := mergeFixture(t)
			output := filepath.Join(t.TempDir(), "combined.json")
			if err := mergeBenchmarkReports(manifest, writeMergeShards(t, mutate(shards)), output); err == nil {
				t.Fatal("invalid shard reports accepted")
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatal("invalid merge published a report")
			}
		})
	}
}
