package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSelectedShardsRemainSeparateAndValidateProvenance(t *testing.T) {
	_, _, shards := mergeFixture(t)
	// Selection validation uses a regular typecheck candidate. Graph remains
	// generation-only even when a selection policy exists for it.
	document := &shards[0].Documents[0]
	directory := t.TempDir()
	manifest := filepath.Join(directory, "manifest.json")
	fixture := benchmarkManifest{SchemaVersion: benchmarkSchemaVersion}
	for _, shard := range shards {
		d := shard.Documents[0]
		fixture.Corpora = append(fixture.Corpora, corpusSpec{ID: d.ID, Cohort: d.Cohort, Input: d.Input, SHA256: d.InputSHA256, GitBlob: d.GitBlob, OpenAPIVersion: d.OpenAPIVersion, SizeClass: d.SizeClass})
	}
	data, _ := json.Marshal(fixture)
	if err := os.WriteFile(manifest, data, 0600); err != nil {
		t.Fatal(err)
	}
	manifestDigest := fmt.Sprintf("%x", sha256.Sum256(data))
	for i := range shards {
		shards[i].ManifestSHA256 = manifestDigest
	}
	selectionDir := filepath.Join(directory, "selections")
	if err := os.Mkdir(selectionDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(selectionDir, "probe.mjs"), []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(selectionDir, document.ID+".toml")
	policyData := fmt.Sprintf("document=%q\ninput_sha256=%q\nruntime_probe='probe.mjs'\n[selection]\nroutes=['GET /selected']\n", document.ID, document.InputSHA256)
	if err := os.WriteFile(policyPath, []byte(policyData), 0600); err != nil {
		t.Fatal(err)
	}
	policy, err := loadBenchmarkSelection(policyPath)
	if err != nil {
		t.Fatal(err)
	}
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
		changed[0].Documents = []documentResult{clone}
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
