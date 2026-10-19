package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoModulePath(t *testing.T) {
	for _, test := range []struct {
		name, source, want string
	}{
		{"ordinary", "module example.test/app\n\ngo 1.27.0\n", "example.test/app"},
		{"quoted", "// module fake.test/comment\r\nmodule \"example.test/app/v2\" // comment\r\n", "example.test/app/v2"},
		{"attached comment", "module example.test/app// comment with an unmatched \" quote\n", "example.test/app"},
		{"attached block comment", "module (// comment\nexample.test/app// comment\n)// comment\n", "example.test/app"},
		{"raw quoted", "module `example.test/app`", ""},
		{"block", "module ( // comment\n\t\"example.test/app\"\n)\nrequire (\n module v1.0.0\n)\n", "example.test/app"},
		{"quoted keyword", "\"module\" example.test/app", ""},
		{"missing", "go 1.27.0\n", ""},
		{"empty", "module\n", ""},
		{"empty block", "module (\n)\n", ""},
		{"incomplete block", "module (\nexample.test/app\n", ""},
		{"multiple paths", "module first.test/app second.test/app\n", ""},
		{"multiple directives", "module first.test/app\nmodule second.test/app\n", ""},
		{"multiple block paths", "module (\nfirst.test/app\nsecond.test/app\n)\n", ""},
		{"invalid string", "module \"unfinished\n", ""},
		{"block comment", "/* module fake.test/app */\nmodule example.test/app", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual, err := goModulePath(test.source)
			if test.want == "" {
				if err == nil {
					t.Fatalf("accepted invalid module directive: %q", actual)
				}
			} else if err != nil || actual != test.want {
				t.Fatalf("got %q, %v; want %q", actual, err, test.want)
			}
		})
	}
}

func TestGoModulePathMatchesGoTool(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("Go toolchain unavailable for module syntax comparison")
	}
	t.Setenv("GOTOOLCHAIN", "local")
	t.Setenv("GOWORK", "off")
	t.Setenv("GOPROXY", "off")
	for _, source := range []string{
		"module example.test/app// comment with an unmatched \" quote\n",
		"module (\n\"example.test/app\"\n)\n",
		"module `example.test/app`\n",
		"\"module\" example.test/app\n",
	} {
		t.Run(source, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "go.mod")
			if err := os.WriteFile(file, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			output, toolErr := exec.Command(goTool, "mod", "edit", "-json", file).CombinedOutput()
			actual, parseErr := goModulePath(source)
			if toolErr != nil {
				if parseErr == nil {
					t.Fatalf("accepted a module directive rejected by Go: %q; Go: %s", actual, output)
				}
				return
			}
			var expected struct {
				Module struct{ Path string }
			}
			if err := json.Unmarshal(output, &expected); err != nil {
				t.Fatal(err)
			}
			if parseErr != nil || actual != expected.Module.Path {
				t.Fatalf("got %q, %v; Go resolves %q", actual, parseErr, expected.Module.Path)
			}
		})
	}
}

func TestResolveGoImportPath(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "go.mod"), "module example.test/parent\n")
	write(filepath.Join(root, "nested", "go.mod"), "module example.test/nested/v2\n")
	write(filepath.Join(root, "broken", "go.mod"), "go 1.27.0\n")
	write(filepath.Join(root, "file"), "not a directory")
	write(filepath.Join(root, "invalid", "go.mod"), "module ../invalid\n")
	for _, test := range []struct {
		name, output, configured, want string
		explicit                       bool
	}{
		{name: "module root", output: root, want: "example.test/parent"},
		{name: "new directories", output: filepath.Join(root, "internal", "api"), want: "example.test/parent/internal/api"},
		{name: "nested module", output: filepath.Join(root, "nested", "sdk"), want: "example.test/nested/v2/sdk"},
		{name: "explicit override", output: filepath.Join(root, "broken", "sdk"), configured: "example.test/override", explicit: true, want: "example.test/override"},
		{name: "empty explicit", output: root, explicit: true, want: ""},
		{name: "check without output", configured: "example.test/check", explicit: true, want: "example.test/check"},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual, err := resolveGoImportPath(test.output, test.configured, test.explicit)
			if err != nil || actual != test.want {
				t.Fatalf("got %q, %v; want %q", actual, err, test.want)
			}
		})
	}
	for _, output := range []string{"", filepath.Join(root, "broken", "sdk"), filepath.Join(root, "file", "sdk"), filepath.Join(root, "invalid", "sdk"), filepath.Join(t.TempDir(), "no-module", "sdk")} {
		if _, err := resolveGoImportPath(output, "", false); err == nil || !strings.Contains(err.Error(), "--go-import-path") {
			t.Fatalf("missing actionable inference error for %q: %v", output, err)
		}
	}
	t.Chdir(root)
	if actual, err := resolveGoImportPath(filepath.Join("nested", "sdk"), "", false); err != nil || actual != "example.test/nested/v2/sdk" {
		t.Fatalf("relative output was resolved incorrectly: %q, %v", actual, err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal")); !os.IsNotExist(err) {
		t.Fatal("inference created the output directory")
	}
}

func TestGoImportPathUsesPhysicalOutputModule(t *testing.T) {
	root := t.TempDir()
	for name, module := range map[string]string{"source": "example.test/source", "destination": "example.test/destination"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name, "go.mod"), []byte("module "+module+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "source", "alias")
	if err := os.Symlink(filepath.Join(root, "destination"), link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	actual, err := resolveGoImportPath(filepath.Join(link, "internal", "api"), "", false)
	if err != nil || actual != "example.test/destination/internal/api" {
		t.Fatalf("used lexical module instead of output module: %q, %v", actual, err)
	}
	deep := filepath.Join(root, "destination", "deep")
	if err := os.Mkdir(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	deepLink := filepath.Join(root, "source", "deep-alias")
	if err := os.Symlink(deep, deepLink); err != nil {
		t.Fatal(err)
	}
	// Joining with filepath.Join would erase the symlink-sensitive parent segment.
	suffix := string(filepath.Separator) + ".." + string(filepath.Separator) + "sdk"
	for _, output := range []string{deepLink + suffix, deepLink + suffix + string(filepath.Separator)} {
		actual, err := resolveGoImportPath(output, "", false)
		if err != nil || actual != "example.test/destination/sdk" {
			t.Fatalf("resolved %q using the wrong module: %q, %v", output, actual, err)
		}
	}
	t.Chdir(filepath.Join(root, "source"))
	actual, err = resolveGoImportPath("deep-alias"+suffix, "", false)
	if err != nil || actual != "example.test/destination/sdk" {
		t.Fatalf("relative symlink parent resolved incorrectly: %q, %v", actual, err)
	}
	previous := version
	version = "10.0.0-physical-go-test"
	t.Cleanup(func() { version = previous })
	input := filepath.Join(root, "input.json")
	if err := os.WriteFile(input, []byte(goHealthInput), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--input", input, "--output", deepLink + suffix, "--target", "golang"}
	if err := generate(args); err != nil {
		t.Fatal(err)
	}
	for _, extra := range [][]string{{"--check"}, {"--incremental"}, {"--check", "--go-import-path", "example.test/destination/sdk"}} {
		if err := generate(append(append([]string{}, args...), extra...)); err != nil {
			t.Fatal(err)
		}
	}
	outputLink := filepath.Join(root, "output-alias")
	if err := os.Symlink(filepath.Join(root, "destination", "sdk"), outputLink); err != nil {
		t.Fatal(err)
	}
	for _, extra := range [][]string{{}, {"--check"}, {"--incremental"}} {
		args := []string{"--input", input, "--output", outputLink, "--target", "golang"}
		if err := generate(append(args, extra...)); err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
			t.Fatalf("output root symlink was not rejected: %v", err)
		}
	}
}

func TestGoGenerationInfersImportPathAndKeepsFingerprint(t *testing.T) {
	previous := version
	version = "10.0.0-inferred-go-test"
	t.Cleanup(func() { version = previous })
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	input, output := filepath.Join(root, "input.json"), filepath.Join(root, "internal", "api")
	module := "module example.test/consumer\n\ngo 1.27.0\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(module), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, []byte(goHealthInput), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--input", input, "--output", output, "--target", "golang"}
	if err := generate(args); err != nil {
		t.Fatal(err)
	}
	for _, extra := range [][]string{{"--check"}, {"--check", "--go-import-path", "example.test/consumer/internal/api"}, {"--incremental"}} {
		if err := generate(append(append([]string{}, args...), extra...)); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := readArtifactManifestRecord(output)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Generation == nil || manifest.Generation.Golang == nil || manifest.Generation.Golang.ImportPath != "example.test/consumer/internal/api" {
		t.Fatal("fingerprint did not use the resolved import path")
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil || string(data) != module {
		t.Fatal("generation changed go.mod")
	}
	// An explicitly empty flag must remain invalid, rather than silently selecting inference.
	if err := generate(append(append([]string{}, args...), "--check", "--go-import-path", "")); err == nil {
		t.Fatal("accepted explicitly empty import path")
	}
}

func TestGoConfigInfersFromConfigOutput(t *testing.T) {
	previous := version
	version = "10.0.0-inferred-config-test"
	t.Cleanup(func() { version = previous })
	root := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":     "module example.test/config\n",
		"input.json": goHealthInput,
		"sdk.toml":   "source='input.json'\ntarget='golang'\noutput='sdk'\n[golang]\npackage='custom'\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	config := filepath.Join(root, "sdk.toml")
	if err := generate([]string{"--config", config}); err != nil {
		t.Fatal(err)
	}
	manifest, err := readArtifactManifestRecord(filepath.Join(root, "sdk"))
	if err != nil || manifest.Generation == nil || manifest.Generation.Golang == nil || manifest.Generation.Golang.ImportPath != "example.test/config/sdk" {
		t.Fatalf("configuration output was not used for inference: %v", err)
	}
	if err := os.WriteFile(config, []byte("source='input.json'\ntarget='golang'\noutput='other'\n[golang]\nimport_path=''\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := generate([]string{"--config", config}); err == nil {
		t.Fatal("accepted explicitly empty configured import path")
	}
}
