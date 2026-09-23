// openapi-sdkgen compiles OpenAPI documents into client SDK packages.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
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

type diagnosticOutputFormat string

const (
	diagnosticOutputHuman diagnosticOutputFormat = "human"
	diagnosticOutputJSON  diagnosticOutputFormat = "json"
)

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
	publish            func(string, []generator.Artifact, *artifactGeneration) error
	publishIncremental func(string, []generator.Artifact, *artifactGeneration) error
	stream             func(generator.Target, generator.Plan, string, *artifactGeneration) error
	streamIncremental  func(generator.Target, generator.Plan, string, *artifactGeneration) error
	check              func(generator.Target, generator.Plan, string, *artifactGeneration) error
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
	input             *string
	inputBase         *string
	targetName        *string
	output            *string
	with              repeatedStrings
	remoteRefs        repeatedStrings
	schemaExtensions  repeatedStrings
	httpHeaderEnv     rawStrings
	refLock           *string
	updateRefLock     *bool
	offline           *bool
	incremental       *bool
	check             *bool
	diagnosticsFormat *string
	help              *bool
	tlsClientCert     *string
	tlsClientKey      *string
	tlsCAFile         *string
}

var defaultGenerationRuntime = generationRuntime{
	compile: compiler.CompileInputResultWithOptions,
	prepare: generator.PrepareCompilation,
	emit: func(target generator.Target, plan generator.Plan) ([]generator.Artifact, error) {
		return target.Emit(plan)
	},
	publish:            writeArtifactsForGeneration,
	publishIncremental: writeArtifactsIncrementalForGeneration,
	stream:             streamArtifactsForGeneration,
	streamIncremental:  streamArtifactsIncrementalForGeneration,
	check:              checkArtifactsForGeneration,
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
	diagnosticsFormat, err := resolveDiagnosticOutputFormat(*values.diagnosticsFormat)
	if err != nil {
		return generateUsageError(err.Error())
	}
	if *values.input == "" || *values.targetName == "" {
		return generateUsageError("--input and --target are required")
	}
	if !*values.check && *values.output == "" {
		return generateUsageError("--output is required unless --check is selected")
	}
	if *values.check && *values.incremental {
		return generateUsageError("--incremental cannot be used with --check")
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
	if !*values.check {
		if err := preflightOutput(*values.output, *values.incremental); err != nil {
			return err
		}
	}
	compileOptions := compiler.CompileOptions{
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
	}
	requestedGeneration := reusableGenerationRequest(*values.input, target.Name(), options, compileOptions)
	if *values.incremental && requestedGeneration != nil {
		noop, err := incrementalGenerationMatches(*values.output, requestedGeneration)
		if err != nil {
			return err
		}
		if noop {
			return nil
		}
	}
	compiled, err := runtime.compile(*values.input, compileOptions)
	if err != nil {
		if renderErr := writeDiagnostics(compiled.Diagnostics, compiled.SkippedPhases, diagnosticsFormat); renderErr != nil {
			return internalFailure("internal diagnostic rendering failure", renderErr)
		}
		return internalFailure("internal compiler failure", err)
	}
	prepared, err := runtime.prepare(target, compiled, options)
	if err != nil {
		if renderErr := writeDiagnostics(prepared.Diagnostics, prepared.SkippedPhases, diagnosticsFormat); renderErr != nil {
			return internalFailure("internal diagnostic rendering failure", renderErr)
		}
		return internalFailure(fmt.Sprintf("internal %s preparation failure", target.Name()), err)
	}
	if err := writeDiagnostics(prepared.Diagnostics, prepared.SkippedPhases, diagnosticsFormat); err != nil {
		return internalFailure("internal diagnostic rendering failure", err)
	}
	if diagnostic.HasErrors(prepared.Diagnostics) {
		return errReportedDiagnostics
	}
	generation := reusableGenerationResult(compiled, target.Name(), options, compileOptions)
	if *values.check {
		if *values.output == "" {
			return nil
		}
		if runtime.check == nil {
			return internalFailure("internal output check failure", errors.New("managed output checker is not configured"))
		}
		if err := runtime.check(target, prepared.Plan, *values.output, generation); err != nil {
			if isOutputFailure(err) {
				return err
			}
			return internalFailure(fmt.Sprintf("internal %s emission failure", target.Name()), err)
		}
		return nil
	}
	stream := runtime.stream
	if *values.incremental {
		stream = runtime.streamIncremental
	}
	if stream != nil {
		if err := stream(target, prepared.Plan, *values.output, generation); err != nil {
			var staged *generationStageError
			if errors.As(err, &staged) && staged.stage == "publish" {
				if isOutputFailure(err) {
					return err
				}
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
	if err := publish(*values.output, artifacts, generation); err != nil {
		if isOutputFailure(err) {
			return err
		}
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
	values.output = flags.String(generationGroup, helpOption{
		Name: "output", Metavariable: "directory", Summary: "Generated-code directory; with --check, verify this managed output",
	}, "")
	values.check = flags.Bool(generationGroup, helpOption{
		Name: "check", Summary: "Validate generation without writing output",
	}, false)
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
		Summary:    "Read an HTTPS input request header from an environment variable",
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
	values.diagnosticsFormat = flags.String(optionsGroup, helpOption{
		Name: "diagnostics-format", Metavariable: "format", Summary: "Diagnostic report format",
		Available: func() []string { return []string{string(diagnosticOutputHuman), string(diagnosticOutputJSON)} },
	}, string(diagnosticOutputHuman))
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

func reusableGeneratorIdentity() string {
	if value := normalizeVersion(version); value != "" {
		return "release:" + value
	}
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if value := versionFromBuildInfo(build); value != "" {
		return "module:" + value
	}
	settings := make(map[string]string, len(build.Settings))
	for _, setting := range build.Settings {
		settings[setting.Key] = setting.Value
	}
	if settings["vcs.revision"] != "" && settings["vcs.modified"] == "false" {
		return "vcs:" + settings["vcs.revision"]
	}
	return ""
}

func reusableGenerationRequest(input, target string, options generator.Options, compileOptions compiler.CompileOptions) *artifactGeneration {
	identity := reusableGeneratorIdentity()
	path, ok := localGenerationInputPath(input)
	if identity == "" || !ok || !reusableCompileOptions(compileOptions) {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	digest := sha256.Sum256(data)
	return newArtifactGeneration(identity, target, options, hex.EncodeToString(digest[:]))
}

func reusableGenerationResult(result compiler.Result, target string, options generator.Options, compileOptions compiler.CompileOptions) *artifactGeneration {
	identity := reusableGeneratorIdentity()
	if identity == "" || result.ReusableInput == nil || !reusableCompileOptions(compileOptions) {
		return nil
	}
	return newArtifactGeneration(identity, target, options, result.ReusableInput.SHA256)
}

func reusableCompileOptions(options compiler.CompileOptions) bool {
	return options.InputBase == "" && len(options.RemoteRefAllowlist) == 0 && options.RefLockPath == "" && !options.UpdateRefLock && !options.Offline &&
		len(options.SchemaExtensionManifests) == 0 && len(options.HTTPHeaderEnv) == 0 && options.TLSClientCert == "" && options.TLSClientKey == "" && options.TLSCAFile == ""
}

func newArtifactGeneration(identity, target string, options generator.Options, inputDigest string) *artifactGeneration {
	addons := options.Addons()
	addonNames := make([]string, len(addons))
	for index, addon := range addons {
		addonNames[index] = string(addon)
	}
	return &artifactGeneration{Generator: identity, Target: target, Addons: addonNames, InputSHA256: inputDigest}
}

func localGenerationInputPath(input string) (string, bool) {
	if input == "" || input == "-" {
		return "", false
	}
	if strings.Contains(input, "://") || (len(input) >= len("file:") && strings.EqualFold(input[:len("file:")], "file:")) {
		parsed, err := url.Parse(input)
		if err != nil || !strings.EqualFold(parsed.Scheme, "file") || (parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost")) || parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", false
		}
		input = filepath.FromSlash(parsed.Path)
	}
	path, err := filepath.Abs(input)
	return path, err == nil
}

func resolveDiagnosticOutputFormat(value string) (diagnosticOutputFormat, error) {
	format := diagnosticOutputFormat(value)
	switch format {
	case diagnosticOutputHuman, diagnosticOutputJSON:
		return format, nil
	default:
		return "", fmt.Errorf("unsupported --diagnostics-format %q (available: human, json)", value)
	}
}

func writeDiagnostics(values []diagnostic.Diagnostic, skipped []diagnostic.SkippedPhase, format diagnosticOutputFormat) error {
	if len(values) == 0 && len(skipped) == 0 {
		return nil
	}
	var report string
	switch format {
	case diagnosticOutputHuman:
		report = diagnostic.RenderHuman(values, skipped)
	case diagnosticOutputJSON:
		var err error
		report, err = diagnostic.RenderJSON(values, skipped)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported diagnostic output format %q", format)
	}
	if _, err := fmt.Fprint(standardError, report); err != nil {
		return fmt.Errorf("write diagnostic report: %w", err)
	}
	return nil
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
