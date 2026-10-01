package sdkgen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"openapi-sdkgen/internal/compiler/compatibility"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
)

const (
	pathParameterConformanceAnalyzer       = "openapi.path-parameter-conformance"
	securityRequirementConformanceAnalyzer = "openapi.security-requirement-conformance"
	operationIdentityConformanceAnalyzer   = "openapi.operation-identity-conformance"
)

type operationConformanceFinding struct {
	finding      compatibility.Finding
	ownerPointer string
	route        string
	operation    string
	related      []diagnostic.Location
}

func finalizeCompilerDocument(document *ir.Document, fallbackSource string, options *CompileOptions) error {
	if document == nil {
		return nil
	}
	findings := pathParameterConformanceFindings(document, fallbackSource)
	findings = append(findings, securityRequirementConformanceFindings(document, fallbackSource)...)
	findings = append(findings, operationIdentityConformanceFindings(document, fallbackSource)...)
	sort.Slice(findings, func(i, j int) bool {
		left := findings[i]
		right := findings[j]
		if left.finding.Source != right.finding.Source {
			return left.finding.Source < right.finding.Source
		}
		if left.finding.Pointer != right.finding.Pointer {
			return left.finding.Pointer < right.finding.Pointer
		}
		if left.finding.RuleID != right.finding.RuleID {
			return left.finding.RuleID < right.finding.RuleID
		}
		return left.ownerPointer < right.ownerPointer
	})
	for _, value := range findings {
		diagnosticValue := compatibilityDiagnostic(value.finding)
		diagnosticValue.Route = value.route
		diagnosticValue.Operation = value.operation
		diagnosticValue.Related = value.related
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

func operationIdentityConformanceFindings(document *ir.Document, fallbackSource string) []operationConformanceFinding {
	type occurrence struct {
		route    string
		location ir.SourceLocation
	}
	seen := make(map[string]occurrence)
	var result []operationConformanceFinding
	for _, operation := range document.Operations {
		if operation.OperationID == "" {
			continue
		}
		route := operationRouteIdentity(operation)
		location := conformanceLocation(document, fallbackSource, operation.Pointer+"/operationId")
		previous, exists := seen[operation.OperationID]
		if !exists {
			seen[operation.OperationID] = occurrence{route: route, location: location}
			continue
		}
		// Repeated resolution of the same effective operation is one occurrence.
		// A Path Item reused at another route still declares another operation.
		if previous.route == route && previous.location == location {
			continue
		}
		result = append(result, operationConformanceFinding{
			finding: compatibility.Finding{
				RuleID:      compatibility.RuleOperationIDUnique,
				Conformance: compatibility.ConformanceNonconforming,
				Disposition: compatibility.DispositionInvalid,
				Action:      compatibility.ActionReject,
				Impact:      compatibility.ImpactRouting,
				Scope:       failure.ScopeDocument,
				Effect:      failure.EffectBlock,
				Source:      location.Source,
				Pointer:     location.Pointer,
				Message:     fmt.Sprintf("operationId %q is duplicated; every declared operationId must be unique.", operation.OperationID),
			},
			route:     route,
			operation: operation.OperationID,
			related:   []diagnostic.Location{{Source: previous.location.Source, Pointer: previous.location.Pointer}},
		})
	}
	return result
}

func securityRequirementConformanceFindings(document *ir.Document, fallbackSource string) []operationConformanceFinding {
	if document == nil || (document.OpenAPIVersionLine != "3.0" && document.OpenAPIVersionLine != "3.1") {
		return nil
	}
	known, declarationSetKnown := declaredSecuritySchemeNames(document.Raw)
	if !declarationSetKnown {
		// A malformed shared securitySchemes container cannot safely support an
		// "undeclared" conclusion for one operation. Leave that shared shape at
		// the existing target/document-blocking boundary.
		return nil
	}
	var result []operationConformanceFinding
	appendRequirementFindings := func(
		requirements []ir.SecurityRequirement,
		basePointer string,
		scope failure.Scope,
		effect failure.Effect,
		ownerPointer, route, operationID string,
	) {
		for requirementIndex, requirement := range requirements {
			for _, scheme := range requirement.Schemes {
				if _, exists := known[scheme.Name]; exists {
					continue
				}
				pointer := basePointer + "/" + strconv.Itoa(requirementIndex) + "/" + escapeConformancePointerToken(scheme.Name)
				location := conformanceLocation(document, fallbackSource, pointer)
				result = append(result, operationConformanceFinding{
					finding: compatibility.Finding{
						RuleID:      compatibility.RuleSecurityRequirement,
						Conformance: compatibility.ConformanceNonconforming,
						Disposition: compatibility.DispositionInvalid,
						Action:      compatibility.ActionReject,
						Impact:      compatibility.ImpactSecurity,
						Scope:       scope,
						Effect:      effect,
						Source:      location.Source,
						Pointer:     location.Pointer,
						Message:     fmt.Sprintf("Security Requirement references undeclared Security Scheme %q.", scheme.Name),
					},
					ownerPointer: ownerPointer,
					route:        route,
					operation:    operationID,
				})
			}
		}
	}
	appendRequirementFindings(document.Security, "#/security", failure.ScopeDocument, failure.EffectBlock, "", "", "")
	for _, operation := range document.Operations {
		if !operation.SecurityDeclared {
			continue
		}
		appendRequirementFindings(
			operation.Security,
			operation.Pointer+"/security",
			failure.ScopeOperation,
			failure.EffectOmitOperation,
			operation.Pointer,
			operationRouteIdentity(operation),
			operation.OperationID,
		)
	}
	return result
}

func declaredSecuritySchemeNames(raw map[string]any) (map[string]struct{}, bool) {
	componentsValue, componentsExists := raw["components"]
	if !componentsExists {
		return map[string]struct{}{}, true
	}
	components, ok := componentsValue.(map[string]any)
	if !ok {
		return nil, false
	}
	value, exists := components["securitySchemes"]
	if !exists {
		return map[string]struct{}{}, true
	}
	schemes, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	result := make(map[string]struct{}, len(schemes))
	for name := range schemes {
		result[name] = struct{}{}
	}
	return result, true
}

func conformanceLocation(document *ir.Document, fallbackSource, pointer string) ir.SourceLocation {
	location := ir.SourceLocation{Source: fallbackSource, Pointer: pointer}
	if provenance, found := document.LookupProvenance(pointer); found {
		location = provenance.Primary
	}
	return location
}

func escapeConformancePointerToken(value string) string {
	value = strings.ReplaceAll(value, "~", "~0")
	return strings.ReplaceAll(value, "/", "~1")
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
