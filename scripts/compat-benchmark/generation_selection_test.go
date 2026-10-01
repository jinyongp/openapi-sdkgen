package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectedShardsRemainSeparateAndValidateProvenance(t *testing.T) {
	manifest, _, shards := mergeFixture(t)
	policy, err := loadBenchmarkSelection(filepath.Join(filepath.Dir(manifest), "selections", "microsoft-graph-beta.toml"))
	if err != nil {
		t.Fatal(err)
	}
	graph := -1
	for i := range shards {
		if shards[i].Documents[0].ID == policy.Document {
			graph = i
		}
	}
	if graph < 0 {
		t.Fatal("missing Graph document")
	}
	document := &shards[graph].Documents[0]
	document.GenerationScope = "selected"
	document.Selection = &generationSelectionEvidence{FixtureSHA256: policy.fixtureSHA256, RuntimeProbeSHA256: policy.probeSHA256, Requested: policy.options,
		Routes: append([]string(nil), policy.options.Routes...), DependencyRoutes: []string{}, ExcludedOperations: document.OperationRetention.Total - len(policy.options.Routes), Runtime: verificationResult{Status: "pass"}}
	document.OperationEmission.Count = len(policy.options.Routes)
	document.Typecheck.Status = "pass"
	document.DocumentSuccess, document.CapabilityAdjustedSuccess = true, true
	output := filepath.Join(t.TempDir(), "report.json")
	if err := mergeBenchmarkReports(manifest, writeMergeShards(t, shards), output); err != nil {
		t.Fatal(err)
	}
	summary := summarizeDocuments("", []documentResult{*document})
	if summary.Documents != 0 || summary.SuccessfulDocuments != 0 || summary.SelectedDocuments != 1 || summary.SelectedSuccessfulDocuments != 1 {
		t.Fatalf("selected counted as full: %#v", summary)
	}
	for _, mutate := range []func(*documentResult){
		func(d *documentResult) { d.Selection.FixtureSHA256 = "wrong" },
		func(d *documentResult) { d.Selection.RuntimeProbeSHA256 = "wrong" },
		func(d *documentResult) { d.Selection.Routes = d.Selection.Routes[1:] },
		func(d *documentResult) { d.Selection.ExcludedOperations++ },
		func(d *documentResult) { d.Selection.DependencyRoutes = []string{d.Selection.Routes[0]} },
		func(d *documentResult) { d.GenerationScope = "full" },
		func(d *documentResult) { d.Selection.Runtime.Status = "fail" },
	} {
		clone := *document
		evidence := *document.Selection
		clone.Selection = &evidence
		mutate(&clone)
		changed := append([]benchmarkReport(nil), shards...)
		changed[graph].Documents = []documentResult{clone}
		if err := mergeBenchmarkReports(manifest, writeMergeShards(t, changed), output); err == nil {
			t.Fatal("invalid selected evidence accepted")
		}
	}
}

func TestSelectionFixtureRejectsInvalidPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "selection.toml")
	if err := os.WriteFile(filepath.Join(dir, "probe.mjs"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	prefix := "document='test'\ninput_sha256='aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'\nruntime_probe='probe.mjs'\n"
	for _, suffix := range []string{"[selection]\n", "[selection]\nroutes=['get invalid']", "unknown=true\n[selection]\nroutes=['GET /ok']"} {
		if err := os.WriteFile(path, []byte(prefix+suffix), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadBenchmarkSelection(path); err == nil {
			t.Fatal("invalid selection policy accepted")
		}
	}
}
