package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"time"

	toml "github.com/pelletier/go-toml/v2"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/target/typescript"
)

type benchmarkSelection struct {
	Document     string `toml:"document"`
	InputSHA256  string `toml:"input_sha256"`
	RuntimeProbe string `toml:"runtime_probe"`
	Selection    struct {
		Operations []string `toml:"operations"`
		Routes     []string `toml:"routes"`
	} `toml:"selection"`
	options       *generator.Selection
	fixtureSHA256 string
	probeSHA256   string
	probePath     string
}

type generationSelectionEvidence struct {
	FixtureSHA256      string               `json:"fixtureSha256"`
	RuntimeProbeSHA256 string               `json:"runtimeProbeSha256"`
	Requested          *generator.Selection `json:"requested"`
	Routes             []string             `json:"routes"`
	DependencyRoutes   []string             `json:"dependencyRoutes"`
	ExcludedOperations int                  `json:"excludedOperations"`
	Runtime            verificationResult   `json:"runtime"`
}

func loadBenchmarkSelection(path string) (*benchmarkSelection, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var selection benchmarkSelection
	if err := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&selection); err != nil {
		return nil, fmt.Errorf("decode generation selection: %w", err)
	}
	selection.options, err = (&generator.Selection{Operations: selection.Selection.Operations, Routes: selection.Selection.Routes}).Canonical()
	if err != nil {
		return nil, err
	}
	if selection.Document == "" || len(selection.InputSHA256) != 64 || selection.RuntimeProbe == "" || filepath.Base(selection.RuntimeProbe) != selection.RuntimeProbe {
		return nil, fmt.Errorf("generation selection requires document, input_sha256 and a sibling runtime_probe filename")
	}
	if _, err := hex.DecodeString(selection.InputSHA256); err != nil {
		return nil, fmt.Errorf("invalid selection input hash: %w", err)
	}
	selection.probePath, err = filepath.Abs(filepath.Join(filepath.Dir(path), selection.RuntimeProbe))
	if err != nil {
		return nil, err
	}
	probe, err := os.ReadFile(selection.probePath)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	selection.fixtureSHA256 = hex.EncodeToString(digest[:])
	digest = sha256.Sum256(probe)
	selection.probeSHA256 = hex.EncodeToString(digest[:])
	return &selection, nil
}

func (selection *benchmarkSelection) evidence(plan generator.Plan, total int) (*generationSelectionEvidence, error) {
	if selection == nil {
		return nil, nil
	}
	details, err := typescript.SelectionDetails(plan)
	if err != nil {
		return nil, err
	}
	if details == nil {
		return nil, fmt.Errorf("selected benchmark received a full generation plan")
	}
	return &generationSelectionEvidence{FixtureSHA256: selection.fixtureSHA256, RuntimeProbeSHA256: selection.probeSHA256, Requested: selection.options,
		Routes: details.Routes, DependencyRoutes: details.DependencyRoutes, ExcludedOperations: total - len(details.Routes) - len(details.DependencyRoutes), Runtime: verificationResult{Status: "not-run"}}, nil
}

func verifySelectionRuntime(directory, typescriptRoot string, selection *benchmarkSelection) verificationResult {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	started := time.Now()
	command := exec.CommandContext(ctx, "node", selection.probePath, directory, typescriptRoot)
	output, err := command.CombinedOutput()
	duration := float64(time.Since(started)) / float64(time.Millisecond)
	status := "pass"
	if ctx.Err() != nil {
		status = "timeout"
	} else if err != nil {
		status = "fail"
	}
	return verificationResult{Status: status, Detail: boundedDetail(string(output)), DurationMillis: &duration}
}

func validateSelectionEvidence(document documentResult, corpus corpusSpec, selection *benchmarkSelection) error {
	if selection == nil {
		if document.GenerationScope != "" && document.GenerationScope != "full" || document.Selection != nil {
			return fmt.Errorf("unexpected generation selection for %s", document.ID)
		}
		return nil
	}
	evidence := document.Selection
	if selection.Document != corpus.ID || selection.InputSHA256 != document.InputSHA256 || document.GenerationScope != "selected" || evidence == nil ||
		evidence.FixtureSHA256 != selection.fixtureSHA256 || evidence.RuntimeProbeSHA256 != selection.probeSHA256 || !reflect.DeepEqual(evidence.Requested, selection.options) {
		return fmt.Errorf("generation selection provenance mismatch for %s", document.ID)
	}
	if (document.Generation.Status == "pass" && len(evidence.Routes) == 0) || evidence.ExcludedOperations < 0 {
		return fmt.Errorf("invalid selected operation counts for %s", document.ID)
	}
	if document.Generation.Status == "pass" {
		if !canonicalRouteList(evidence.Routes) || !canonicalRouteList(evidence.DependencyRoutes) ||
			(len(selection.options.Operations) == 0 && !slices.Equal(evidence.Routes, selection.options.Routes)) ||
			document.OperationRetention.Total != len(evidence.Routes)+len(evidence.DependencyRoutes)+evidence.ExcludedOperations ||
			document.OperationEmission == nil || !document.OperationEmission.Available || document.OperationEmission.Count != len(evidence.Routes) {
			return fmt.Errorf("selected API membership or counts differ for %s", document.ID)
		}
		for _, route := range evidence.DependencyRoutes {
			if slices.Contains(evidence.Routes, route) {
				return fmt.Errorf("public/dependency overlap for %s", document.ID)
			}
		}
	}
	return nil
}

func canonicalRouteList(routes []string) bool {
	for index, route := range routes {
		if route == "" || (index > 0 && routes[index-1] >= route) {
			return false
		}
	}
	return true
}

func documentVerificationSuccess(document documentResult) bool {
	passed := document.DiscoveryComplete && document.Diagnostics.Errors == 0 && document.Generation.Status == "pass" && document.Typecheck.Status == "pass"
	if document.GenerationScope == "selected" {
		passed = passed && document.Selection != nil && document.Selection.Runtime.Status == "pass"
	}
	return passed
}
