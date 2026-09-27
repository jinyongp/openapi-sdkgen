package sdkgen

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pb33f/libopenapi/bundler"
	"github.com/pb33f/libopenapi/datamodel"
	"go.yaml.in/yaml/v4"

	"openapi-sdkgen/internal/compiler/ir"
	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/openapiwalk"
)

type compilationMetrics struct {
	SourceDecodes                int
	ReferenceSourceDecodes       int
	RemoteReferenceSourceDecodes int
	Bundles                      int
	ModelBuilds                  int
	FileFilter                   []string
}

func Compile(data []byte) (*ir.Document, error) {
	return compile(data, true)
}

func CompileProject(data []byte) (*ir.Document, error) {
	return Compile(data)
}

func CompileFile(path string) (*ir.Document, error) {
	return CompileFileWithOptions(path, CompileOptions{})
}

// CompileFileWithOptions compiles an OpenAPI document using explicit opt-in
// reference and extension capabilities. It never fetches a remote reference
// unless RemoteRefAllowlist is populated.
func CompileFileWithOptions(path string, options CompileOptions) (*ir.Document, error) {
	options.DiagnosticMode = diagnostic.ModeFailFast
	if options.InputBase != "" || options.InputReader != nil {
		return nil, errors.New("CompileFileWithOptions does not accept stdin input options")
	}
	if err := validateNonHTTPInputOptions(options); err != nil {
		return nil, err
	}
	source, err := loadFileInput(path)
	if err != nil {
		return nil, err
	}
	return compileInput(source, false, options)
}

// CompileInputWithOptions compiles an OpenAPI document read from a path, file
// URL, HTTP(S) URL, or standard input (-).
func CompileInputWithOptions(input string, options CompileOptions) (*ir.Document, error) {
	options.DiagnosticMode = diagnostic.ModeFailFast
	source, err := loadInputSource(input, options)
	if err != nil {
		return nil, err
	}
	return compileInput(source, false, options)
}

func CompileProjectFile(path string) (*ir.Document, error) {
	return CompileFile(path)
}

func compileInput(source inputSource, project bool, options CompileOptions) (*ir.Document, error) {
	if options.sourceCache == nil {
		options.sourceCache = newDecodedSourceCache(options.metrics)
	}
	value, err := decodeInputValue(source.data, options.metrics)
	if err != nil {
		return nil, phaseError(diagnostic.PhaseDecode, fmt.Errorf("decode OpenAPI input: %w", err))
	}
	return compileInputValue(source, value, project, options)
}

func compileInputValue(source inputSource, value any, project bool, options CompileOptions) (*ir.Document, error) {
	sourceMetadata, err := sourceMetadataFromOwnedInput(source.data, value)
	if err != nil {
		return nil, phaseError(diagnostic.PhaseNormalize, err)
	}
	if options.sourceCache != nil && source.filePath != "" {
		if err := options.sourceCache.remember(source.filePath, decodedSource{data: source.data, value: value}); err != nil {
			return nil, phaseError(diagnostic.PhaseReferences, fmt.Errorf("snapshot OpenAPI entry source: %w", err))
		}
	}
	effective, changed, err := prepareCompatibilityValue(source.display, value, &options)
	if err != nil {
		return nil, phaseError(diagnostic.PhaseNormalize, err)
	}
	data := source.data
	if changed {
		data, err = json.Marshal(effective)
		if err != nil {
			return nil, phaseError(diagnostic.PhaseNormalize, fmt.Errorf("encode effective OpenAPI input: %w", err))
		}
	}
	return compilePreparedInputValue(source, sourceMetadata, data, effective, project, options)
}

func compilePreparedInputValue(source inputSource, sourceMetadata, data []byte, value any, project bool, options CompileOptions) (*ir.Document, error) {
	if findings := reservedExtensionDiagnosticsValue(value, source.display); len(findings) != 0 {
		location := diagnostic.NewSourceRegistry([]string{findings[0].Location.Source}).Display(findings[0].Location.Source)
		return nil, phaseError(diagnostic.PhaseOpenAPI, fmt.Errorf("%s at %s%s", findings[0].Message, location, findings[0].Location.Pointer))
	}
	if source.remoteBase != nil {
		var err error
		value, err = absolutizeRelativeRemoteReferencesValue(value, source.remoteBase)
		if err != nil {
			return nil, phaseError(diagnostic.PhaseNormalize, err)
		}
		data, err = json.Marshal(value)
		if err != nil {
			return nil, phaseError(diagnostic.PhaseNormalize, fmt.Errorf("normalize OpenAPI remote references: %w", err))
		}
	}
	registerRemoteReferenceContexts(options.compatibilitySession, value, "")
	if project && (len(options.RemoteRefAllowlist) != 0 || len(options.SchemaExtensionManifests) != 0 || options.UpdateRefLock || options.Offline || options.RefLockPath != "") {
		return nil, phaseError(diagnostic.PhaseInput, errors.New("project compilation does not support remote references or schema extensions"))
	}
	if project {
		if err := rejectProjectExternalReferencesValue(value); err != nil {
			return nil, phaseError(diagnostic.PhaseReferences, err)
		}
	}
	if source.stdin && source.fileBase == "" && source.remoteBase == nil && hasRelativeExternalReferenceValue(value) {
		return nil, phaseError(diagnostic.PhaseReferences, errors.New("standard input contains a relative $ref; pass --input-base with the source document location"))
	}
	referenceState, err := ensureReferenceResolutionState(source, &options)
	if err != nil {
		return nil, phaseError(diagnostic.PhaseReferences, err)
	}
	lockPath := referenceState.lockPath
	lock := referenceState.lock
	remoteResolver := referenceState.remote
	hasExternalReferences := hasExternalReference(value, nil)
	if !hasExternalReferences {
		// The structured result path already validates reserved keywords,
		// references, version features, and every generator-consumed shape. Avoid
		// duplicating its document tree solely for a libopenapi model build. Legacy
		// direct compiler APIs retain full model validation for compatibility.
		validateModel := options.diagnostics == nil
		document, err := compileValue(value, false, validateModel, options, lock)
		if err != nil {
			return nil, err
		}
		attachSourceMetadata(document, sourceMetadata)
		attachDocumentProvenanceValue(document, source, document.Raw, nil, options.sourceCache)
		if err := finalizeCompilerDocument(document, source.display, &options); err != nil {
			return nil, err
		}
		if lock != nil && options.UpdateRefLock && !compilerDiagnosticsBlocked(options.diagnostics) {
			if err := writeReferenceLock(lockPath, lock); err != nil {
				return nil, phaseError(diagnostic.PhaseReferences, err)
			}
		}
		return document, nil
	}
	bundleData, err := referenceState.opaqueReferences.data(value, data)
	if err != nil {
		return nil, phaseError(diagnostic.PhaseReferences, err)
	}

	var fileFilter []string
	if !project && source.fileBase != "" {
		var err error
		fileFilter, err = validatedReferenceFileFilter(source, value, remoteResolver != nil, options.sourceCache, options.compatibilitySession)
		if err != nil {
			return nil, phaseError(diagnostic.PhaseReferences, err)
		}
		if err := syncCompatibilityDiagnostics(&options); err != nil {
			return nil, err
		}
		if options.diagnostics != nil && compilerDiagnosticsBlocked(options.diagnostics) {
			return nil, nil
		}
		if options.metrics != nil {
			options.metrics.FileFilter = append([]string(nil), fileFilter...)
		}
	}
	bundlerConfiguration := &datamodel.DocumentConfiguration{
		BasePath:               source.fileBase,
		SpecFilePath:           source.filePath,
		AllowFileReferences:    source.fileBase != "",
		AllowRemoteReferences:  remoteResolver != nil,
		FileFilter:             fileFilter,
		SkipMetadataCollection: true,
	}
	if source.fileBase != "" && hasLocalReferenceDependencies(source, fileFilter) {
		localFS, err := newReferenceSourceFS(source.fileBase, fileFilter, options.sourceCache, options.compatibilitySession, referenceState.opaqueReferences)
		if err != nil {
			return nil, phaseError(diagnostic.PhaseReferences, err)
		}
		bundlerConfiguration.LocalFS = localFS
	}
	if remoteResolver != nil {
		bundlerConfiguration.RemoteURLHandler = remoteResolver.handle
	}
	if options.metrics != nil {
		options.metrics.Bundles++
		options.metrics.ModelBuilds++
	}
	bundled, err := bundler.BundleBytesComposed(bundleData, bundlerConfiguration, nil)
	if remoteResolver != nil {
		if remoteErr := remoteResolver.firstError(); remoteErr != nil {
			return nil, phaseError(diagnostic.PhaseReferences, fmt.Errorf("resolve OpenAPI references: %w", remoteErr))
		}
	}
	if err != nil {
		return nil, phaseError(diagnostic.PhaseReferences, fmt.Errorf("resolve OpenAPI references: %w", err))
	}
	if err := syncCompatibilityDiagnostics(&options); err != nil {
		return nil, err
	}
	if options.diagnostics != nil && compilerDiagnosticsBlocked(options.diagnostics) {
		return nil, nil
	}
	var bundledValue any
	if err := yaml.Unmarshal(bundled, &bundledValue); err != nil {
		return nil, phaseError(diagnostic.PhaseNormalize, fmt.Errorf("decode bundled OpenAPI document: %w", err))
	}
	bundledValue, err = referenceState.opaqueReferences.restore(bundledValue)
	if err != nil {
		return nil, phaseError(diagnostic.PhaseNormalize, fmt.Errorf("restore opaque OpenAPI reference data: %w", err))
	}
	merged := mergeBundledDocument(value, bundledValue)
	document, err := compileValue(merged, false, false, options, lock)
	if err != nil {
		return nil, err
	}
	var remoteSources map[string][]byte
	if remoteResolver != nil {
		remoteSources = remoteResolver.sourceSnapshot()
	}
	attachSourceMetadata(document, sourceMetadata)
	attachDocumentProvenanceValue(document, source, value, remoteSources, options.sourceCache)
	if err := finalizeCompilerDocument(document, source.display, &options); err != nil {
		return nil, err
	}
	if lock != nil && options.UpdateRefLock && !compilerDiagnosticsBlocked(options.diagnostics) {
		if err := writeReferenceLock(lockPath, lock); err != nil {
			return nil, phaseError(diagnostic.PhaseReferences, err)
		}
	}
	return document, nil
}

func compilerDiagnosticsBlocked(collector *diagnostic.Collector) bool {
	return collector != nil && diagnostic.HasErrors(collector.Diagnostics())
}

func absolutizeRelativeRemoteReferences(data []byte, base *url.URL) ([]byte, error) {
	var document any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode OpenAPI input for remote reference resolution: %w", err)
	}
	document, err := absolutizeRelativeRemoteReferencesValue(document, base)
	if err != nil {
		return nil, err
	}
	normalized, err := yaml.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("normalize OpenAPI remote references: %w", err)
	}
	return normalized, nil
}

func absolutizeRelativeRemoteReferencesValue(document any, base *url.URL) (any, error) {
	var visit func(any, []string) error
	visit = func(value any, path []string) error {
		switch typed := value.(type) {
		case map[string]any:
			if reference, _ := typed["$ref"].(string); reference != "" && !strings.HasPrefix(reference, "#") {
				parsed, err := url.Parse(reference)
				if err != nil {
					return fmt.Errorf("parse OpenAPI reference %q: %w", reference, err)
				}
				if !parsed.IsAbs() {
					typed["$ref"] = base.ResolveReference(parsed).String()
				}
			}
			for name, item := range typed {
				if name == "$ref" || openapiwalk.ReferenceChildOpaque(path, name, item) {
					continue
				}
				if err := visit(item, append(path, name)); err != nil {
					return err
				}
			}
		case []any:
			for index, item := range typed {
				if err := visit(item, append(path, fmt.Sprint(index))); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(document, nil); err != nil {
		return nil, err
	}
	return document, nil
}

// mergeBundledDocument keeps extensions and newly standardized OpenAPI fields
// that the bundler does not model yet, while retaining its resolved $ref output.
// This matters for an OpenAPI 3.2 document such as a Path Item's
// additionalOperations: it must reach the IR even when the CLI compiles a file.
func mergeBundledDocument(source, bundled any) any {
	return mergeBundledDocumentAt(source, bundled, nil, false)
}

func mergeBundledDocumentAt(source, bundled any, path []string, opaque bool) any {
	if opaque {
		return source
	}
	sourceObject, sourceIsObject := source.(map[string]any)
	bundledObject, bundledIsObject := bundled.(map[string]any)
	if sourceIsObject && bundledIsObject {
		result := make(map[string]any, len(sourceObject)+len(bundledObject))
		for key, value := range bundledObject {
			result[key] = value
		}
		for key, sourceValue := range sourceObject {
			if key == "$ref" {
				// The bundled value is the resolved reference. Restoring the
				// source value would undo active reference resolution.
				continue
			}
			childOpaque := openapiwalk.ReferenceChildOpaque(path, key, sourceValue)
			if childOpaque {
				result[key] = sourceValue
				continue
			}
			if bundledValue, exists := bundledObject[key]; exists {
				result[key] = mergeBundledDocumentAt(sourceValue, bundledValue, append(path, key), false)
				continue
			}
			result[key] = sourceValue
		}
		return result
	}

	sourceArray, sourceIsArray := source.([]any)
	bundledArray, bundledIsArray := bundled.([]any)
	if sourceIsArray && bundledIsArray && len(sourceArray) == len(bundledArray) {
		result := append([]any(nil), bundledArray...)
		for index := range sourceArray {
			result[index] = mergeBundledDocumentAt(
				sourceArray[index],
				bundledArray[index],
				append(path, strconv.Itoa(index)),
				false,
			)
		}
		return result
	}
	return bundled
}

func rejectEscapingFileReferences(path, root string) error {
	return rejectEscapingFileReferencesWithRemote(path, root, false)
}

func rejectEscapingFileReferencesWithRemote(path, root string, allowRemote bool) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve OpenAPI input directory: %w", err)
	}
	return inspectReferenceFile(path, resolvedRoot, make(map[string]bool), allowRemote, nil, nil, openapiwalk.ObjectUnknown)
}

func rejectEscapingFileReferenceData(data []byte, root string, allowRemote bool) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve OpenAPI input directory: %w", err)
	}
	return inspectReferenceData(data, resolvedRoot, resolvedRoot, make(map[string]bool), allowRemote)
}

func hasLocalReferenceDependencies(source inputSource, filters []string) bool {
	root := ""
	if source.filePath != "" {
		if relative, err := filepath.Rel(source.fileBase, source.filePath); err == nil {
			root = filepath.ToSlash(relative)
		}
	}
	for _, filter := range filters {
		if filter == ".openapi-sdkgen-no-local-references" || filter == root {
			continue
		}
		return true
	}
	return false
}

func validatedReferenceFileFilter(source inputSource, document any, allowRemote bool, cache *decodedSourceCache, session *compatibilitySession) ([]string, error) {
	root, err := filepath.EvalSymlinks(source.fileBase)
	if err != nil {
		return nil, fmt.Errorf("resolve OpenAPI input directory: %w", err)
	}
	directory := root
	visited := make(map[string]bool)
	if source.filePath != "" {
		resolved, err := filepath.EvalSymlinks(source.filePath)
		if err != nil {
			return nil, fmt.Errorf("resolve OpenAPI reference file %s: %w", source.filePath, err)
		}
		if err := requireContainedPath(resolved, root); err != nil {
			return nil, err
		}
		visited[resolved] = true
		directory = filepath.Dir(resolved)
	}
	if err := inspectReferenceValue(document, directory, root, visited, allowRemote, cache, session, openapiwalk.ObjectOpenAPI); err != nil {
		return nil, err
	}
	filters := make([]string, 0, len(visited))
	for path := range visited {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil, fmt.Errorf("make OpenAPI reference path relative: %w", err)
		}
		filters = append(filters, filepath.ToSlash(relative))
	}
	if len(filters) == 0 {
		// A non-empty impossible filter prevents libopenapi from recursively
		// indexing unrelated siblings when the document has remote refs only.
		filters = append(filters, ".openapi-sdkgen-no-local-references")
	}
	sort.Strings(filters)
	return filters, nil
}

func inspectReferenceFile(path, root string, visited map[string]bool, allowRemote bool, cache *decodedSourceCache, session *compatibilitySession, context openapiwalk.ObjectContext) error {
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve OpenAPI reference file %s: %w", path, err)
	}
	if err := requireContainedPath(resolvedPath, root); err != nil {
		return err
	}
	session.registerSourceContext(resolvedPath, context)
	if visited[resolvedPath] {
		return nil
	}
	visited[resolvedPath] = true
	source, err := cache.load(resolvedPath)
	if err != nil {
		return fmt.Errorf("read OpenAPI reference file %s: %w", resolvedPath, err)
	}
	effective, err := session.effectiveSource(resolvedPath, source, context)
	if err != nil {
		return err
	}
	return inspectReferenceValue(effective.value, filepath.Dir(resolvedPath), root, visited, allowRemote, cache, session, context)
}

func inspectReferenceData(data []byte, directory, root string, visited map[string]bool, allowRemote bool) error {
	var document any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("inspect OpenAPI references: %w", err)
	}
	return inspectReferenceValue(document, directory, root, visited, allowRemote, nil, nil, openapiwalk.ObjectOpenAPI)
}

func inspectReferenceValue(document any, directory, root string, visited map[string]bool, allowRemote bool, cache *decodedSourceCache, session *compatibilitySession, rootContext openapiwalk.ObjectContext) error {
	var visit func(any, []string) error
	visit = func(value any, path []string) error {
		switch typed := value.(type) {
		case map[string]any:
			if reference, _ := typed["$ref"].(string); reference != "" {
				context := openapiwalk.StructuralPositionAtRoot(rootContext, path).Object
				target, err := resolveContainedReference(reference, directory, root, allowRemote)
				if err != nil {
					return err
				}
				if target != "" {
					if err := inspectReferenceFile(target, root, visited, allowRemote, cache, session, context); err != nil {
						return err
					}
				} else if source := externalReferenceSource(reference); source != "" {
					session.registerSourceContext(source, context)
				}
			}
			for name, item := range typed {
				if name == "$ref" || openapiwalk.ReferenceChildOpaque(path, name, item) {
					continue
				}
				if err := visit(item, append(path, name)); err != nil {
					return err
				}
			}
		case []any:
			for index, item := range typed {
				if err := visit(item, append(path, fmt.Sprint(index))); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(document, nil)
}

func externalReferenceSource(reference string) string {
	source, _, _ := strings.Cut(reference, "#")
	if source == "" || !strings.Contains(source, "://") {
		return ""
	}
	return source
}

func hasRelativeExternalReference(data []byte) bool {
	var document any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return false
	}
	return hasRelativeExternalReferenceValue(document)
}

func hasRelativeExternalReferenceValue(document any) bool {
	var visit func(any, []string) bool
	visit = func(value any, path []string) bool {
		switch typed := value.(type) {
		case map[string]any:
			if reference, _ := typed["$ref"].(string); reference != "" && !strings.HasPrefix(reference, "#") {
				file, _, _ := strings.Cut(reference, "#")
				if file != "" && !strings.Contains(file, "://") && !strings.HasPrefix(file, "file:") && !filepath.IsAbs(file) {
					return true
				}
			}
			for name, item := range typed {
				if name == "$ref" || openapiwalk.ReferenceChildOpaque(path, name, item) {
					continue
				}
				if visit(item, append(path, name)) {
					return true
				}
			}
		case []any:
			for index, item := range typed {
				if visit(item, append(path, fmt.Sprint(index))) {
					return true
				}
			}
		}
		return false
	}
	return visit(document, nil)
}

func resolveContainedReference(reference, directory, root string, allowRemote bool) (string, error) {
	file, _, _ := strings.Cut(reference, "#")
	if file == "" {
		return "", nil
	}
	if allowRemote && (strings.HasPrefix(file, "https://") || strings.HasPrefix(file, "http://")) {
		return "", nil
	}
	if filepath.IsAbs(file) || strings.Contains(file, "://") || strings.HasPrefix(file, "file:") {
		return "", fmt.Errorf("OpenAPI reference %q must stay inside the input directory", reference)
	}
	candidate := filepath.Join(directory, filepath.FromSlash(file))
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve OpenAPI reference %q: %w", reference, err)
	}
	if err := requireContainedPath(resolved, root); err != nil {
		return "", fmt.Errorf("OpenAPI reference %q escapes the input directory: %w", reference, err)
	}
	return resolved, nil
}

func requireContainedPath(path, root string) error {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path %s escapes input directory", path)
	}
	return nil
}

func rejectProjectExternalReferences(data []byte) error {
	var root any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("inspect project references: %w", err)
	}
	return rejectProjectExternalReferencesValue(root)
}

func rejectProjectExternalReferencesValue(root any) error {
	var visit func(any, []string) error
	visit = func(value any, path []string) error {
		switch typed := value.(type) {
		case map[string]any:
			if reference, _ := typed["$ref"].(string); reference != "" && !strings.HasPrefix(reference, "#") {
				return fmt.Errorf("project OpenAPI artifacts must be self-contained; external reference %q is not allowed", reference)
			}
			for name, item := range typed {
				if name == "$ref" || openapiwalk.ReferenceChildOpaque(path, name, item) {
					continue
				}
				if err := visit(item, append(path, name)); err != nil {
					return err
				}
			}
		case []any:
			for index, item := range typed {
				if err := visit(item, append(path, fmt.Sprint(index))); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(root, nil)
}

func compile(data []byte, source bool) (*ir.Document, error) {
	raw, err := decodeInputValue(data, nil)
	if err != nil {
		return nil, phaseError(diagnostic.PhaseDecode, fmt.Errorf("decode OpenAPI document: %w", err))
	}
	sourceMetadata, err := encodeSourceMetadata(raw)
	if err != nil {
		return nil, phaseError(diagnostic.PhaseNormalize, err)
	}
	options := CompileOptions{}
	effective, _, err := prepareCompatibilityValue("in-memory OpenAPI document", raw, &options)
	if err != nil {
		return nil, phaseError(diagnostic.PhaseNormalize, err)
	}
	model, err := compileValue(effective, source, true, options, nil)
	if err != nil {
		return nil, err
	}
	attachSourceMetadata(model, sourceMetadata)
	if source {
		attachDocumentProvenanceValue(model, inputSource{data: data, display: "in-memory OpenAPI document"}, model.Raw, nil, nil)
	}
	if err := finalizeCompilerDocument(model, "in-memory OpenAPI document", &options); err != nil {
		return nil, err
	}
	return model, nil
}

func decodeInputValue(data []byte, metrics *compilationMetrics) (any, error) {
	if metrics != nil {
		metrics.SourceDecodes++
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) != 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		value, err := decodeJSONInputValue(data)
		if err == nil {
			return value, nil
		}
		if errors.Is(err, jsontext.ErrDuplicateName) {
			return nil, err
		}
		// YAML flow mappings can start with a brace. Fall through to the YAML
		// decoder when strict JSON decoding does not accept the input.
	}
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func decodeJSONInputValue(data []byte) (any, error) {
	var value any
	err := jsonv2.Unmarshal(data, &value, jsonv2.WithUnmarshalers(
		jsonv2.UnmarshalFromFunc(func(decoder *jsontext.Decoder, target *any) error {
			if decoder.PeekKind() == '0' {
				*target = jsontext.Value(nil)
			}
			return errors.ErrUnsupported
		}),
	))
	if err != nil {
		return nil, err
	}
	normalizeJSONNumbers(value)
	return value, nil
}

func normalizeJSONNumbers(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if number, ok := child.(jsontext.Value); ok {
				typed[key] = compatibleJSONNumber(string(number))
				continue
			}
			normalizeJSONNumbers(child)
		}
	case []any:
		for index, child := range typed {
			if number, ok := child.(jsontext.Value); ok {
				typed[index] = compatibleJSONNumber(string(number))
				continue
			}
			normalizeJSONNumbers(child)
		}
	}
}

func compatibleJSONNumber(value string) any {
	if integer, err := strconv.ParseInt(value, 10, 64); err == nil {
		if int64(int(integer)) == integer {
			return int(integer)
		}
		return integer
	}
	if unsigned, err := strconv.ParseUint(value, 10, 64); err == nil {
		return unsigned
	}
	if decimal, err := strconv.ParseFloat(value, 64); err == nil {
		return decimal
	}
	return value
}

func compileValue(raw any, source, validateModel bool, options CompileOptions, lock *referenceLock) (*ir.Document, error) {
	if source {
		if findings := reservedExtensionDiagnosticsValue(raw, "in-memory OpenAPI document"); len(findings) != 0 {
			location := diagnostic.NewSourceRegistry([]string{findings[0].Location.Source}).Display(findings[0].Location.Source)
			return nil, phaseError(diagnostic.PhaseOpenAPI, fmt.Errorf("%s at %s%s", findings[0].Message, location, findings[0].Location.Pointer))
		}
	}
	// libopenapi resolves `$ref` while it reads. JSON Schema anchors are valid
	// in OpenAPI 3.1/3.2 but are not component pointers, so normalize them
	// before the library's OpenAPI reference resolver sees the document.
	normalized, err := normalizeNestedSchemaReferences(raw)
	if err != nil {
		return nil, phaseError(diagnostic.PhaseNormalize, fmt.Errorf("normalize nested OpenAPI schema references: %w", err))
	}
	normalized, err = lowerSchemaExtensionsValue(normalized, options, lock)
	if err != nil {
		return nil, phaseError(diagnostic.PhaseNormalize, err)
	}
	normalizedDocument, ok := normalized.(map[string]any)
	if !ok {
		return nil, phaseError(diagnostic.PhaseNormalize, errors.New("normalized OpenAPI document must be an object"))
	}
	if err := validatePathItemReferences(normalizedDocument); err != nil {
		return nil, phaseError(diagnostic.PhaseReferences, err)
	}
	if validateModel && options.metrics != nil {
		options.metrics.ModelBuilds++
	}
	document, err := openapidoc.ReadParsed(normalizedDocument, validateModel)
	if err != nil {
		return nil, phaseError(diagnostic.PhaseOpenAPI, err)
	}
	if options.DiagnosticMode == diagnostic.ModeCollect {
		if findings := ir.ValidatePrerequisites(document); len(findings) != 0 {
			return nil, phaseError(diagnostic.PhaseIR, &irPrerequisiteValidationError{findings: findings})
		}
	}
	model, err := ir.Build(document)
	if err != nil {
		if ir.IsReferenceError(err) {
			return nil, phaseError(diagnostic.PhaseReferences, err)
		}
		return nil, phaseError(diagnostic.PhaseIR, err)
	}
	model.SemanticRestrictions = compatibilityRestrictions(options.compatibilitySession)
	return model, nil
}

func validatePathItemReferences(document map[string]any) error {
	paths, _ := document["paths"].(map[string]any)
	names := make([]string, 0, len(paths))
	for path := range paths {
		names = append(names, path)
	}
	sort.Strings(names)
	for _, path := range names {
		if openapiwalk.IsExtensionKey([]string{"paths"}, path) {
			continue
		}
		pathItem, ok := paths[path].(map[string]any)
		if !ok {
			continue
		}
		if _, hasReference := pathItem["$ref"]; !hasReference {
			continue
		}
		if _, err := ir.ResolvePathItem(document, pathItem); err != nil {
			return fmt.Errorf("path item %q: %w", path, err)
		}
	}
	return nil
}
