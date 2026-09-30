package typescript

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/compatibility"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/failure"
)

func prepareTargetDiagnostics(plan *sourcePlan) []diagnostic.Diagnostic {
	document := plan.document
	var result []diagnostic.Diagnostic
	result = append(result, semanticOperationRestrictionDiagnostics(plan)...)
	result = append(result, methodCapabilityDiagnostics(plan)...)
	for _, feature := range unsupportedSchemasForTarget(document) {
		result = append(result, unsupportedFeatureDiagnostic(
			document,
			plan.ownership,
			feature,
			"SDKGEN-E501",
			"TypeScript cannot represent this Schema Object feature",
			"Remove the unsupported keyword or express the contract with supported OpenAPI and JSON Schema features.",
		))
	}
	if plan.includeServer {
		for _, feature := range unsupportedServerInboundSchemas(document) {
			result = append(result, unsupportedFeatureDiagnostic(
				document,
				plan.ownership,
				feature,
				"SDKGEN-E506",
				"The TypeScript server add-on cannot represent this inbound Schema Object feature",
				"Remove the unsupported keyword or express the inbound contract with supported Schema Object features.",
			))
		}
	}
	for _, feature := range unsupportedOpenAPIFeatures(document, plan.includeServer) {
		pointer, detail := splitUnsupportedFeature(feature)
		if !plan.includeServer && (detail == "generated inbound webhook contracts" || detail == "generated callback contracts") {
			kind := "callback"
			if detail == "generated inbound webhook contracts" {
				kind = "webhook"
			}
			value := sourceTargetDiagnostic(
				document,
				plan.ownership,
				pointer,
				"SDKGEN-E505",
				fmt.Sprintf("The OpenAPI feature %s requires the TypeScript server add-on for inbound %s contracts.", feature, kind),
				"Generate again with --with server, or remove the inbound contract.",
			)
			value.Scope = failure.ScopeDocument
			value.Effect = failure.EffectBlock
			result = append(result, value)
			continue
		}
		result = append(result, unsupportedFeatureDiagnostic(
			document,
			plan.ownership,
			feature,
			"SDKGEN-E502",
			"TypeScript cannot represent this OpenAPI feature",
			"Remove the unsupported construct or use a supported OpenAPI representation.",
		))
	}
	result = append(result, operationIdentityDiagnostics(document, plan.ownership)...)
	result = append(result, securityPreparationDiagnostics(plan)...)
	result = append(result, cookieSecurityOwnershipDiagnostics(plan)...)
	return diagnostic.Sort(result)
}

func semanticOperationRestrictionDiagnostics(plan *sourcePlan) []diagnostic.Diagnostic {
	if plan.omittedOperations == nil {
		plan.omittedOperations = make(map[string]bool)
	}
	if plan.resourceReservationExcluded == nil {
		plan.resourceReservationExcluded = make(map[string]bool)
	}
	var result []diagnostic.Diagnostic
	for _, restriction := range plan.document.SemanticRestrictions {
		if restriction.Scope != failure.ScopeOperation || restriction.Effect != failure.EffectOmitOperation {
			continue
		}
		var operation ir.Operation
		var found bool
		if restriction.OwnerPointer != "" {
			operation, found = plan.ownership.operationByExactPointer(restriction.OwnerPointer)
		} else {
			operation, found = plan.ownership.operationAtLocation(restriction.Location)
		}
		if !found {
			result = append(result, diagnostic.Diagnostic{
				Severity: diagnostic.SeverityError,
				Code:     "SDKGEN-E507",
				Phase:    diagnostic.PhaseTarget,
				Location: diagnostic.Location{Source: restriction.Location.Source, Pointer: restriction.Location.Pointer},
				Target:   "typescript",
				Scope:    failure.ScopeDocument,
				Effect:   failure.EffectBlock,
				Message:  "The TypeScript target cannot prove the owner of an operation-scoped semantic restriction.",
				Hint:     "Keep this construct blocking until its operation ownership can be resolved safely.",
			})
			continue
		}
		routeKey := operationRouteKey(operation)
		plan.omittedOperations[routeKey] = true
		if restriction.RuleID == compatibility.RulePathParameterBinding {
			plan.resourceReservationExcluded[routeKey] = true
		}
	}
	return result
}

func methodCapabilityDiagnostics(plan *sourcePlan) []diagnostic.Diagnostic {
	document := plan.document
	var result []diagnostic.Diagnostic
	for _, operation := range document.Operations {
		method := strings.ToUpper(strings.TrimSpace(operation.Method))
		pointer := operation.Pointer
		if pointer == "" {
			pointer = "#/paths/" + escapePointerToken(operation.Path) + "/" + strings.ToLower(operation.Method)
		}
		switch method {
		case "CONNECT", "TRACE", "TRACK":
			plan.omittedOperations[operationRouteKey(operation)] = true
			result = append(result, operationOmissionDiagnostic(
				document,
				plan.ownership,
				pointer,
				operation,
				fmt.Sprintf("The TypeScript Fetch target cannot issue %s requests.", method),
				"This operation is omitted for the TypeScript Fetch target.",
			))
			continue
		}
		if (method == "GET" || method == "HEAD") && requestBodyRequiresFetchPayload(operation.RequestBody) {
			plan.omittedOperations[operationRouteKey(operation)] = true
			result = append(result, operationOmissionDiagnostic(
				document,
				plan.ownership,
				pointer+"/requestBody",
				operation,
				fmt.Sprintf("The TypeScript Fetch target cannot send a request body with %s.", method),
				"This operation is omitted for the TypeScript Fetch target.",
			))
		}
	}
	return result
}

func operationOmissionDiagnostic(document *ir.Document, ownership *sourceOwnershipIndex, pointer string, operation ir.Operation, message, hint string) diagnostic.Diagnostic {
	value := sourceTargetDiagnostic(document, ownership, pointer, "SDKGEN-W511", message, hint)
	value.Severity = diagnostic.SeverityWarning
	value.Scope = failure.ScopeOperation
	value.Effect = failure.EffectOmitOperation
	value.Route = operationRouteKey(operation)
	value.Operation = operation.OperationID
	return value
}

func noMeaningfulEntrySurfaceDiagnostic(document *ir.Document, ownership *sourceOwnershipIndex) diagnostic.Diagnostic {
	value := sourceTargetDiagnostic(
		document,
		ownership,
		"#",
		"SDKGEN-E512",
		"The TypeScript target has no meaningful requested entry surface after scoped omissions.",
		"Keep at least one supported client operation, or request the server add-on with an independently useful inbound entry surface.",
	)
	value.Scope = failure.ScopeDocument
	value.Effect = failure.EffectBlock
	return value
}

func requestBodyRequiresFetchPayload(body *ir.RequestBody) bool {
	return body != nil && (body.Required || len(body.Content) != 0)
}

func unsupportedFeatureDiagnostic(document *ir.Document, ownership *sourceOwnershipIndex, feature, code, message, hint string) diagnostic.Diagnostic {
	pointer, detail := splitUnsupportedFeature(feature)
	if detail != "" {
		message += " at " + feature + "."
	} else {
		message += " at " + pointer + "."
	}
	return sourceTargetDiagnostic(document, ownership, pointer, code, message, hint)
}

func splitUnsupportedFeature(feature string) (string, string) {
	pointer := feature
	detail := ""
	if index := strings.Index(feature, " ("); index >= 0 && strings.HasSuffix(feature, ")") {
		pointer = feature[:index]
		detail = strings.TrimSuffix(feature[index+2:], ")")
	}
	return pointer, detail
}

func sourceTargetDiagnostic(document *ir.Document, ownership *sourceOwnershipIndex, pointer, code, message, hint string) diagnostic.Diagnostic {
	location, related := extensionDiagnosticLocation(document, pointer)
	value := diagnostic.Diagnostic{
		Severity: diagnostic.SeverityError,
		Code:     code,
		Phase:    diagnostic.PhaseTarget,
		Location: location,
		Related:  related,
		Target:   "typescript",
		Scope:    failure.ScopeDocument,
		Effect:   failure.EffectBlock,
		Message:  message,
		Hint:     hint,
	}
	if operation, ok := ownership.operationAt(pointer); ok {
		value.Route = operationRouteKey(operation)
		value.Operation = operation.OperationID
	}
	return value
}

func promoteResourceOmissionDiagnostics(values []diagnostic.Diagnostic) []diagnostic.Diagnostic {
	result := make([]diagnostic.Diagnostic, len(values))
	for index, value := range values {
		result[index] = value
		if value.Code == "SDKGEN-W513" {
			result[index].Severity = diagnostic.SeverityError
			result[index].Code = "SDKGEN-E513"
		}
	}
	return result
}

func resourceOmissionDiagnostics(document *ir.Document, ownership *sourceOwnershipIndex, omissions []resourceOmission) []diagnostic.Diagnostic {
	result := make([]diagnostic.Diagnostic, 0, len(omissions))
	for _, omission := range omissions {
		operation := omission.operation.compiled
		if operation.Method == "" {
			operation = findOperation(document, manifestRouteKey(omission.operation))
		}
		pointer := operation.Pointer
		if pointer == "" {
			pointer = "#/paths/" + escapePointerToken(operation.Path) + "/" + strings.ToLower(operation.Method)
		}
		if omission.hasParameterConflict {
			if schemaPointer := resourceParameterSchemaPointer(omission.parameter); schemaPointer != "" {
				pointer = schemaPointer
			}
		}
		message, hint := resourceOmissionDiagnosticText(document, omission)
		value := sourceTargetDiagnostic(document, ownership, pointer, "SDKGEN-W513", message, hint)
		value.Severity = diagnostic.SeverityWarning
		value.Route = operationRouteKey(operation)
		value.Operation = operation.OperationID
		value.Capability = "resource"
		value.Scope = failure.ScopeCapability
		value.Effect = failure.EffectOmitCapability
		if omission.hasParameterConflict {
			conflictingPointer := resourceParameterSchemaPointer(omission.conflictingParameter)
			if conflictingPointer == "" {
				conflictingPointer = omission.conflictingParameter.Pointer
			}
			if conflictingPointer == "" {
				result = append(result, value)
				continue
			}
			location, related := extensionDiagnosticLocation(document, conflictingPointer)
			if location != value.Location {
				value.Related = append(value.Related, location)
			}
			value.Related = append(value.Related, related...)
			value.Related = sortTargetLocations(value.Related)
		}
		result = append(result, value)
	}
	return diagnostic.Sort(result)
}

func resourceParameterSchemaPointer(parameter operationParameter) string {
	if parameter.Pointer == "" {
		return ""
	}
	if parameter.ContentType != "" {
		return parameter.Pointer + "/content/" + escapePointerToken(parameter.ContentType) + "/schema"
	}
	return parameter.Pointer + "/schema"
}

func resourceOmissionDiagnosticText(document *ir.Document, omission resourceOmission) (string, string) {
	selector := omission.selectorPath
	if selector == "" {
		selector = omission.operation.Path
	}
	operation := omission.operation.compiled
	if operation.Method == "" {
		operation = findOperation(document, manifestRouteKey(omission.operation))
	}
	label := operationRouteKey(operation)
	hint := "Use the exact $operations or $routes surface for this operation, or make the colliding resource contracts compatible."
	switch omission.reason {
	case resourceOmissionParameterConflict:
		details := resourceParameterConflictDetails(document, omission.parameter, omission.conflictingParameter)
		parameterDetail := "path parameter contracts differ"
		if omission.parameter.Name != "" && omission.conflictingParameter.Name != "" {
			if omission.parameter.Name == omission.conflictingParameter.Name {
				parameterDetail = fmt.Sprintf("path parameter %q contracts differ", omission.parameter.Name)
			} else {
				parameterDetail = fmt.Sprintf("path parameter %q conflicts with %q", omission.parameter.Name, omission.conflictingParameter.Name)
			}
		}
		if len(details) > 0 {
			parameterDetail += ": " + strings.Join(details, "; ")
		}
		return fmt.Sprintf("Resource selector %q is omitted for %s because %s.", selector, label, parameterDetail), hint
	case resourceOmissionDuplicatePathParameter:
		return fmt.Sprintf("Resource selector %q is omitted for %s because the OpenAPI path repeats one path parameter.", selector, label), hint
	case resourceOmissionUnsupportedPathSegment:
		return fmt.Sprintf("Resource selector %q is omitted for %s because one path segment cannot be represented as a resource member.", selector, label), hint
	case resourceOmissionRootPathParameter:
		return fmt.Sprintf("Resource selector %q is omitted for %s because a resource selector cannot begin with a path parameter.", selector, label), hint
	case resourceOmissionLiteralCollision:
		return fmt.Sprintf("Resource selector %q is omitted for %s because normalized literal resource members collide.", selector, label), hint
	case resourceOmissionOperationCollision:
		return fmt.Sprintf("Resource selector %q is omitted for %s because resource operation members collide.", selector, label), hint
	default:
		return fmt.Sprintf("Resource selector %q is omitted for %s because the resource member shape collides with another generated capability.", selector, label), hint
	}
}

func resourceParameterConflictDetails(document *ir.Document, left, right operationParameter) []string {
	if left.Name == "" || right.Name == "" {
		return nil
	}
	result := make([]string, 0, 8)
	leftSchema := resourceParameterCompatibilitySchema(document, left.Schema)
	rightSchema := resourceParameterCompatibilitySchema(document, right.Schema)
	collectResourceSchemaDifferences("schema", leftSchema, rightSchema, &result, 6)
	appendResourceParameterDifference(&result, "style", left.Style, right.Style)
	appendResourceParameterDifference(&result, "explode", left.Explode, right.Explode)
	appendResourceParameterDifference(&result, "allowReserved", left.AllowReserved, right.AllowReserved)
	appendResourceParameterDifference(&result, "allowEmptyValue", left.AllowEmptyValue, right.AllowEmptyValue)
	appendResourceParameterDifference(&result, "contentType", left.ContentType, right.ContentType)
	leftType, leftErr := schemaTypeForScope(document, leftSchema, projectionInput, typeRenderContract)
	rightType, rightErr := schemaTypeForScope(document, rightSchema, projectionInput, typeRenderContract)
	if leftErr == nil && rightErr == nil && leftType != rightType {
		result = append(result, "selectorType: "+resourceDifferenceValue(leftType)+" vs "+resourceDifferenceValue(rightType))
	}
	if len(result) == 0 {
		wire := newWireRenderContext(wirePropertiesLiteral)
		leftWire, leftWireErr := wire.wireSchemaDescriptorForDocument(document, leftSchema, projectionInput)
		rightWire, rightWireErr := wire.wireSchemaDescriptorForDocument(document, rightSchema, projectionInput)
		if leftWireErr == nil && rightWireErr == nil && leftWire != rightWire {
			result = append(result, "wire schema differs")
		}
	}
	return result
}

func appendResourceParameterDifference[T comparable](result *[]string, name string, left, right T) {
	if left == right {
		return
	}
	*result = append(*result, name+": "+resourceDifferenceValue(left)+" vs "+resourceDifferenceValue(right))
}

func collectResourceSchemaDifferences(path string, left, right any, result *[]string, limit int) {
	if len(*result) >= limit || reflect.DeepEqual(left, right) {
		return
	}
	leftMap, leftMapOK := left.(map[string]any)
	rightMap, rightMapOK := right.(map[string]any)
	if leftMapOK && rightMapOK {
		keys := make(map[string]bool, len(leftMap)+len(rightMap))
		for key := range leftMap {
			keys[key] = true
		}
		for key := range rightMap {
			keys[key] = true
		}
		sorted := make([]string, 0, len(keys))
		for key := range keys {
			sorted = append(sorted, key)
		}
		sort.Strings(sorted)
		for _, key := range sorted {
			if len(*result) >= limit {
				return
			}
			leftValue, leftExists := leftMap[key]
			rightValue, rightExists := rightMap[key]
			childPath := path + "." + key
			if !leftExists || !rightExists {
				if !leftExists {
					*result = append(*result, childPath+": <missing> vs "+resourceDifferenceValue(rightValue))
				} else {
					*result = append(*result, childPath+": "+resourceDifferenceValue(leftValue)+" vs <missing>")
				}
				continue
			}
			collectResourceSchemaDifferences(childPath, leftValue, rightValue, result, limit)
		}
		return
	}
	*result = append(*result, path+": "+resourceDifferenceValue(left)+" vs "+resourceDifferenceValue(right))
}

func resourceDifferenceValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%v", value)
	}
	const limit = 120
	if len(encoded) > limit {
		return string(encoded[:limit]) + "..."
	}
	return string(encoded)
}

type identityOccurrence struct {
	pointer string
}

func operationIdentityDiagnostics(document *ir.Document, ownership *sourceOwnershipIndex) []diagnostic.Diagnostic {
	seenRoutes := make(map[string]identityOccurrence, len(document.Operations))
	seenIDs := make(map[string]identityOccurrence, len(document.Operations))
	var result []diagnostic.Diagnostic
	for _, operation := range document.Operations {
		operationPointer := operation.Pointer
		if operationPointer == "" {
			operationPointer = "#/paths/" + escapePointerToken(operation.Path) + "/" + strings.ToLower(operation.Method)
		}
		routeKey := operationRouteKey(operation)
		if previous, exists := seenRoutes[routeKey]; exists {
			value := sourceTargetDiagnostic(
				document,
				ownership,
				operationPointer,
				"SDKGEN-E503",
				fmt.Sprintf("OpenAPI route identity %q is duplicated.", routeKey),
				"Keep exactly one operation for each exact HTTP method and OpenAPI path.",
			)
			previousLocation, _ := extensionDiagnosticLocation(document, previous.pointer)
			value.Related = append(value.Related, previousLocation)
			value.Related = sortTargetLocations(value.Related)
			result = append(result, value)
		} else {
			seenRoutes[routeKey] = identityOccurrence{pointer: operationPointer}
		}
		if operation.OperationID == "" {
			continue
		}
		idPointer := operationPointer + "/operationId"
		if previous, exists := seenIDs[operation.OperationID]; exists {
			value := sourceTargetDiagnostic(
				document,
				ownership,
				idPointer,
				"SDKGEN-E503",
				fmt.Sprintf("operationId %q is duplicated.", operation.OperationID),
				"Give every declared operationId an exact unique value, or omit it and use the exact route key.",
			)
			previousLocation, _ := extensionDiagnosticLocation(document, previous.pointer)
			value.Related = append(value.Related, previousLocation)
			value.Related = sortTargetLocations(value.Related)
			result = append(result, value)
		} else {
			seenIDs[operation.OperationID] = identityOccurrence{pointer: idPointer}
		}
	}
	return result
}

func securityPreparationDiagnostics(plan *sourcePlan) []diagnostic.Diagnostic {
	document := plan.document
	var result []diagnostic.Diagnostic
	for _, operation := range document.Operations {
		if plan.omittedOperations[operationRouteKey(operation)] {
			continue
		}
		if _, _, err := operationSecurityDefinition(document, operation); err != nil {
			pointer := operation.Pointer
			if pointer == "" {
				pointer = "#/paths/" + escapePointerToken(operation.Path) + "/" + strings.ToLower(operation.Method)
			}
			if _, declared := operation.Raw["security"]; declared {
				pointer += "/security"
			} else {
				pointer = "#/security"
			}
			value := sourceTargetDiagnostic(
				document,
				plan.ownership,
				pointer,
				"SDKGEN-E508",
				"Security requirements for this operation are invalid: "+strings.TrimSuffix(err.Error(), ".")+".",
				"Declare valid Security Requirement Objects that reference compatible component security schemes.",
			)
			value.Route = operationRouteKey(operation)
			value.Operation = operation.OperationID
			value.Scope = failure.ScopeDocument
			value.Effect = failure.EffectBlock
			result = append(result, value)
		}
	}
	return result
}

func cookieSecurityOwnershipDiagnostics(plan *sourcePlan) []diagnostic.Diagnostic {
	document := plan.document
	components, _ := document.Raw["components"].(map[string]any)
	schemes, _ := components["securitySchemes"].(map[string]any)
	var result []diagnostic.Diagnostic
	for _, operation := range document.Operations {
		if plan.omittedOperations[operationRouteKey(operation)] {
			continue
		}
		value, exists := operation.Raw["security"]
		if !exists {
			value, exists = document.Raw["security"]
		}
		if !exists {
			continue
		}
		requirements, ok := value.([]any)
		if !ok || len(requirements) == 0 {
			continue
		}
		cookieSchemes := make(map[string][]string)
		for _, item := range requirements {
			requirement, ok := item.(map[string]any)
			if !ok {
				continue
			}
			for _, schemeName := range sortedAnyKeys(requirement) {
				scheme, ok := schemes[schemeName].(map[string]any)
				if !ok {
					continue
				}
				kind, _ := scheme["type"].(string)
				location, _ := scheme["in"].(string)
				if kind != "apiKey" || location != "cookie" {
					continue
				}
				cookieName, _ := scheme["name"].(string)
				if cookieName == "" {
					continue
				}
				pointer := "#/components/securitySchemes/" + escapePointerToken(schemeName) + "/name"
				if !containsString(cookieSchemes[cookieName], pointer) {
					cookieSchemes[cookieName] = append(cookieSchemes[cookieName], pointer)
				}
			}
		}
		if len(cookieSchemes) == 0 {
			continue
		}
		parameters, err := operationParameters(document, operation)
		if err != nil {
			continue
		}
		for _, parameter := range parameters {
			relatedPointers := cookieSchemes[parameter.Name]
			if parameter.Location != "cookie" || len(relatedPointers) == 0 {
				continue
			}
			pointer := parameter.Pointer
			if pointer == "" {
				pointer = operation.Pointer + "/parameters"
			}
			value := sourceTargetDiagnostic(
				document,
				plan.ownership,
				pointer,
				"SDKGEN-E509",
				fmt.Sprintf("Cookie %q is declared as both an operation parameter and security credential.", parameter.Name),
				"Remove the duplicate cookie Parameter Object and let the OpenAPI security scheme own this credential.",
			)
			value.Route = operationRouteKey(operation)
			value.Operation = operation.OperationID
			for _, relatedPointer := range relatedPointers {
				location, _ := extensionDiagnosticLocation(document, relatedPointer)
				value.Related = append(value.Related, location)
			}
			value.Related = sortTargetLocations(value.Related)
			result = append(result, value)
		}
	}
	return result
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func serverPreparationDiagnostic(document *ir.Document, ownership *sourceOwnershipIndex, kind string, err error) diagnostic.Diagnostic {
	pointer, message := sourcePointerErrorDetails(err)
	if pointer == "" {
		pointer = "#"
	}
	value := sourceTargetDiagnostic(
		document,
		ownership,
		pointer,
		"SDKGEN-E506",
		fmt.Sprintf("The TypeScript server add-on cannot prepare %s: %s.", kind, strings.TrimSuffix(message, ".")),
		"Fix the inbound OpenAPI contract, or generate without --with server if inbound adapters are not required.",
	)
	value.Scope = failure.ScopeDocument
	value.Effect = failure.EffectBlock
	return value
}

func loweringPreparationDiagnostic(document *ir.Document, ownership *sourceOwnershipIndex, err error) diagnostic.Diagnostic {
	pointer := "#"
	route := ""
	operationID := ""
	for _, operation := range document.Operations {
		if !strings.Contains(err.Error(), operationLabel(operation)) {
			continue
		}
		pointer = operation.Pointer
		if pointer == "" {
			pointer = "#/paths/" + escapePointerToken(operation.Path) + "/" + strings.ToLower(operation.Method)
		}
		route = operationRouteKey(operation)
		operationID = operation.OperationID
		break
	}
	value := sourceTargetDiagnostic(
		document,
		ownership,
		pointer,
		"SDKGEN-E507",
		"The OpenAPI contract cannot be lowered into a TypeScript operation contract: "+strings.TrimSuffix(err.Error(), ".")+".",
		"Correct the referenced schema, parameter, response, or operation identity shown by this diagnostic.",
	)
	value.Route = route
	value.Operation = operationID
	return value
}

func linkPreparationDiagnostic(document *ir.Document, ownership *sourceOwnershipIndex, failureValue linkPreparationFailure) diagnostic.Diagnostic {
	pointer := failureValue.Pointer
	if pointer == "" {
		pointer = "#"
	}
	value := sourceTargetDiagnostic(
		document,
		ownership,
		pointer,
		"SDKGEN-E509",
		"The TypeScript target cannot prepare response link: "+strings.TrimSuffix(failureValue.Error(), ".")+".",
		"Correct the referenced operation, response, or Link Object shown by this diagnostic.",
	)
	value.Route = operationRouteKey(failureValue.SourceOperation)
	value.Operation = failureValue.SourceOperation.OperationID
	value.Capability = "response-link"
	value.Scope = failure.ScopeDocument
	value.Effect = failure.EffectBlock
	if !failureValue.Blocking {
		value.Severity = diagnostic.SeverityWarning
		value.Code = "SDKGEN-W509"
		value.Scope = failure.ScopeCapability
		value.Effect = failure.EffectOmitCapability
		value.Hint = "This Link helper is omitted; the base response operation remains generated."
	}
	return value
}

func helperPreparationDiagnostic(document *ir.Document, ownership *sourceOwnershipIndex, kind string, code string, err error) diagnostic.Diagnostic {
	pointer, message := sourcePointerErrorDetails(err)
	route := ""
	operationID := ""
	if pointer == "" {
		pointer = "#"
		message = err.Error()
		for _, operation := range document.Operations {
			if !strings.Contains(err.Error(), operationLabel(operation)) {
				continue
			}
			pointer = operation.Pointer
			if pointer == "" {
				pointer = "#/paths/" + escapePointerToken(operation.Path) + "/" + strings.ToLower(operation.Method)
			}
			route = operationRouteKey(operation)
			operationID = operation.OperationID
			break
		}
	}
	value := sourceTargetDiagnostic(
		document,
		ownership,
		pointer,
		code,
		fmt.Sprintf("The TypeScript target cannot prepare %s: %s.", kind, strings.TrimSuffix(message, ".")),
		"Correct the referenced operation, response, or helper definition shown by this diagnostic.",
	)
	if route != "" {
		value.Route = route
		value.Operation = operationID
	}
	value.Scope = failure.ScopeDocument
	value.Effect = failure.EffectBlock
	return value
}

func splitPointerError(message string) (string, string) {
	if !strings.HasPrefix(message, "#") {
		return "", message
	}
	end := len(message)
	for _, marker := range []string{": ", " must "} {
		if index := strings.Index(message, marker); index >= 0 && index < end {
			end = index
		}
	}
	if end == len(message) {
		return "", message
	}
	pointer := message[:end]
	rest := strings.TrimSpace(strings.TrimPrefix(message[end:], ":"))
	return pointer, rest
}

func sortTargetLocations(values []diagnostic.Location) []diagnostic.Location {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Source == values[j].Source {
			return values[i].Pointer < values[j].Pointer
		}
		return values[i].Source < values[j].Source
	})
	return values
}
