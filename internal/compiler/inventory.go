package sdkgen

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	openapidoc "openapi-sdkgen/internal/compiler/openapi"
	"openapi-sdkgen/internal/diagnostic"
)

// Inventory describes the entry document's mounted paths, independently of SDK
// generation and schema compatibility analysis.
type Inventory struct {
	Title          string               `json:"title"`
	Version        string               `json:"version"`
	OpenAPIVersion string               `json:"openapiVersion"`
	Operations     []InventoryOperation `json:"operations"`
	DocumentsRead  int                  `json:"documentsRead"`
}

type InventoryOperation struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Route       string   `json:"route"`
	OperationID *string  `json:"operationId"`
	Tags        []string `json:"tags"`
	Summary     string   `json:"summary"`
	Deprecated  bool     `json:"deprecated"`
	Source      string   `json:"source"`
	Pointer     string   `json:"pointer"`
}

type InventoryResult struct {
	Inventory   *Inventory
	Diagnostics []diagnostic.Diagnostic
}

type inventorySource struct {
	input inputSource
	raw   map[string]any
}
type inventoryResolver struct {
	options   CompileOptions
	entry     inputSource
	root      string
	sources   map[string]inventorySource
	resolved  map[string]map[string]any
	resolving map[string]bool
	documents int
}

// InspectInputResult lists API declarations using the compiler's secure input,
// decoding, reference acquisition and global operation-identity checks. Only
// Path Item references are acquired; schema contracts remain opaque.
func InspectInputResult(input string, options CompileOptions) (InventoryResult, error) {
	collector := &diagnostic.Collector{}
	fail := func(err error, source string) (InventoryResult, error) {
		collector.Add(compileErrorDiagnostic(err, source))
		return InventoryResult{Diagnostics: diagnostic.Sort(collector.Diagnostics())}, nil
	}
	if options.UpdateRefLock {
		return fail(phaseError(diagnostic.PhaseInput, fmt.Errorf("inspect does not update reference locks")), input)
	}
	options.SchemaExtensionManifests = nil
	source, err := loadInputSource(input, options)
	if err != nil {
		return fail(phaseError(diagnostic.PhaseInput, err), safeInputDisplay(input))
	}
	decoded, err := decodeInputValue(source.data, options.metrics)
	if err != nil {
		return fail(phaseError(diagnostic.PhaseDecode, err), source.display)
	}
	raw, ok := decoded.(map[string]any)
	if !ok {
		return fail(phaseError(diagnostic.PhaseOpenAPI, fmt.Errorf("OpenAPI document must be an object")), source.display)
	}
	version, _ := raw["openapi"].(string)
	line, err := openapidoc.DetectVersionLine(version)
	if err != nil {
		return fail(phaseError(diagnostic.PhaseOpenAPI, err), source.display)
	}
	resolver := &inventoryResolver{options: options, entry: source, sources: map[string]inventorySource{}, resolved: map[string]map[string]any{}, resolving: map[string]bool{}, documents: 1}
	resolver.sources[source.effective] = inventorySource{source, raw}
	if source.fileBase != "" {
		resolver.root, err = filepath.EvalSymlinks(source.fileBase)
		if err != nil {
			return fail(phaseError(diagnostic.PhaseInput, err), source.display)
		}
		if source.filePath != "" {
			canonical, err := filepath.EvalSymlinks(source.filePath)
			if err != nil {
				return fail(phaseError(diagnostic.PhaseInput, err), source.display)
			}
			resolver.sources[canonical] = inventorySource{source, raw}
		}
	}
	info, _ := raw["info"].(map[string]any)
	result := &Inventory{Title: inventoryString(info, "title"), Version: inventoryString(info, "version"), OpenAPIVersion: version, Operations: []InventoryOperation{}}
	paths, exists := raw["paths"]
	if !exists {
		paths = map[string]any{}
	}
	pathMap, ok := paths.(map[string]any)
	if !ok {
		return fail(phaseError(diagnostic.PhaseOpenAPI, fmt.Errorf("OpenAPI paths must be an object")), source.display)
	}
	identityDocument := &ir.Document{}
	seenRoutes := map[string]bool{}
	for _, path := range sortedOpaqueMapKeys(pathMap) {
		if strings.HasPrefix(path, "x-") {
			continue
		}
		if !strings.HasPrefix(path, "/") {
			return fail(phaseError(diagnostic.PhaseOpenAPI, fmt.Errorf("path %q must start with /", path)), source.display)
		}
		item, ok := pathMap[path].(map[string]any)
		if !ok {
			return fail(phaseError(diagnostic.PhaseOpenAPI, fmt.Errorf("path item %q must be an object", path)), source.display)
		}
		item, err = resolver.resolve(inventorySource{source, raw}, item, 0)
		if err != nil {
			return fail(phaseError(diagnostic.PhaseReferences, err), source.display)
		}
		headers, err := ir.ReadOperationHeaders(path, item, line)
		if err != nil {
			return fail(phaseError(diagnostic.PhaseOpenAPI, err), source.display)
		}
		for _, header := range headers {
			operation, err := inventoryOperation(header.Method, path, header.Pointer, source.display, header.Raw)
			if err != nil {
				return fail(phaseError(diagnostic.PhaseOpenAPI, err), source.display)
			}
			if seenRoutes[operation.Route] {
				return fail(phaseError(diagnostic.PhaseOpenAPI, fmt.Errorf("duplicate route %q", operation.Route)), source.display)
			}
			seenRoutes[operation.Route] = true
			result.Operations = append(result.Operations, operation)
			identityDocument.Operations = append(identityDocument.Operations, ir.Operation{RouteKey: operation.Route, Method: operation.Method, Path: path, Pointer: header.Pointer, OperationID: inventoryString(header.Raw, "operationId")})
		}
	}
	for _, finding := range operationIdentityConformanceFindings(identityDocument, source.display) {
		value := compatibilityDiagnostic(finding.finding)
		value.Route, value.Operation, value.Related = finding.route, finding.operation, finding.related
		collector.Add(value)
	}
	if diagnostic.HasErrors(collector.Diagnostics()) {
		return InventoryResult{Diagnostics: diagnostic.Sort(collector.Diagnostics())}, nil
	}
	sort.Slice(result.Operations, func(i, j int) bool {
		a, b := result.Operations[i], result.Operations[j]
		if a.Path == b.Path {
			return a.Method < b.Method
		}
		return a.Path < b.Path
	})
	result.DocumentsRead = resolver.documents
	return InventoryResult{Inventory: result, Diagnostics: diagnostic.Sort(collector.Diagnostics())}, nil
}

func inventoryString(value map[string]any, key string) string {
	text, _ := value[key].(string)
	return text
}

func inventoryOperation(method, path, pointer, source string, raw map[string]any) (InventoryOperation, error) {
	result := InventoryOperation{Method: method, Path: path, Route: method + " " + path, Tags: []string{}, Source: safeInputDisplay(source), Pointer: pointer}
	for _, key := range []string{"operationId", "summary"} {
		value, exists := raw[key]
		if !exists {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return result, fmt.Errorf("%s at %s must be a string", key, pointer)
		}
		if key == "operationId" {
			if strings.TrimSpace(text) == "" {
				return result, fmt.Errorf("operationId at %s must not be blank", pointer)
			}
			result.OperationID = &text
		} else {
			result.Summary = text
		}
	}
	if value, exists := raw["deprecated"]; exists {
		flag, ok := value.(bool)
		if !ok {
			return result, fmt.Errorf("deprecated at %s must be a boolean", pointer)
		}
		result.Deprecated = flag
	}
	if value, exists := raw["tags"]; exists {
		tags, ok := value.([]any)
		if !ok {
			return result, fmt.Errorf("tags at %s must be an array", pointer)
		}
		for _, value := range tags {
			tag, ok := value.(string)
			if !ok {
				return result, fmt.Errorf("tag at %s must be a string", pointer)
			}
			result.Tags = append(result.Tags, tag)
		}
	}
	return result, nil
}

// InventoryFromDocument describes an already compiled entry without acquiring
// or decoding the input again during optional target analysis.
func InventoryFromDocument(document *ir.Document, source string) (*Inventory, error) {
	if document == nil {
		return nil, fmt.Errorf("compiler document is unavailable")
	}
	result := &Inventory{Title: document.Title, Version: document.ContractVersion, OpenAPIVersion: document.OpenAPIVersion, Operations: []InventoryOperation{}}
	for _, operation := range document.Operations {
		value, err := inventoryOperation(operation.Method, operation.Path, operation.Pointer, source, operation.Raw)
		if err != nil {
			return nil, err
		}
		result.Operations = append(result.Operations, value)
	}
	return result, nil
}

func (r *inventoryResolver) resolve(source inventorySource, item map[string]any, depth int) (map[string]any, error) {
	if depth > 256 {
		return nil, fmt.Errorf("Path Item reference depth exceeds 256")
	}
	value, exists := item["$ref"]
	if !exists {
		return item, nil
	}
	reference, ok := value.(string)
	if !ok || reference == "" {
		return nil, fmt.Errorf("Path Item $ref must be a nonempty string")
	}
	target, pointer, err := r.target(source, reference)
	if err != nil {
		return nil, err
	}
	key := target.input.effective + pointer
	if r.resolving[key] {
		return nil, fmt.Errorf("cyclic Path Item reference %q", reference)
	}
	resolved, ok := r.resolved[key]
	if !ok {
		r.resolving[key] = true
		resolved, err = ir.LookupPathItemReference(target.raw, pointer)
		if err == nil {
			resolved, err = r.resolve(target, resolved, depth+1)
		}
		delete(r.resolving, key)
		if err != nil {
			return nil, err
		}
		r.resolved[key] = resolved
	}
	return ir.MergePathItemReference(item, resolved, reference)
}

func (r *inventoryResolver) target(source inventorySource, reference string) (inventorySource, string, error) {
	parsed, err := url.Parse(reference)
	if err != nil {
		return inventorySource{}, "", err
	}
	pointer := "#" + parsed.EscapedFragment()
	parsed.Fragment, parsed.RawFragment = "", ""
	location := parsed.String()
	if location == "" {
		return source, pointer, nil
	}
	if source.input.remoteBase != nil || parsed.Scheme == "https" || parsed.Scheme == "http" {
		if source.input.remoteBase != nil {
			parsed = source.input.remoteBase.ResolveReference(parsed)
		}
		state, err := ensureReferenceResolutionState(r.entry, &r.options)
		if err != nil {
			return inventorySource{}, "", err
		}
		if state.remote == nil {
			return inventorySource{}, "", fmt.Errorf("remote Path Item references require an allowlisted origin")
		}
		key, err := state.remote.prefetch(parsed.String())
		if err != nil {
			return inventorySource{}, "", err
		}
		if cached, exists := r.sources[key]; exists {
			return cached, pointer, nil
		}
		data, _ := state.remote.source(key)
		decoded, err := state.remote.decodedSourceSnapshot(key, data)
		if err != nil {
			return inventorySource{}, "", err
		}
		raw, ok := decoded.value.(map[string]any)
		if !ok {
			return inventorySource{}, "", fmt.Errorf("Path Item document must be an object")
		}
		base, err := url.Parse(key)
		if err != nil {
			return inventorySource{}, "", err
		}
		target := inventorySource{inputSource{effective: key, display: safeInputDisplay(key), remoteBase: base}, raw}
		r.sources[key] = target
		r.documents++
		return target, pointer, nil
	}
	if r.root == "" || source.input.fileBase == "" {
		return inventorySource{}, "", fmt.Errorf("relative Path Item reference %q requires an input base", reference)
	}
	path, err := resolveContainedReference(location, source.input.fileBase, r.root, false)
	if err != nil {
		return inventorySource{}, "", err
	}
	if cached, exists := r.sources[path]; exists {
		return cached, pointer, nil
	}
	if r.options.sourceCache == nil {
		r.options.sourceCache = newDecodedSourceCache(r.options.metrics)
	}
	decoded, err := r.options.sourceCache.load(path)
	if err != nil {
		return inventorySource{}, "", err
	}
	raw, ok := decoded.value.(map[string]any)
	if !ok {
		return inventorySource{}, "", fmt.Errorf("Path Item document must be an object")
	}
	target := inventorySource{inputSource{effective: path, display: path, filePath: path, fileBase: filepath.Dir(path)}, raw}
	r.sources[path] = target
	r.documents++
	return target, pointer, nil
}
