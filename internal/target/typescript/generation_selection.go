package typescript

import (
	"encoding/json"
	"fmt"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
)

// generationSelection separates public entry points from Link execution roots.
// It belongs to the target plan; compiler IR and source metadata stay immutable.
type generationSelection struct {
	direct       map[string]bool
	dependencies map[string]bool
}

func selectGenerationDocument(document *ir.Document, selection *generator.Selection, includeMetadata bool) (*ir.Document, *generationSelection, []diagnostic.Diagnostic, error) {
	if selection == nil {
		return document, nil, nil, nil
	}
	canonical, err := selection.Canonical()
	if err != nil {
		return document, nil, []diagnostic.Diagnostic{selectionDiagnostic("", err.Error())}, nil
	}
	ownership := newSourceOwnershipIndex(document)
	identities := operationIdentityDiagnostics(document, ownership)
	if diagnostic.HasErrors(identities) {
		return document, nil, identities, nil
	}
	ids := make(map[string]ir.Operation)
	routes := make(map[string]ir.Operation)
	for _, operation := range document.Operations {
		routes[operationRouteKey(operation)] = operation
		if operation.OperationID != "" {
			ids[operation.OperationID] = operation
		}
	}
	chosen := &generationSelection{direct: make(map[string]bool), dependencies: make(map[string]bool)}
	var diagnostics []diagnostic.Diagnostic
	choose := func(value string, operation ir.Operation, exists bool) {
		if !exists {
			diagnostics = append(diagnostics, selectionDiagnostic(value, fmt.Sprintf("Selection %q does not name an OpenAPI operation.", value)))
			return
		}
		visibility := operationStringExtension(operation, "x-sdk-visibility", operation.Extensions.Visibility)
		if operation.Visibility == "hidden" || visibility.Value == "hidden" {
			diagnostics = append(diagnostics, selectionDiagnostic(value, fmt.Sprintf("Selection %q names a hidden operation.", value)))
			return
		}
		chosen.direct[operationRouteKey(operation)] = true
	}
	for _, id := range canonical.Operations {
		operation, exists := ids[id]
		choose(id, operation, exists)
	}
	for _, route := range canonical.Routes {
		operation, exists := routes[route]
		choose(route, operation, exists)
	}
	if diagnostic.HasErrors(diagnostics) {
		return document, chosen, diagnostics, nil
	}
	retained := make(map[string]bool)
	queue := make([]ir.Operation, 0, len(chosen.direct))
	for _, operation := range document.Operations {
		if chosen.direct[operationRouteKey(operation)] {
			queue = append(queue, operation)
			retained[operationRouteKey(operation)] = true
		}
	}
	for index := 0; index < len(queue); index++ {
		operation := queue[index]
		responses, responseErr := operationResponses(document, operation)
		if responseErr != nil {
			continue
		} // The ordinary target analyzer owns this error.
		for _, response := range responses {
			linksPointer, pointerErr := componentObjectFieldPointer(document, response.SourceRaw, "responses", response.Pointer, "links")
			if pointerErr != nil {
				continue
			}
			links, _ := response.Raw["links"].(map[string]any)
			for _, name := range sortedAnyKeys(links) {
				pointer := linksPointer + "/" + escapePointerToken(name)
				if linkCapabilityRestricted(ownership.restrictionsAt(document, pointer)) {
					continue
				}
				link, _ := links[name].(map[string]any)
				refPointer, refErr := componentObjectFieldPointer(document, link, "links", pointer, "operationRef")
				resolved, resolveErr := resolveComponentObject(document, link, "links")
				if refErr != nil || resolveErr != nil {
					continue
				}
				target, targetErr := ownership.linkTarget(resolved, refPointer)
				if targetErr != nil {
					continue
				} // Existing W509 and trust-boundary semantics.
				route := operationRouteKey(target)
				if !retained[route] {
					retained[route] = true
					chosen.dependencies[route] = !chosen.direct[route]
					queue = append(queue, target)
				}
			}
		}
	}
	view := *document
	view.Operations = make([]ir.Operation, 0, len(retained))
	for _, operation := range document.Operations {
		if retained[operationRouteKey(operation)] {
			view.Operations = append(view.Operations, operation)
		}
	}
	// Raw remains the complete reference environment. Extension diagnostics are
	// scoped by effective operation ownership after preparation.
	// Only source-export consumers need a synthetic snapshot before callback
	// scoping narrows Raw. Default generation never serializes this fallback.
	if includeMetadata && len(view.SourceMetadataJSON) == 0 && document.Raw != nil {
		view.SourceMetadataJSON, err = json.Marshal(document.Raw)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("preserve selected source metadata: %w", err)
		}
	}
	return &view, chosen, nil, nil
}

func selectionDiagnostic(value, message string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{Severity: diagnostic.SeverityError, Code: "SDKGEN-E530", Phase: diagnostic.PhaseTarget, Target: "typescript", Capability: "generation selection", Message: message, Hint: "Use an exact operationId or METHOD and OpenAPI path template from the input document.", Operation: value}
}

func (selection *generationSelection) publicManifest(manifest Manifest) Manifest {
	if selection == nil {
		return manifest
	}
	result := manifest
	result.Operations = make([]ManifestOperation, 0, len(selection.direct))
	for _, operation := range manifest.Operations {
		if selection.direct[manifestRouteKey(operation)] {
			result.Operations = append(result.Operations, operation)
		}
	}
	return result
}

func (selection *generationSelection) publicLinks(links []generatedLink) []generatedLink {
	if selection == nil {
		return links
	}
	result := make([]generatedLink, 0, len(links))
	for _, link := range links {
		if selection.direct[operationRouteKey(link.SourceOperation)] {
			result = append(result, link)
		}
	}
	return result
}

func scopeSelectionDiagnostics(document *ir.Document, selection *generationSelection, values []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	if selection == nil {
		return values
	}
	ownership := newSourceOwnershipIndex(document)
	result := make([]diagnostic.Diagnostic, 0, len(values))
	for _, value := range values {
		operation, known := ownership.operationAtLocation(ir.SourceLocation{Source: value.Location.Source, Pointer: value.Location.Pointer})
		if !known {
			operation, known = ownership.operationAt(value.Location.Pointer)
		}
		if known && selection.dependencies[operationRouteKey(operation)] && strings.HasPrefix(value.Location.Pointer, operation.Pointer+"/callbacks") {
			continue
		}
		if known && !selection.direct[operationRouteKey(operation)] && !selection.dependencies[operationRouteKey(operation)] {
			continue
		}
		result = append(result, value)
	}
	return result
}

func selectedServerDocument(document *ir.Document, selection *generationSelection) *ir.Document {
	if selection == nil {
		return document
	}
	view := *document
	view.Operations = append([]ir.Operation(nil), document.Operations...)
	used := make(map[string]any)
	components, _ := document.Raw["components"].(map[string]any)
	callbacks, _ := components["callbacks"].(map[string]any)
	var includeCallback func(map[string]any)
	includeCallback = func(value map[string]any) {
		ref, _ := value["$ref"].(string)
		const prefix = "#/components/callbacks/"
		if !strings.HasPrefix(ref, prefix) {
			return
		}
		name := strings.ReplaceAll(strings.ReplaceAll(strings.TrimPrefix(ref, prefix), "~1", "/"), "~0", "~")
		if _, exists := used[name]; exists {
			return
		}
		if target, exists := callbacks[name]; exists {
			used[name] = target
			raw, _ := target.(map[string]any)
			includeCallback(raw)
		}
	}
	for index := range view.Operations {
		operation := &view.Operations[index]
		if !selection.direct[operationRouteKey(*operation)] {
			operation.Raw = copySelectionObject(operation.Raw)
			delete(operation.Raw, "callbacks")
			continue
		}
		values, _ := operation.Raw["callbacks"].(map[string]any)
		for _, value := range values {
			raw, _ := value.(map[string]any)
			includeCallback(raw)
		}
	}
	view.Raw = copySelectionObject(document.Raw)
	selectedComponents := copySelectionObject(components)
	selectedComponents["callbacks"] = used
	view.Raw["components"] = selectedComponents
	return &view
}

func copySelectionObject(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for name, item := range value {
		result[name] = item
	}
	return result
}
