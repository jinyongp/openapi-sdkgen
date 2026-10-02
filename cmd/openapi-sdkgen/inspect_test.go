package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/generator"
)

const inspectFixture = `{"openapi":"3.2.1","info":{"title":"Catalog","version":"1"},"paths":{
"/users":{"get":{"operationId":"listUsers","tags":["Users"],"summary":"Find accounts","responses":{"204":{"description":"OK"}}},"post":{"operationId":"createUser","tags":["Users"],"deprecated":true,"responses":{"204":{"description":"OK"}}}},
"/health":{"get":{"responses":{"204":{"description":"OK"}}}},
"/orders":{"get":{"operationId":"listOrders","tags":["Orders"],"responses":{"204":{"description":"OK"}}}},
"/custom":{"additionalOperations":{"CuStOm":{"operationId":"custom","responses":{"204":{"description":"OK"}}}}}
}}`

func inspectFixtureInput(t *testing.T) {
	t.Helper()
	previous := standardInput
	standardInput = strings.NewReader(inspectFixture)
	t.Cleanup(func() { standardInput = previous })
}

func TestInspectFiltersAndStableJSON(t *testing.T) {
	for _, test := range []struct {
		name   string
		flags  []string
		routes []string
	}{
		{"all", nil, []string{"CuStOm /custom", "GET /health", "GET /orders", "GET /users", "POST /users"}},
		{"search", []string{"--search", "ACCOUNTS"}, []string{"GET /users"}},
		{"methods", []string{"--method", "get", "--method", "post", "--tag", "Users", "--deprecated", "false"}, []string{"GET /users"}},
		{"custom", []string{"--method", "CuStOm"}, []string{"CuStOm /custom"}},
		{"combined exact", []string{"--operation", "listUsers", "--route", "GET /health"}, []string{"GET /health", "GET /users"}},
		{"tag union", []string{"--tag", "Users", "--tag", "Orders", "--method", "GET"}, []string{"GET /orders", "GET /users"}},
		{"search union", []string{"--search", "health", "--search", "orders"}, []string{"GET /health", "GET /orders"}},
		{"empty", []string{"--search", "absent"}, []string{}},
		{"exact intersection empty", []string{"--operation", "listUsers", "--tag", "Orders"}, []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			output, stderr := captureCLIOutput(t)
			inspectFixtureInput(t)
			args := append([]string{"inspect", "--input", "-", "--format", "json"}, test.flags...)
			if err := run(args); err != nil {
				t.Fatal(err)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr=%s", stderr)
			}
			var report inspectReport
			if err := json.Unmarshal(output.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if report.SchemaVersion != 1 || report.Total != 5 || report.Matched != len(test.routes) || report.DocumentsRead != 1 {
				t.Fatalf("report=%#v", report)
			}
			routes := []string{}
			for _, operation := range report.Operations {
				routes = append(routes, operation.Route)
			}
			if !reflect.DeepEqual(routes, test.routes) {
				t.Fatalf("routes=%v want=%v", routes, test.routes)
			}
		})
	}
}

func TestInspectInvalidFiltersAndEmptyExportHaveNoStdout(t *testing.T) {
	for _, flags := range [][]string{
		{"--operation", "missing"}, {"--route", "GET /missing"}, {"--route", "get /users"},
		{"--search", " "}, {"--tag", ""}, {"--method", "bad method"}, {"--deprecated", "1"}, {"--deprecated", ""},
		{"--format", "yaml"}, {"--search", "absent", "--format", "selection"}, {"--update-ref-lock"}, {"--output", "sdk"},
	} {
		t.Run(strings.Join(flags, " "), func(t *testing.T) {
			output, _ := captureCLIOutput(t)
			inspectFixtureInput(t)
			if err := run(append([]string{"inspect", "--input", "-"}, flags...)); err == nil {
				t.Fatal("expected failure")
			}
			if output.Len() != 0 {
				t.Fatalf("failure emitted stdout: %q", output.String())
			}
		})
	}
	output, stderr := captureCLIOutput(t)
	previous := standardInput
	standardInput = strings.NewReader(duplicateOperationInputForInspect)
	t.Cleanup(func() { standardInput = previous })
	if err := run([]string{"inspect", "--input", "-", "--operation", "same", "--diagnostics-format", "json"}); !errors.Is(err, errReportedDiagnostics) {
		t.Fatalf("error=%v", err)
	}
	if output.Len() != 0 || !strings.Contains(stderr.String(), "duplicated") {
		t.Fatalf("stdout=%s stderr=%s", output, stderr)
	}
}

const duplicateOperationInputForInspect = `{"openapi":"3.2.1","paths":{"/a":{"get":{"operationId":"same"}},"/b":{"get":{"operationId":"same"}}}}`

func TestInspectConfigUsesInputSettingsAndExportsUsableSelection(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	config := filepath.Join(dir, "project.toml")
	outputPath := filepath.Join(dir, "sdk")
	if err := os.WriteFile(input, []byte(inspectFixture), 0600); err != nil {
		t.Fatal(err)
	}
	configText := "source = 'input.json'\ntarget = 'typescript'\noutput = 'sdk'\naddons = ['metadata']\n[selection]\noperations = ['listOrders']\n"
	if err := os.WriteFile(config, []byte(configText), 0600); err != nil {
		t.Fatal(err)
	}
	output, _ := captureCLIOutput(t)
	if err := run([]string{"inspect", "--config", config, "--format", "json"}); err != nil {
		t.Fatal(err)
	}
	var report inspectReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Total != 5 || report.Matched != 5 {
		t.Fatalf("generation selection narrowed inspect: %#v", report)
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("inspect wrote output: %v", err)
	}
	output.Reset()
	if err := run([]string{"inspect", "--config", config, "--search", "health", "--search", "users", "--method", "GET", "--format", "selection"}); err != nil {
		t.Fatal(err)
	}
	var fragment generateProjectConfig
	if err := toml.Unmarshal(output.Bytes(), &fragment); err != nil {
		t.Fatal(err)
	}
	if fragment.Selection == nil || !reflect.DeepEqual(fragment.Selection.Routes, []string{"GET /health", "GET /users"}) {
		t.Fatalf("selection=%#v", fragment.Selection)
	}
	generatedConfig := filepath.Join(dir, "selected.toml")
	if err := os.WriteFile(generatedConfig, append([]byte("source = 'input.json'\ntarget = 'typescript'\noutput = 'sdk'\n"), output.Bytes()...), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"generate", "--config", generatedConfig}); err != nil {
		t.Fatal(err)
	}
	all, err := os.ReadFile(filepath.Join(outputPath, "selective", "all.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"GET /health", "GET /users"} {
		if !strings.Contains(string(all), route) {
			t.Fatalf("missing selected %q", route)
		}
	}
	for _, route := range []string{"GET /orders", "POST /users", "CuStOm /custom"} {
		if strings.Contains(string(all), route) {
			t.Fatalf("export leaked %q", route)
		}
	}
	data, err := os.ReadFile(config)
	if err != nil || string(data) != configText {
		t.Fatalf("inspect changed config: %v", err)
	}
}

func TestInspectSafeTableAndLosslessMachineOutputs(t *testing.T) {
	id := "한글\x1b[31m\nname"
	operation := compiler.InventoryOperation{Method: "GET", Path: "/한글/\"quoted\\path", Route: "GET /한글/\"quoted\\path", OperationID: &id, Summary: "line\n\x1b[31mtext", Tags: []string{"a\tb"}}
	report := inspectReport{SchemaVersion: 1, Total: 1, Matched: 1, Operations: []inspectOperation{{InventoryOperation: operation}}}
	var table bytes.Buffer
	if err := writeInspectReport(&table, report, "table"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(table.String(), "\x1b") || strings.Contains(table.String(), "a\tb") || !strings.Contains(table.String(), `\x1b`) {
		t.Fatalf("unsafe table: %q", table.String())
	}
	if inspectCellWidth("한글") != 4 || inspectCellWidth("e\u0301") != 1 {
		t.Fatal("incorrect display widths")
	}
	var selection bytes.Buffer
	if err := writeInspectReport(&selection, report, "selection"); err != nil {
		t.Fatal(err)
	}
	var parsed generateProjectConfig
	if err := toml.Unmarshal(selection.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Selection.Routes[0] != operation.Route {
		t.Fatalf("route roundtrip=%q", parsed.Selection.Routes[0])
	}
	var encoded bytes.Buffer
	if err := writeInspectReport(&encoded, report, "json"); err != nil {
		t.Fatal(err)
	}
	var decoded inspectReport
	if err := json.Unmarshal(encoded.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(report, decoded) {
		t.Fatalf("JSON not lossless: %#v", decoded)
	}
	if err := writeInspectReport(inspectFailWriter{}, report, "table"); err == nil {
		t.Fatal("output failure ignored")
	}
}

type inspectFailWriter struct{}

func (inspectFailWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestInspectDoesNotCompilePrepareOrPublishByDefault(t *testing.T) {
	output, _ := captureCLIOutput(t)
	inspectFixtureInput(t)
	registries, err := newCLIRegistries()
	if err != nil {
		t.Fatal(err)
	}
	runtime := generationRuntime{compile: func(string, compiler.CompileOptions) (compiler.Result, error) {
		t.Fatal("unexpected full compilation")
		return compiler.Result{}, nil
	}, emit: func(generator.Target, generator.Plan) ([]generator.Artifact, error) {
		t.Fatal("unexpected emit")
		return nil, nil
	}}
	if err := runWithRegistries([]string{"inspect", "--input", "-", "--format", "json"}, runtime, registries); err != nil {
		t.Fatal(err)
	}
	if output.Len() == 0 {
		t.Fatal("missing inventory")
	}
}

func TestInspectHelpAliases(t *testing.T) {
	var expected string
	for _, args := range [][]string{{"inspect", "--help"}, {"inspect", "-h"}, {"help", "inspect"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			output, stderr := captureCLIOutput(t)
			if err := run(args); err != nil {
				t.Fatal(err)
			}
			if stderr.Len() != 0 {
				t.Fatal(stderr.String())
			}
			if expected == "" {
				expected = output.String()
			}
			if output.String() != expected || !strings.Contains(expected, "--format") || strings.Contains(expected, "--update-ref-lock") {
				t.Fatalf("help=%s", output)
			}
		})
	}
}

func TestInspectTypeScriptUsesFullScopeBeforeFilteringWithoutEmission(t *testing.T) {
	output, _ := captureCLIOutput(t)
	inspectFixtureInput(t)
	registries, err := newCLIRegistries()
	if err != nil {
		t.Fatal(err)
	}
	runtime := defaultGenerationRuntime
	compile, prepare := runtime.compile, runtime.prepare
	compiles, prepares := 0, 0
	runtime.compile = func(input string, options compiler.CompileOptions) (compiler.Result, error) {
		compiles++
		return compile(input, options)
	}
	runtime.prepare = func(target generator.Target, compiled compiler.Result, options generator.Options) (generator.Preparation, error) {
		prepares++
		if len(compiled.Document.Operations) != 5 || options.Selection != nil || len(options.Addons()) != 0 {
			t.Fatal("target scope was narrowed")
		}
		return prepare(target, compiled, options)
	}
	runtime.emit = func(generator.Target, generator.Plan) ([]generator.Artifact, error) {
		t.Fatal("inspect emitted SDK")
		return nil, nil
	}
	runtime.publish = func(string, []generator.Artifact, *artifactGeneration) error {
		t.Fatal("inspect published SDK")
		return nil
	}
	if err := runWithRegistries([]string{"inspect", "--input", "-", "--target", "typescript", "--operation", "listUsers", "--format", "json"}, runtime, registries); err != nil {
		t.Fatal(err)
	}
	var report inspectReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if compiles != 1 || prepares != 1 || report.Total != 5 || report.Matched != 1 || report.Target != "typescript" || report.AnalysisScope != "full-document-client" {
		t.Fatalf("report=%#v compiles=%d prepares=%d", report, compiles, prepares)
	}
	inspection := report.Operations[0].TypeScript
	if inspection == nil || inspection.ResourceCall == nil || !strings.Contains(*inspection.ResourceCall, "api.users.list(") || !inspection.Routes || !inspection.Operations {
		t.Fatalf("inspection=%#v", inspection)
	}
}
