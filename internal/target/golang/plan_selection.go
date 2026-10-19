package golang

import (
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/generator"
	httpplan "openapi-sdkgen/internal/http/plan"
	"strings"
)

// Direct entries and execution dependencies have separate ownership. A Link
// dependency can execute without adding a method to the public client.
type generationSelection struct {
	Direct, Retained map[string]bool
}

func selectOperations(document *ir.Document, selection *generator.Selection) (*generationSelection, []diagnostic.Diagnostic) {
	if selection == nil {
		return nil, nil
	}
	failure := func(value, message string) diagnostic.Diagnostic {
		return diagnostic.Diagnostic{Severity: diagnostic.SeverityError, Phase: diagnostic.PhaseTarget, Target: "golang", Code: "SDKGEN-E530", Capability: "generation selection", Operation: value, Message: message}
	}
	canonical, err := selection.Canonical()
	if err != nil {
		return nil, []diagnostic.Diagnostic{failure("", err.Error())}
	}
	ids, routes := make(map[string]ir.Operation), make(map[string]ir.Operation)
	visible := make(map[string]bool)
	for _, operation := range document.Operations {
		routes[operation.RouteKey] = operation
		if operation.OperationID != "" {
			ids[operation.OperationID] = operation
		}
		// Retain hidden targets for the ordinary visibility diagnostic; never
		// turn a forbidden Link into an unresolved-target warning by filtering.
		visible[operation.RouteKey] = true
	}
	result := &generationSelection{Direct: make(map[string]bool), Retained: make(map[string]bool)}
	var diagnostics []diagnostic.Diagnostic
	choose := func(value string, operation ir.Operation, exists bool) {
		if !exists {
			diagnostics = append(diagnostics, failure(value, fmt.Sprintf("Selection %q does not name an OpenAPI operation.", value)))
			return
		}
		if operation.Visibility == "hidden" {
			diagnostics = append(diagnostics, failure(value, fmt.Sprintf("Selection %q names a hidden operation.", value)))
			return
		}
		result.Direct[operation.RouteKey] = true
		result.Retained[operation.RouteKey] = true
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
		return result, diagnostics
	}
	links, _ := (&httpplan.LinkPlanner{Visible: visible}).Prepare(document)
	for changed := true; changed; {
		changed = false
		for _, link := range links {
			if result.Retained[link.SourceOperation.RouteKey] && !result.Retained[link.TargetOperation.RouteKey] {
				result.Retained[link.TargetOperation.RouteKey] = true
				changed = true
			}
		}
	}
	return result, nil
}

func (selection *generationSelection) view(document *ir.Document) *ir.Document {
	if selection == nil {
		return document
	}
	result := *document
	result.Operations = make([]ir.Operation, 0, len(selection.Retained))
	for _, operation := range document.Operations {
		if selection.Retained[operation.RouteKey] {
			result.Operations = append(result.Operations, operation)
		}
	}
	return &result
}

func (selection *generationSelection) diagnostics(document *ir.Document, values []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	if selection == nil {
		return values
	}
	ownership := httpplan.NewOwnershipIndex(document)
	var result []diagnostic.Diagnostic
	for _, value := range values {
		operation, known := ownership.OperationAtLocation(ir.SourceLocation{Source: value.Location.Source, Pointer: value.Location.Pointer})
		if !known {
			operation, known = ownership.OperationAt(value.Location.Pointer)
		}
		if known && !selection.Retained[operation.RouteKey] {
			continue
		}
		if known && !selection.Direct[operation.RouteKey] && strings.HasPrefix(value.Location.Pointer, operation.Pointer+"/callbacks") {
			continue
		}
		result = append(result, value)
	}
	return result
}
