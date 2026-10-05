package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
)

func TestCLIInternalErrorProcess(t *testing.T) {
	if command := os.Getenv("SDKGEN_TEST_ERROR_COMMAND"); command != "" {
		defaultGenerationRuntime.compile = func(string, compiler.CompileOptions) (compiler.Result, error) {
			return compiler.Result{}, errors.New("synthetic invariant violated; process-environment-secret")
		}
		os.Args = []string{"openapi-sdkgen", command, "--input", "unused.json", "--target", "typescript"}
		if command == "generate" {
			os.Args = append(os.Args, "--output", "generated", "--diagnostics-format", "json")
		}
		main()
		return
	}
	for _, command := range []string{"generate", "inspect"} {
		t.Run(command, func(t *testing.T) {
			process := exec.Command(os.Args[0], "-test.run=^TestCLIInternalErrorProcess$")
			process.Dir = t.TempDir()
			process.Env = append(os.Environ(), "SDKGEN_TEST_ERROR_COMMAND="+command, "SDKGEN_TEST_TOKEN=process-environment-secret")
			var stdout, stderr bytes.Buffer
			process.Stdout, process.Stderr = &stdout, &stderr
			err := process.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || stdout.Len() != 0 {
				t.Fatalf("exit = %v, stdout = %s, stderr = %s", err, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), "cause: synthetic invariant violated;") || strings.Contains(stderr.String(), "process-environment-secret") {
				t.Fatalf("unsafe or missing cause: %s", stderr.String())
			}
			paths, err := filepath.Glob(filepath.Join(process.Dir, ".tmp", "runs", "internal-error-*", "diagnostic.json"))
			if err != nil || len(paths) != 1 {
				t.Fatalf("missing report: %v, %v", paths, err)
			}
			data, err := os.ReadFile(paths[0])
			if err != nil || strings.Contains(string(data), "process-environment-secret") || !strings.Contains(string(data), "synthetic invariant violated") {
				t.Fatalf("unsafe or missing saved cause: %s, %v", data, err)
			}
		})
	}
}

func TestCLIInternalErrorPreservesSafeCausesAndPrivateReport(t *testing.T) {
	root := errors.New("missing component SyntheticItem; environment-token; https://user:url-secret@example.test/spec?token=query-secret#fragment-secret; credential={\"nested\":[\"field-secret\"]}")
	err := fmt.Errorf("dispatch: %w", internalFailure("internal typescript preparation failure", fmt.Errorf("build execution plan: %w", root)))
	directory := filepath.Join(t.TempDir(), "runs")
	var output bytes.Buffer
	reportCLIError(&output, err, directory, []string{"API_TOKEN=environment-token"})
	if !strings.Contains(output.String(), "cause: build execution plan: missing component SyntheticItem") || !strings.Contains(output.String(), "internal error diagnostics:") {
		t.Fatalf("cause and diagnostic path not reported: %s", output.String())
	}
	if !errors.Is(err, root) {
		t.Fatal("original error identity lost")
	}
	paths, globErr := filepath.Glob(filepath.Join(directory, "internal-error-*", "diagnostic.json"))
	if globErr != nil || len(paths) != 1 {
		t.Fatalf("diagnostics files = %v, error = %v", paths, globErr)
	}
	data, readErr := os.ReadFile(paths[0])
	if readErr != nil {
		t.Fatal(readErr)
	}
	var report internalErrorReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.Version == "" || report.Stage != "internal typescript preparation failure" || len(report.Causes) != 2 || !strings.Contains(report.Causes[1], "missing component SyntheticItem") {
		t.Fatalf("incomplete report: %+v", report)
	}
	for _, secret := range []string{"environment-token", "url-secret", "query-secret", "fragment-secret", "field-secret"} {
		if strings.Contains(output.String()+string(data), secret) {
			t.Fatalf("credential exposed: %s", secret)
		}
	}
	if runtime.GOOS != "windows" {
		for path, want := range map[string]os.FileMode{filepath.Dir(paths[0]): 0o700, paths[0]: 0o600} {
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != want {
				t.Fatalf("permissions for %s: info=%v, error=%v", path, info, err)
			}
		}
	}
}

func TestCLIInternalReportFailureKeepsOriginalCause(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	reportCLIError(&output, internalFailure("internal compiler failure", errors.New("invariant violated")), filepath.Join(blocker, "runs"), nil)
	if !strings.Contains(output.String(), "cause: invariant violated") || !strings.Contains(output.String(), "could not save internal error diagnostics:") || strings.Contains(output.String(), "\ninternal error diagnostics:") {
		t.Fatalf("original cause or storage failure lost: %s", output.String())
	}
}

type cyclicInternalCause struct{}

func (*cyclicInternalCause) Error() string       { return strings.Repeat("가", internalCauseLimit+1) }
func (value *cyclicInternalCause) Unwrap() error { return value }

func TestInternalCauseCollectionBoundsCyclesAndHandlesJoinedErrors(t *testing.T) {
	var causes []string
	collectInternalCauses(&cyclicInternalCause{}, newErrorRedactor(nil), &causes)
	if len(causes) != internalCauseCount {
		t.Fatalf("cyclic cause count = %d", len(causes))
	}
	for _, cause := range causes {
		if len([]rune(cause)) != internalCauseLimit+1 || !strings.HasSuffix(cause, "…") {
			t.Fatal("unbounded or invalid Unicode cause")
		}
	}
	causes = nil
	collectInternalCauses(errors.Join(errors.New("first cause"), errors.New("second cause")), newErrorRedactor(nil), &causes)
	if len(causes) != 3 || causes[1] != "first cause" || causes[2] != "second cause" {
		t.Fatalf("joined causes = %v", causes)
	}
}

func TestCLIReportedAndOrdinaryErrorsDoNotCreateInternalReports(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "runs")
	var output bytes.Buffer
	reportCLIError(&output, nil, directory, nil)
	reportCLIError(&output, fmt.Errorf("wrapped: %w", errReportedDiagnostics), directory, nil)
	if output.Len() != 0 {
		t.Fatalf("duplicate diagnostics: %s", output.String())
	}
	reportCLIError(&output, errors.New("output already exists"), directory, nil)
	if output.String() != "openapi-sdkgen: output already exists\n" {
		t.Fatalf("ordinary error = %s", output.String())
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatalf("unexpected diagnostic directory: %v", err)
	}
}

func TestErrorRedactionPreservesCauseWithoutCredentials(t *testing.T) {
	redactor := newErrorRedactor([]string{"API_TOKEN=env-sensitive/value", "PRIVATE_KEY=line-one\nline-two", "DEVTOOLS_TASK_CONTEXT=private-context", "DISPLAY=public-display"})
	for _, test := range []struct{ name, message, forbidden string }{
		{"environment", "execution plan invariant: env-sensitive/value", "env-sensitive/value"},
		{"escaped environment", "execution plan invariant: env-sensitive%2Fvalue", "env-sensitive%2Fvalue"},
		{"context", "execution plan invariant: private-context", "private-context"},
		{"URL", "execution plan invariant: https://user:url-password@example.test/spec?token=query-secret#fragment-secret", "url-password"},
		{"malformed URL", "execution plan invariant: HTTPS://user:bad-password@%invalid/spec?key=query-secret#fragment-secret", "bad-password"},
		{"authorization", "execution plan invariant: Bearer bearer-sensitive", "bearer-sensitive"},
		{"JSON array", `execution plan invariant: {"refreshToken":["array-sensitive",{"value":"nested-sensitive"}]}`, "sensitive"},
		{"JSON escaped string", `execution plan invariant: {"password":"quote\"sensitive"}`, "sensitive"},
		{"key value", "execution plan invariant: api_key=kv-sensitive; retry later", "kv-sensitive"},
		{"single quoted", "execution plan invariant: authorization='two words sensitive'", "sensitive"},
		{"unfinished quoted", "execution plan invariant: password=\"two words sensitive", "sensitive"},
		{"unfinished single quoted", "execution plan invariant: password='two words sensitive", "sensitive"},
		{"unfinished JSON array", "execution plan invariant: credential=[\"sensitive\", \"also-sensitive\"", "sensitive"},
		{"private key", "execution plan invariant: -----BEGIN PRIVATE KEY-----\nkey-sensitive\n-----END PRIVATE KEY-----", "key-sensitive"},
		{"unfinished private key", "execution plan invariant: -----BEGIN PRIVATE KEY-----\nkey-sensitive", "key-sensitive"},
		{"control characters", "execution plan invariant:\n\x1b[31m failed", "\x1b"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := redactor.sanitize(test.message, 1000)
			if !strings.Contains(got, "execution plan invariant:") || strings.Contains(got, test.forbidden) || strings.ContainsAny(got, "\n\r\x1b") || strings.Contains(got, "query-secret") || strings.Contains(got, "fragment-secret") {
				t.Fatalf("unsafe or uninformative result: %s", got)
			}
		})
	}
	if got := redactor.sanitize("execution plan invariant: public-display", 1000); !strings.Contains(got, "public-display") {
		t.Fatalf("ordinary environment value removed: %s", got)
	}
	// Redact before truncation, even when the credential straddles the limit.
	got := redactor.sanitize("execution plan invariant: "+strings.Repeat("x", 100)+"password='straddling-sensitive'", 150)
	if strings.Contains(got, "straddling") {
		t.Fatalf("partial credential exposed: %s", got)
	}
}
