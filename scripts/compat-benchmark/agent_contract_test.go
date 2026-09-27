package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompatibilityCommandsRemainOptInFromStandardAgentFlows(t *testing.T) {
	for _, relative := range []string{
		filepath.Join("..", "agent", "check"),
		filepath.Join("..", "agent", "ci"),
		filepath.Join("..", "agent", "release-check"),
	} {
		data, err := os.ReadFile(relative)
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if strings.Contains(text, "compatibility-fetch") ||
			strings.Contains(text, "compatibility-benchmark") ||
			strings.Contains(text, "test/compatibility/holdout.json") {
			t.Fatalf("%s invokes opt-in compatibility corpus work", relative)
		}
	}
}

func TestAgentJustfileExposesCompatibilityCommands(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "just", "agent.just"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, recipe := range []string{
		"compatibility-fetch ",
		"compatibility-verify ",
		"compatibility-benchmark ",
	} {
		if !strings.Contains(text, recipe) {
			t.Fatalf("missing agent recipe %q", recipe)
		}
	}
}

func TestCompatibilityBenchmarkUsesPinnedNodeEnvironment(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "agent", "compatibility-benchmark"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, required := range []string{
		`fnm exec --using "$NODE_VERSION"`,
		"require_system_node",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("compatibility benchmark wrapper is missing pinned Node guard %q", required)
		}
	}
}
