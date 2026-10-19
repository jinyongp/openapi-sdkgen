package golang

import (
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
	"openapi-sdkgen/internal/http/extension"
	httpplan "openapi-sdkgen/internal/http/plan"
	"strings"
)

func duplicateResourceParameter(path string) bool {
	seen := make(map[string]bool)
	for _, part := range httpplan.ResourcePathParts(path) {
		name, parameter, supported := httpplan.ResourcePathPart(part)
		if supported && parameter {
			if seen[name] {
				return true
			}
			seen[name] = true
		}
	}
	return false
}

func resourceOmissionReasons(operations []operationPlan, resources []resourceNodePlan) map[string]string {
	reachable := make(map[string]bool)
	for _, resource := range resources {
		for _, call := range resource.Calls {
			reachable[call.Operation.Route] = true
		}
	}
	result := make(map[string]string)
	for _, operation := range operations {
		if operation.DependencyOnly || reachable[operation.Route] {
			continue
		}
		if operation.Visibility != "" && operation.Visibility != "public" {
			result[operation.Route] = "visibility"
			continue
		}
		reason := "parameter-conflict"
		if duplicateResourceParameter(operation.Path) {
			reason = "duplicate-path-parameter"
		} else {
			for _, part := range httpplan.ResourcePathParts(operation.Path) {
				name, parameter, supported := httpplan.ResourcePathPart(part)
				if !supported {
					reason = "unsupported-path-segment"
					break
				}
				if parameter {
					found := false
					for _, candidate := range operation.Parameters {
						found = found || candidate.Location == "path" && candidate.Name == name
					}
					if !found {
						reason = "missing-path-parameter"
						break
					}
				}
			}
		}
		result[operation.Route] = reason
	}
	return result
}

func resourceDiagnostics(document *ir.Document, operations []operationPlan, resources []resourceNodePlan, strict bool) []diagnostic.Diagnostic {
	reasons := resourceOmissionReasons(operations, resources)
	sources := make(map[string]ir.Operation, len(document.Operations))
	for _, operation := range document.Operations {
		sources[operation.RouteKey] = operation
	}
	var result []diagnostic.Diagnostic
	for _, operation := range operations {
		reason := reasons[operation.Route]
		if reason == "" || reason == "visibility" {
			continue
		}
		severity, code := diagnostic.SeverityWarning, "SDKGEN-W513"
		if strict {
			severity, code = diagnostic.SeverityError, "SDKGEN-E513"
		}
		pointer := sources[operation.Route].Pointer
		if pointer == "" {
			pointer = "#/paths/" + pointerToken(operation.Path) + "/" + strings.ToLower(operation.Method)
		}
		location, related := (&extension.Planner{}).DiagnosticLocation(document, pointer)
		result = append(result, diagnostic.Diagnostic{Severity: severity, Code: code, Phase: diagnostic.PhaseTarget, Target: "golang", Route: operation.Route, Operation: operation.OperationID, Location: location, Related: related, Scope: failure.ScopeCapability, Effect: failure.EffectOmitCapability, Capability: "resource", Message: fmt.Sprintf("Resource API is omitted for %s: %s.", operation.Route, reason), Hint: "Use the operation or exact Routes entry for this operation."})
	}
	return diagnostic.Sort(result)
}
