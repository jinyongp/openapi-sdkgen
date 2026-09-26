package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestIncrementalGenerationPreservesOutputWhenScopedOmissionsLeaveNoEntry(t *testing.T) {
	directory := t.TempDir()
	input := filepath.Join(directory, "openapi.json")
	output := filepath.Join(directory, "generated")
	if err := os.WriteFile(input, []byte(minimalDocument), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"generate", "--input", input, "--target", "typescript", "--output", output}); err != nil {
		t.Fatal(err)
	}
	before := snapshotGeneratedDirectory(t, output)

	blocked := `{
  "openapi":"3.1.1",
  "info":{"title":"No client entry","version":"1"},
  "paths":{
    "/items":{"get":{
      "operationId":"getItems",
      "requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"string"}}}},
      "responses":{"204":{"description":"OK"}}
    }}
  }
}`
	if err := os.WriteFile(input, []byte(blocked), 0o600); err != nil {
		t.Fatal(err)
	}
	previousError := standardError
	var report bytes.Buffer
	standardError = &report
	t.Cleanup(func() { standardError = previousError })

	err := run([]string{"generate", "--input", input, "--target", "typescript", "--output", output, "--incremental"})
	if !errors.Is(err, errReportedDiagnostics) {
		t.Fatalf("blocked incremental generation error = %v", err)
	}
	for _, expected := range []string{"SDKGEN-W511", "SDKGEN-E512", "scope: document", "effect: block"} {
		if !strings.Contains(report.String(), expected) {
			t.Fatalf("blocked incremental diagnostics missing %q:\n%s", expected, report.String())
		}
	}
	after := snapshotGeneratedDirectory(t, output)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("blocked incremental generation changed managed output\nbefore=%v\nafter=%v", sortedSnapshotKeys(before), sortedSnapshotKeys(after))
	}
	assertIncrementalLockReusable(t, output)
}

func snapshotGeneratedDirectory(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = string(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}

func sortedSnapshotKeys(values map[string]string) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
