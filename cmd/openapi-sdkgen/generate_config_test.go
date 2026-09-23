package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/generator"
)

func TestGenerateProjectConfigSuppliesGenerationSettings(t *testing.T) {
	root := t.TempDir()
	configDirectory := filepath.Join(root, "project")
	if err := os.MkdirAll(configDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDirectory, "openapi-sdkgen.toml")
	config := `
source = "./api/openapi.yaml"
target = "typescript"
output = "./generated"
addons = ["server"]
diagnostics_format = "json"

[input]
base = "./refs"
tls_client_cert = "./certs/client.crt"
tls_client_key = "./certs/client.key"
tls_ca_file = "./certs/ca.pem"

[input.headers_from_env]
Authorization = "SDKGEN_AUTH"
X-API-Key = "SDKGEN_API_KEY"

[references]
allow = ["https://schemas.example.com"]
lock = "./refs.lock"
offline = true

[schema]
extensions = ["./extensions/custom.json"]
`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	var compiledInput string
	var compiledOptions compiler.CompileOptions
	var publishedOutput string
	runtime := generationRuntime{
		compile: func(input string, options compiler.CompileOptions) (compiler.Result, error) {
			compiledInput = input
			compiledOptions = options
			return compiler.Result{Document: &ir.Document{}}, nil
		},
		prepare: func(generator.Target, compiler.Result, generator.Options) (generator.Preparation, error) {
			return generator.Preparation{Plan: generator.NewPlan("typescript", struct{}{})}, nil
		},
		emit: func(generator.Target, generator.Plan) ([]generator.Artifact, error) {
			return nil, nil
		},
		publish: func(output string, _ []generator.Artifact, _ *artifactGeneration) error {
			publishedOutput = output
			return nil
		},
	}
	if err := generateWithRuntime([]string{"--config", configPath}, runtime); err != nil {
		t.Fatal(err)
	}

	wantPath := func(parts ...string) string {
		return filepath.Join(append([]string{configDirectory}, parts...)...)
	}
	if compiledInput != wantPath("api", "openapi.yaml") {
		t.Fatalf("compiled input = %q", compiledInput)
	}
	if publishedOutput != wantPath("generated") {
		t.Fatalf("published output = %q", publishedOutput)
	}
	if compiledOptions.InputBase != wantPath("refs") ||
		compiledOptions.RefLockPath != wantPath("refs.lock") ||
		compiledOptions.TLSClientCert != wantPath("certs", "client.crt") ||
		compiledOptions.TLSClientKey != wantPath("certs", "client.key") ||
		compiledOptions.TLSCAFile != wantPath("certs", "ca.pem") {
		t.Fatalf("path options = %#v", compiledOptions)
	}
	if !compiledOptions.Offline {
		t.Fatal("offline config was not applied")
	}
	if !reflect.DeepEqual(compiledOptions.RemoteRefAllowlist, []string{"https://schemas.example.com"}) {
		t.Fatalf("remote refs = %#v", compiledOptions.RemoteRefAllowlist)
	}
	if !reflect.DeepEqual(compiledOptions.SchemaExtensionManifests, []string{wantPath("extensions", "custom.json")}) {
		t.Fatalf("schema extensions = %#v", compiledOptions.SchemaExtensionManifests)
	}
	if !reflect.DeepEqual(compiledOptions.HTTPHeaderEnv, []string{
		"Authorization=SDKGEN_AUTH",
		"X-API-Key=SDKGEN_API_KEY",
	}) {
		t.Fatalf("header env mappings = %#v", compiledOptions.HTTPHeaderEnv)
	}
}

func TestGenerateProjectConfigCLIOverridesAndReplacesLists(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "openapi-sdkgen.toml")
	config := `
source = "./config-openapi.yaml"
target = "typescript"
output = "./config-output"
addons = ["server"]
incremental = true

[input.headers_from_env]
Authorization = "CONFIG_AUTH"

[references]
allow = ["https://config.example.com"]
offline = true

[schema]
extensions = ["./config-extension.json"]
`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	registries, err := newCLIRegistries()
	if err != nil {
		t.Fatal(err)
	}
	flags, values := newGenerateFlagSet(registries)
	args := []string{
		"--config", configPath,
		"--input", "cli-openapi.yaml",
		"--output", "cli-output",
		"--with", "server",
		"--allow-remote-ref", "https://cli.example.com",
		"--schema-extension", "cli-extension.json",
		"--http-header-env", "X-Token=CLI_TOKEN",
		"--offline=false",
		"--incremental=false",
	}
	if err := flags.Flags.Parse(args); err != nil {
		t.Fatal(err)
	}
	loaded, base, err := loadGenerateProjectConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	applyGenerateProjectConfig(loaded, base, values, visitedGenerateFlags(flags.Flags))

	if *values.input != "cli-openapi.yaml" || *values.output != "cli-output" {
		t.Fatalf("CLI scalar override lost: input=%q output=%q", *values.input, *values.output)
	}
	if *values.incremental || *values.offline {
		t.Fatalf("explicit false CLI booleans lost: incremental=%v offline=%v", *values.incremental, *values.offline)
	}
	if !reflect.DeepEqual([]string(values.with), []string{"server"}) {
		t.Fatalf("addons = %#v", values.with)
	}
	if !reflect.DeepEqual([]string(values.remoteRefs), []string{"https://cli.example.com"}) {
		t.Fatalf("remote refs merged instead of replaced: %#v", values.remoteRefs)
	}
	if !reflect.DeepEqual([]string(values.schemaExtensions), []string{"cli-extension.json"}) {
		t.Fatalf("schema extensions merged instead of replaced: %#v", values.schemaExtensions)
	}
	if !reflect.DeepEqual([]string(values.httpHeaderEnv), []string{"X-Token=CLI_TOKEN"}) {
		t.Fatalf("header env mappings merged instead of replaced: %#v", values.httpHeaderEnv)
	}
}

func TestGenerateProjectConfigRejectsUnknownFields(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "openapi-sdkgen.toml")
	if err := os.WriteFile(configPath, []byte("source = \"openapi.yaml\"\nunknown_setting = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := generateWithRuntime([]string{"--config", configPath}, generationRuntime{})
	if err == nil || !strings.Contains(err.Error(), "unknown field") || !strings.Contains(err.Error(), configPath) {
		t.Fatalf("error = %v", err)
	}
}

func TestGenerateProjectConfigMissingFileFailsBeforeGeneration(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.toml")
	err := generateWithRuntime([]string{"--config", configPath}, generationRuntime{})
	if err == nil || !strings.Contains(err.Error(), "open --config") || !strings.Contains(err.Error(), configPath) {
		t.Fatalf("error = %v", err)
	}
}

func TestGenerateProjectConfigLeavesURLsAndStdinSourcesUnchanged(t *testing.T) {
	root := t.TempDir()
	for _, value := range []string{"-", "https://api.example.com/openapi.yaml", "file:///tmp/openapi.yaml"} {
		t.Run(strings.ReplaceAll(value, "/", "_"), func(t *testing.T) {
			if got := resolveConfigSource(root, value); got != value {
				t.Fatalf("resolveConfigSource(%q) = %q", value, got)
			}
		})
	}
}
