package extension

import (
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"openapi-sdkgen/internal/openapiwalk"
	"sort"
	"strings"
)

var recognizedExtensionNames = map[string]bool{
	"x-envelope":       true,
	"x-error-category": true,
	"x-pagination":     true,
	"x-sdk-visibility": true,
	"x-sort":           true,
}

type extensionOccurrence struct {
	Name    string
	Pointer string
	Object  map[string]any
}

func (planner *Planner) Prepare(document *ir.Document) (*ir.Document, []diagnostic.Diagnostic, error) {
	prepared := planner.cloneDocumentForPreparation(document)
	consumed := make(map[string]bool)
	var diagnostics []diagnostic.Diagnostic
	occurrences := planner.collectRecognizedExtensionOccurrences(prepared.Raw)

	for index := range prepared.Operations {
		operation := &prepared.Operations[index]
		findings, err := planner.prepareOperationExtensions(prepared, operation, consumed)
		if err != nil {
			return nil, nil, err
		}
		diagnostics = append(diagnostics, findings...)
		values, err := planner.parameters(prepared, *operation)
		if err != nil {
			return nil, nil, err
		}
		for _, parameter := range values {
			if _, present := parameter.Raw["x-sort"]; !present {
				continue
			}
			pointer := parameter.Pointer + "/x-sort"
			consumed[pointer] = true
			plan, findings := planner.validateSortExtension(prepared, *operation, parameter)
			diagnostics = append(diagnostics, findings...)
			if plan != nil {
				if operation.SortParameters == nil {
					operation.SortParameters = make(map[string]ir.SortParameterPlan)
				}
				operation.SortParameters[parameter.Location+"\x00"+parameter.Name] = *plan
			}
		}
	}

	categoryDiagnostics := planner.prepareErrorCategories(prepared, consumed)
	diagnostics = append(diagnostics, categoryDiagnostics...)

	for _, occurrence := range occurrences {
		if consumed[occurrence.Pointer] {
			continue
		}
		location, related := planner.DiagnosticLocation(prepared, occurrence.Pointer)
		diagnostics = append(diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.SeverityError,
			Code:     "SDKGEN-E600",
			Phase:    diagnostic.PhaseTarget,
			Location: location,
			Related:  related,
			Target:   planner.Target,
			Message:  fmt.Sprintf("Recognized extension %q is declared at an unsupported location.", occurrence.Name),
			Hint:     planner.recognizedExtensionLocationHint(occurrence.Name),
		})
	}
	return prepared, diagnostic.Sort(diagnostics), nil
}

func (planner *Planner) extensionStringValue(object map[string]any, name string) string {
	value, _ := object[name].(string)
	return value
}

func (planner *Planner) cloneDocumentForPreparation(document *ir.Document) *ir.Document {
	prepared := planner.snapshot(document)
	prepared.ErrorCategories = make(map[string]string)
	prepared.ParameterSortPlans = make(map[string]ir.SortParameterPlan)
	for index := range prepared.Operations {
		operation := &prepared.Operations[index]
		operation.SortParameters = nil
		operation.Envelope = ""
		operation.Pagination = ""
		operation.PaginationPlan = nil
		operation.Visibility = ""
	}
	return prepared
}

func (planner *Planner) prepareOperationExtensions(document *ir.Document, operation *ir.Operation, consumed map[string]bool) ([]diagnostic.Diagnostic, error) {
	var result []diagnostic.Diagnostic
	envelope := planner.StringExtension(*operation, "x-envelope", operation.Extensions.Envelope)
	if envelope.Present {
		consumed[envelope.Pointer] = true
		switch {
		case !envelope.Valid:
			result = append(result, planner.operationExtensionDiagnostic(document, *operation, envelope.Pointer, "SDKGEN-E610", "x-envelope must be the string \"data\".", "Use x-envelope: data, or omit the extension for the complete response body."))
		case envelope.Value != "data":
			result = append(result, planner.operationExtensionDiagnostic(document, *operation, envelope.Pointer, "SDKGEN-E611", fmt.Sprintf("x-envelope value %q is not supported.", envelope.Value), "Use x-envelope: data, or omit the extension for baseline behavior."))
		default:
			findings := planner.validateEnvelopeRepresentations(document, *operation, envelope.Pointer)
			result = append(result, findings...)
			if len(findings) == 0 {
				operation.Envelope = "data"
			}
		}
	}

	visibility := planner.StringExtension(*operation, "x-sdk-visibility", operation.Extensions.Visibility)
	if visibility.Present {
		consumed[visibility.Pointer] = true
		switch {
		case !visibility.Valid:
			result = append(result, planner.operationExtensionDiagnostic(document, *operation, visibility.Pointer, "SDKGEN-E620", "x-sdk-visibility must be the string \"internal\" or \"hidden\".", "Use internal or hidden, or omit the extension for public visibility."))
		case visibility.Value == "public":
			location, related := planner.DiagnosticLocation(document, visibility.Pointer)
			result = append(result, diagnostic.Diagnostic{
				Severity:  diagnostic.SeverityWarning,
				Code:      "SDKGEN-W620",
				Phase:     diagnostic.PhaseTarget,
				Location:  location,
				Related:   related,
				Target:    planner.Target,
				Route:     operationRouteKey(*operation),
				Operation: operation.OperationID,
				Message:   "x-sdk-visibility: public is redundant.",
				Hint:      "Omit x-sdk-visibility; public is the default.",
			})
		case visibility.Value == "internal", visibility.Value == "hidden":
			operation.Visibility = visibility.Value
		default:
			result = append(result, planner.operationExtensionDiagnostic(document, *operation, visibility.Pointer, "SDKGEN-E620", fmt.Sprintf("x-sdk-visibility value %q is not supported.", visibility.Value), "Use internal or hidden, or omit the extension for public visibility."))
		}
	}

	pagination := planner.operationValueExtension(*operation, "x-pagination", operation.Extensions.Pagination)
	if pagination.Present {
		consumed[pagination.Pointer] = true
		plan, findings, err := planner.preparePaginationExtension(document, *operation, pagination)
		if err != nil {
			return nil, err
		}
		result = append(result, findings...)
		if plan != nil {
			operation.Pagination = plan.Mode
			operation.PaginationPlan = plan
		}
	}
	return result, nil
}

func (planner *Planner) StringExtension(operation ir.Operation, name string, compiled ir.StringExtension) ir.StringExtension {
	if compiled.Present {
		return compiled
	}
	raw, present := operation.Raw[name]
	value, valid := raw.(string)
	return ir.StringExtension{
		Present: present,
		Valid:   present && valid,
		Value:   value,
		Raw:     raw,
		Pointer: planner.operationExtensionPointer(operation, name),
	}
}

func (planner *Planner) operationValueExtension(operation ir.Operation, name string, compiled ir.ValueExtension) ir.ValueExtension {
	if compiled.Present {
		return compiled
	}
	raw, present := operation.Raw[name]
	return ir.ValueExtension{Present: present, Raw: raw, Pointer: planner.operationExtensionPointer(operation, name)}
}

func (planner *Planner) operationExtensionPointer(operation ir.Operation, name string) string {
	pointer := operation.Pointer
	if pointer == "" {
		pointer = "#/paths/" + planner.EscapePointerToken(operation.Path) + "/" + strings.ToLower(operation.Method)
	}
	return pointer + "/" + name
}

func (planner *Planner) operationExtensionDiagnostic(document *ir.Document, operation ir.Operation, pointer, code, message, hint string) diagnostic.Diagnostic {
	location, related := planner.DiagnosticLocation(document, pointer)
	return diagnostic.Diagnostic{
		Severity:  diagnostic.SeverityError,
		Code:      code,
		Phase:     diagnostic.PhaseTarget,
		Location:  location,
		Related:   related,
		Target:    planner.Target,
		Route:     operationRouteKey(operation),
		Operation: operation.OperationID,
		Message:   message,
		Hint:      hint,
	}
}

func (planner *Planner) validateEnvelopeRepresentations(document *ir.Document, operation ir.Operation, extensionPointer string) []diagnostic.Diagnostic {
	responses, err := planner.responses(document, operation)
	if err != nil {
		return []diagnostic.Diagnostic{planner.operationExtensionDiagnostic(document, operation, extensionPointer, "SDKGEN-E612", "x-envelope: data could not inspect successful response representations.", "Resolve response references or omit x-envelope.")}
	}
	bodyRepresentations := 0
	var incompatible []string
	for _, response := range reachableSuccessResponses(responses) {
		for _, media := range response.Content {
			bodyRepresentations++
			_, exists := media.Raw["schema"]
			schema, schemaIsObject := media.Schema.(map[string]any)
			resolvedSchema := planner.ResolveSchema(document, schema, make(map[string]bool))
			if !exists || !schemaIsObject || media.Schema == false || !planner.schemaCanDescribeObject(resolvedSchema) || isBinaryMediaForDocument(document, media.ContentType, schema) || isTextMedia(media.ContentType) || len(planner.EnvelopeSchema(document, schema, make(map[string]bool))) == 0 {
				incompatible = append(incompatible, response.Status+" "+media.ContentType)
			}
		}
	}
	if bodyRepresentations == 0 {
		return []diagnostic.Diagnostic{planner.operationExtensionDiagnostic(document, operation, extensionPointer, "SDKGEN-E612", "x-envelope: data requires at least one body-bearing successful response.", "Declare a successful object response with a data property, or omit x-envelope.")}
	}
	if len(incompatible) == 0 {
		return nil
	}
	sort.Strings(incompatible)
	return []diagnostic.Diagnostic{planner.operationExtensionDiagnostic(document, operation, extensionPointer, "SDKGEN-E612", "x-envelope: data is incompatible with successful representations: "+strings.Join(incompatible, ", ")+".", "Make every body-bearing successful representation an object with a declared data property, or omit x-envelope.")}
}

func (planner *Planner) schemaCanDescribeObject(schema map[string]any) bool {
	if planner.schemaHasType(schema, "object") {
		return true
	}
	if _, exists := schema["properties"]; exists && schema["type"] == nil {
		return true
	}
	for _, keyword := range []string{"allOf", "oneOf", "anyOf"} {
		if values, ok := schema[keyword].([]any); ok && len(values) > 0 {
			return true
		}
	}
	return false
}

func (planner *Planner) validateSortExtension(document *ir.Document, operation ir.Operation, parameter ir.Parameter) (*ir.SortParameterPlan, []diagnostic.Diagnostic) {
	pointer := parameter.Pointer + "/x-sort"
	fail := func(message, hint string) (*ir.SortParameterPlan, []diagnostic.Diagnostic) {
		return nil, []diagnostic.Diagnostic{planner.operationExtensionDiagnostic(document, operation, pointer, "SDKGEN-E630", message, hint)}
	}
	if parameter.Location != "query" {
		return fail("x-sort is only valid on a query Parameter Object.", "Move x-sort to the query array parameter it transforms.")
	}
	declaration, ok := parameter.Raw["x-sort"].(map[string]any)
	if !ok || len(declaration) != 1 || declaration["format"] != "field-direction" {
		return fail("x-sort must be exactly {\"format\":\"field-direction\"}.", "Use the documented field-direction declaration.")
	}
	schema, ok := parameter.Schema.(map[string]any)
	if !ok {
		return fail("x-sort requires a schema-based query parameter.", "Declare an array schema with string enum items.")
	}
	schema = planner.ResolveSchema(document, schema, make(map[string]bool))
	if !planner.schemaHasType(schema, "array") {
		return fail("x-sort requires an array parameter schema.", "Set type: array and declare string enum items.")
	}
	items, _ := schema["items"].(map[string]any)
	items = planner.ResolveSchema(document, items, make(map[string]bool))
	if !planner.schemaHasType(items, "string") {
		return fail("x-sort array items must have string type.", "Declare string items with exact field:direction enum values.")
	}
	values, ok := items["enum"].([]any)
	if !ok || len(values) == 0 {
		return fail("x-sort array items require a non-empty enum.", "Declare exact field:asc and field:desc wire values.")
	}
	seenWire := make(map[string]bool)
	plan := &ir.SortParameterPlan{}
	for _, raw := range values {
		wire, ok := raw.(string)
		if !ok || seenWire[wire] {
			return fail("x-sort enum values must be unique strings.", "Remove duplicate or non-string enum values.")
		}
		seenWire[wire] = true
		field, direction, found := strings.Cut(wire, ":")
		if !found || field == "" || strings.Contains(direction, ":") || (direction != "asc" && direction != "desc") {
			return fail(fmt.Sprintf("x-sort enum value %q is not field:asc or field:desc.", wire), "Use non-empty field names followed by :asc or :desc.")
		}
		plan.Values = append(plan.Values, ir.SortValue{Wire: wire, Field: field, Direction: direction})
	}
	return plan, nil
}

func (planner *Planner) schemaHasType(schema map[string]any, expected string) bool {
	switch value := schema["type"].(type) {
	case string:
		return value == expected
	case []any:
		for _, item := range value {
			if item == expected {
				return true
			}
		}
	}
	return false
}

func (planner *Planner) prepareErrorCategories(document *ir.Document, consumed map[string]bool) []diagnostic.Diagnostic {
	var result []diagnostic.Diagnostic
	codeCategories := make(map[string]map[string][]string)
	reachable := planner.ReachableErrorSchemas(document)
	for _, schemaName := range sortedAnyKeys(mapStringAny(document.ComponentSchemas)) {
		if !reachable[schemaName] {
			continue
		}
		schema := document.ComponentSchemas[schemaName]
		variants := planner.ErrorEnvelopes(document, schema)
		if len(variants) == 0 {
			continue
		}
		pointer := "#/components/schemas/" + planner.EscapePointerToken(schemaName) + "/x-error-category"
		rawCategory, extensionPresent := schema["x-error-category"]
		if extensionPresent {
			consumed[pointer] = true
		}
		static, staticValid := rawCategory.(string)
		if extensionPresent {
			if !staticValid || static == "" {
				result = append(result, planner.schemaExtensionDiagnostic(document, pointer, "SDKGEN-E640", "x-error-category must be a non-empty string.", "Use a non-empty static category, or omit the extension."))
			}
		}
		selectedCategories := make(map[string]bool)
		allVariantsRepeatStatic := extensionPresent && staticValid && static != ""
		nonExactWireCategory := false
		conflictingWireCategories := make(map[string]bool)
		for _, variant := range variants {
			category, categoryPresent, categoryExact := planner.WireCategory(document, variant.ErrorSchema)
			selected := ""
			if categoryExact {
				selected = category
			}
			if extensionPresent && staticValid && static != "" {
				switch {
				case categoryPresent && !categoryExact:
					nonExactWireCategory = true
					allVariantsRepeatStatic = false
				case categoryExact && static != category:
					conflictingWireCategories[category] = true
					allVariantsRepeatStatic = false
				case !categoryPresent:
					selected = static
					allVariantsRepeatStatic = false
				}
			} else {
				allVariantsRepeatStatic = false
			}
			if selected == "" {
				continue
			}
			selectedCategories[selected] = true
			for _, code := range variant.Codes {
				if codeCategories[code] == nil {
					codeCategories[code] = make(map[string][]string)
				}
				codeCategories[code][selected] = append(codeCategories[code][selected], schemaName)
			}
		}
		if nonExactWireCategory {
			result = append(result, planner.schemaExtensionDiagnostic(document, pointer, "SDKGEN-E641", "x-error-category cannot override an optional or non-exact wire error.category property.", "Make error.category required with a string const or one-value enum, or remove one declaration."))
		}
		if len(conflictingWireCategories) > 0 {
			categories := sortedStringKeys(conflictingWireCategories)
			result = append(result, planner.schemaExtensionDiagnostic(document, pointer, "SDKGEN-E642", fmt.Sprintf("x-error-category %q conflicts with wire categories %s.", static, strings.Join(quotedStrings(categories), ", ")), "Remove x-error-category and use the required wire categories."))
		}
		if allVariantsRepeatStatic {
			location, related := planner.DiagnosticLocation(document, pointer)
			result = append(result, diagnostic.Diagnostic{
				Severity: diagnostic.SeverityWarning, Code: "SDKGEN-W641", Phase: diagnostic.PhaseTarget,
				Location: location, Related: related, Target: planner.Target,
				Message: "x-error-category repeats the exact required wire error.category value.",
				Hint:    "Remove x-error-category; the wire schema is authoritative.",
			})
		}
		if len(selectedCategories) == 1 {
			for selected := range selectedCategories {
				document.ErrorCategories[schemaName] = selected
			}
		}
	}
	for code, categories := range codeCategories {
		if len(categories) < 2 {
			continue
		}
		names := sortedAnyKeys(mapStringAny(categories))
		result = append(result, diagnostic.Diagnostic{
			Severity: diagnostic.SeverityError, Code: "SDKGEN-E643", Phase: diagnostic.PhaseTarget,
			Location: diagnostic.Location{Source: planner.extensionRootSource(document), Pointer: "#/components/schemas"},
			Target:   planner.Target,
			Message:  fmt.Sprintf("Error code %q has conflicting exact categories: %s.", code, strings.Join(names, ", ")),
			Hint:     "Assign one exact category to each runtime error code.",
		})
	}
	return result
}

type ErrorEnvelopeVariant struct {
	Codes       []string
	ErrorSchema map[string]any
}

func (planner *Planner) ErrorEnvelopes(document *ir.Document, schema map[string]any) []ErrorEnvelopeVariant {
	var result []ErrorEnvelopeVariant
	for _, outerAlternative := range planner.errorSchemaAlternatives(document, schema, make(map[string]bool)) {
		outerSchema := planner.effectiveErrorObjectSchema(document, outerAlternative, make(map[string]bool))
		if !planner.stringListContains(outerSchema["required"], "error") {
			continue
		}
		properties, _ := outerSchema["properties"].(map[string]any)
		errorSchema, _ := properties["error"].(map[string]any)
		for _, errorAlternative := range planner.errorSchemaAlternatives(document, errorSchema, make(map[string]bool)) {
			effective := planner.effectiveErrorObjectSchema(document, errorAlternative, make(map[string]bool))
			if !planner.stringListContains(effective["required"], "code") {
				continue
			}
			errorProperties, _ := effective["properties"].(map[string]any)
			codeSchema, _ := errorProperties["code"].(map[string]any)
			codes := planner.exactStringValuesFromSchema(document, codeSchema)
			if len(codes) == 0 {
				continue
			}
			result = append(result, ErrorEnvelopeVariant{Codes: codes, ErrorSchema: effective})
		}
	}
	return result
}

func (planner *Planner) errorSchemaAlternatives(document *ir.Document, schema map[string]any, seen map[string]bool) []map[string]any {
	base := make(map[string]any)
	for key, value := range schema {
		switch key {
		case "$ref", "allOf", "oneOf", "anyOf":
			continue
		default:
			base[key] = value
		}
	}
	result := []map[string]any{base}
	if reference, _ := schema["$ref"].(string); reference != "" {
		if name, err := componentSchemaReferenceName(reference); err == nil && !seen[name] {
			nextSeen := planner.copyStringBoolMap(seen)
			nextSeen[name] = true
			result = planner.intersectErrorSchemaAlternatives(result, planner.errorSchemaAlternatives(document, document.ComponentSchemas[name], nextSeen))
		}
	}
	if variants, _ := schema["allOf"].([]any); len(variants) > 0 {
		for _, value := range variants {
			variant, _ := value.(map[string]any)
			result = planner.intersectErrorSchemaAlternatives(result, planner.errorSchemaAlternatives(document, variant, planner.copyStringBoolMap(seen)))
		}
	}
	for _, keyword := range []string{"oneOf", "anyOf"} {
		variants, _ := schema[keyword].([]any)
		if len(variants) == 0 {
			continue
		}
		var union []map[string]any
		for _, value := range variants {
			variant, _ := value.(map[string]any)
			union = append(union, planner.errorSchemaAlternatives(document, variant, planner.copyStringBoolMap(seen))...)
		}
		result = planner.intersectErrorSchemaAlternatives(result, union)
	}
	return result
}

func (planner *Planner) intersectErrorSchemaAlternatives(left, right []map[string]any) []map[string]any {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(left)*len(right))
	for _, leftSchema := range left {
		for _, rightSchema := range right {
			result = append(result, map[string]any{"allOf": []any{leftSchema, rightSchema}})
		}
	}
	return result
}

func (planner *Planner) exactStringValuesFromSchema(document *ir.Document, schema map[string]any) []string {
	values := make(map[string]bool)
	for _, alternative := range planner.errorSchemaAlternatives(document, schema, make(map[string]bool)) {
		exact, _ := planner.exactStringValuesFromIntersection(alternative)
		for _, value := range exact {
			values[value] = true
		}
	}
	return sortedStringKeys(values)
}

func (planner *Planner) exactStringValuesFromIntersection(schema map[string]any) ([]string, bool) {
	var selected map[string]bool
	constrained := false
	if values := planner.exactStringValues(schema); len(values) > 0 {
		constrained = true
		selected = make(map[string]bool, len(values))
		for _, value := range values {
			selected[value] = true
		}
	}
	variants, _ := schema["allOf"].([]any)
	for _, value := range variants {
		variant, _ := value.(map[string]any)
		values, variantConstrained := planner.exactStringValuesFromIntersection(variant)
		if !variantConstrained {
			continue
		}
		constrained = true
		if selected == nil {
			selected = make(map[string]bool, len(values))
			for _, exact := range values {
				selected[exact] = true
			}
			continue
		}
		intersection := make(map[string]bool)
		for _, exact := range values {
			if selected[exact] {
				intersection[exact] = true
			}
		}
		selected = intersection
	}
	return sortedStringKeys(selected), constrained
}

func (planner *Planner) effectiveErrorObjectSchema(document *ir.Document, schema map[string]any, seen map[string]bool) map[string]any {
	result := make(map[string]any)
	if reference, _ := schema["$ref"].(string); reference != "" {
		if name, err := componentSchemaReferenceName(reference); err == nil && !seen[name] {
			seen[name] = true
			planner.mergeErrorObjectSchema(result, planner.effectiveErrorObjectSchema(document, document.ComponentSchemas[name], seen))
		}
	}
	if variants, _ := schema["allOf"].([]any); len(variants) > 0 {
		for _, variant := range variants {
			item, _ := variant.(map[string]any)
			planner.mergeErrorObjectSchema(result, planner.effectiveErrorObjectSchema(document, item, planner.copyStringBoolMap(seen)))
		}
	}
	planner.mergeErrorObjectSchema(result, schema)
	return result
}

func (planner *Planner) mergeErrorObjectSchema(target, source map[string]any) {
	required := make(map[string]bool)
	for _, current := range []any{target["required"], source["required"]} {
		values, _ := current.([]any)
		for _, value := range values {
			if text, ok := value.(string); ok {
				required[text] = true
			}
		}
	}
	if len(required) > 0 {
		values := make([]any, 0, len(required))
		for _, name := range sortedAnyKeys(mapStringAny(required)) {
			values = append(values, name)
		}
		target["required"] = values
	}
	properties := make(map[string]any)
	if current, ok := target["properties"].(map[string]any); ok {
		for name, value := range current {
			properties[name] = value
		}
	}
	if current, ok := source["properties"].(map[string]any); ok {
		for name, value := range current {
			properties[name] = value
		}
	}
	if len(properties) > 0 {
		target["properties"] = properties
	}
	if value, exists := source["type"]; exists {
		target["type"] = value
	}
}

func (planner *Planner) WireCategory(document *ir.Document, errorSchema map[string]any) (string, bool, bool) {
	properties, _ := errorSchema["properties"].(map[string]any)
	categorySchema, present := properties["category"].(map[string]any)
	if !present {
		return "", false, false
	}
	categorySchema = planner.ResolveSchema(document, categorySchema, make(map[string]bool))
	values := planner.exactStringValues(categorySchema)
	required := planner.stringListContains(errorSchema["required"], "category")
	if required && len(values) == 1 && values[0] != "" {
		return values[0], true, true
	}
	return "", true, false
}

func (planner *Planner) exactStringValues(schema map[string]any) []string {
	if value, ok := schema["const"].(string); ok && value != "" {
		return []string{value}
	}
	values, ok := schema["enum"].([]any)
	if !ok || len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok || text == "" {
			return nil
		}
		result = append(result, text)
	}
	return result
}

func (planner *Planner) stringListContains(value any, expected string) bool {
	values, _ := value.([]any)
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func (planner *Planner) schemaExtensionDiagnostic(document *ir.Document, pointer, code, message, hint string) diagnostic.Diagnostic {
	location, related := planner.DiagnosticLocation(document, pointer)
	return diagnostic.Diagnostic{
		Severity: diagnostic.SeverityError, Code: code, Phase: diagnostic.PhaseTarget,
		Location: location, Related: related, Target: planner.Target, Message: message, Hint: hint,
	}
}

func (planner *Planner) collectRecognizedExtensionOccurrences(root map[string]any) []extensionOccurrence {
	var result []extensionOccurrence
	var visit func(any, []string)
	visit = func(value any, path []string) {
		switch typed := value.(type) {
		case map[string]any:
			namedMap := openapiwalk.IsNamedMap(path)
			for _, name := range sortedAnyKeys(typed) {
				child := typed[name]
				extensionKey := openapiwalk.IsExtensionKey(path, name)
				if extensionKey && recognizedExtensionNames[name] {
					result = append(result, extensionOccurrence{Name: name, Pointer: planner.pointerFromParts(append(path, name)), Object: typed})
					continue
				}
				if extensionKey {
					continue
				}
				if !namedMap && openapiwalk.IsOpaqueDataField(name, child) {
					continue
				}
				visit(child, append(path, name))
			}
		case []any:
			for index, child := range typed {
				visit(child, append(path, fmt.Sprintf("%d", index)))
			}
		}
	}
	visit(root, nil)
	sort.Slice(result, func(left, right int) bool {
		if result[left].Pointer == result[right].Pointer {
			return result[left].Name < result[right].Name
		}
		return result[left].Pointer < result[right].Pointer
	})
	return result
}

func (planner *Planner) pointerFromParts(parts []string) string {
	pointer := "#"
	for _, part := range parts {
		pointer += "/" + planner.EscapePointerToken(part)
	}
	return pointer
}

func (planner *Planner) EscapePointerToken(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func (planner *Planner) DiagnosticLocation(document *ir.Document, pointer string) (diagnostic.Location, []diagnostic.Location) {
	if provenance, exists := document.LookupProvenance(pointer); exists {
		related := make([]diagnostic.Location, 0, len(provenance.Related))
		for _, value := range provenance.Related {
			related = append(related, diagnostic.Location{Source: value.Source, Pointer: value.Pointer})
		}
		return diagnostic.Location{Source: provenance.Primary.Source, Pointer: provenance.Primary.Pointer}, related
	}
	return diagnostic.Location{Source: planner.extensionRootSource(document), Pointer: pointer}, nil
}

func (planner *Planner) extensionRootSource(document *ir.Document) string {
	if provenance, exists := document.LookupProvenance("#"); exists && provenance.Primary.Source != "" {
		return provenance.Primary.Source
	}
	return "OpenAPI document"
}

func (planner *Planner) recognizedExtensionLocationHint(name string) string {
	switch name {
	case "x-envelope", "x-pagination", "x-sdk-visibility":
		return "Declare it only on an ordinary Paths operation."
	case "x-sort":
		return "Declare it only on the query Parameter Object it transforms."
	case "x-error-category":
		return "Declare it only on a recognized outer error-envelope schema."
	default:
		return "Move or remove the extension declaration."
	}
}
