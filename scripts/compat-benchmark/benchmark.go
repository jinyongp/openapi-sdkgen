package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sdkgen "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/generator"
	"openapi-sdkgen/internal/target/typescript"
)

const benchmarkSchemaVersion = 1
const benchmarkReportSchemaVersion = 2

type benchmarkManifest struct {
	SchemaVersion int                `json:"schemaVersion"`
	Pinned        bool               `json:"pinned,omitempty"`
	Files         []pinnedCorpusFile `json:"files,omitempty"`
	Source        *corpusSource      `json:"source,omitempty"`
	Selection     *selectionMetadata `json:"selection,omitempty"`
	Corpora       []corpusSpec       `json:"corpora"`
}

type pinnedCorpusFile struct {
	Input  string `json:"input"`
	SHA256 string `json:"sha256"`
}

type corpusSource struct {
	Repository string `json:"repository"`
	Commit     string `json:"commit"`
	APITree    string `json:"apiTree,omitempty"`
	RawBaseURL string `json:"rawBaseUrl,omitempty"`
}

type selectionMetadata struct {
	Method            string        `json:"method"`
	CandidateCount    int           `json:"candidateCount"`
	ExcludedProviders []string      `json:"excludedProviders,omitempty"`
	SizeStrata        []sizeStratum `json:"sizeStrata,omitempty"`
}

type sizeStratum struct {
	Name       string `json:"name"`
	MinBytes   int64  `json:"minBytes"`
	MaxBytes   int64  `json:"maxBytes,omitempty"`
	Candidates int    `json:"candidates"`
	Selected   int    `json:"selected"`
}

type corpusSpec struct {
	SHA256         string `json:"sha256,omitempty"`
	OpenAPIVersion string `json:"openapiVersion,omitempty"`
	EvidenceKind   string `json:"evidenceKind,omitempty"`
	SourceURL      string `json:"sourceUrl,omitempty"`
	Revision       string `json:"revision,omitempty"`
	Trust          string `json:"trust,omitempty"`
	ID             string `json:"id"`
	Provider       string `json:"provider,omitempty"`
	Cohort         string `json:"cohort"`
	Input          string `json:"input"`
	GitBlob        string `json:"gitBlob,omitempty"`
	Bytes          int64  `json:"bytes,omitempty"`
	SizeClass      string `json:"sizeClass,omitempty"`
}

type benchmarkReport struct {
	SchemaVersion  int              `json:"schemaVersion"`
	ManifestSHA256 string           `json:"manifestSha256"`
	FeatureCatalog []string         `json:"featureCatalog"`
	Documents      []documentResult `json:"documents"`
	Cohorts        []summaryResult  `json:"cohorts"`
	Overall        summaryResult    `json:"overall"`
}

type documentResult struct {
	EvidenceKind              string                        `json:"evidenceKind,omitempty"`
	ID                        string                        `json:"id"`
	Cohort                    string                        `json:"cohort"`
	Input                     string                        `json:"input"`
	InputSHA256               string                        `json:"inputSha256"`
	GitBlob                   string                        `json:"gitBlob,omitempty"`
	SizeClass                 string                        `json:"sizeClass,omitempty"`
	OpenAPIVersion            string                        `json:"openapiVersion,omitempty"`
	DiscoveryComplete         bool                          `json:"discoveryComplete"`
	Diagnostics               diagnostic.Counts             `json:"diagnostics"`
	Findings                  []diagnostic.Diagnostic       `json:"findings"`
	OperationRetention        operationMetric               `json:"operationRetention"`
	OperationEmission         *emissionMetric               `json:"operationEmission,omitempty"`
	Compatibility             compatibilityMetric           `json:"compatibility"`
	Features                  []string                      `json:"features"`
	Generation                verificationResult            `json:"generation"`
	Typecheck                 verificationResult            `json:"typecheck"`
	DocumentSuccess           bool                          `json:"documentSuccess"`
	SupportProfiles           []supportProfileResult        `json:"supportProfiles,omitempty"`
	CapabilityAdjustedSuccess bool                          `json:"capabilityAdjustedSuccess"`
	Coverage                  []diagnostic.AnalysisCoverage `json:"coverage"`
	SkippedPhases             []diagnostic.SkippedPhase     `json:"skippedPhases"`
}

type supportProfileResult struct {
	Name              string                        `json:"name"`
	Applicable        bool                          `json:"applicable"`
	DiscoveryComplete bool                          `json:"discoveryComplete"`
	Diagnostics       diagnostic.Counts             `json:"diagnostics"`
	Findings          []diagnostic.Diagnostic       `json:"findings"`
	Generation        verificationResult            `json:"generation"`
	Typecheck         verificationResult            `json:"typecheck"`
	Success           bool                          `json:"success"`
	OperationEmission *emissionMetric               `json:"operationEmission,omitempty"`
	Coverage          []diagnostic.AnalysisCoverage `json:"coverage,omitempty"`
	SkippedPhases     []diagnostic.SkippedPhase     `json:"skippedPhases,omitempty"`
}

type operationMetric struct {
	Available           bool     `json:"available"`
	Total               int      `json:"total"`
	Retained            int      `json:"retained"`
	Omitted             int      `json:"omitted"`
	UnresolvedOmissions int      `json:"unresolvedOmissions"`
	Rate                *float64 `json:"rate"`
}

type emissionMetric struct {
	Available          bool `json:"available"`
	Count              int  `json:"count"`
	OperationOmissions int  `json:"operationOmissions"`
	HelperOmissions    int  `json:"helperOmissions"`
}

type aggregateEmission struct {
	AvailableDocuments int `json:"availableDocuments"`
	Count              int `json:"count"`
	OperationOmissions int `json:"operationOmissions"`
	HelperOmissions    int `json:"helperOmissions"`
}

type compatibilityMetric struct {
	Preserved int            `json:"preserved"`
	Rejected  int            `json:"rejected"`
	Total     int            `json:"total"`
	Rate      *float64       `json:"rate"`
	Actions   map[string]int `json:"actions"`
	Rules     map[string]int `json:"rules"`
}

type verificationResult struct {
	Status        string `json:"status"`
	ArtifactCount int    `json:"artifactCount,omitempty"`
	ArtifactBytes int64  `json:"artifactBytes,omitempty"`
	Detail        string `json:"detail,omitempty"`
}

type summaryResult struct {
	Cohort                        string              `json:"cohort,omitempty"`
	Documents                     int                 `json:"documents"`
	SuccessfulDocuments           int                 `json:"successfulDocuments"`
	DocumentSuccessRate           *float64            `json:"documentSuccessRate"`
	CapabilityAdjustedDocuments   int                 `json:"capabilityAdjustedDocuments"`
	CapabilityAdjustedSuccessRate *float64            `json:"capabilityAdjustedSuccessRate"`
	DiscoveryComplete             int                 `json:"discoveryComplete"`
	GeneratedDocuments            int                 `json:"generatedDocuments"`
	TypecheckedDocuments          int                 `json:"typecheckedDocuments"`
	GeneratedVerificationRate     *float64            `json:"generatedVerificationRate"`
	Operations                    aggregateOperations `json:"operations"`
	OperationEmission             *aggregateEmission  `json:"operationEmission,omitempty"`
	Compatibility                 compatibilityMetric `json:"compatibility"`
	FeatureCoverage               featureCoverage     `json:"featureCoverage"`
}

type aggregateOperations struct {
	AvailableDocuments int      `json:"availableDocuments"`
	Total              int      `json:"total"`
	Retained           int      `json:"retained"`
	Omitted            int      `json:"omitted"`
	Rate               *float64 `json:"rate"`
}

type featureCoverage struct {
	Observed int      `json:"observed"`
	Total    int      `json:"total"`
	Rate     *float64 `json:"rate"`
	Missing  []string `json:"missing"`
}

type typecheckFunc func(string) verificationResult

func runBenchmark(manifestPath, corpusRoot, outputPath, typescriptRoot string, timeout time.Duration) error {
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	var manifest benchmarkManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}
	if err := validateManifest(manifest); err != nil {
		return err
	}
	manifestDigest := sha256.Sum256(manifestData)
	manifestSHA := hex.EncodeToString(manifestDigest[:])
	if manifest.Source != nil || manifest.Pinned {
		if err := verifyMaterializedCorpus(corpusRoot, manifest, manifestSHA); err != nil {
			return fmt.Errorf("verify corpus before benchmark: %w", err)
		}
	}

	typescriptRoot, err = filepath.Abs(typescriptRoot)
	if err != nil {
		return err
	}
	if err := validateTypecheckToolchain(typescriptRoot); err != nil {
		return fmt.Errorf("strict TypeScript toolchain: %w", err)
	}
	typecheck := func(directory string) verificationResult {
		return strictTypecheck(directory, typescriptRoot, timeout)
	}

	documents := make([]documentResult, 0, len(manifest.Corpora))
	for _, corpus := range manifest.Corpora {
		inputPath, err := safeCorpusPath(corpusRoot, corpus.Input)
		if err != nil {
			return fmt.Errorf("%s: %w", corpus.ID, err)
		}
		result, err := benchmarkDocument(corpus, inputPath, typecheck)
		if err != nil {
			return fmt.Errorf("%s: %w", corpus.ID, err)
		}
		documents = append(documents, result)
	}
	sort.Slice(documents, func(i, j int) bool {
		if documents[i].Cohort != documents[j].Cohort {
			return documents[i].Cohort < documents[j].Cohort
		}
		return documents[i].ID < documents[j].ID
	})

	report := benchmarkReport{
		SchemaVersion:  benchmarkReportSchemaVersion,
		ManifestSHA256: hex.EncodeToString(manifestDigest[:]),
		FeatureCatalog: featureCatalog(),
		Documents:      documents,
		Cohorts:        summarizeCohorts(documents),
		Overall:        summarizeDocuments("", documents),
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("encode benchmark report: %w", err)
	}
	encoded = append(encoded, '\n')
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, encoded, 0o644); err != nil {
		return fmt.Errorf("write benchmark report: %w", err)
	}
	return nil
}

func validateManifest(manifest benchmarkManifest) error {
	if manifest.SchemaVersion != benchmarkSchemaVersion {
		return fmt.Errorf("manifest schemaVersion = %d, want %d", manifest.SchemaVersion, benchmarkSchemaVersion)
	}
	if len(manifest.Corpora) == 0 {
		return errors.New("manifest requires at least one corpus")
	}
	if manifest.Source != nil {
		if strings.TrimSpace(manifest.Source.Repository) == "" || strings.TrimSpace(manifest.Source.Commit) == "" {
			return errors.New("manifest source requires repository and commit")
		}
		if manifest.Source.RawBaseURL != "" {
			base, err := url.Parse(manifest.Source.RawBaseURL)
			if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" {
				return fmt.Errorf("manifest source rawBaseUrl must be an absolute HTTP(S) URL")
			}
		}
	}
	ids := map[string]bool{}
	providers := map[string]bool{}
	for _, corpus := range manifest.Corpora {
		if manifest.Pinned {
			if !validSHA256(corpus.SHA256) || corpus.OpenAPIVersion == "" || corpus.Trust == "" || corpus.SourceURL == "" || corpus.Revision == "" {
				return fmt.Errorf("pinned corpus %q requires sha256, openapiVersion, sourceUrl, revision and trust", corpus.ID)
			}
			switch corpus.EvidenceKind {
			case "real-document", "normative", "reference", "target-boundary":
			default:
				return fmt.Errorf("pinned corpus %q has invalid evidenceKind", corpus.ID)
			}
		}
		if strings.TrimSpace(corpus.ID) == "" || strings.TrimSpace(corpus.Cohort) == "" || strings.TrimSpace(corpus.Input) == "" {
			return errors.New("every corpus requires id, cohort, and input")
		}
		if ids[corpus.ID] {
			return fmt.Errorf("duplicate corpus id %q", corpus.ID)
		}
		ids[corpus.ID] = true
		input := filepath.ToSlash(corpus.Input)
		clean := filepath.ToSlash(filepath.Clean(corpus.Input))
		if filepath.IsAbs(corpus.Input) || input == ".." || strings.HasPrefix(input, "../") ||
			strings.HasPrefix(input, "//") || strings.Contains(input, "://") || clean != input {
			return fmt.Errorf("corpus %q has non-canonical relative input %q", corpus.ID, corpus.Input)
		}
		if corpus.Provider != "" {
			if providers[corpus.Provider] {
				return fmt.Errorf("duplicate corpus provider %q", corpus.Provider)
			}
			providers[corpus.Provider] = true
		}
		if manifest.Source != nil {
			if corpus.GitBlob == "" || corpus.Bytes <= 0 {
				return fmt.Errorf("source-backed corpus %q requires gitBlob and positive bytes", corpus.ID)
			}
		}
	}
	for _, file := range manifest.Files {
		if !validSHA256(file.SHA256) {
			return fmt.Errorf("pinned auxiliary file %q requires sha256", file.Input)
		}
		if _, err := safeCorpusPath(".", file.Input); err != nil {
			return err
		}
		if filepath.ToSlash(filepath.Clean(file.Input)) != file.Input || file.Input == "." {
			return fmt.Errorf("non-canonical auxiliary input %q", file.Input)
		}
	}
	if manifest.Selection != nil {
		if strings.TrimSpace(manifest.Selection.Method) == "" || manifest.Selection.CandidateCount <= 0 {
			return errors.New("selection metadata requires method and positive candidateCount")
		}
		for _, stratum := range manifest.Selection.SizeStrata {
			if stratum.Name == "" || stratum.MinBytes < 0 || stratum.MaxBytes < 0 || stratum.Candidates <= 0 || stratum.Selected <= 0 {
				return fmt.Errorf("invalid selection size stratum %#v", stratum)
			}
			if stratum.MaxBytes != 0 && stratum.MaxBytes <= stratum.MinBytes {
				return fmt.Errorf("selection size stratum %q has invalid bounds", stratum.Name)
			}
		}
	}
	return nil
}

func safeCorpusPath(root, input string) (string, error) {
	if root == "" {
		return "", errors.New("corpus root is required")
	}
	if filepath.IsAbs(input) {
		return "", fmt.Errorf("corpus input must be relative: %q", input)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	full := filepath.Join(rootAbs, filepath.FromSlash(input))
	relative, err := filepath.Rel(rootAbs, full)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("corpus input escapes root: %q", input)
	}
	return full, nil
}

func benchmarkDocument(corpus corpusSpec, inputPath string, typecheck typecheckFunc) (documentResult, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return documentResult{}, fmt.Errorf("read input: %w", err)
	}
	sum := sha256.Sum256(data)
	decoded, err := decodeFeatureInput(data)
	if err != nil {
		return documentResult{}, fmt.Errorf("decode feature input: %w", err)
	}
	features, version := detectFeatures(decoded)

	mode := diagnostic.ModeCollect
	compiled, err := sdkgen.CompileInputResultWithOptions(inputPath, sdkgen.CompileOptions{DiagnosticMode: mode})
	if err != nil {
		return documentResult{}, fmt.Errorf("internal compile failure: %w", err)
	}
	prepared, err := generator.PrepareCompilation(typescript.Generator{}, compiled, generator.Options{DiagnosticMode: mode})
	if err != nil {
		return documentResult{}, fmt.Errorf("internal target preparation failure: %w", err)
	}
	report := diagnostic.NewReport(prepared.Diagnostics, prepared.SkippedPhases, prepared.Coverage...)
	normalizeBenchmarkReportSources(&report, inputPath)
	result := documentResult{
		EvidenceKind:       corpus.EvidenceKind,
		ID:                 corpus.ID,
		Cohort:             corpus.Cohort,
		Input:              filepath.ToSlash(corpus.Input),
		InputSHA256:        hex.EncodeToString(sum[:]),
		GitBlob:            corpus.GitBlob,
		SizeClass:          corpus.SizeClass,
		OpenAPIVersion:     version,
		DiscoveryComplete:  discoveryComplete(report),
		Diagnostics:        report.Counts,
		Findings:           report.Diagnostics,
		OperationRetention: operationRetention(compiled.Document, prepared.Diagnostics),
		Compatibility:      compatibilityMetrics(prepared.Diagnostics),
		Features:           features,
		Coverage:           report.Coverage,
		SkippedPhases:      report.SkippedPhases,
	}
	defaultProfile, err := verifyPreparedProfile("default-client", compiled.Document != nil, prepared, inputPath, typecheck)
	if err != nil {
		return documentResult{}, err
	}
	result.Generation = defaultProfile.Generation
	result.OperationEmission = defaultProfile.OperationEmission
	result.OperationEmission.OperationOmissions = result.OperationRetention.Omitted
	result.Typecheck = defaultProfile.Typecheck
	result.DocumentSuccess = defaultProfile.Success
	result.CapabilityAdjustedSuccess = result.DocumentSuccess

	if hasInboundContractFeature(features) {
		registry, err := generator.NewAddonRegistry(generator.AddonServer)
		if err != nil {
			return documentResult{}, fmt.Errorf("create add-on registry: %w", err)
		}
		serverOptions, err := registry.Resolve([]string{string(generator.AddonServer)})
		if err != nil {
			return documentResult{}, fmt.Errorf("resolve server add-on: %w", err)
		}
		serverOptions.DiagnosticMode = mode
		serverPrepared, err := generator.PrepareCompilation(typescript.Generator{}, compiled, serverOptions)
		if err != nil {
			return documentResult{}, fmt.Errorf("internal server target preparation failure: %w", err)
		}
		serverProfile, err := verifyPreparedProfile("server-addon", compiled.Document != nil, serverPrepared, inputPath, typecheck)
		if err != nil {
			return documentResult{}, err
		}
		serverProfile.OperationEmission.OperationOmissions = operationRetention(compiled.Document, serverPrepared.Diagnostics).Omitted
		result.SupportProfiles = append(result.SupportProfiles, serverProfile)
		if serverProfile.Success {
			result.CapabilityAdjustedSuccess = true
		}
	}
	return result, nil
}

func verifyPreparedProfile(name string, hasDocument bool, prepared generator.Preparation, inputPath string, typecheck typecheckFunc) (supportProfileResult, error) {
	report := diagnostic.NewReport(prepared.Diagnostics, prepared.SkippedPhases, prepared.Coverage...)
	normalizeBenchmarkReportSources(&report, inputPath)
	result := supportProfileResult{
		Name:              name,
		Applicable:        true,
		DiscoveryComplete: discoveryComplete(report),
		Diagnostics:       report.Counts,
		Findings:          report.Diagnostics,
		Generation:        verificationResult{Status: "blocked"},
		Typecheck:         verificationResult{Status: "not-run"},
		Coverage:          report.Coverage,
		SkippedPhases:     report.SkippedPhases,
		OperationEmission: &emissionMetric{Available: true, HelperOmissions: helperOmissionCount(prepared.Diagnostics)},
	}
	if diagnostic.HasErrors(prepared.Diagnostics) || !hasDocument {
		return result, nil
	}

	temporary, err := os.MkdirTemp("", "openapi-sdkgen-compat-benchmark-*")
	if err != nil {
		return supportProfileResult{}, err
	}
	defer os.RemoveAll(temporary)

	artifactCount := 0
	var artifactBytes int64
	emitErr := generator.EmitTo(typescript.Generator{}, prepared.Plan, generator.ArtifactSinkFunc(func(artifact generator.Artifact) error {
		path, err := safeArtifactPath(temporary, artifact.Path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		data := artifact.Data
		if strings.HasSuffix(artifact.Path, ".ts") {
			data = stripTSNoCheck(data)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		artifactCount++
		artifactBytes += int64(len(data))
		return nil
	}))
	if emitErr != nil {
		result.Generation = verificationResult{Status: "fail", Detail: boundedDetail(emitErr.Error())}
		result.OperationEmission.Available = false
		return result, nil
	}
	routes, err := typescript.EmissionRoutes(prepared.Plan)
	if err != nil {
		return supportProfileResult{}, fmt.Errorf("read emitted operation manifest: %w", err)
	}
	result.OperationEmission.Count = len(routes)
	result.Generation = verificationResult{Status: "pass", ArtifactCount: artifactCount, ArtifactBytes: artifactBytes}
	result.Typecheck = typecheck(temporary)
	result.Success = result.DiscoveryComplete && result.Diagnostics.Errors == 0 &&
		result.Generation.Status == "pass" && result.Typecheck.Status == "pass"
	return result, nil
}

func helperOmissionCount(values []diagnostic.Diagnostic) int {
	seen := map[string]bool{}
	for _, value := range values {
		if value.Scope == failure.ScopeCapability && value.Effect == failure.EffectOmitCapability {
			seen[value.Location.Source+"\x00"+value.Location.Pointer+"\x00"+value.Capability] = true
		}
	}
	return len(seen)
}

func hasInboundContractFeature(features []string) bool {
	for _, feature := range features {
		if feature == "document.webhooks" || feature == "operation.callbacks" {
			return true
		}
	}
	return false
}

func normalizeBenchmarkReportSources(report *diagnostic.Report, rootInput string) {
	rootInput, _ = filepath.Abs(rootInput)
	rootDirectory := filepath.Dir(rootInput)
	normalize := func(source string) string {
		if source == "" {
			return ""
		}
		absolute, err := filepath.Abs(source)
		if err != nil {
			return source
		}
		if absolute == rootInput {
			return "$root"
		}
		relative, err := filepath.Rel(rootDirectory, absolute)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative) {
			return filepath.ToSlash(relative)
		}
		return source
	}
	for index := range report.Diagnostics {
		report.Diagnostics[index].Location.Source = normalize(report.Diagnostics[index].Location.Source)
		for related := range report.Diagnostics[index].Related {
			report.Diagnostics[index].Related[related].Source = normalize(report.Diagnostics[index].Related[related].Source)
		}
	}
	for index := range report.Coverage {
		if report.Coverage[index].Location != nil {
			report.Coverage[index].Location.Source = normalize(report.Coverage[index].Location.Source)
		}
	}
}

func discoveryComplete(report diagnostic.Report) bool {
	for _, phase := range report.SkippedPhases {
		if phase.Phase != diagnostic.PhaseEmit && phase.Phase != diagnostic.PhasePublish {
			return false
		}
	}
	for _, item := range report.Coverage {
		if item.Status != diagnostic.CoverageComplete {
			return false
		}
	}
	return true
}

func operationRetention(document *ir.Document, diagnostics []diagnostic.Diagnostic) operationMetric {
	if document == nil {
		return operationMetric{}
	}
	total := len(document.Operations)
	keysByPointer := map[string]string{}
	keysByRoute := map[string]string{}
	keysByID := map[string][]string{}
	for index, operation := range document.Operations {
		key := operation.Pointer
		if key == "" {
			key = fmt.Sprintf("operation:%d:%s:%s", index, operation.Method, operation.Path)
		}
		keysByPointer[operation.Pointer] = key
		route := strings.ToUpper(strings.TrimSpace(operation.Method)) + " " + operation.Path
		keysByRoute[route] = key
		if operation.OperationID != "" {
			keysByID[operation.OperationID] = append(keysByID[operation.OperationID], key)
		}
	}
	omitted := map[string]bool{}
	unresolved := 0
	resolvePointer := func(pointer string) (string, bool) {
		best := ""
		for operationPointer, key := range keysByPointer {
			if operationPointer == "" {
				continue
			}
			if pointer == operationPointer || strings.HasPrefix(pointer, operationPointer+"/") {
				if len(operationPointer) > len(best) {
					best = operationPointer
					_ = key
				}
			}
		}
		if best == "" {
			return "", false
		}
		return keysByPointer[best], true
	}
	for _, restriction := range document.SemanticRestrictions {
		if restriction.Scope != failure.ScopeOperation || restriction.Effect != failure.EffectOmitOperation {
			continue
		}
		if key, exists := keysByPointer[restriction.OwnerPointer]; exists {
			omitted[key] = true
			continue
		}
		if key, exists := resolvePointer(restriction.Location.Pointer); exists {
			omitted[key] = true
			continue
		}
		unresolved++
	}
	for _, value := range diagnostics {
		if value.Scope != failure.ScopeOperation || value.Effect != failure.EffectOmitOperation {
			continue
		}
		if key, exists := keysByRoute[value.Route]; value.Route != "" && exists {
			omitted[key] = true
			continue
		}
		if value.Operation != "" {
			if keys := keysByID[value.Operation]; len(keys) == 1 {
				omitted[keys[0]] = true
				continue
			}
		}
		if key, exists := resolvePointer(value.Location.Pointer); exists {
			omitted[key] = true
			continue
		}
		unresolved++
	}
	if unresolved != 0 || len(omitted) > total {
		return operationMetric{Total: total, Omitted: len(omitted), UnresolvedOmissions: unresolved}
	}
	retained := total - len(omitted)
	return operationMetric{
		Available: true,
		Total:     total,
		Retained:  retained,
		Omitted:   len(omitted),
		Rate:      ratio(retained, total),
	}
}

func compatibilityMetrics(values []diagnostic.Diagnostic) compatibilityMetric {
	result := compatibilityMetric{Actions: map[string]int{}, Rules: map[string]int{}}
	for _, value := range values {
		if value.Rule == "" || value.Action == "" {
			continue
		}
		result.Actions[value.Action]++
		result.Rules[value.Rule]++
		switch value.Action {
		case "preserve", "preserve-extension", "normalize", "ignore":
			result.Preserved++
		case "reject":
			result.Rejected++
		default:
			continue
		}
		result.Total++
	}
	result.Rate = ratio(result.Preserved, result.Total)
	return result
}

func validateTypecheckToolchain(typescriptRoot string) error {
	tsc := filepath.Join(typescriptRoot, "node_modules", "typescript", "lib", "tsc.js")
	if info, err := os.Stat(tsc); err != nil {
		return fmt.Errorf("typescript compiler unavailable: %w", err)
	} else if info.IsDir() {
		return fmt.Errorf("typescript compiler path is a directory: %s", tsc)
	}
	output, err := exec.Command("node", "--version").CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("node unavailable: %s", boundedDetail(detail))
	}
	if strings.TrimSpace(string(output)) == "" {
		return errors.New("node --version returned empty output")
	}
	return nil
}

func strictTypecheck(directory, typescriptRoot string, timeout time.Duration) verificationResult {
	if err := writeTypecheckFiles(directory); err != nil {
		return verificationResult{Status: "fail", Detail: boundedDetail(err.Error())}
	}
	tsc := filepath.Join(typescriptRoot, "node_modules", "typescript", "lib", "tsc.js")
	if _, err := os.Stat(tsc); err != nil {
		return verificationResult{Status: "fail", Detail: boundedDetail("typescript compiler unavailable: " + err.Error())}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, "node", tsc, "--project", filepath.Join(directory, "tsconfig.json"))
	command.Dir = directory
	output, err := command.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return verificationResult{Status: "timeout", Detail: boundedDetail(string(output))}
	}
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return verificationResult{Status: "fail", Detail: boundedDetail(detail)}
	}
	return verificationResult{Status: "pass"}
}

func writeTypecheckFiles(directory string) error {
	files := map[string]string{
		"package.json": "{\"type\":\"module\",\"private\":true}\n",
		"tsconfig.json": `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "NodeNext",
    "moduleResolution": "NodeNext",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "skipLibCheck": false,
    "noEmit": true
  },
  "include": ["**/*.ts"]
}
`,
		"compat-benchmark.consumer.ts": `import { createClient, type Components, type Operations } from "./index.js"

type Equal<Left, Right> =
  (<Value>() => Value extends Left ? 1 : 2) extends
  (<Value>() => Value extends Right ? 1 : 2) ? true : false
type Assert<Value extends true> = Value

type OperationID = keyof Operations
type Operation = Operations[OperationID]
type _Input = Operation["input"]
type _Output = Operation["output"]
type _Error = Operation["error"]
type _Options = Operation["options"]
type _Call = Operation["call"]
type _Raw = Operation["call"]["raw"]
type _RawResponse = Operation["rawResponse"]
type _Pagination = Operation["pagination"]

type ComponentName = keyof Components
type Component = Components[ComponentName]
type _ComponentInput = Component["input"]
type _ComponentOutput = Component["output"]

type Client = ReturnType<typeof createClient>
type _ExactOperations = Assert<Equal<keyof Client["$operations"], OperationID>>

export type CompatibilityBenchmarkConsumerProbe = {
  readonly operation: [_Input, _Output, _Error, _Options, _Call, _Raw, _RawResponse, _Pagination]
  readonly component: [_ComponentInput, _ComponentOutput]
  readonly operations: _ExactOperations
}
`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func safeArtifactPath(root, artifact string) (string, error) {
	if artifact == "" || filepath.IsAbs(artifact) {
		return "", fmt.Errorf("invalid generated artifact path %q", artifact)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	full := filepath.Join(rootAbs, filepath.FromSlash(artifact))
	relative, err := filepath.Rel(rootAbs, full)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("generated artifact escapes output root: %q", artifact)
	}
	return full, nil
}

func stripTSNoCheck(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("// @ts-nocheck\r\n"), nil)
	return bytes.ReplaceAll(data, []byte("// @ts-nocheck\n"), nil)
}

func boundedDetail(value string) string {
	value = strings.TrimSpace(value)
	const limit = 4096
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}

func ratio(numerator, denominator int) *float64 {
	if denominator == 0 {
		return nil
	}
	value := float64(numerator) / float64(denominator)
	return &value
}

func summarizeCohorts(documents []documentResult) []summaryResult {
	groups := map[string][]documentResult{}
	for _, document := range documents {
		groups[document.Cohort] = append(groups[document.Cohort], document)
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]summaryResult, 0, len(names))
	for _, name := range names {
		result = append(result, summarizeDocuments(name, groups[name]))
	}
	return result
}

func summarizeDocuments(cohort string, documents []documentResult) summaryResult {
	result := summaryResult{
		Cohort:        cohort,
		Documents:     len(documents),
		Compatibility: compatibilityMetric{Actions: map[string]int{}, Rules: map[string]int{}},
	}
	observed := map[string]bool{}
	for _, document := range documents {
		if document.DocumentSuccess {
			result.SuccessfulDocuments++
		}
		if document.CapabilityAdjustedSuccess {
			result.CapabilityAdjustedDocuments++
		}
		if document.DiscoveryComplete {
			result.DiscoveryComplete++
		}
		if document.Generation.Status == "pass" {
			result.GeneratedDocuments++
		}
		if document.Typecheck.Status == "pass" {
			result.TypecheckedDocuments++
		}
		if document.OperationRetention.Available {
			result.Operations.AvailableDocuments++
			result.Operations.Total += document.OperationRetention.Total
			result.Operations.Retained += document.OperationRetention.Retained
			result.Operations.Omitted += document.OperationRetention.Omitted
		}
		if document.OperationEmission != nil {
			if result.OperationEmission == nil {
				result.OperationEmission = &aggregateEmission{}
			}
			if document.OperationEmission.Available {
				result.OperationEmission.AvailableDocuments++
				result.OperationEmission.Count += document.OperationEmission.Count
			}
			result.OperationEmission.OperationOmissions += document.OperationEmission.OperationOmissions
			result.OperationEmission.HelperOmissions += document.OperationEmission.HelperOmissions
		}
		result.Compatibility.Preserved += document.Compatibility.Preserved
		result.Compatibility.Rejected += document.Compatibility.Rejected
		result.Compatibility.Total += document.Compatibility.Total
		for action, count := range document.Compatibility.Actions {
			result.Compatibility.Actions[action] += count
		}
		for rule, count := range document.Compatibility.Rules {
			result.Compatibility.Rules[rule] += count
		}
		for _, feature := range document.Features {
			observed[feature] = true
		}
	}
	result.DocumentSuccessRate = ratio(result.SuccessfulDocuments, result.Documents)
	result.CapabilityAdjustedSuccessRate = ratio(result.CapabilityAdjustedDocuments, result.Documents)
	result.GeneratedVerificationRate = ratio(result.TypecheckedDocuments, result.GeneratedDocuments)
	result.Operations.Rate = ratio(result.Operations.Retained, result.Operations.Total)
	result.Compatibility.Rate = ratio(result.Compatibility.Preserved, result.Compatibility.Total)
	catalog := featureCatalog()
	for _, feature := range catalog {
		if !observed[feature] {
			result.FeatureCoverage.Missing = append(result.FeatureCoverage.Missing, feature)
		}
	}
	result.FeatureCoverage.Observed = len(catalog) - len(result.FeatureCoverage.Missing)
	result.FeatureCoverage.Total = len(catalog)
	result.FeatureCoverage.Rate = ratio(result.FeatureCoverage.Observed, result.FeatureCoverage.Total)
	return result
}
