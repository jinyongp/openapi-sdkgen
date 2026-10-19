package extension

import (
	"encoding/json"
	"fmt"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/diagnostic"
	"sort"
	"strconv"
	"strings"
)

type paginationRepresentation struct {
	Label  string
	Schema map[string]any
}

// PaginationItemSchemas retains every validated representation. A language may
// need a union even when its public structural signature omits constraints.
func (planner *Planner) PaginationItemSchemas(document *ir.Document, operation ir.Operation, itemsPointer []string) ([]map[string]any, error) {
	representations, _, err := planner.paginationRepresentations(document, operation, operation.Extensions.Pagination.Pointer)
	if err != nil {
		return nil, err
	}
	var result []map[string]any
	seen := make(map[string]bool)
	for _, representation := range representations {
		array, found := planner.paginationSchemaAtPointer(document, representation.Schema, itemsPointer)
		if !found {
			continue
		}
		items, valid := planner.paginationArrayItemsSchema(document, array)
		if !valid {
			continue
		}
		encoded, err := json.Marshal(items)
		if err != nil {
			return nil, err
		}
		if !seen[string(encoded)] {
			seen[string(encoded)] = true
			result = append(result, items)
		}
	}
	return result, nil
}

type paginationRow struct {
	Name       string
	Items      []string
	NextCursor []string
	Offset     []string
	Limit      []string
	Total      []string
}

var canonicalPaginationRows = []paginationRow{
	{
		Name:       "root collection",
		Items:      []string{"items"},
		NextCursor: []string{"pagination", "nextCursor"},
		Offset:     []string{"pagination", "offset"},
		Limit:      []string{"pagination", "limit"},
		Total:      []string{"pagination", "total"},
	},
	{
		Name:       "nested collection",
		Items:      []string{"data", "items"},
		NextCursor: []string{"data", "pagination", "nextCursor"},
		Offset:     []string{"data", "pagination", "offset"},
		Limit:      []string{"data", "pagination", "limit"},
		Total:      []string{"data", "pagination", "total"},
	},
	{
		Name:       "data-array envelope",
		Items:      []string{"data"},
		NextCursor: []string{"meta", "pagination", "nextCursor"},
		Offset:     []string{"meta", "pagination", "offset"},
		Limit:      []string{"meta", "pagination", "limit"},
		Total:      []string{"meta", "pagination", "total"},
	},
}

func (planner *Planner) preparePaginationExtension(document *ir.Document, operation ir.Operation, extension ir.ValueExtension) (*ir.PaginationPlan, []diagnostic.Diagnostic, error) {
	fail := func(code, message, hint string) diagnostic.Diagnostic {
		return planner.operationExtensionDiagnostic(document, operation, extension.Pointer, code, message, hint)
	}
	mode := ""
	explicit := false
	request := ir.PaginationRequestPlan{}
	response := ir.PaginationResponsePlan{}
	var diagnostics []diagnostic.Diagnostic

	switch value := extension.Raw.(type) {
	case string:
		mode = value
		if mode != "cursor" && mode != "offset" && mode != "both" {
			diagnostics = append(diagnostics, fail("SDKGEN-E650", fmt.Sprintf("x-pagination value %q is not supported.", mode), "Use cursor, offset, both, or the documented explicit object form."))
		}
	case map[string]any:
		explicit = true
		var findings []diagnostic.Diagnostic
		mode, request, response, findings = planner.parseExplicitPagination(document, operation, extension.Pointer, value)
		diagnostics = append(diagnostics, findings...)
	default:
		diagnostics = append(diagnostics, fail("SDKGEN-E650", "x-pagination must be cursor, offset, both, or an explicit object.", "Use a documented string shorthand or explicit request/response mapping."))
	}
	if mode != "cursor" && mode != "offset" && mode != "both" {
		return nil, diagnostics, nil
	}

	parameters, err := planner.parameters(document, operation)
	if err != nil {
		return nil, nil, err
	}
	if !explicit {
		request = planner.shorthandPaginationRequest(mode)
	}
	diagnostics = append(diagnostics, planner.validatePaginationRequest(document, operation, extension.Pointer, mode, request, parameters, explicit)...)

	representations, representationDiagnostics, err := planner.paginationRepresentations(document, operation, extension.Pointer)
	if err != nil {
		return nil, nil, err
	}
	diagnostics = append(diagnostics, representationDiagnostics...)
	if len(representations) == 0 {
		if len(representationDiagnostics) == 0 {
			diagnostics = append(diagnostics, fail("SDKGEN-E652", "x-pagination requires at least one body-bearing successful JSON representation.", "Declare a successful JSON response body or remove x-pagination."))
		}
		return nil, diagnostic.Sort(diagnostics), nil
	}

	var itemSchema map[string]any
	if explicit {
		var findings []diagnostic.Diagnostic
		itemSchema, findings = planner.validatePaginationResponse(document, operation, extension.Pointer, mode, response, representations, true)
		diagnostics = append(diagnostics, findings...)
	} else {
		var findings []diagnostic.Diagnostic
		response, itemSchema, findings = planner.inferPaginationResponse(document, operation, extension.Pointer, mode, representations)
		diagnostics = append(diagnostics, findings...)
	}
	if len(diagnostics) > 0 {
		return nil, diagnostic.Sort(diagnostics), nil
	}
	return &ir.PaginationPlan{Mode: mode, Request: request, Response: response, ItemSchema: itemSchema}, nil, nil
}

func (planner *Planner) parseExplicitPagination(document *ir.Document, operation ir.Operation, pointer string, value map[string]any) (string, ir.PaginationRequestPlan, ir.PaginationResponsePlan, []diagnostic.Diagnostic) {
	var diagnostics []diagnostic.Diagnostic
	add := func(code, suffix, message, hint string) {
		diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, pointer+suffix, code, message, hint))
	}
	for key := range value {
		if key != "mode" && key != "request" && key != "response" {
			add("SDKGEN-E650", "/"+planner.EscapePointerToken(key), fmt.Sprintf("x-pagination contains unknown property %q.", key), "Use only mode, request, and response.")
		}
	}
	mode, _ := value["mode"].(string)
	if mode != "cursor" && mode != "offset" && mode != "both" {
		add("SDKGEN-E650", "/mode", "x-pagination.mode must be cursor, offset, or both.", "Choose one documented pagination mode.")
	}
	requestObject, requestOK := value["request"].(map[string]any)
	if !requestOK {
		add("SDKGEN-E651", "/request", "x-pagination.request must be an object.", "Map pagination controls to exact query parameter names.")
		requestObject = map[string]any{}
	}
	responseObject, responseOK := value["response"].(map[string]any)
	if !responseOK {
		add("SDKGEN-E653", "/response", "x-pagination.response must be an object.", "Map pagination values to decoded-body JSON Pointers.")
		responseObject = map[string]any{}
	}

	request := ir.PaginationRequestPlan{
		Cursor: planner.paginationStringProperty(requestObject, "cursor"),
		Offset: planner.paginationStringProperty(requestObject, "offset"),
		Limit:  planner.paginationStringProperty(requestObject, "limit"),
	}
	response := ir.PaginationResponsePlan{
		Items:      planner.paginationPointerProperty(document, operation, pointer+"/response/items", responseObject, "items", &diagnostics),
		NextCursor: planner.paginationPointerProperty(document, operation, pointer+"/response/nextCursor", responseObject, "nextCursor", &diagnostics),
		Offset:     planner.paginationPointerProperty(document, operation, pointer+"/response/offset", responseObject, "offset", &diagnostics),
		Limit:      planner.paginationPointerProperty(document, operation, pointer+"/response/limit", responseObject, "limit", &diagnostics),
		Total:      planner.paginationPointerProperty(document, operation, pointer+"/response/total", responseObject, "total", &diagnostics),
	}
	for key, raw := range requestObject {
		if key != "cursor" && key != "offset" && key != "limit" {
			add("SDKGEN-E651", "/request/"+planner.EscapePointerToken(key), fmt.Sprintf("x-pagination.request contains unknown property %q.", key), "Use only cursor, offset, and limit.")
			continue
		}
		if text, ok := raw.(string); !ok || text == "" {
			add("SDKGEN-E651", "/request/"+planner.EscapePointerToken(key), fmt.Sprintf("x-pagination.request.%s must be a non-empty query parameter name.", key), "Use the exact declared query parameter name.")
		}
	}
	for key := range responseObject {
		if key != "items" && key != "nextCursor" && key != "offset" && key != "limit" && key != "total" {
			add("SDKGEN-E653", "/response/"+planner.EscapePointerToken(key), fmt.Sprintf("x-pagination.response contains unknown property %q.", key), "Use only items, nextCursor, offset, limit, and total.")
		}
	}

	requireRequest := func(key, value string) {
		if value == "" {
			add("SDKGEN-E651", "/request/"+key, fmt.Sprintf("x-pagination.request.%s is required for %s mode.", key, mode), "Map it to an exact declared query parameter name.")
		}
	}
	requireResponse := func(key string, value []string) {
		if value == nil {
			add("SDKGEN-E653", "/response/"+key, fmt.Sprintf("x-pagination.response.%s is required for %s mode.", key, mode), "Declare an RFC 6901 JSON Pointer into the complete decoded response body.")
		}
	}
	if mode == "cursor" || mode == "both" {
		requireRequest("cursor", request.Cursor)
		requireResponse("nextCursor", response.NextCursor)
	}
	if mode == "offset" || mode == "both" {
		requireRequest("offset", request.Offset)
		requireRequest("limit", request.Limit)
	}
	requireResponse("items", response.Items)
	return mode, request, response, diagnostics
}

func (planner *Planner) paginationStringProperty(object map[string]any, name string) string {
	value, _ := object[name].(string)
	return value
}

func (planner *Planner) paginationPointerProperty(document *ir.Document, operation ir.Operation, pointer string, object map[string]any, name string, diagnostics *[]diagnostic.Diagnostic) []string {
	raw, present := object[name]
	if !present {
		return nil
	}
	value, ok := raw.(string)
	if !ok {
		*diagnostics = append(*diagnostics, planner.operationExtensionDiagnostic(document, operation, pointer, "SDKGEN-E653", fmt.Sprintf("x-pagination.response.%s must be an RFC 6901 JSON Pointer string.", name), "Use an empty root pointer or a slash-prefixed decoded-body pointer."))
		return nil
	}
	tokens, err := planner.parsePaginationPointer(value)
	if err != nil {
		*diagnostics = append(*diagnostics, planner.operationExtensionDiagnostic(document, operation, pointer, "SDKGEN-E653", fmt.Sprintf("x-pagination.response.%s is invalid: %v.", name, err), "Use RFC 6901 escapes ~0 and ~1 in a slash-prefixed pointer."))
		return nil
	}
	return tokens
}

func (planner *Planner) parsePaginationPointer(value string) ([]string, error) {
	if value == "" {
		return []string{}, nil
	}
	if !strings.HasPrefix(value, "/") {
		return nil, fmt.Errorf("pointer must be empty or start with /")
	}
	rawTokens := strings.Split(strings.TrimPrefix(value, "/"), "/")
	tokens := make([]string, 0, len(rawTokens))
	for _, raw := range rawTokens {
		var token strings.Builder
		for index := 0; index < len(raw); index++ {
			if raw[index] != '~' {
				token.WriteByte(raw[index])
				continue
			}
			if index+1 >= len(raw) {
				return nil, fmt.Errorf("trailing ~ escape")
			}
			index++
			switch raw[index] {
			case '0':
				token.WriteByte('~')
			case '1':
				token.WriteByte('/')
			default:
				return nil, fmt.Errorf("unsupported ~%c escape", raw[index])
			}
		}
		tokens = append(tokens, token.String())
	}
	return tokens, nil
}

func (planner *Planner) shorthandPaginationRequest(mode string) ir.PaginationRequestPlan {
	request := ir.PaginationRequestPlan{Limit: "limit"}
	if mode == "cursor" || mode == "both" {
		request.Cursor = "cursor"
	}
	if mode == "offset" || mode == "both" {
		request.Offset = "offset"
	}
	return request
}

func (planner *Planner) validatePaginationRequest(document *ir.Document, operation ir.Operation, pointer, mode string, request ir.PaginationRequestPlan, parameters []ir.Parameter, explicit bool) []diagnostic.Diagnostic {
	byName := make(map[string]ir.Parameter)
	for _, parameter := range parameters {
		if parameter.Location == "query" {
			byName[parameter.Name] = parameter
		}
	}
	var diagnostics []diagnostic.Diagnostic
	validate := func(role, name, expected string, minimum int) {
		if name == "" {
			return
		}
		location := pointer
		if explicit {
			location += "/request/" + role
		}
		parameter, exists := byName[name]
		if !exists {
			diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, location, "SDKGEN-E651", fmt.Sprintf("Pagination %s query parameter %q is not declared.", role, name), "Declare the exact query Parameter Object or correct x-pagination.request."))
			return
		}
		location = parameter.Pointer + "/schema"
		schema, ok := parameter.Schema.(map[string]any)
		if !ok {
			diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, location, "SDKGEN-E651", fmt.Sprintf("Pagination %s query parameter %q requires a schema.", role, name), "Declare a compatible scalar schema on the Parameter Object."))
			return
		}
		if expected == "string" {
			if !planner.SchemaHasOnlyTypes(document, schema, map[string]bool{"string": true}) {
				diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, location, "SDKGEN-E651", fmt.Sprintf("Pagination cursor query parameter %q must be a non-null string.", name), "Use a string schema for the cursor parameter."))
			}
			return
		}
		if !planner.SchemaHasOnlyTypes(document, schema, map[string]bool{"integer": true}) {
			diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, location, "SDKGEN-E651", fmt.Sprintf("Pagination %s query parameter %q must be a non-null integer.", role, name), "Use an integer schema with the required lower bound."))
			return
		}
		if !planner.paginationSchemaEnsuresIntegerMinimum(document, schema, minimum, make(map[string]bool)) {
			word := "non-negative"
			if minimum == 1 {
				word = "positive"
			}
			diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, location, "SDKGEN-E651", fmt.Sprintf("Pagination %s query parameter %q must declare a %s integer bound.", role, name, word), fmt.Sprintf("Declare minimum: %d or an equivalent exclusiveMinimum.", minimum)))
		}
	}
	if mode == "cursor" || mode == "both" {
		validate("cursor", request.Cursor, "string", 0)
	}
	if mode == "offset" || mode == "both" {
		validate("offset", request.Offset, "integer", 0)
	}
	if request.Limit != "" {
		validate("limit", request.Limit, "integer", 1)
	}
	roles := map[string]string{"cursor": request.Cursor, "offset": request.Offset, "limit": request.Limit}
	byParameter := make(map[string][]string)
	for role, name := range roles {
		if name != "" {
			byParameter[name] = append(byParameter[name], role)
		}
	}
	for name, values := range byParameter {
		if len(values) < 2 {
			continue
		}
		sort.Strings(values)
		diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, pointer+"/request", "SDKGEN-E651", fmt.Sprintf("Pagination query parameter %q is assigned to multiple controls: %s.", name, strings.Join(values, ", ")), "Assign a distinct exact query parameter to each control."))
	}
	return diagnostics
}

func (planner *Planner) paginationRepresentations(document *ir.Document, operation ir.Operation, pointer string) ([]paginationRepresentation, []diagnostic.Diagnostic, error) {
	responses, err := planner.responses(document, operation)
	if err != nil {
		return nil, nil, err
	}
	var result []paginationRepresentation
	var diagnostics []diagnostic.Diagnostic
	for _, response := range reachableSuccessResponses(responses) {
		for _, media := range response.Content {
			_, hasSchema := media.Raw["schema"]
			label := response.Status + " " + media.ContentType
			if !hasSchema {
				diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, pointer, "SDKGEN-E652", fmt.Sprintf("Pagination cannot validate schemaless successful representation %s.", label), "Declare a body schema consistently or remove x-pagination."))
				continue
			}
			schema, ok := media.Schema.(map[string]any)
			if !isJSONMediaType(media.ContentType) || !ok || len(schema) == 0 {
				diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, pointer, "SDKGEN-E652", fmt.Sprintf("Pagination cannot consume successful representation %s.", label), "Use body-bearing JSON object/array schemas consistently or remove x-pagination."))
				continue
			}
			result = append(result, paginationRepresentation{Label: label, Schema: schema})
		}
	}
	return result, diagnostics, nil
}

func (planner *Planner) inferPaginationResponse(document *ir.Document, operation ir.Operation, pointer, mode string, representations []paginationRepresentation) (ir.PaginationResponsePlan, map[string]any, []diagnostic.Diagnostic) {
	var diagnostics []diagnostic.Diagnostic
	selectedRows := make([]paginationRow, 0, len(representations))
	for _, representation := range representations {
		var matches []paginationRow
		for _, row := range canonicalPaginationRows {
			itemsSchema, itemsFound := planner.paginationSchemaAtPointer(document, representation.Schema, row.Items)
			_, itemsValid := planner.paginationArrayItemsSchema(document, itemsSchema)
			if !itemsFound || !itemsValid {
				continue
			}
			if mode == "cursor" || mode == "both" {
				cursorSchema, found := planner.paginationSchemaAtPointer(document, representation.Schema, row.NextCursor)
				if !found || !planner.SchemaHasOnlyTypes(document, cursorSchema, map[string]bool{"string": true, "null": true}) || !planner.paginationSchemaIncludesType(document, cursorSchema, "string") {
					continue
				}
			}
			matches = append(matches, row)
		}
		if len(matches) != 1 {
			names := make([]string, 0, len(matches))
			for _, match := range matches {
				names = append(names, match.Name)
			}
			message := fmt.Sprintf("Pagination shorthand does not match one canonical layout in %s.", representation.Label)
			if len(names) > 1 {
				message = fmt.Sprintf("Pagination shorthand is ambiguous in %s: %s.", representation.Label, strings.Join(names, ", "))
			}
			diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, pointer, "SDKGEN-E654", message, "Use the explicit x-pagination request/response JSON Pointer form."))
			continue
		}
		selected := matches[0]
		mixed := false
		for _, row := range canonicalPaginationRows {
			if row.Name == selected.Name {
				continue
			}
			for _, metadataPointer := range [][]string{row.NextCursor, row.Offset, row.Limit, row.Total} {
				if _, found := planner.paginationSchemaAtPointer(document, representation.Schema, metadataPointer); found {
					mixed = true
				}
			}
		}
		if mixed {
			diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, pointer, "SDKGEN-E654", fmt.Sprintf("Pagination shorthand mixes the %s items path with metadata from another canonical layout in %s.", selected.Name, representation.Label), "Use one complete canonical row or the explicit x-pagination object form."))
			continue
		}
		selectedRows = append(selectedRows, selected)
	}
	if len(diagnostics) > 0 || len(selectedRows) != len(representations) {
		return ir.PaginationResponsePlan{}, nil, diagnostics
	}
	row := selectedRows[0]
	for index := 1; index < len(selectedRows); index++ {
		if selectedRows[index].Name != row.Name {
			diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, pointer, "SDKGEN-E654", "Pagination shorthand resolves to different canonical layouts across successful representations.", "Use one consistent layout or the explicit x-pagination object form."))
			return ir.PaginationResponsePlan{}, nil, diagnostics
		}
	}
	response := ir.PaginationResponsePlan{Items: row.Items}
	if mode == "cursor" || mode == "both" {
		response.NextCursor = row.NextCursor
	}
	for _, field := range []struct {
		name    string
		pointer []string
		target  *[]string
		minimum int
	}{
		{name: "offset", pointer: row.Offset, target: &response.Offset, minimum: 0},
		{name: "limit", pointer: row.Limit, target: &response.Limit, minimum: 1},
		{name: "total", pointer: row.Total, target: &response.Total, minimum: 0},
	} {
		present := 0
		valid := true
		for _, representation := range representations {
			schema, found := planner.paginationSchemaAtPointer(document, representation.Schema, field.pointer)
			if !found {
				continue
			}
			present++
			if !planner.paginationIntegerResponseSchemaValid(document, schema, field.minimum) {
				valid = false
				diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, pointer, "SDKGEN-E653", fmt.Sprintf("Canonical %s pointer %s has an incompatible schema in %s.", field.name, planner.paginationPointerString(field.pointer), representation.Label), planner.paginationIntegerHint(field.name, field.minimum)))
			}
		}
		if present != 0 && present != len(representations) {
			valid = false
			diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, pointer, "SDKGEN-E653", fmt.Sprintf("Canonical %s metadata is present in only some successful representations.", field.name), "Use consistent response metadata or the explicit object form."))
		}
		if valid && present == len(representations) {
			*field.target = field.pointer
		}
	}
	itemSchema, findings := planner.validatePaginationResponse(document, operation, pointer, mode, response, representations, false)
	diagnostics = append(diagnostics, findings...)
	return response, itemSchema, diagnostics
}

func (planner *Planner) validatePaginationResponse(document *ir.Document, operation ir.Operation, pointer, mode string, response ir.PaginationResponsePlan, representations []paginationRepresentation, explicit bool) (map[string]any, []diagnostic.Diagnostic) {
	var diagnostics []diagnostic.Diagnostic
	var itemSchema map[string]any
	itemSignature := ""
	for _, representation := range representations {
		arraySchema, found := planner.paginationSchemaAtPointer(document, representation.Schema, response.Items)
		items, valid := planner.paginationArrayItemsSchema(document, arraySchema)
		if !found || !valid {
			diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, planner.paginationResponseDiagnosticPointer(pointer, "items", explicit), "SDKGEN-E653", fmt.Sprintf("Pagination items pointer %s does not resolve to an array schema in %s.", planner.paginationPointerString(response.Items), representation.Label), "Point response.items to an array in every successful JSON representation."))
		} else {
			signature := planner.itemSignature(document, items)
			if itemSignature == "" {
				itemSignature = signature
				itemSchema = items
			} else if signature != itemSignature {
				diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, planner.paginationResponseDiagnosticPointer(pointer, "items", explicit), "SDKGEN-E653", fmt.Sprintf("Pagination items resolve to different schemas in %s.", representation.Label), "Use one item schema across every successful JSON representation."))
			}
		}
		if mode == "cursor" || mode == "both" {
			schema, found := planner.paginationSchemaAtPointer(document, representation.Schema, response.NextCursor)
			if !found || !planner.SchemaHasOnlyTypes(document, schema, map[string]bool{"string": true, "null": true}) || !planner.paginationSchemaIncludesType(document, schema, "string") {
				diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, planner.paginationResponseDiagnosticPointer(pointer, "nextCursor", explicit), "SDKGEN-E653", fmt.Sprintf("Pagination nextCursor pointer %s must resolve to a string schema that may be nullable in %s.", planner.paginationPointerString(response.NextCursor), representation.Label), "Point nextCursor to a string or string/null schema."))
			}
		}
		for _, field := range []struct {
			name    string
			tokens  []string
			minimum int
		}{
			{name: "offset", tokens: response.Offset, minimum: 0},
			{name: "limit", tokens: response.Limit, minimum: 1},
			{name: "total", tokens: response.Total, minimum: 0},
		} {
			if field.tokens == nil {
				continue
			}
			schema, found := planner.paginationSchemaAtPointer(document, representation.Schema, field.tokens)
			if !found || !planner.paginationIntegerResponseSchemaValid(document, schema, field.minimum) {
				diagnostics = append(diagnostics, planner.operationExtensionDiagnostic(document, operation, planner.paginationResponseDiagnosticPointer(pointer, field.name, explicit), "SDKGEN-E653", fmt.Sprintf("Pagination %s pointer %s has an incompatible schema in %s.", field.name, planner.paginationPointerString(field.tokens), representation.Label), planner.paginationIntegerHint(field.name, field.minimum)))
			}
		}
	}
	return itemSchema, diagnostics
}

func (planner *Planner) paginationResponseDiagnosticPointer(pointer, field string, explicit bool) string {
	if explicit {
		return pointer + "/response/" + field
	}
	return pointer
}

func (planner *Planner) paginationIntegerHint(name string, minimum int) string {
	word := "non-negative"
	if minimum == 1 {
		word = "positive"
	}
	return fmt.Sprintf("Point %s to an integer or integer/null schema declaring a %s bound.", name, word)
}

func (planner *Planner) paginationIntegerResponseSchemaValid(document *ir.Document, schema map[string]any, minimum int) bool {
	return planner.SchemaHasOnlyTypes(document, schema, map[string]bool{"integer": true, "null": true}) &&
		planner.paginationSchemaIncludesType(document, schema, "integer") &&
		planner.paginationSchemaEnsuresIntegerMinimum(document, schema, minimum, make(map[string]bool))
}

func (planner *Planner) paginationSchemaAtPointer(document *ir.Document, schema map[string]any, tokens []string) (map[string]any, bool) {
	current := schema
	for _, token := range tokens {
		next, found := planner.paginationSchemaChild(document, current, token, make(map[string]bool))
		if !found {
			return nil, false
		}
		current = next
	}
	return current, current != nil
}

func (planner *Planner) paginationSchemaChild(document *ir.Document, schema map[string]any, token string, resolving map[string]bool) (map[string]any, bool) {
	var direct []map[string]any
	if properties, _ := schema["properties"].(map[string]any); properties != nil {
		if child, ok := properties[token].(map[string]any); ok {
			direct = append(direct, child)
		}
	}
	if reference, _ := schema["$ref"].(string); reference != "" {
		if name, err := componentSchemaReferenceName(reference); err == nil && !resolving[name] {
			resolving[name] = true
			if child, found := planner.paginationSchemaChild(document, document.ComponentSchemas[name], token, resolving); found {
				direct = append(direct, child)
			}
			delete(resolving, name)
		}
	}
	for _, keyword := range []string{"allOf"} {
		variants, _ := schema[keyword].([]any)
		for _, raw := range variants {
			variant, _ := raw.(map[string]any)
			if child, found := planner.paginationSchemaChild(document, variant, token, planner.copyStringBoolMap(resolving)); found {
				direct = append(direct, child)
			}
		}
	}
	if len(direct) > 0 {
		return planner.paginationCombinedSchema("allOf", direct), true
	}
	for _, keyword := range []string{"oneOf", "anyOf"} {
		variants, _ := schema[keyword].([]any)
		if len(variants) == 0 {
			continue
		}
		children := make([]map[string]any, 0, len(variants))
		for _, raw := range variants {
			variant, _ := raw.(map[string]any)
			child, found := planner.paginationSchemaChild(document, variant, token, planner.copyStringBoolMap(resolving))
			if !found {
				children = nil
				break
			}
			children = append(children, child)
		}
		if len(children) == len(variants) {
			return planner.paginationCombinedSchema(keyword, children), true
		}
	}
	if index, err := strconv.Atoi(token); err == nil && index >= 0 {
		if prefix, _ := schema["prefixItems"].([]any); index < len(prefix) {
			child, ok := prefix[index].(map[string]any)
			return child, ok
		}
		if child, ok := schema["items"].(map[string]any); ok {
			return child, true
		}
	}
	return nil, false
}

func (planner *Planner) paginationCombinedSchema(keyword string, schemas []map[string]any) map[string]any {
	if len(schemas) == 1 {
		return schemas[0]
	}
	values := make([]any, 0, len(schemas))
	for _, schema := range schemas {
		values = append(values, schema)
	}
	return map[string]any{keyword: values}
}

func (planner *Planner) paginationArrayItemsSchema(document *ir.Document, schema map[string]any) (map[string]any, bool) {
	if schema == nil {
		return nil, false
	}
	if reference, _ := schema["$ref"].(string); reference != "" {
		if name, err := componentSchemaReferenceName(reference); err == nil {
			return planner.paginationArrayItemsSchema(document, document.ComponentSchemas[name])
		}
	}
	if planner.schemaHasType(schema, "array") {
		items, ok := schema["items"].(map[string]any)
		return items, ok
	}
	for _, keyword := range []string{"allOf", "oneOf", "anyOf"} {
		variants, _ := schema[keyword].([]any)
		if len(variants) == 0 {
			continue
		}
		items := make([]map[string]any, 0, len(variants))
		for _, raw := range variants {
			variant, _ := raw.(map[string]any)
			item, ok := planner.paginationArrayItemsSchema(document, variant)
			if !ok {
				if keyword == "allOf" {
					continue
				}
				items = nil
				break
			}
			items = append(items, item)
		}
		if len(items) > 0 {
			return planner.paginationCombinedSchema(keyword, items), true
		}
	}
	return nil, false
}

func (planner *Planner) SchemaHasOnlyTypes(document *ir.Document, schema map[string]any, allowed map[string]bool) bool {
	types := planner.paginationSchemaTypeSet(document, schema, make(map[string]bool))
	if len(types) == 0 {
		return false
	}
	for value := range types {
		if !allowed[value] {
			return false
		}
	}
	return true
}

func (planner *Planner) paginationSchemaIncludesType(document *ir.Document, schema map[string]any, expected string) bool {
	return planner.paginationSchemaTypeSet(document, schema, make(map[string]bool))[expected]
}

func (planner *Planner) paginationSchemaTypeSet(document *ir.Document, schema map[string]any, resolving map[string]bool) map[string]bool {
	result := make(map[string]bool)
	constrained := false
	intersect := func(values map[string]bool, present bool) {
		if !present {
			return
		}
		if !constrained {
			result = planner.copyStringBoolMap(values)
			constrained = true
			return
		}
		for value := range result {
			if !values[value] {
				delete(result, value)
			}
		}
	}
	direct := make(map[string]bool)
	for _, value := range schemaTypes(schema["type"]) {
		direct[value] = true
		if value == "number" {
			direct["integer"] = true
		}
	}
	if boolValue(schema, "nullable") {
		direct["null"] = true
	}
	if len(direct) != 0 {
		intersect(direct, true)
	}
	if reference, _ := schema["$ref"].(string); reference != "" {
		if name, err := componentSchemaReferenceName(reference); err == nil && !resolving[name] {
			resolving[name] = true
			referenced := planner.paginationSchemaTypeSet(document, document.ComponentSchemas[name], resolving)
			delete(resolving, name)
			intersect(referenced, len(referenced) != 0)
		}
	}
	allOf, _ := schema["allOf"].([]any)
	for _, raw := range allOf {
		variant, _ := raw.(map[string]any)
		values := planner.paginationSchemaTypeSet(document, variant, planner.copyStringBoolMap(resolving))
		intersect(values, len(values) != 0)
	}
	for _, keyword := range []string{"oneOf", "anyOf"} {
		alternatives := make(map[string]bool)
		alternativeConstrained := true
		variants, _ := schema[keyword].([]any)
		for _, raw := range variants {
			variant, _ := raw.(map[string]any)
			values := planner.paginationSchemaTypeSet(document, variant, planner.copyStringBoolMap(resolving))
			if len(values) == 0 {
				alternativeConstrained = false
				break
			}
			for value := range values {
				alternatives[value] = true
			}
		}
		intersect(alternatives, len(variants) != 0 && alternativeConstrained)
	}
	literals := make(map[string]bool)
	if value, exists := schema["const"]; exists {
		if inferred := planner.paginationJSONType(value); inferred != "" {
			literals[inferred] = true
		}
	}
	if values, ok := schema["enum"].([]any); ok {
		for _, value := range values {
			if inferred := planner.paginationJSONType(value); inferred != "" {
				literals[inferred] = true
			}
		}
	}
	if len(literals) != 0 {
		intersect(literals, true)
	}
	if !constrained {
		return map[string]bool{}
	}
	return result
}

func (planner *Planner) paginationJSONType(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case json.Number:
		if strings.ContainsAny(string(typed), ".eE") {
			return "number"
		}
		return "integer"
	case float64:
		if typed == float64(int64(typed)) {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return ""
	}
}

func (planner *Planner) paginationSchemaEnsuresIntegerMinimum(document *ir.Document, schema map[string]any, minimum int, resolving map[string]bool) bool {
	if value, exclusive, exists := planner.paginationDirectLowerBound(schema); exists {
		if (!exclusive && value >= float64(minimum)) || (exclusive && value >= float64(minimum-1)) {
			return true
		}
	}
	if reference, _ := schema["$ref"].(string); reference != "" {
		if name, err := componentSchemaReferenceName(reference); err == nil && !resolving[name] {
			resolving[name] = true
			if planner.paginationSchemaEnsuresIntegerMinimum(document, document.ComponentSchemas[name], minimum, resolving) {
				delete(resolving, name)
				return true
			}
			delete(resolving, name)
		}
	}
	if variants, _ := schema["allOf"].([]any); len(variants) > 0 {
		for _, raw := range variants {
			variant, _ := raw.(map[string]any)
			if planner.paginationSchemaEnsuresIntegerMinimum(document, variant, minimum, planner.copyStringBoolMap(resolving)) {
				return true
			}
		}
	}
	for _, keyword := range []string{"oneOf", "anyOf"} {
		variants, _ := schema[keyword].([]any)
		if len(variants) == 0 {
			continue
		}
		all := true
		for _, raw := range variants {
			variant, _ := raw.(map[string]any)
			if !planner.paginationSchemaEnsuresIntegerMinimum(document, variant, minimum, planner.copyStringBoolMap(resolving)) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

func (planner *Planner) paginationDirectLowerBound(schema map[string]any) (float64, bool, bool) {
	if exclusive, ok := planner.paginationNumber(schema["exclusiveMinimum"]); ok {
		return exclusive, true, true
	}
	minimum, exists := planner.paginationNumber(schema["minimum"])
	if !exists {
		return 0, false, false
	}
	exclusive, _ := schema["exclusiveMinimum"].(bool)
	return minimum, exclusive, true
}

func (planner *Planner) paginationNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func (planner *Planner) paginationPointerString(tokens []string) string {
	if tokens == nil {
		return ""
	}
	if len(tokens) == 0 {
		return "<root>"
	}
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		parts = append(parts, strings.ReplaceAll(strings.ReplaceAll(token, "~", "~0"), "/", "~1"))
	}
	return "/" + strings.Join(parts, "/")
}
