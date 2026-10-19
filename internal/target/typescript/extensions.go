package typescript

import (
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/http/extension"
)

func validateVisibilityDependencies(document *ir.Document, omitted map[string]bool) []diagnostic.Diagnostic {
	byID := make(map[string]ir.Operation)
	byRoute := make(map[string]ir.Operation)
	byPathMethod := make(map[string]ir.Operation)
	for _, operation := range document.Operations {
		if operation.OperationID != "" {
			byID[operation.OperationID] = operation
		}
		byRoute[operationRouteKey(operation)] = operation
		byPathMethod[operationPathMethodKey(operation.Path, operation.Method)] = operation
	}
	var result []diagnostic.Diagnostic
	for _, source := range document.Operations {
		if source.Visibility == "hidden" || omitted[operationRouteKey(source)] {
			continue
		}
		responses, err := operationResponses(document, source)
		if err != nil {
			continue
		}
		for _, response := range responses {
			linksPointer, err := componentObjectFieldPointer(document, response.SourceRaw, "responses", response.Pointer, "links")
			if err != nil {
				continue
			}
			links, _ := response.Raw["links"].(map[string]any)
			for _, name := range sortedAnyKeys(links) {
				link, _ := links[name].(map[string]any)
				link, err = resolveComponentObject(document, link, "links")
				if err != nil {
					continue
				}
				target, exists := visibilityLinkTarget(byID, byRoute, byPathMethod, link)
				if !exists || target.Visibility != "hidden" || omitted[operationRouteKey(target)] {
					continue
				}
				pointer := linksPointer + "/" + escapePointerToken(name)
				location, related := extensionDiagnosticLocation(document, pointer)
				result = append(result, diagnostic.Diagnostic{
					Severity: diagnostic.SeverityError, Code: "SDKGEN-E621", Phase: diagnostic.PhaseTarget,
					Location: location, Related: related, Target: "typescript",
					Route: operationRouteKey(source), Operation: source.OperationID,
					Message: fmt.Sprintf("Visible response link %q targets hidden operation %s.", name, operationLabel(target)),
					Hint:    "Make the target internal/public, hide the source, or remove the link.",
				})
			}
		}
	}
	return result
}

func visibilityLinkTarget(byID, byRoute, byPathMethod map[string]ir.Operation, link map[string]any) (ir.Operation, bool) {
	if operationID, _ := link["operationId"].(string); operationID != "" {
		target, exists := byID[operationID]
		return target, exists
	}
	reference, _ := link["operationRef"].(string)
	if reference == "" {
		return ir.Operation{}, false
	}
	target, err := linkTargetOperation(byID, byPathMethod, link)
	if err != nil {
		return ir.Operation{}, false
	}
	if exact, exists := byRoute[operationRouteKey(target)]; exists {
		return exact, true
	}
	return target, true
}

func newExtensionPlanner() *extension.Planner {
	return &extension.Planner{
		Target: "typescript", Snapshot: snapshotIRDocument, Responses: operationResponses, ItemTypeSignature: paginationSchemaSignature,
		Parameters: func(document *ir.Document, operation ir.Operation) ([]ir.Parameter, error) {
			parameters, err := operationParameters(document, operation)
			if err != nil {
				return nil, err
			}
			result := make([]ir.Parameter, len(parameters))
			for index, parameter := range parameters {
				result[index] = ir.Parameter{Name: parameter.Name, Location: parameter.Location, Required: parameter.Required, ContentType: parameter.ContentType, Schema: parameter.Schema, Raw: parameter.Raw, Pointer: parameter.Pointer}
			}
			return result, nil
		},
	}
}
func prepareKnownExtensions(document *ir.Document) (*ir.Document, []diagnostic.Diagnostic, error) {
	return newExtensionPlanner().Prepare(document)
}
func operationStringExtension(operation ir.Operation, name string, compiled ir.StringExtension) ir.StringExtension {
	return newExtensionPlanner().StringExtension(operation, name, compiled)
}
func escapePointerToken(value string) string { return newExtensionPlanner().EscapePointerToken(value) }
func extensionDiagnosticLocation(document *ir.Document, pointer string) (diagnostic.Location, []diagnostic.Location) {
	return newExtensionPlanner().DiagnosticLocation(document, pointer)
}
func nestedWireCategory(document *ir.Document, schema map[string]any) (string, bool, bool) {
	return newExtensionPlanner().WireCategory(document, schema)
}

type recognizedErrorEnvelopeVariant = extension.ErrorEnvelopeVariant

func recognizedErrorEnvelopes(document *ir.Document, schema map[string]any) []recognizedErrorEnvelopeVariant {
	return newExtensionPlanner().ErrorEnvelopes(document, schema)
}
func paginationSchemaHasOnlyTypes(document *ir.Document, schema map[string]any, allowed map[string]bool) bool {
	return newExtensionPlanner().SchemaHasOnlyTypes(document, schema, allowed)
}
