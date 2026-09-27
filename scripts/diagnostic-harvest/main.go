package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/target/typescript"
)

type manifest struct {
	Providers []provider `json:"providers"`
}

type provider struct {
	ID     string `json:"id"`
	Input  string `json:"input"`
	SHA256 string `json:"sha256,omitempty"`
}

type inventory struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Providers     []providerInventory `json:"providers"`
}

type providerInventory struct {
	ID            string                        `json:"id"`
	InputSHA256   string                        `json:"inputSha256"`
	Complete      bool                          `json:"complete"`
	Counts        diagnostic.Counts             `json:"counts"`
	Diagnostics   []diagnostic.Diagnostic       `json:"diagnostics"`
	Coverage      []diagnostic.AnalysisCoverage `json:"coverage"`
	SkippedPhases []diagnostic.SkippedPhase     `json:"skippedPhases"`
}

func main() {
	var manifestPath, outputPath string
	flag.StringVar(&manifestPath, "manifest", "", "provider corpus manifest JSON")
	flag.StringVar(&outputPath, "output", "", "normalized inventory JSON destination")
	flag.Parse()
	if manifestPath == "" || outputPath == "" {
		fatal(errors.New("--manifest and --output are required"))
	}
	if err := run(manifestPath, outputPath); err != nil {
		fatal(err)
	}
}

func run(manifestPath, outputPath string) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	var input manifest
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}
	base, err := filepath.Abs(filepath.Dir(manifestPath))
	if err != nil {
		return err
	}

	result := inventory{SchemaVersion: 1, Providers: make([]providerInventory, 0, len(input.Providers))}
	seen := map[string]bool{}
	for _, item := range input.Providers {
		if item.ID == "" || item.Input == "" {
			return errors.New("every provider requires id and input")
		}
		if seen[item.ID] {
			return fmt.Errorf("duplicate provider id %q", item.ID)
		}
		seen[item.ID] = true
		path := item.Input
		if !filepath.IsAbs(path) {
			path = filepath.Join(base, path)
		}
		value, err := harvestProvider(item.ID, path, item.SHA256)
		if err != nil {
			return err
		}
		result.Providers = append(result.Providers, value)
	}
	sort.Slice(result.Providers, func(i, j int) bool { return result.Providers[i].ID < result.Providers[j].ID })

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, encoded, 0o644); err != nil {
		return fmt.Errorf("write inventory: %w", err)
	}
	return nil
}

func harvestProvider(id, path, expectedSHA string) (providerInventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return providerInventory{}, fmt.Errorf("%s: read input: %w", id, err)
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	if expectedSHA != "" && digest != expectedSHA {
		return providerInventory{}, fmt.Errorf("%s: sha256 mismatch: got %s want %s", id, digest, expectedSHA)
	}

	mode := diagnostic.ModeCollect
	compiled, err := sdkgen.CompileInputResultWithOptions(path, sdkgen.CompileOptions{DiagnosticMode: mode})
	if err != nil {
		return providerInventory{}, fmt.Errorf("%s: internal compile failure: %w", id, err)
	}
	prepared, err := generator.PrepareCompilation(typescript.Generator{}, compiled, generator.Options{DiagnosticMode: mode})
	if err != nil {
		return providerInventory{}, fmt.Errorf("%s: internal target preparation failure: %w", id, err)
	}
	report := diagnostic.NewReport(prepared.Diagnostics, prepared.SkippedPhases, prepared.Coverage...)
	normalizeInventorySources(&report, path)
	return providerInventory{
		ID:            id,
		InputSHA256:   digest,
		Complete:      discoveryComplete(report),
		Counts:        report.Counts,
		Diagnostics:   report.Diagnostics,
		Coverage:      report.Coverage,
		SkippedPhases: report.SkippedPhases,
	}, nil
}

func normalizeInventorySources(report *diagnostic.Report, rootInput string) {
	rootInput, _ = filepath.Abs(rootInput)
	rootDirectory := filepath.Dir(rootInput)
	normalize := func(source string) string {
		if source == "" {
			return ""
		}
		absolute, err := filepath.Abs(source)
		if err != nil {
			return source
		}
		if absolute == rootInput {
			return "$root"
		}
		relative, err := filepath.Rel(rootDirectory, absolute)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
			return filepath.ToSlash(relative)
		}
		return source
	}
	for index := range report.Diagnostics {
		report.Diagnostics[index].Location.Source = normalize(report.Diagnostics[index].Location.Source)
		for related := range report.Diagnostics[index].Related {
			report.Diagnostics[index].Related[related].Source = normalize(report.Diagnostics[index].Related[related].Source)
		}
	}
	for index := range report.Coverage {
		if report.Coverage[index].Location != nil {
			report.Coverage[index].Location.Source = normalize(report.Coverage[index].Location.Source)
		}
	}
}

func discoveryComplete(report diagnostic.Report) bool {
	for _, phase := range report.SkippedPhases {
		if phase.Phase != diagnostic.PhaseEmit && phase.Phase != diagnostic.PhasePublish {
			return false
		}
	}
	for _, item := range report.Coverage {
		if item.Status != diagnostic.CoverageComplete {
			return false
		}
	}
	return true
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "diagnostic-harvest:", err)
	os.Exit(1)
}
