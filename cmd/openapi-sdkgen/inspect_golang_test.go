package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectGolangPreservesFullScopeAndJSONKeys(t *testing.T) {
	output, _ := captureCLIOutput(t)
	inspectFixtureInput(t)
	if err := run([]string{"inspect", "--input", "-", "--target", "golang", "--operation", "listUsers", "--format", "json"}); err != nil {
		t.Fatal(err)
	}
	var report inspectReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Target != "golang" || report.Total != 5 || report.Matched != 1 || report.AnalysisScope != "full-document-client" {
		t.Fatal(report)
	}
	inspection := report.Operations[0].Golang
	if inspection == nil || inspection.ResourceCall == nil || *inspection.ResourceCall != "api.Users.List().Do(ctx)" || !inspection.Routes || !inspection.Operations || report.Operations[0].TypeScript != nil {
		t.Fatalf("inspection %#v", inspection)
	}
	if strings.Contains(output.String(), "typescript") {
		t.Fatal("TypeScript JSON key leaked")
	}
}

func TestInspectGolangIgnoresGenerationConfiguration(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input.json")
	config := filepath.Join(root, "sdk.toml")
	if err := os.WriteFile(input, []byte(inspectFixture), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("source='input.json'\ntarget='golang'\noutput='sdk'\naddons=['server']\n[golang]\npackage='main'\nimport_path=''\n[selection]\noperations=['missing']\n[clients.other.selection]\noperations=['missing']\n"), 0600); err != nil {
		t.Fatal(err)
	}
	output, _ := captureCLIOutput(t)
	if err := run([]string{"inspect", "--config", config, "--target", "golang", "--format", "json"}); err != nil {
		t.Fatal(err)
	}
	var report inspectReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Total != 5 || report.Matched != 5 || report.Target != "golang" {
		t.Fatal(report)
	}
	if _, err := os.Stat(filepath.Join(root, "sdk")); !os.IsNotExist(err) {
		t.Fatal("inspect created output", err)
	}
	if err := run([]string{"inspect", "--config", config, "--target", "golang", "--go-package", "main"}); err == nil || !strings.Contains(err.Error(), "inspect --help") {
		t.Fatal("explicit invalid Go options not rejected as usage", err)
	}
}
