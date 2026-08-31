// openapi-sdkgen compiles OpenAPI documents into client SDK packages.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/target/typescript"
)

var standardInput io.Reader = os.Stdin
var standardOutput io.Writer = os.Stdout
var standardError io.Writer = os.Stderr

var version string

var errReportedDiagnostics = errors.New("generation blocked by reported diagnostics")

type internalGenerationError struct {
	label string
	cause error
}

func (value *internalGenerationError) Error() string { return value.label }
func (value *internalGenerationError) Unwrap() error { return value.cause }

func internalFailure(label string, cause error) error {
	return &internalGenerationError{label: label, cause: cause}
}

type generationRuntime struct {
	compile            func(string, compiler.CompileOptions) (compiler.Result, error)
	prepare            func(generator.Target, compiler.Result, generator.Options) (generator.Preparation, error)
	emit               func(generator.Target, generator.Plan) ([]generator.Artifact, error)
	publish            func(string, []generator.Artifact) error
	publishIncremental func(string, []generator.Artifact) error
	stream             func(generator.Target, generator.Plan, string) error
	streamIncremental  func(generator.Target, generator.Plan, string) error
}

type generationStageError struct {
	stage string
	err   error
}

func (value *generationStageError) Error() string { return value.err.Error() }
func (value *generationStageError) Unwrap() error { return value.err }

type cliRegistries struct {
	targets *generator.Registry
	addons  *generator.AddonRegistry
}

type cliRootOption struct {
	Metadata helpOption
	Aliases  []string
	Run      func(string, []string) error
}

type cliApplication struct {
	registries cliRegistries
	commands   []cliCommand
	options    []cliRootOption
}

type generateFlagValues struct {
	input            *string
	inputBase        *string
	targetName       *string
	output           *string
	with             repeatedStrings
	remoteRefs       repeatedStrings
	schemaExtensions repeatedStrings
	httpHeaderEnv    rawStrings
	refLock          *string
	updateRefLock    *bool
	offline          *bool
	incremental      *bool
	help             *bool
	tlsClientCert    *string
	tlsClientKey     *string
	tlsCAFile        *string
}

var defaultGenerationRuntime = generationRuntime{
	compile: compiler.CompileInputResultWithOptions,
	prepare: generator.PrepareCompilation,
	emit: func(target generator.Target, plan generator.Plan) ([]generator.Artifact, error) {
		return target.Emit(plan)
	},
	publish:            writeArtifacts,
	publishIncremental: writeArtifactsIncremental,
	stream:             streamArtifacts,
	streamIncremental:  streamArtifactsIncremental,
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		if !errors.Is(err, errReportedDiagnostics) {
			fmt.Fprintf(standardError, "openapi-sdkgen: %v\n", err)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	registries, err := newCLIRegistries()
	if err != nil {
		return err
	}
	return runWithRegistries(args, defaultGenerationRuntime, registries)
}

func runWithRegistries(args []string, runtime generationRuntime, registries cliRegistries) error {
	application := newCLIApplication(runtime, registries)
	return application.run(args)
}

func newCLIApplication(runtime generationRuntime, registries cliRegistries) *cliApplication {
	application := &cliApplication{registries: registries}
	application.commands = []cliCommand{
		{
			Name:    "generate",
			Summary: "Generate SDK source",
			Run: func(args []string) error {
				return generateWithRegistries(args, runtime, registries)
			},
			Help: func() error {
				return writeGenerateHelp(registries)
			},
		},
	}
	application.options = newRootOptions(application)
	return application
}

func newRootOptions(application *cliApplication) []cliRootOption {
	return []cliRootOption{
		{
			Metadata: helpOption{Name: "help", Short: "h", Summary: "Show help"},
			Aliases:  []string{"help"},
			Run: func(invoked string, args []string) error {
				if strings.HasPrefix(invoked, "-") {
					if len(args) != 0 {
						return rootUsageError("help does not accept additional arguments")
					}
					return application.writeRootHelp()
				}
				switch len(args) {
				case 0:
					return application.writeRootHelp()
				case 1:
					if command, ok := lookupCommand(application.commands, args[0]); ok {
						return command.Help()
					}
					return rootUsageError(fmt.Sprintf("unknown command %q", args[0]))
				default:
					return rootUsageError("help accepts at most one command")
				}
			},
		},
		{
			Metadata: helpOption{Name: "version", Summary: "Show version"},
			Run: func(_ string, args []string) error {
				if len(args) != 0 {
					return rootUsageError("version does not accept additional arguments")
				}
				return writeVersion()
			},
		},
	}
}

func (application *cliApplication) run(args []string) error {
	if len(args) == 0 {
		return application.writeRootHelp()
	}
	if option, ok := lookupRootOption(application.options, args[0]); ok {
		return option.Run(args[0], args[1:])
	}
	if command, ok := lookupCommand(application.commands, args[0]); ok {
		return command.Run(args[1:])
	}
	return rootUsageError(fmt.Sprintf("unknown command %q", args[0]))
}

func lookupCommand(commands []cliCommand, name string) (cliCommand, bool) {
	for _, command := range commands {
		if command.Name == name {
			return command, true
		}
	}
	return cliCommand{}, false
}

func lookupRootOption(options []cliRootOption, name string) (cliRootOption, bool) {
	for _, option := range options {
		if name == "--"+option.Metadata.Name ||
			option.Metadata.Short != "" && name == "-"+option.Metadata.Short {
			return option, true
		}
		for _, alias := range option.Aliases {
			if name == alias {
				return option, true
			}
		}
	}
	return cliRootOption{}, false
}

func generate(args []string) error {
	return generateWithRuntime(args, defaultGenerationRuntime)
}

func generateWithRuntime(args []string, runtime generationRuntime) error {
	registries, err := newCLIRegistries()
	if err != nil {
		return err
	}
	return generateWithRegistries(args, runtime, registries)
}

func generateWithRegistries(args []string, runtime generationRuntime, registries cliRegistries) error {
	flags, values := newGenerateFlagSet(registries)
	if err := flags.Flags.Parse(args); err != nil {
		return generateUsageError(fmt.Sprintf("parse generate arguments: %v", err))
	}
	if *values.help {
		return writeGenerateHelpWithFlags(registries, flags)
	}
	if flags.Flags.NArg() != 0 {
		return generateUsageError(fmt.Sprintf("unexpected arguments: %s", strings.Join(flags.Flags.Args(), " ")))
	}
	if *values.input == "" || *values.targetName == "" || *values.output == "" {
		return generateUsageError("--input, --target, and --output are required")
	}
	target, err := registries.targets.Lookup(*values.targetName)
	if err != nil {
		return err
	}
	options, err := registries.addons.Resolve(values.with)
	if err != nil {
		return err
	}
	if err := generator.ValidateTargetOptions(target, options); err != nil {
		return err
	}
	compiled, err := runtime.compile(*values.input, compiler.CompileOptions{
		InputBase:                *values.inputBase,
		InputReader:              standardInput,
		RemoteRefAllowlist:       values.remoteRefs,
		RefLockPath:              *values.refLock,
		UpdateRefLock:            *values.updateRefLock,
		Offline:                  *values.offline,
		SchemaExtensionManifests: values.schemaExtensions,
		HTTPHeaderEnv:            values.httpHeaderEnv,
		TLSClientCert:            *values.tlsClientCert,
		TLSClientKey:             *values.tlsClientKey,
		TLSCAFile:                *values.tlsCAFile,
	})
	if err != nil {
		writeDiagnostics(compiled.Diagnostics, compiled.SkippedPhases)
		return internalFailure("internal compiler failure", err)
	}
	prepared, err := runtime.prepare(target, compiled, options)
	if err != nil {
		writeDiagnostics(prepared.Diagnostics, prepared.SkippedPhases)
		return internalFailure(fmt.Sprintf("internal %s preparation failure", target.Name()), err)
	}
	writeDiagnostics(prepared.Diagnostics, prepared.SkippedPhases)
	if diagnostic.HasErrors(prepared.Diagnostics) {
		return errReportedDiagnostics
	}
	stream := runtime.stream
	if *values.incremental {
		stream = runtime.streamIncremental
	}
	if stream != nil {
		if err := stream(target, prepared.Plan, *values.output); err != nil {
			var staged *generationStageError
			if errors.As(err, &staged) && staged.stage == "publish" {
				return internalFailure("internal output publication failure", err)
			}
			return internalFailure(fmt.Sprintf("internal %s emission failure", target.Name()), err)
		}
		return nil
	}
	artifacts, err := runtime.emit(target, prepared.Plan)
	if err != nil {
		return internalFailure(fmt.Sprintf("internal %s emission failure", target.Name()), err)
	}
	publish := runtime.publish
	if *values.incremental {
		publish = runtime.publishIncremental
		if publish == nil {
			return internalFailure("internal output publication failure", errors.New("incremental publisher is not configured"))
		}
	}
	if err := publish(*values.output, artifacts); err != nil {
		return internalFailure("internal output publication failure", err)
	}
	return nil
}

func newCLIRegistries() (cliRegistries, error) {
	targets, err := generator.NewRegistry(typescript.Generator{})
	if err != nil {
		return cliRegistries{}, err
	}
	addons, err := generator.NewAddonRegistry(generator.AddonServer)
	if err != nil {
		return cliRegistries{}, err
	}
	return cliRegistries{targets: targets, addons: addons}, nil
}

func newGenerateFlagSet(registries cliRegistries) (*commandFlagSet, *generateFlagValues) {
	const (
		requiredGroup = iota
		generationGroup
		inputGroup
		remoteReferenceGroup
		schemaExtensionGroup
		optionsGroup
	)
	flags := newCommandFlagSet(
		"generate",
		"Required",
		"Generation",
		"Input",
		"Remote references",
		"Schema extensions",
		"Options",
	)
	values := &generateFlagValues{}
	values.input = flags.String(requiredGroup, helpOption{
		Name: "input", Metavariable: "source",
		Summary: "OpenAPI file, file:// URL, HTTP(S) URL, or -",
	}, "")
	values.targetName = flags.String(requiredGroup, helpOption{
		Name: "target", Metavariable: "name", Summary: "SDK target",
		Available: registries.targets.Names,
	}, "")
	values.output = flags.String(requiredGroup, helpOption{
		Name: "output", Metavariable: "directory", Summary: "Generated-code directory",
	}, "")
	values.incremental = flags.Bool(generationGroup, helpOption{
		Name: "incremental", Summary: "Update a manifest-owned output directory",
	}, false)
	flags.Var(generationGroup, helpOption{
		Name: "with", Metavariable: "addon", Summary: "Add generated artifacts",
		Repeatable: true, Available: registries.addons.Names,
	}, &values.with)
	values.inputBase = flags.String(inputGroup, helpOption{
		Name: "input-base", Metavariable: "source",
		Summary: "Base location for relative references from stdin",
	}, "")
	flags.Var(inputGroup, helpOption{
		Name: "http-header-env", Metavariable: "header=env",
		Summary:    "Read an input request header from an environment variable",
		Repeatable: true,
	}, &values.httpHeaderEnv)
	values.tlsClientCert = flags.String(inputGroup, helpOption{
		Name: "tls-client-cert", Metavariable: "path",
		Summary: "PEM client certificate for an HTTPS input",
	}, "")
	values.tlsClientKey = flags.String(inputGroup, helpOption{
		Name: "tls-client-key", Metavariable: "path",
		Summary: "PEM private key for an HTTPS input",
	}, "")
	values.tlsCAFile = flags.String(inputGroup, helpOption{
		Name: "tls-ca-file", Metavariable: "path",
		Summary: "Additional PEM certificate authorities for an HTTPS input",
	}, "")
	flags.Var(remoteReferenceGroup, helpOption{
		Name: "allow-remote-ref", Metavariable: "origin",
		Summary: "Allow an exact HTTPS remote-reference origin", Repeatable: true,
	}, &values.remoteRefs)
	values.refLock = flags.String(remoteReferenceGroup, helpOption{
		Name: "ref-lock", Metavariable: "path",
		Summary: "Remote-reference and extension lock path",
	}, "")
	values.updateRefLock = flags.Bool(remoteReferenceGroup, helpOption{
		Name: "update-ref-lock", Summary: "Create or update the integrity lock",
	}, false)
	values.offline = flags.Bool(remoteReferenceGroup, helpOption{
		Name: "offline", Summary: "Use only locked cached remote references",
	}, false)
	flags.Var(schemaExtensionGroup, helpOption{
		Name: "schema-extension", Metavariable: "manifest",
		Summary: "Register a trusted schema-extension manifest", Repeatable: true,
	}, &values.schemaExtensions)
	values.help = flags.Bool(optionsGroup, helpOption{
		Name: "help", Short: "h", Summary: "Show help",
	}, false)
	return flags, values
}

func (application *cliApplication) writeRootHelp() error {
	options := make([]helpOption, 0, len(application.options))
	for _, option := range application.options {
		options = append(options, option.Metadata)
	}
	document := helpDocument{
		Description: "openapi-sdkgen generates application SDK source from OpenAPI documents.",
		Usage:       "openapi-sdkgen <command> [options]",
		Commands:    application.commands,
		Groups: []helpOptionGroup{
			{
				Title:   "Options",
				Options: options,
			},
		},
		Footer: `Run "openapi-sdkgen <command> --help" for command details.`,
	}
	if application.registries.targets == nil || application.registries.addons == nil {
		return errors.New("CLI registries are not configured")
	}
	if err := renderHelp(standardOutput, document); err != nil {
		return fmt.Errorf("render root help: %w", err)
	}
	return nil
}

func writeGenerateHelp(registries cliRegistries) error {
	if registries.targets == nil || registries.addons == nil {
		return errors.New("CLI registries are not configured")
	}
	flags, _ := newGenerateFlagSet(registries)
	return writeGenerateHelpWithFlags(registries, flags)
}

func writeGenerateHelpWithFlags(registries cliRegistries, flags *commandFlagSet) error {
	if registries.targets == nil || registries.addons == nil {
		return errors.New("CLI registries are not configured")
	}
	document := helpDocument{
		Description: "Generate application SDK source from an OpenAPI document.",
		Usage:       "openapi-sdkgen generate [options]",
		Groups:      flags.Groups,
		Examples: []string{`openapi-sdkgen generate \
  --input ./openapi.yaml \
  --target typescript \
  --output ./src/generated/api`},
	}
	if err := renderHelp(standardOutput, document); err != nil {
		return fmt.Errorf("render generate help: %w", err)
	}
	return nil
}

func rootUsageError(message string) error {
	return fmt.Errorf("%s\nTry \"openapi-sdkgen --help\" for usage", message)
}

func generateUsageError(message string) error {
	return fmt.Errorf("%s\nTry \"openapi-sdkgen generate --help\" for usage", message)
}

func writeVersion() error {
	if _, err := fmt.Fprintf(standardOutput, "openapi-sdkgen %s\n", resolvedVersion()); err != nil {
		return fmt.Errorf("write version: %w", err)
	}
	return nil
}

func resolvedVersion() string {
	if value := normalizeVersion(version); value != "" {
		return value
	}
	if build, ok := debug.ReadBuildInfo(); ok {
		if value := versionFromBuildInfo(build); value != "" {
			return value
		}
	}
	return "dev"
}

func versionFromBuildInfo(build *debug.BuildInfo) string {
	for _, setting := range build.Settings {
		if strings.HasPrefix(setting.Key, "vcs.") {
			return ""
		}
	}
	return normalizeVersion(build.Main.Version)
}

func normalizeVersion(value string) string {
	if value == "" || value == "(devel)" {
		return ""
	}
	return strings.TrimPrefix(value, "v")
}

func writeDiagnostics(values []diagnostic.Diagnostic, skipped []diagnostic.SkippedPhase) {
	if len(values) == 0 && len(skipped) == 0 {
		return
	}
	fmt.Fprint(standardError, diagnostic.RenderHuman(values, skipped))
}

type repeatedStrings []string

func (values *repeatedStrings) String() string {
	return strings.Join(*values, ",")
}

type rawStrings []string

func (values *rawStrings) String() string {
	return strings.Join(*values, ",")
}

func (values *rawStrings) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func (values *repeatedStrings) Set(value string) error {
	if value == "" {
		return errors.New("--with requires a non-empty add-on name")
	}
	*values = append(*values, value)
	return nil
}

func writeArtifacts(output string, artifacts []generator.Artifact) error {
	return writeArtifactsWithMode(output, artifacts, false)
}

func writeArtifactsIncremental(output string, artifacts []generator.Artifact) error {
	return writeArtifactsWithMode(output, artifacts, true)
}

func writeArtifactsWithMode(output string, artifacts []generator.Artifact, incremental bool) error {
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	seen := make(map[string]bool, len(artifacts))
	for _, artifact := range artifacts {
		cleanPath, err := safeArtifactPath(artifact.Path)
		if err != nil {
			return err
		}
		if seen[cleanPath] {
			return fmt.Errorf("duplicate generated artifact %q", cleanPath)
		}
		seen[cleanPath] = true
	}
	publisher, err := newArtifactPublisherWithMode(output, incremental)
	if err != nil {
		return err
	}
	defer publisher.Rollback()
	for _, artifact := range artifacts {
		if err := publisher.WriteArtifact(artifact); err != nil {
			return err
		}
	}
	return publisher.Commit()
}

func streamArtifacts(target generator.Target, plan generator.Plan, output string) error {
	return streamArtifactsWithMode(target, plan, output, false)
}

func streamArtifactsIncremental(target generator.Target, plan generator.Plan, output string) error {
	return streamArtifactsWithMode(target, plan, output, true)
}

func streamArtifactsWithMode(target generator.Target, plan generator.Plan, output string, incremental bool) error {
	publisher, err := newArtifactPublisherWithMode(output, incremental)
	if err != nil {
		return &generationStageError{stage: "publish", err: err}
	}
	defer publisher.Rollback()
	if err := generator.EmitTo(target, plan, publisher); err != nil {
		stage := "emit"
		if publisher.failure != nil {
			stage = "publish"
			err = publisher.failure
		}
		return &generationStageError{stage: stage, err: err}
	}
	if err := publisher.Commit(); err != nil {
		return &generationStageError{stage: "publish", err: err}
	}
	return nil
}

type artifactPublisher struct {
	output      string
	staging     string
	seen        map[string]bool
	hashes      map[string]string
	previous    map[string]string
	incremental bool
	lockPath    string
	committed   bool
	failure     error
}

const artifactManifestName = ".openapi-sdkgen-manifest.json"

type artifactManifest struct {
	Version int               `json:"version"`
	Files   map[string]string `json:"files"`
}

func newArtifactPublisher(output string) (*artifactPublisher, error) {
	return newArtifactPublisherWithMode(output, false)
}

func newArtifactPublisherWithMode(output string, incremental bool) (*artifactPublisher, error) {
	if incremental {
		return newIncrementalArtifactPublisher(output)
	}
	if info, err := os.Lstat(output); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("output path %s must not be a symlink", output)
		}
		return nil, fmt.Errorf("output path %s already exists; choose a fresh directory", output)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect output path %s: %w", output, err)
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return nil, fmt.Errorf("create output parent directory: %w", err)
	}
	staging, err := os.MkdirTemp(filepath.Dir(output), ".openapi-sdkgen-output-*")
	if err != nil {
		return nil, fmt.Errorf("create output staging directory: %w", err)
	}
	return &artifactPublisher{output: output, staging: staging, seen: make(map[string]bool), hashes: make(map[string]string)}, nil
}

func (publisher *artifactPublisher) WriteArtifact(artifact generator.Artifact) error {
	cleanPath, err := safeArtifactPath(artifact.Path)
	if err != nil {
		publisher.failure = err
		return err
	}
	if publisher.seen[cleanPath] {
		err := fmt.Errorf("duplicate generated artifact %q", cleanPath)
		publisher.failure = err
		return err
	}
	if cleanPath == artifactManifestName {
		err := fmt.Errorf("generated artifact path %q is reserved", cleanPath)
		publisher.failure = err
		return err
	}
	publisher.seen[cleanPath] = true
	hash := artifactContentHash(artifact.Data)
	publisher.hashes[cleanPath] = hash
	if publisher.incremental && publisher.previous[cleanPath] == hash {
		return nil
	}
	path := filepath.Join(publisher.staging, cleanPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		err = fmt.Errorf("create artifact directory %s: %w", filepath.Dir(path), err)
		publisher.failure = err
		return err
	}
	if err := writeFile(path, artifact.Data); err != nil {
		publisher.failure = err
		return err
	}
	return nil
}

func (publisher *artifactPublisher) Commit() error {
	if publisher.incremental {
		return publisher.commitIncremental()
	}
	if err := writeArtifactManifest(publisher.staging, publisher.hashes); err != nil {
		return err
	}
	if err := os.Rename(publisher.staging, publisher.output); err != nil {
		return fmt.Errorf("publish generated output %s: %w", publisher.output, err)
	}
	publisher.committed = true
	return nil
}

func (publisher *artifactPublisher) Rollback() {
	if publisher != nil && !publisher.committed {
		_ = os.RemoveAll(publisher.staging)
		publisher.releaseLock()
	}
}

func newIncrementalArtifactPublisher(output string) (*artifactPublisher, error) {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return nil, fmt.Errorf("create output parent directory: %w", err)
	}
	lockPath := output + ".openapi-sdkgen.lock"
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("incremental output %s is locked by another generation", output)
		}
		return nil, fmt.Errorf("lock incremental output %s: %w", output, err)
	}
	if closeErr := lock.Close(); closeErr != nil {
		_ = os.Remove(lockPath)
		return nil, fmt.Errorf("close incremental output lock %s: %w", lockPath, closeErr)
	}
	fail := func(err error) (*artifactPublisher, error) {
		_ = os.Remove(lockPath)
		return nil, err
	}

	previous := map[string]string{}
	if info, statErr := os.Lstat(output); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fail(fmt.Errorf("output path %s must not be a symlink", output))
		}
		if !info.IsDir() {
			return fail(fmt.Errorf("incremental output path %s must be a directory", output))
		}
		previous, err = readArtifactManifest(output)
		if err != nil {
			return fail(err)
		}
		if err := validateManifestOwnedFiles(output, previous); err != nil {
			return fail(err)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fail(fmt.Errorf("inspect output path %s: %w", output, statErr))
	}
	staging, err := os.MkdirTemp(filepath.Dir(output), ".openapi-sdkgen-output-*")
	if err != nil {
		return fail(fmt.Errorf("create output staging directory: %w", err))
	}
	return &artifactPublisher{
		output: output, staging: staging, seen: make(map[string]bool), hashes: make(map[string]string),
		previous: previous, incremental: true, lockPath: lockPath,
	}, nil
}

func artifactContentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func writeArtifactManifest(directory string, hashes map[string]string) error {
	data, err := json.MarshalIndent(artifactManifest{Version: 1, Files: hashes}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode generated artifact manifest: %w", err)
	}
	data = append(data, '\n')
	if err := writeFile(filepath.Join(directory, artifactManifestName), data); err != nil {
		return fmt.Errorf("write generated artifact manifest: %w", err)
	}
	return nil
}

func readArtifactManifest(output string) (map[string]string, error) {
	path := filepath.Join(output, artifactManifestName)
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("incremental output %s requires a valid %s: %w", output, artifactManifestName, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("incremental output manifest %s must be a regular file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read incremental output manifest %s: %w", path, err)
	}
	var manifest artifactManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode incremental output manifest %s: %w", path, err)
	}
	if manifest.Version != 1 || manifest.Files == nil {
		return nil, fmt.Errorf("incremental output manifest %s has unsupported or incomplete content", path)
	}
	for path, hash := range manifest.Files {
		clean, err := safeArtifactPath(path)
		if err != nil || clean != path || path == artifactManifestName || len(hash) != sha256.Size*2 {
			return nil, fmt.Errorf("incremental output manifest contains invalid artifact %q", path)
		}
		if _, err := hex.DecodeString(hash); err != nil {
			return nil, fmt.Errorf("incremental output manifest contains invalid hash for %q", path)
		}
	}
	return manifest.Files, nil
}

func validateManifestOwnedFiles(output string, files map[string]string) error {
	for path, expected := range files {
		fullPath := filepath.Join(output, path)
		if err := validateSafeArtifactParents(output, filepath.Dir(fullPath)); err != nil {
			return err
		}
		info, err := os.Lstat(fullPath)
		if err != nil {
			return fmt.Errorf("manifest-owned generated artifact %s is missing or unreadable: %w", fullPath, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("manifest-owned generated artifact %s must be a regular file", fullPath)
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("read manifest-owned generated artifact %s: %w", fullPath, err)
		}
		if actual := artifactContentHash(data); actual != expected {
			return fmt.Errorf("manifest-owned generated artifact %s was edited; refusing incremental overwrite", fullPath)
		}
	}
	return nil
}

func (publisher *artifactPublisher) commitIncremental() error {
	if err := validateManifestOwnedFiles(publisher.output, publisher.previous); err != nil && len(publisher.previous) != 0 {
		return err
	}
	if _, err := os.Stat(publisher.output); errors.Is(err, os.ErrNotExist) {
		if err := writeArtifactManifest(publisher.staging, publisher.hashes); err != nil {
			return err
		}
		if err := os.Rename(publisher.staging, publisher.output); err != nil {
			return fmt.Errorf("publish generated output %s: %w", publisher.output, err)
		}
		publisher.committed = true
		publisher.releaseLock()
		return nil
	}

	changed := make([]string, 0)
	stale := make([]string, 0)
	for path, hash := range publisher.hashes {
		if publisher.previous[path] != hash {
			changed = append(changed, path)
		}
	}
	for path := range publisher.previous {
		if _, exists := publisher.hashes[path]; !exists {
			stale = append(stale, path)
		}
	}
	sort.Strings(changed)
	sort.Strings(stale)
	for _, path := range changed {
		if _, owned := publisher.previous[path]; owned {
			continue
		}
		fullPath := filepath.Join(publisher.output, path)
		if _, err := os.Lstat(fullPath); err == nil {
			return fmt.Errorf("generated artifact %s conflicts with an unowned existing path", fullPath)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect generated artifact path %s: %w", fullPath, err)
		}
	}

	backup, err := os.MkdirTemp(filepath.Dir(publisher.output), ".openapi-sdkgen-backup-*")
	if err != nil {
		return fmt.Errorf("create incremental backup: %w", err)
	}
	defer os.RemoveAll(backup)
	backed := make([]string, 0, len(changed)+len(stale)+1)
	installed := make([]string, 0, len(changed)+1)
	rollback := func(cause error) error {
		for index := len(installed) - 1; index >= 0; index-- {
			_ = os.Remove(filepath.Join(publisher.output, installed[index]))
		}
		for index := len(backed) - 1; index >= 0; index-- {
			path := backed[index]
			_ = os.MkdirAll(filepath.Dir(filepath.Join(publisher.output, path)), 0o755)
			_ = os.Rename(filepath.Join(backup, path), filepath.Join(publisher.output, path))
		}
		return cause
	}
	backupPath := func(path string) error {
		source := filepath.Join(publisher.output, path)
		target := filepath.Join(backup, path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.Rename(source, target); err != nil {
			return err
		}
		backed = append(backed, path)
		return nil
	}
	if err := backupPath(artifactManifestName); err != nil {
		return rollback(fmt.Errorf("backup incremental manifest: %w", err))
	}
	for _, path := range append(append([]string(nil), changed...), stale...) {
		if _, owned := publisher.previous[path]; !owned {
			continue
		}
		if err := backupPath(path); err != nil {
			return rollback(fmt.Errorf("backup generated artifact %s: %w", path, err))
		}
	}
	for _, path := range changed {
		target := filepath.Join(publisher.output, path)
		if err := ensureSafeArtifactParents(publisher.output, filepath.Dir(target)); err != nil {
			return rollback(err)
		}
		if err := os.Rename(filepath.Join(publisher.staging, path), target); err != nil {
			return rollback(fmt.Errorf("replace generated artifact %s: %w", target, err))
		}
		installed = append(installed, path)
	}
	if err := writeArtifactManifest(publisher.output, publisher.hashes); err != nil {
		return rollback(err)
	}
	installed = append(installed, artifactManifestName)
	for _, path := range stale {
		removeEmptyArtifactParents(publisher.output, filepath.Dir(filepath.Join(publisher.output, path)))
	}
	publisher.committed = true
	_ = os.RemoveAll(publisher.staging)
	publisher.releaseLock()
	return nil
}

func ensureSafeArtifactParents(output, directory string) error {
	return safeArtifactParents(output, directory, true)
}

func validateSafeArtifactParents(output, directory string) error {
	return safeArtifactParents(output, directory, false)
}

func safeArtifactParents(output, directory string, create bool) error {
	relative, err := filepath.Rel(output, directory)
	if err != nil {
		return fmt.Errorf("resolve generated artifact directory: %w", err)
	}
	current := output
	for _, segment := range strings.Split(relative, string(filepath.Separator)) {
		if segment == "." || segment == "" {
			continue
		}
		current = filepath.Join(current, segment)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if !create {
				return fmt.Errorf("artifact directory %s is missing", current)
			}
			if err := os.Mkdir(current, 0o755); err != nil {
				return fmt.Errorf("create artifact directory %s: %w", current, err)
			}
			continue
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("artifact directory %s is not a safe directory", current)
		}
	}
	return nil
}

func removeEmptyArtifactParents(output, directory string) {
	for directory != output && strings.HasPrefix(directory, output+string(filepath.Separator)) {
		if err := os.Remove(directory); err != nil {
			return
		}
		directory = filepath.Dir(directory)
	}
}

func (publisher *artifactPublisher) releaseLock() {
	if publisher.lockPath != "" {
		_ = os.Remove(publisher.lockPath)
		publisher.lockPath = ""
	}
}

func safeArtifactPath(value string) (string, error) {
	cleanPath := filepath.Clean(filepath.FromSlash(value))
	if cleanPath == "." || filepath.IsAbs(cleanPath) || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid generated artifact path %q", value)
	}
	return cleanPath, nil
}

func writeFile(path string, data []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".openapi-sdkgen-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary artifact %s: %w", path, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write generated artifact %s: %w", path, err)
	}
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return fmt.Errorf("set generated artifact mode %s: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close generated artifact %s: %w", path, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace generated artifact %s: %w", path, err)
	}
	return nil
}
