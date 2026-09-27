package sdkgen

import (
	"fmt"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/compatibility"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
)

const pathParameterConformanceAnalyzer = "openapi.path-parameter-conformance"

type operationConformanceFinding struct {
	finding      compatibility.Finding
	ownerPointer string
	route        string
	operation    string
}

func finalizeCompilerDocument(document *ir.Document, fallbackSource string, options *CompileOptions) error {
	if document == nil {
		return nil
	}
	findings := pathParameterConformanceFindings(document, fallbackSource)
	for _, value := range findings {
		diagnosticValue := compatibilityDiagnostic(value.finding)
		diagnosticValue.Route = value.route
		diagnosticValue.Operation = value.operation
		if options != nil && options.diagnostics != nil {
			options.diagnostics.Add(diagnosticValue)
		} else if diagnosticValue.Severity == diagnostic.SeverityError {
			return phaseError(
				diagnostic.PhaseOpenAPI,
				fmt.Errorf("%s at %s%s", diagnosticValue.Message, safeInputDisplay(diagnosticValue.Location.Source), diagnosticValue.Location.Pointer),
			)
		}
		if restriction, ok := semanticRestrictionForFinding(value.finding, value.ownerPointer); ok {
			document.SemanticRestrictions = append(document.SemanticRestrictions, restriction)
		}
	}
	sort.Slice(document.SemanticRestrictions, func(i, j int) bool {
		left := document.SemanticRestrictions[i]
		right := document.SemanticRestrictions[j]
		if left.OwnerPointer != right.OwnerPointer {
			return left.OwnerPointer < right.OwnerPointer
		}
		if left.Location.Source != right.Location.Source {
			return left.Location.Source < right.Location.Source
		}
		if left.Location.Pointer != right.Location.Pointer {
			return left.Location.Pointer < right.Location.Pointer
		}
		return left.RuleID < right.RuleID
	})
	return nil
}

func pathParameterConformanceFindings(document *ir.Document, fallbackSource string) []operationConformanceFinding {
	if document == nil {
		return nil
	}
	result := make([]operationConformanceFinding, 0)
	for _, operation := range document.Operations {
		expected := make(map[string]struct{}, len(operation.PathParameterOrder))
		for _, name := range operation.PathParameterOrder {
			expected[name] = struct{}{}
		}
		actual := make(map[string]struct{})
		for _, parameter := range operation.Parameters {
			if parameter.Location == "path" {
				actual[parameter.Name] = struct{}{}
			}
		}

		var missing []string
		for name := range expected {
			if _, exists := actual[name]; !exists {
				missing = append(missing, name)
			}
		}
		var unexpected []string
		for name := range actual {
			if _, exists := expected[name]; !exists {
				unexpected = append(unexpected, name)
			}
		}
		if len(missing) == 0 && len(unexpected) == 0 {
			continue
		}
		sort.Strings(missing)
		sort.Strings(unexpected)

		location := ir.SourceLocation{Source: fallbackSource, Pointer: operation.Pointer}
		if provenance, found := document.LookupProvenance(operation.Pointer); found {
			location = provenance.Primary
		}
		message := pathParameterConformanceMessage(missing, unexpected)
		result = append(result, operationConformanceFinding{
			finding: compatibility.Finding{
				RuleID:      compatibility.RulePathParameterBinding,
				Conformance: compatibility.ConformanceNonconforming,
				Disposition: compatibility.DispositionInvalid,
				Action:      compatibility.ActionReject,
				Impact:      compatibility.ImpactRouting,
				Scope:       failure.ScopeOperation,
				Effect:      failure.EffectOmitOperation,
				Source:      location.Source,
				Pointer:     location.Pointer,
				Message:     message,
			},
			ownerPointer: operation.Pointer,
			route:        operationRouteIdentity(operation),
			operation:    operation.OperationID,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ownerPointer != result[j].ownerPointer {
			return result[i].ownerPointer < result[j].ownerPointer
		}
		return result[i].finding.Message < result[j].finding.Message
	})
	return result
}

func pathParameterConformanceMessage(missing, unexpected []string) string {
	var details []string
	if len(missing) != 0 {
		details = append(details, "path template expressions without matching path Parameters: "+strings.Join(missing, ", "))
	}
	if len(unexpected) != 0 {
		details = append(details, "path Parameters without matching template expressions: "+strings.Join(unexpected, ", "))
	}
	return "The operation has a nonconforming path-template/Parameter contract (" + strings.Join(details, "; ") + ")."
}

func operationRouteIdentity(operation ir.Operation) string {
	if operation.RouteKey != "" {
		return strings.ToUpper(strings.TrimSpace(operation.Method)) + " " + operation.Path
	}
	return strings.ToUpper(strings.TrimSpace(operation.Method)) + " " + operation.Path
}
