package sdkgen

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"net/url"
	"os"
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
	SourceDecodes int
	Bundles       int
	ModelBuilds   int
	FileFilter    []string
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
	value, err := decodeInputValue(source.data, options.metrics)
	if err != nil {
		return nil, phaseError(diagnostic.PhaseDecode, fmt.Errorf("decode OpenAPI input: %w", err))
	}
	return compileInputValue(source, value, project, options)
}

func compileInputValue(source inputSource, value any, project bool, options CompileOptions) (*ir.Document, error) {
	data := source.data
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
	lockPath := options.RefLockPath
	if lockPath == "" && source.filePath != "" {
		lockPath = defaultReferenceLockPath(source.filePath)
	}
	if lockPath == "" && len(options.SchemaExtensionManifests) != 0 {
		return nil, phaseError(diagnostic.PhaseReferences, errors.New("schema extensions with URL or stdin input require --ref-lock"))
	}
	var lock *referenceLock
	shouldLoadLock := len(options.RemoteRefAllowlist) != 0 || len(options.SchemaExtensionManifests) != 0 || options.UpdateRefLock
	if source.filePath == "" && options.RefLockPath != "" {
		shouldLoadLock = true
	}
	if lockPath != "" && shouldLoadLock {
		// A remote allowlist alone does not imply a network request. Defer a
		// missing-lock failure until a remote document or extension is actually
		// used, so offline documents stay reproducible without empty lockfiles.
		var err error
		lock, err = loadReferenceLock(lockPath, true)
		if err != nil {
			return nil, phaseError(diagnostic.PhaseReferences, err)
		}
	}
	var remoteResolver *remoteReferenceResolver
	if len(options.RemoteRefAllowlist) != 0 || source.remoteBase != nil {
		cache := ""
		if lockPath != "" {
			cache = filepath.Join(filepath.Dir(lockPath), ".openapi-sdkgen-cache")
		}
		var err error
		remoteResolver, err = newRemoteReferenceResolver(options, lock, cache, source.remoteBase, source.httpConfig)
		if err != nil {
			return nil, phaseError(diagnostic.PhaseReferences, err)
		}
	}
	hasExternalReferences := externalReferenceCount(value) != 0
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
		attachDocumentProvenanceValue(document, source, document.Raw, nil)
		if lock != nil && options.UpdateRefLock {
			if err := writeReferenceLock(lockPath, lock); err != nil {
				return nil, phaseError(diagnostic.PhaseReferences, err)
			}
		}
		return document, nil
	}
	var fileFilter []string
	if !project && source.fileBase != "" {
		var err error
		fileFilter, err = validatedReferenceFileFilter(source, value, remoteResolver != nil)
		if err != nil {
			return nil, phaseError(diagnostic.PhaseReferences, err)
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
	if remoteResolver != nil {
		bundlerConfiguration.RemoteURLHandler = remoteResolver.handle
	}
	if options.metrics != nil {
		options.metrics.Bundles++
		options.metrics.ModelBuilds++
	}
	bundled, err := bundler.BundleBytesComposed(data, bundlerConfiguration, nil)
	if remoteResolver != nil {
		if remoteErr := remoteResolver.firstError(); remoteErr != nil {
			return nil, phaseError(diagnostic.PhaseReferences, fmt.Errorf("resolve OpenAPI references: %w", remoteErr))
		}
	}
	if err != nil {
		return nil, phaseError(diagnostic.PhaseReferences, fmt.Errorf("resolve OpenAPI references: %w", err))
	}
	var bundledValue any
	if err := yaml.Unmarshal(bundled, &bundledValue); err != nil {
		return nil, phaseError(diagnostic.PhaseNormalize, fmt.Errorf("decode bundled OpenAPI document: %w", err))
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
	attachDocumentProvenanceValue(document, source, value, remoteSources)
	if lock != nil && options.UpdateRefLock {
		if err := writeReferenceLock(lockPath, lock); err != nil {
			return nil, phaseError(diagnostic.PhaseReferences, err)
		}
	}
	return document, nil
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
				if name == "$ref" || referenceTraversalOpaque(path, name, item) {
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
	sourceObject, sourceIsObject := source.(map[string]any)
	bundledObject, bundledIsObject := bundled.(map[string]any)
	if !sourceIsObject || !bundledIsObject {
		return bundled
	}
	result := make(map[string]any, len(sourceObject)+len(bundledObject))
	for key, value := range bundledObject {
		result[key] = value
	}
	for key, sourceValue := range sourceObject {
		if key == "$ref" {
			// The bundled value is the resolved reference. Restoring the source
			// value would undo external reference resolution.
			continue
		}
		if bundledValue, exists := bundledObject[key]; exists {
			result[key] = mergeBundledDocument(sourceValue, bundledValue)
			continue
		}
		result[key] = sourceValue
	}
	return result
}

func rejectEscapingFileReferences(path, root string) error {
	return rejectEscapingFileReferencesWithRemote(path, root, false)
}

func rejectEscapingFileReferencesWithRemote(path, root string, allowRemote bool) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve OpenAPI input directory: %w", err)
	}
	return inspectReferenceFile(path, resolvedRoot, make(map[string]bool), allowRemote)
}

func rejectEscapingFileReferenceData(data []byte, root string, allowRemote bool) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve OpenAPI input directory: %w", err)
	}
	return inspectReferenceData(data, resolvedRoot, resolvedRoot, make(map[string]bool), allowRemote)
}

func validatedReferenceFileFilter(source inputSource, document any, allowRemote bool) ([]string, error) {
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
	if err := inspectReferenceValue(document, directory, root, visited, allowRemote); err != nil {
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

func inspectReferenceFile(path, root string, visited map[string]bool, allowRemote bool) error {
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("resolve OpenAPI reference file %s: %w", path, err)
	}
	if err := requireContainedPath(resolvedPath, root); err != nil {
		return err
	}
	if visited[resolvedPath] {
		return nil
	}
	visited[resolvedPath] = true
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return fmt.Errorf("read OpenAPI reference file %s: %w", resolvedPath, err)
	}
	return inspectReferenceData(data, filepath.Dir(resolvedPath), root, visited, allowRemote)
}

func inspectReferenceData(data []byte, directory, root string, visited map[string]bool, allowRemote bool) error {
	var document any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("inspect OpenAPI references: %w", err)
	}
	return inspectReferenceValue(document, directory, root, visited, allowRemote)
}

func inspectReferenceValue(document any, directory, root string, visited map[string]bool, allowRemote bool) error {
	var visit func(any, []string) error
	visit = func(value any, path []string) error {
		switch typed := value.(type) {
		case map[string]any:
			if reference, _ := typed["$ref"].(string); reference != "" {
				target, err := resolveContainedReference(reference, directory, root, allowRemote)
				if err != nil {
					return err
				}
				if target != "" {
					if err := inspectReferenceFile(target, root, visited, allowRemote); err != nil {
						return err
					}
				}
			}
			for name, item := range typed {
				if name == "$ref" || referenceTraversalOpaque(path, name, item) {
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
				if name == "$ref" || referenceTraversalOpaque(path, name, item) {
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

func externalReferenceCount(document any) int {
	var references []string
	collectExternalReferences(document, nil, &references)
	return len(references)
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
				if name == "$ref" || referenceTraversalOpaque(path, name, item) {
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
	model, err := compileValue(raw, source, true, CompileOptions{}, nil)
	if err == nil && source {
		attachDocumentProvenanceValue(model, inputSource{data: data, display: "in-memory OpenAPI document"}, model.Raw, nil)
	}
	return model, err
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
	model, err := ir.Build(document)
	if err != nil {
		if ir.IsReferenceError(err) {
			return nil, phaseError(diagnostic.PhaseReferences, err)
		}
		return nil, phaseError(diagnostic.PhaseIR, err)
	}
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
