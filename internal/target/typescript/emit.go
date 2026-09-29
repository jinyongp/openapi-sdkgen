package typescript

import (
	"bytes"
	"embed"
	"fmt"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/compiler/naming"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

//go:embed runtime/internal/*.ts
var runtimeTemplates embed.FS

//go:embed runtime/server/runtime.ts
var serverRuntimeTemplate []byte

type runtimeTemplateArtifact struct {
	source string
	path   string
}

var runtimeTemplateArtifacts = []runtimeTemplateArtifact{
	{source: "operation-loader.ts", path: "internal/runtime/operation-loader.ts"},
	{source: "selection.ts", path: "internal/runtime/selection.ts"},
	{source: "selection-types.ts", path: "internal/runtime/selection-types.ts"},
	{source: "runtime-support.ts", path: "internal/runtime/runtime-support.ts"},
	{source: "http-stream.ts", path: "internal/runtime/http-stream.ts"},
	{source: "http-json-stream.ts", path: "internal/runtime/http-json-stream.ts"},
	{source: "http-buffered.ts", path: "internal/runtime/http-buffered.ts"},
	{source: "http-advanced.ts", path: "internal/runtime/http-advanced.ts"},
	{source: "http-json.ts", path: "internal/runtime/http-json.ts"},
	{source: "http-types.ts", path: "internal/runtime/http-types.ts"},
	{source: "http-codecs.ts", path: "internal/runtime/http-codecs.ts"},
	{source: "http-core.ts", path: "internal/runtime/http-core.ts"},
	{source: "media-type.ts", path: "internal/runtime/media-type.ts"},
	{source: "wire-xml.ts", path: "internal/runtime/wire-xml.ts"},
	{source: "wire-engine.ts", path: "internal/runtime/wire-engine.ts"},
	{source: "objects.ts", path: "internal/runtime/objects.ts"},
	{source: "identity.ts", path: "internal/runtime/identity.ts"},
	{source: "request.ts", path: "internal/runtime/request.ts"},
	{source: "errors.ts", path: "internal/runtime/errors.ts"},
	{source: "transport.ts", path: "internal/runtime/transport.ts"},
	{source: "security.ts", path: "internal/runtime/security.ts"},
	{source: "operation.ts", path: "internal/runtime/operation.ts"},
	{source: "configuration.ts", path: "internal/runtime/configuration.ts"},
	{source: "links.ts", path: "internal/runtime/links.ts"},
	{source: "callables.ts", path: "internal/runtime/callables.ts"},
	{source: "pagination.ts", path: "internal/runtime/pagination.ts"},
	{source: "codecs.ts", path: "internal/runtime/codecs.ts"},
	{source: "wire-properties.ts", path: "internal/runtime/wire-properties.ts"},
	{source: "streaming.ts", path: "internal/runtime/streaming.ts"},
	{source: "http.ts", path: "internal/runtime/http.ts"},
	{source: "constants.ts", path: "internal/runtime/constants.ts"},
}

func readRuntimeTemplate(name string) ([]byte, error) {
	source, err := runtimeTemplates.ReadFile("runtime/internal/" + name)
	if err != nil {
		return nil, fmt.Errorf("read TypeScript runtime template %q: %w", name, err)
	}
	return source, nil
}

func emitRuntimeTemplateArtifacts() ([]Artifact, error) {
	artifacts := make([]Artifact, 0, len(runtimeTemplateArtifacts))
	for _, template := range runtimeTemplateArtifacts {
		source, err := readRuntimeTemplate(template.source)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, Artifact{Path: template.path, Data: generatedSource(source)})
	}
	return artifacts, nil
}

// Artifact is a generated TypeScript source file.
type Artifact = generator.Artifact

// Generator emits TypeScript SDK source files from compiler IR.
type Generator struct{}

// Name returns the CLI target name.
func (Generator) Name() string { return "typescript" }

// SupportsAddon reports artifact sets available for TypeScript source output.
func (Generator) SupportsAddon(addon generator.Addon) bool {
	return addon == generator.AddonServer
}

type sourcePlan struct {
	document                    *ir.Document
	ownership                   *sourceOwnershipIndex
	includeServer               bool
	omittedOperations           map[string]bool
	resourceReservationExcluded map[string]bool
	reservationManifest         *Manifest
	manifest                    *Manifest
	modules                     *semanticModulePlan
	executions                  map[string]operationExecutionPlan
	links                       []generatedLink
	streams                     []generatedStream
	webhooks                    []webhookDefinition
	callbacks                   []callbackDefinition
	resourceTree                *resourceNode
	resourceReachable           map[string]bool
}

// Prepare validates author input for the TypeScript target using the
// historical fail-fast contract. Policy-aware orchestration uses
// PrepareWithCoverage explicitly.
func (Generator) Prepare(document *ir.Document, options generator.Options) (generator.Plan, []diagnostic.Diagnostic, error) {
	options.DiagnosticMode = diagnostic.ModeFailFast
	plan, diagnostics, _, err := (Generator{}).PrepareWithCoverage(document, options)
	return plan, diagnostics, err
}

// PrepareWithCoverage validates target-owned constraints and reports which
// independent analyzers were or were not able to run before plan construction.
func (Generator) PrepareWithCoverage(document *ir.Document, options generator.Options) (generator.Plan, []diagnostic.Diagnostic, []diagnostic.AnalysisCoverage, error) {
	if document == nil {
		return generator.Plan{}, nil, nil, fmt.Errorf("internal TypeScript target: IR document is nil")
	}
	mode, err := options.DiagnosticMode.Resolve()
	if err != nil {
		return generator.Plan{}, nil, nil, fmt.Errorf("internal TypeScript target: diagnostic mode: %w", err)
	}
	plan, diagnostics, coverage, err := prepareSourcePlanWithCoverage(document, options.HasAddon(generator.AddonServer), mode)
	if err != nil {
		return generator.Plan{}, diagnostics, coverage, err
	}
	if !diagnostic.HasErrors(diagnostics) {
		if !hasMeaningfulEntrySurface(plan) {
			diagnostics = append(diagnostics, noMeaningfulEntrySurfaceDiagnostic(plan.document, plan.ownership))
			plan.modules = nil
		}
		coverage = append(coverage, targetAnalysisCoverage("target.entry-surface", diagnostic.CoverageComplete, ""))
	} else {
		coverage = append(coverage, targetAnalysisCoverage("target.entry-surface", diagnostic.CoverageSkipped, "blocking target diagnostics prevented meaningful-entry validation"))
	}
	// Ownership is a preparation-only index. Emission consumes the frozen
	// decisions and must not retain or recompute ownership.
	plan.ownership = nil
	diagnostics = diagnostic.Sort(diagnostics)
	if mode == diagnostic.ModeCollect && diagnostic.HasErrors(diagnostics) {
		return generator.Plan{}, diagnostics, coverage, nil
	}
	return generator.NewPlan("typescript", plan), diagnostics, coverage, nil
}

// Emit emits a previously validated TypeScript plan.
func (Generator) Emit(plan generator.Plan) ([]generator.Artifact, error) {
	artifacts := make([]generator.Artifact, 0)
	err := (Generator{}).EmitTo(plan, generator.ArtifactSinkFunc(func(artifact generator.Artifact) error {
		artifacts = append(artifacts, artifact)
		return nil
	}))
	if err != nil {
		return nil, err
	}
	sort.Slice(artifacts, func(i, j int) bool {
		return artifactEmissionOrder(artifacts[i].Path) < artifactEmissionOrder(artifacts[j].Path)
	})
	return artifacts, nil
}

// EmitTo writes each validated source file to the supplied sink as soon as its
// bytes are ready.
func (Generator) EmitTo(plan generator.Plan, sink generator.ArtifactSink) error {
	value, err := plan.Value("typescript")
	if err != nil {
		return fmt.Errorf("internal TypeScript target: %w", err)
	}
	prepared, ok := value.(*sourcePlan)
	if !ok {
		return fmt.Errorf("internal TypeScript target: unexpected plan type %T", value)
	}
	if sink == nil {
		return fmt.Errorf("internal TypeScript target: artifact sink is nil")
	}
	return emitSourcePlanTo(prepared, sink.WriteArtifact)
}

// Generate is the compatibility convenience for direct target callers.
func (target Generator) Generate(document *ir.Document, options generator.Options) ([]generator.Artifact, error) {
	plan, diagnostics, err := target.Prepare(document, options)
	if err != nil {
		return nil, err
	}
	if diagnostic.HasErrors(diagnostics) {
		return nil, fmt.Errorf("%s", strings.TrimSpace(diagnostic.RenderHuman(diagnostics, nil)))
	}
	return target.Emit(plan)
}

type Manifest struct {
	Operations []ManifestOperation `json:"operations"`
}

type ManifestOperation struct {
	RouteKey           string                    `json:"routeKey"`
	OperationID        string                    `json:"operationID"`
	Summary            string                    `json:"summary,omitempty"`
	Description        string                    `json:"description,omitempty"`
	Method             string                    `json:"method"`
	Path               string                    `json:"path"`
	CallExpression     string                    `json:"callExpression"`
	ResourceSegments   []string                  `json:"resourceSegments"`
	PathParameterOrder []string                  `json:"pathParameterOrder"`
	InputSections      operationInputSectionList `json:"inputSections"`
	OutputType         string                    `json:"outputType"`
	ErrorType          string                    `json:"errorType"`
	Envelope           string                    `json:"envelope,omitempty"`
	Pagination         string                    `json:"pagination,omitempty"`
	Auth               string                    `json:"auth"`
	Visibility         string                    `json:"visibility"`
	Deprecated         bool                      `json:"deprecated"`
	outputExpression   typeExpression
	errorExpression    typeExpression
	rawResponse        typeExpression
	mediaOutputs       map[string]typeExpression
	renderedMedia      map[string]string
	mediaTypes         []string
	streamMediaTypes   []string
	security           []operationSecurityRequirement
	hasSecurity        bool
	optionsRequired    bool
	paginationRequest  ir.PaginationRequestPlan
	compiled           ir.Operation
	prepared           preparedOperation
}

const generatedFileHeader = `// cspell:disable
/** @noprettier -- generated by openapi-sdkgen */
// <auto-generated />
// @generated by openapi-sdkgen
// Code generated by openapi-sdkgen. DO NOT EDIT.
// This file is generated. Do not edit, lint, or format it manually.
// @ts-nocheck
/* eslint-disable */
/* oxlint-disable */
/* biome-ignore-all lint: generated file */
/* deno-lint-ignore-file */
// dprint-ignore-file

`

// SourceArtifacts emits TypeScript source that a consumer compiles as part of
// its own application. It deliberately contains no package metadata or build
// configuration.
func SourceArtifacts(document *ir.Document) ([]Artifact, error) {
	return sourceArtifacts(document, false)
}

func sourceArtifacts(document *ir.Document, includeServer bool) ([]Artifact, error) {
	plan, diagnostics, err := prepareSourcePlan(document, includeServer)
	if err != nil {
		return nil, err
	}
	if diagnostic.HasErrors(diagnostics) {
		return nil, fmt.Errorf("%s", strings.TrimSpace(diagnostic.RenderHuman(diagnostics, nil)))
	}
	return emitSourcePlan(plan)
}

func prepareSourcePlan(document *ir.Document, includeServer bool) (*sourcePlan, []diagnostic.Diagnostic, error) {
	plan, diagnostics, _, err := prepareSourcePlanWithCoverage(document, includeServer, diagnostic.ModeFailFast)
	return plan, diagnostics, err
}

func prepareSourcePlanWithCoverage(document *ir.Document, includeServer bool, mode diagnostic.Mode) (*sourcePlan, []diagnostic.Diagnostic, []diagnostic.AnalysisCoverage, error) {
	if document == nil {
		return nil, nil, nil, fmt.Errorf("IR document is nil")
	}
	if _, err := mode.Resolve(); err != nil {
		return nil, nil, nil, err
	}
	var coverage []diagnostic.AnalysisCoverage
	prepared, diagnostics, err := prepareKnownExtensions(document)
	if err != nil {
		return nil, nil, coverage, err
	}
	coverage = append(coverage, targetAnalysisCoverage("target.extensions", diagnostic.CoverageComplete, ""))
	plan := &sourcePlan{document: prepared, ownership: newSourceOwnershipIndex(prepared), includeServer: includeServer}
	targetDiagnostics := prepareTargetDiagnostics(plan)
	diagnostics = append(diagnostics, targetDiagnostics...)
	coverage = append(coverage, targetAnalysisCoverage("target.support", diagnostic.CoverageComplete, ""))
	diagnostics = append(diagnostics, validateVisibilityDependencies(prepared, plan.omittedOperations)...)
	coverage = append(coverage, targetAnalysisCoverage("target.visibility", diagnostic.CoverageComplete, ""))
	manifest, manifestErrors := buildManifestDiagnostics(prepared, plan.resourceReservationExcluded)
	for _, manifestErr := range manifestErrors {
		diagnostics = append(diagnostics, loweringPreparationDiagnostic(prepared, plan.ownership, manifestErr))
	}
	coverage = append(coverage, targetAnalysisCoverage("target.lowering", diagnostic.CoverageComplete, ""))
	helperManifest := filterManifestOperations(manifest, plan.omittedOperations)
	if len(manifestErrors) != 0 {
		helperManifest = filterManifestOperations(visibilityManifest(prepared), plan.omittedOperations)
	} else {
		reservationManifest := manifest
		emissionManifest := filterManifestOperations(manifest, plan.omittedOperations)
		plan.reservationManifest = &reservationManifest
		plan.manifest = &emissionManifest
	}
	links, linkFailures := generatedLinksDiagnostics(prepared, helperManifest, plan.ownership)
	blockingLinkFailure := false
	for _, linkFailure := range linkFailures {
		value := linkPreparationDiagnostic(prepared, plan.ownership, linkFailure)
		diagnostics = append(diagnostics, value)
		if value.Severity == diagnostic.SeverityError {
			blockingLinkFailure = true
		}
	}
	plan.links = links
	coverage = append(coverage, targetAnalysisCoverage("target.links", diagnostic.CoverageComplete, ""))
	streams, streamErrors := generatedStreamsDiagnostics(prepared, helperManifest)
	for _, streamsErr := range streamErrors {
		diagnostics = append(diagnostics, helperPreparationDiagnostic(prepared, plan.ownership, "response streams", "SDKGEN-E510", streamsErr))
	}
	if len(streamErrors) == 0 {
		plan.streams = streams
	}
	coverage = append(coverage, targetAnalysisCoverage("target.streams", diagnostic.CoverageComplete, ""))
	if len(manifestErrors) == 0 && !blockingLinkFailure && len(streamErrors) == 0 {
		if plan.manifest != nil && plan.reservationManifest != nil {
			resourceReservations := filterManifestOperations(*plan.reservationManifest, plan.resourceReservationExcluded)
			tree, reachable, reconcileErr := reconcileResourceCapabilities(prepared, resourceReservations, plan.manifest, links, streams)
			if reconcileErr != nil {
				diagnostics = append(diagnostics, loweringPreparationDiagnostic(prepared, plan.ownership, reconcileErr))
			} else {
				plan.resourceTree = tree
				plan.resourceReachable = reachable
			}
			coverage = append(coverage, targetAnalysisCoverage("target.resources", diagnostic.CoverageComplete, ""))
		}
	} else {
		coverage = append(coverage, targetAnalysisCoverage(
			"target.resources",
			diagnostic.CoverageSkipped,
			"lowering, Link, or stream prerequisites were unavailable",
		))
	}
	if includeServer {
		webhooks, webhookErrors := collectWebhooksDiagnostics(prepared)
		for _, webhookErr := range webhookErrors {
			diagnostics = append(diagnostics, serverPreparationDiagnostic(prepared, plan.ownership, "webhook contracts", webhookErr))
		}
		if len(webhookErrors) == 0 {
			plan.webhooks = webhooks
		}
		coverage = append(coverage, targetAnalysisCoverage("target.webhooks", diagnostic.CoverageComplete, ""))
		callbacks, callbackErrors := collectCallbacksDiagnostics(prepared, plan.omittedOperations)
		for _, callbackErr := range callbackErrors {
			diagnostics = append(diagnostics, serverPreparationDiagnostic(prepared, plan.ownership, "callback contracts", callbackErr))
		}
		if len(callbackErrors) == 0 {
			plan.callbacks = callbacks
		}
		coverage = append(coverage, targetAnalysisCoverage("target.callbacks", diagnostic.CoverageComplete, ""))
	}
	if plan.manifest != nil && plan.reservationManifest != nil && !diagnostic.HasErrors(diagnostics) {
		modules, moduleErr := buildSemanticModulePlan(prepared, *plan.reservationManifest, *plan.manifest, plan.resourceTree, includeServer)
		if moduleErr != nil {
			return nil, diagnostic.Sort(diagnostics), coverage, fmt.Errorf("build TypeScript semantic module plan: %w", moduleErr)
		}
		executions, executionErr := prepareOperationExecutions(prepared, *plan.manifest, modules, plan.streams)
		if executionErr != nil {
			return nil, diagnostic.Sort(diagnostics), coverage, fmt.Errorf("build TypeScript execution plan: %w", executionErr)
		}
		plan.modules = modules
		plan.executions = executions
		coverage = append(coverage, targetAnalysisCoverage("target.modules", diagnostic.CoverageComplete, ""))
	} else {
		coverage = append(coverage, targetAnalysisCoverage(
			"target.modules",
			diagnostic.CoverageSkipped,
			"blocking target diagnostics or lowering prerequisites prevented immutable module-plan construction",
		))
	}
	return plan, diagnostic.Sort(diagnostics), coverage, nil
}

func targetAnalysisCoverage(analyzer string, status diagnostic.CoverageStatus, reason string) diagnostic.AnalysisCoverage {
	coverage := diagnostic.AnalysisCoverage{
		Phase:    diagnostic.PhaseTarget,
		Analyzer: analyzer,
		Status:   status,
		Target:   "typescript",
	}
	if status == diagnostic.CoverageSkipped {
		coverage.Prerequisites = []diagnostic.CoveragePrerequisite{{
			Name:      "target-analysis-prerequisites",
			Available: false,
			Reason:    reason,
		}}
		coverage.Reason = reason
	}
	return coverage
}

func hasMeaningfulEntrySurface(plan *sourcePlan) bool {
	if plan == nil || plan.manifest == nil {
		return false
	}
	for _, operation := range plan.manifest.Operations {
		if operation.Visibility != "hidden" {
			return true
		}
	}
	if !plan.includeServer {
		return false
	}
	if len(plan.webhooks) != 0 {
		return true
	}
	for _, callback := range plan.callbacks {
		if callback.componentName != "" {
			return true
		}
	}
	return false
}

func filterManifestOperations(manifest Manifest, omitted map[string]bool) Manifest {
	result := manifest
	result.Operations = make([]ManifestOperation, 0, len(manifest.Operations))
	for _, operation := range manifest.Operations {
		if omitted[manifestRouteKey(operation)] {
			continue
		}
		result.Operations = append(result.Operations, operation)
	}
	return result
}

func reconcileResourceCapabilities(document *ir.Document, reservations Manifest, manifest *Manifest, links []generatedLink, streams []generatedStream) (*resourceNode, map[string]bool, error) {
	tree, err := buildResourceTree(document, reservations, resourceCapabilityMembers(links, streams))
	if err != nil {
		return nil, nil, err
	}
	eligible := make(map[string]bool, len(manifest.Operations))
	for _, operation := range manifest.Operations {
		eligible[manifestRouteKey(operation)] = true
	}
	retainEligibleResourceOperations(tree, eligible)
	pruneEmptyResourceNodes(tree)
	reachable := make(map[string]bool)
	resourceOperationIDs(tree, reachable)
	for index := range manifest.Operations {
		item := &manifest.Operations[index]
		if item.Visibility != "public" || reachable[item.RouteKey] {
			continue
		}
		operation := item.compiled
		item.CallExpression = exactOperationCall(document, operation, item.InputSections)
		item.ResourceSegments = nil
	}
	return tree, reachable, nil
}

func retainEligibleResourceOperations(node *resourceNode, eligible map[string]bool) {
	for name, operation := range node.operations {
		if !eligible[manifestRouteKey(operation)] {
			delete(node.operations, name)
		}
	}
	if node.pagination != nil && !eligible[manifestRouteKey(*node.pagination)] {
		node.pagination = nil
	}
	if node.parameterChild != nil {
		retainEligibleResourceOperations(node.parameterChild, eligible)
	}
	for _, child := range node.children {
		retainEligibleResourceOperations(child, eligible)
	}
}

func emitSourcePlan(plan *sourcePlan) ([]Artifact, error) {
	artifacts := make([]Artifact, 0)
	if err := emitSourcePlanTo(plan, func(artifact Artifact) error {
		artifacts = append(artifacts, artifact)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Slice(artifacts, func(i, j int) bool {
		return artifactEmissionOrder(artifacts[i].Path) < artifactEmissionOrder(artifacts[j].Path)
	})
	return artifacts, nil
}

func emitSourcePlanTo(plan *sourcePlan, sink func(Artifact) error) error {
	document := plan.document
	includeServer := plan.includeServer
	if plan.manifest == nil {
		return fmt.Errorf("internal TypeScript target: prepared plan has no manifest")
	}
	manifest := *plan.manifest
	if plan.modules == nil {
		return fmt.Errorf("internal TypeScript target: prepared plan has no semantic modules")
	}
	// Execution dependencies must be complete before the first artifact reaches
	// the sink. Emission consumes the prepared plan rather than analyzing inputs.
	for _, module := range plan.modules.operations {
		if _, exists := plan.executions[module.routeKey]; !exists {
			return fmt.Errorf("internal TypeScript target: missing prepared execution for %q", module.routeKey)
		}
	}
	write := validatedArtifactWriter(sink)
	typesSource, err := emitSchemaArtifactsTo(document, plan.modules, write)
	if err != nil {
		return err
	}
	if err := emitOperationArtifactsTo(document, manifest, plan.modules, plan.executions, plan.resourceTree, plan.resourceReachable, plan.links, plan.streams, write); err != nil {
		return err
	}
	if err := emitRouteArtifactsTo(manifest, plan.modules, write); err != nil {
		return err
	}
	registrySource, err := emitClientRegistry(document, manifest, plan.modules, plan.links, plan.streams)
	if err != nil {
		return err
	}
	if err := write(Artifact{Path: plan.modules.fixed["client-registry"], Data: generatedSource(registrySource)}); err != nil {
		return err
	}
	constantsSource, err := readRuntimeTemplate("constants.ts")
	if err != nil {
		return err
	}
	enumsSource, err := emitEnums(document)
	if err != nil {
		return err
	}
	errorsSource, err := emitErrors(document)
	if err != nil {
		return err
	}
	metadataSource, err := emitMetadata(document, true)
	if err != nil {
		return err
	}
	if err := emitResourceArtifactsTo(document, plan.modules, plan.resourceTree, write); err != nil {
		return err
	}
	clientIndexSource, err := emitClientArtifactsTo(document, manifest, plan.modules, plan.links, plan.streams, write)
	if err != nil {
		return err
	}
	if err := validateSourceExportSymbols(map[string][]byte{
		"client":    clientIndexSource,
		"constants": constantsSource,
		"enums":     enumsSource,
		"errors":    errorsSource,
		"metadata":  metadataSource,
		"types":     typesSource,
	}); err != nil {
		return err
	}
	indexSource := generatedIndexSource(enumsSource)
	runtimeArtifacts, err := emitRuntimeTemplateArtifacts()
	if err != nil {
		return err
	}
	for _, artifact := range []Artifact{
		{Path: "index.ts", Data: generatedSource([]byte("export * from \"./internal/index.js\"\n"))},
		{Path: "internal/enums.ts", Data: generatedSource(enumsSource)},
		{Path: "internal/errors.ts", Data: generatedSource(errorsSource)},
		{Path: "internal/index.ts", Data: generatedSource(indexSource)},
		{Path: "enums.ts", Data: generatedSource([]byte("export * from \"./internal/enums.js\"\n"))},
		{Path: "metadata.ts", Data: generatedSource(metadataSource)},
	} {
		if err := write(artifact); err != nil {
			return err
		}
	}
	for _, artifact := range runtimeArtifacts {
		if err := write(artifact); err != nil {
			return err
		}
	}
	if includeServer {
		serverArtifacts, err := emitPreparedServerArtifacts(document, plan.webhooks, plan.callbacks)
		if err != nil {
			return err
		}
		for _, artifact := range serverArtifacts {
			if err := write(artifact); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatedArtifactWriter(sink func(Artifact) error) func(Artifact) error {
	seen := make(map[string]string)
	return func(artifact Artifact) error {
		if err := validateArtifactPath(artifact.Path); err != nil {
			return fmt.Errorf("generated artifact %q: %w", artifact.Path, err)
		}
		key := portableArtifactPathKey(artifact.Path)
		if previous, exists := seen[key]; exists {
			return fmt.Errorf("generated artifact %q collides with %q", artifact.Path, previous)
		}
		seen[key] = artifact.Path
		if !bytes.HasPrefix(artifact.Data, []byte(generatedFileHeader)) {
			return fmt.Errorf("generated artifact %q is missing the standard header", artifact.Path)
		}
		return sink(artifact)
	}
}

func artifactEmissionOrder(path string) string {
	if strings.HasPrefix(path, "internal/") {
		return "0/" + path
	}
	switch path {
	case "index.ts":
		return "1/" + path
	case "enums.ts":
		return "2/" + path
	case "metadata.ts":
		return "3/" + path
	default:
		return "4/" + path
	}
}

func generatedIndexSource(enumsSource []byte) []byte {
	var output strings.Builder
	output.WriteString("export type * from \"./schemas/index.js\"\n")
	output.WriteString("export type { BothPaginationInput, CursorPaginationInput, OffsetPaginationInput } from \"./runtime/pagination.js\"\n")
	for _, module := range []string{"enums", "errors"} {
		fmt.Fprintf(&output, "export type * from %s\n", quoteTS("./"+module+".js"))
	}
	output.WriteString("export type * from \"./routes/index.js\"\n")
	output.WriteString("export type * from \"./routes/helpers.js\"\n")
	output.WriteString("export type * from \"./client/index.js\"\n")
	output.Write(publicRuntimeExportSource())
	output.WriteString("export { SortDirection } from \"./runtime/constants.js\"\n")
	if exportedSymbols(string(enumsSource))["isEnumValue"] {
		output.WriteString("export { Enums, isEnumValue } from \"./enums.js\"\n")
	} else {
		output.WriteString("export { Enums } from \"./enums.js\"\n")
	}
	output.WriteString("export { isErrorCategory } from \"./errors.js\"\n")
	output.WriteString("export { createClient } from \"./client/index.js\"\n")
	output.WriteString("export { APIError, TransportErrorCode, getErrorCode, getRequestID, isAPIError, isErrorCode } from \"./runtime/errors.js\"\n")
	return []byte(output.String())
}

func generatedSource(source []byte) []byte {
	result := make([]byte, len(generatedFileHeader)+len(source))
	copy(result, generatedFileHeader)
	copy(result[len(generatedFileHeader):], source)
	return result
}

func validateSourceExportSymbols(modules map[string][]byte) error {
	owners := make(map[string]string)
	moduleNames := make([]string, 0, len(modules))
	for module := range modules {
		moduleNames = append(moduleNames, module)
	}
	sort.Strings(moduleNames)
	for _, module := range moduleNames {
		exports := exportedSymbols(string(modules[module]))
		for symbol := range exports {
			if previous, exists := owners[symbol]; exists && previous != module {
				return fmt.Errorf("generated source export %q is declared by both %s and %s modules", symbol, previous, module)
			}
			owners[symbol] = module
		}
	}
	return nil
}

func exportedSymbols(source string) map[string]bool {
	result := make(map[string]bool)
	inExportBlock := false
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if inExportBlock {
			if trimmed == "}" {
				inExportBlock = false
				continue
			}
			name := strings.TrimSuffix(trimmed, ",")
			if before, after, found := strings.Cut(name, " as "); found {
				_ = before
				name = after
			}
			if name != "" {
				result[name] = true
			}
			continue
		}
		if !strings.HasPrefix(trimmed, "export ") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
		if rest == "{" || rest == "type {" {
			inExportBlock = true
			continue
		}
		if strings.HasPrefix(rest, "{") || strings.HasPrefix(rest, "type {") {
			open := strings.Index(rest, "{")
			close := strings.Index(rest, "}")
			if open >= 0 && close > open {
				for _, name := range strings.Split(rest[open+1:close], ",") {
					name = strings.TrimSpace(name)
					if before, after, found := strings.Cut(name, " as "); found {
						_ = before
						name = after
					}
					if name != "" {
						result[name] = true
					}
				}
			}
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "type", "interface", "const", "function", "class":
			result[strings.Split(fields[1], "<")[0]] = true
		}
	}
	return result
}

func buildManifest(document *ir.Document) (Manifest, error) {
	manifest, failures := buildManifestDiagnostics(document, nil)
	if len(failures) != 0 {
		return Manifest{}, failures[0]
	}
	return manifest, nil
}

func buildManifestDiagnostics(document *ir.Document, resourceReservationExcluded map[string]bool) (Manifest, []error) {
	manifest := Manifest{
		Operations: make([]ManifestOperation, 0, len(document.Operations)),
	}
	_, errorsBySchema, failures := errorContractsDiagnostics(document)
	for _, operation := range document.Operations {
		operationFailed := false
		prepared, prepareErr := prepareOperation(document, operation)
		if prepareErr != nil {
			failures = append(failures, fmt.Errorf("operation %s parameters: %w", operationLabel(operation), prepareErr))
			operationFailed = true
		}
		outputExpression, err := operationOutputTypeExpression(document, operation)
		if err != nil {
			failures = append(failures, fmt.Errorf("operation %s output: %w", operationLabel(operation), err))
			operationFailed = true
		}
		inputSections, inputErr := operationInputSectionsFromPrepared(document, operation, prepared)
		if inputErr != nil {
			failures = append(failures, fmt.Errorf("operation %s input: %w", operationLabel(operation), inputErr))
			operationFailed = true
		}
		errorExpression, err := operationErrorTypeExpression(document, operation, errorsBySchema)
		if err != nil {
			failures = append(failures, fmt.Errorf("operation %s error: %w", operationLabel(operation), err))
			operationFailed = true
		}
		rawResponse, err := operationRawResponseTypeExpression(document, operation)
		if err != nil {
			failures = append(failures, fmt.Errorf("operation %s raw response: %w", operationLabel(operation), err))
			operationFailed = true
		}
		mediaOutputs, err := operationMediaOutputTypeExpressions(document, operation)
		if err != nil {
			failures = append(failures, fmt.Errorf("operation %s media outputs: %w", operationLabel(operation), err))
			operationFailed = true
		}
		renderedMedia := renderMediaOutputTypes(mediaOutputs, typeRenderContract)
		mediaTypes := make([]string, 0, len(renderedMedia))
		for mediaType := range renderedMedia {
			mediaTypes = append(mediaTypes, mediaType)
		}
		sort.Strings(mediaTypes)
		streamMediaTypes, err := operationStreamingResponseMediaTypes(document, operation)
		if err != nil {
			failures = append(failures, fmt.Errorf("operation %s stream media: %w", operationLabel(operation), err))
			operationFailed = true
		}
		security, hasSecurity, err := operationSecurityRequirements(document, operation)
		if err != nil {
			failures = append(failures, fmt.Errorf("operation %s security: %w", operationLabel(operation), err))
			operationFailed = true
		}
		callExpression := ""
		var segments []string
		if inputErr == nil {
			var callErr error
			callExpression, segments, callErr = operationCallFromPrepared(document, operation, inputSections, prepared)
			if callErr != nil {
				failures = append(failures, fmt.Errorf("operation %s call expression: %w", operationLabel(operation), callErr))
				operationFailed = true
			}
		}
		if operationFailed {
			continue
		}
		prepared.inputRequired, err = prepared.clientInputRequired(document, operation, inputSections, false)
		if err != nil {
			failures = append(failures, fmt.Errorf("operation %s input requirement: %w", operationLabel(operation), err))
			continue
		}
		prepared.resourceInputRequired, err = prepared.clientInputRequired(document, operation, inputSections, true)
		if err != nil {
			failures = append(failures, fmt.Errorf("operation %s resource input requirement: %w", operationLabel(operation), err))
			continue
		}
		visibility := operation.Visibility
		if visibility == "" {
			visibility = "public"
		}
		manifest.Operations = append(manifest.Operations, ManifestOperation{
			RouteKey:           operationRouteKey(operation),
			OperationID:        operation.OperationID,
			Summary:            operation.Summary,
			Description:        operation.Description,
			Method:             operation.Method,
			Path:               operation.Path,
			CallExpression:     callExpression,
			ResourceSegments:   segments,
			PathParameterOrder: append([]string{}, operation.PathParameterOrder...),
			InputSections:      append(operationInputSectionList{}, inputSections...),
			OutputType:         outputExpression.render(typeRenderLocal),
			ErrorType:          errorExpression.render(typeRenderLocal),
			Envelope:           operation.Envelope,
			Pagination:         operation.Pagination,
			Auth:               operationAuth(operation),
			Visibility:         visibility,
			Deprecated:         boolValue(operation.Raw, "deprecated"),
			outputExpression:   outputExpression,
			errorExpression:    errorExpression,
			rawResponse:        rawResponse,
			mediaOutputs:       mediaOutputs,
			renderedMedia:      renderedMedia,
			mediaTypes:         mediaTypes,
			streamMediaTypes:   streamMediaTypes,
			security:           security,
			hasSecurity:        hasSecurity,
			optionsRequired:    len(security) > 1,
			paginationRequest: func() ir.PaginationRequestPlan {
				if operation.PaginationPlan == nil {
					return ir.PaginationRequestPlan{}
				}
				return operation.PaginationPlan.Request
			}(),
			compiled: operation,
			prepared: prepared,
		})
	}
	if len(failures) != 0 {
		return manifest, failures
	}
	resourceManifest := filterManifestOperations(manifest, resourceReservationExcluded)
	tree, err := buildResourceTree(document, resourceManifest)
	if err != nil {
		return Manifest{}, append(failures, err)
	}
	reachable := make(map[string]bool)
	resourceOperationIDs(tree, reachable)
	for index := range manifest.Operations {
		item := &manifest.Operations[index]
		if resourceReservationExcluded[item.RouteKey] {
			item.ResourceSegments = nil
			continue
		}
		if item.Visibility == "public" && !reachable[item.RouteKey] {
			operation := item.compiled
			item.CallExpression = exactOperationCall(document, operation, item.InputSections)
			item.ResourceSegments = nil
		}
	}
	return manifest, failures
}

func visibilityManifest(document *ir.Document) Manifest {
	result := Manifest{Operations: make([]ManifestOperation, 0, len(document.Operations))}
	for _, operation := range document.Operations {
		visibility := operation.Visibility
		if visibility == "" {
			visibility = "public"
		}
		result.Operations = append(result.Operations, ManifestOperation{
			RouteKey:    operationRouteKey(operation),
			OperationID: operation.OperationID,
			Method:      operation.Method,
			Path:        operation.Path,
			Visibility:  visibility,
			compiled:    operation,
		})
	}
	return result
}

func (operation ManifestOperation) renderOutput(scope typeRenderScope) string {
	if operation.outputExpression.local == "" && operation.outputExpression.contract == "" {
		return operation.OutputType
	}
	return operation.outputExpression.render(scope)
}

func (operation ManifestOperation) renderError(scope typeRenderScope) string {
	if operation.errorExpression.local == "" && operation.errorExpression.contract == "" {
		return operation.ErrorType
	}
	return operation.errorExpression.render(scope)
}

func operationCall(document *ir.Document, operation ir.Operation, inputSections operationInputSectionList) (string, []string, error) {
	prepared, err := prepareOperation(document, operation)
	if err != nil {
		return "", nil, err
	}
	return operationCallFromPrepared(document, operation, inputSections, prepared)
}

func operationCallFromPrepared(document *ir.Document, operation ir.Operation, inputSections operationInputSectionList, prepared preparedOperation) (string, []string, error) {
	if hasDuplicateStrings(operation.PathParameterOrder) {
		return exactOperationCall(document, operation, inputSections), nil, nil
	}
	pathBindings := prepared.pathBindings
	parts := resourcePathParts(operation.Path)
	segments := make([]string, 0, len(parts))
	chain := "api"
	for _, part := range parts {
		name, parameterPart, supported := resourcePathPart(part)
		if !supported {
			return exactOperationCall(document, operation, inputSections), nil, nil
		}
		if parameterPart {
			binding := pathBindings[name]
			if binding == "" {
				return exactOperationCall(document, operation, inputSections), nil, nil
			}
			chain += "(" + binding + ")"
			continue
		}
		property, err := naming.Property(part)
		if err != nil {
			return exactOperationCall(document, operation, inputSections), nil, nil
		}
		segments = append(segments, property)
		chain += "." + property
	}
	terminal, err := resourceTerminalName(operation, parts)
	if err != nil {
		return exactOperationCall(document, operation, inputSections), nil, nil
	}
	if operation.Visibility == "internal" {
		return exactOperationCall(document, operation, inputSections), segments, nil
	}
	return chain + "." + terminal + callInput(operation, inputSections, len(operation.PathParameterOrder) > 0, operation.PathParameterOrder, pathBindings), segments, nil
}

func exactOperationCall(document *ir.Document, operation ir.Operation, inputSections operationInputSectionList) string {
	pathBindings, _ := operationPathBindings(document, operation)
	if operation.OperationID != "" {
		return "api.$operations[" + quoteTS(operation.OperationID) + "]" + callInput(operation, inputSections, false, operation.PathParameterOrder, pathBindings)
	}
	return "api.$routes[" + quoteTS(operationRouteKey(operation)) + "]" + callInput(operation, inputSections, false, operation.PathParameterOrder, pathBindings)
}

func resourcePathParts(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func resourcePathPart(part string) (string, bool, bool) {
	if !strings.ContainsAny(part, "{}") {
		return part, false, true
	}
	if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") &&
		strings.Count(part, "{") == 1 && strings.Count(part, "}") == 1 && len(part) > 2 {
		return strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}"), true, true
	}
	return "", false, false
}

func hasDuplicateStrings(values []string) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}

// resourceTerminalName keeps literal path segments as namespaces. For example,
// POST /auth/login becomes api.auth.login.post(), rather than api.auth.login().
func resourceTerminalName(operation ir.Operation, parts []string) (string, error) {
	terminal, err := terminalName(operation, parts)
	if err != nil {
		return "", err
	}
	if len(parts) == 0 {
		return terminal, nil
	}
	last := parts[len(parts)-1]
	if strings.HasPrefix(last, "{") {
		return terminal, nil
	}
	property, err := naming.Property(last)
	if err != nil {
		return "", err
	}
	if terminal != property {
		return terminal, nil
	}
	return methodTerminal(operation.Method)
}

func methodTerminal(method string) (string, error) {
	switch method {
	case "GET":
		return "get", nil
	case "POST":
		return "post", nil
	case "PUT":
		return "replace", nil
	case "PATCH":
		return "patch", nil
	case "DELETE":
		return "delete", nil
	case "QUERY":
		return "query", nil
	default:
		return naming.Property(strings.ToLower(method))
	}
}

func operationPathBindings(document *ir.Document, operation ir.Operation) (map[string]string, error) {
	parameters, err := operationParameters(document, operation)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, parameter := range parameters {
		if parameter.Location == "path" {
			result[parameter.Name] = parameter.Binding
		}
	}
	return result, nil
}

func callInput(operation ir.Operation, inputSections operationInputSectionList, pathBound bool, pathParameters []string, pathBindings map[string]string) string {
	var fields []string
	for _, section := range inputSections {
		switch section {
		case inputSectionPath:
			if pathBound {
				continue
			}
			values := make([]string, 0, len(pathParameters))
			seen := make(map[string]bool, len(pathParameters))
			for _, parameter := range pathParameters {
				if seen[parameter] {
					continue
				}
				seen[parameter] = true
				binding := pathBindings[parameter]
				if binding == "" {
					binding = stablePrivateIdentifier("example-path-parameter", parameter)
				}
				values = append(values, quoteTS(parameter)+": "+binding)
			}
			fields = append(fields, "path: { "+strings.Join(values, ", ")+" }")
		case inputSectionQuery:
			fields = append(fields, "query")
		case inputSectionQuerystring:
			fields = append(fields, "querystring")
		case inputSectionHeader:
			fields = append(fields, "headerParams")
		case inputSectionCookie:
			fields = append(fields, "cookieParams")
		case inputSectionBody:
			fields = append(fields, "body")
		}
	}
	var arguments []string
	if len(fields) > 0 {
		arguments = append(arguments, "{ "+strings.Join(fields, ", ")+" }")
	}
	return "(" + strings.Join(arguments, ", ") + ")"
}

func terminalName(operation ir.Operation, parts []string) (string, error) {
	last := ""
	if len(parts) > 0 {
		last = parts[len(parts)-1]
	}
	if isTerminalAction(operation, parts, len(parts)-1) {
		return naming.Property(last)
	}
	lowerID := strings.ToLower(operation.OperationID)
	switch operation.Method {
	case "GET":
		if strings.HasPrefix(lowerID, "list") || strings.HasPrefix(lowerID, "search") {
			return "list", nil
		}
		return "get", nil
	case "POST":
		if strings.HasPrefix(lowerID, "create") {
			return "create", nil
		}
		return "post", nil
	case "PUT":
		return "replace", nil
	case "PATCH":
		return "patch", nil
	case "DELETE":
		return "delete", nil
	case "QUERY":
		return "query", nil
	default:
		return naming.Property(strings.ToLower(operation.Method))
	}
}

func isTerminalAction(operation ir.Operation, parts []string, index int) bool {
	if index < 0 || index != len(parts)-1 || operation.Method == "GET" || len(parts) < 2 {
		return false
	}
	last := parts[index]
	if last == "" || strings.HasPrefix(last, "{") {
		return false
	}
	return strings.Contains(operation.OperationID, strings.ToUpper(last[:1])+last[1:]) || !strings.HasPrefix(strings.ToLower(operation.OperationID), "create")
}

// Every operation owns one module. Its private semantic type family therefore
// needs no route-derived identity in the lexical spelling.
const operationLocalTypePrefix = "__sdkgen_"

func operationAuth(operation ir.Operation) string {
	if operation.Parameters != nil {
		if !operation.SecurityDeclared {
			return "inherited"
		}
		if len(operation.Security) == 0 {
			return "public"
		}
		return "required"
	}
	security, exists := operation.Raw["security"]
	if !exists {
		return "inherited"
	}
	items, _ := security.([]any)
	if len(items) == 0 {
		return "public"
	}
	return "required"
}

func boolValue(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}
