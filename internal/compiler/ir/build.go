package ir

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	openapidoc "openapi-sdkgen/internal/compiler/openapi"
)

var standardMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace", "query"}

func Build(document *openapidoc.Document) (*Document, error) {
	if document == nil {
		return nil, fmt.Errorf("OpenAPI document is nil")
	}
	versionLine := document.Version
	if versionLine == "" {
		var err error
		versionLine, err = openapidoc.DetectVersionLine(stringValue(document.Raw, "openapi"))
		if err != nil {
			return nil, err
		}
	}
	info, _ := document.Raw["info"].(map[string]any)
	security, err := readSecurityRequirements(document.Raw["security"])
	if err != nil {
		return nil, fmt.Errorf("root security: %w", err)
	}
	result := &Document{
		Title:              stringValue(info, "title"),
		ContractVersion:    stringValue(info, "version"),
		OpenAPIVersion:     stringValue(document.Raw, "openapi"),
		OpenAPIVersionLine: string(versionLine),
		Servers:            readServers(document.Raw["servers"], "#/servers"),
		Security:           security,
		SecuritySchemes:    readSecuritySchemes(document.Raw),
		ComponentSchemas:   readComponentSchemas(document.Raw),
		Schemas:            readCompiledSchemas(document.Raw, versionLine),
		Raw:                document.Raw,
	}

	paths := map[string]any{}
	if value, exists := document.Raw["paths"]; exists {
		var ok bool
		paths, ok = value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("OpenAPI paths must be an object")
		}
	}
	pathNames := sortedKeys(paths)
	for _, path := range pathNames {
		if strings.HasPrefix(path, "x-") {
			continue
		}
		pathItem, ok := paths[path].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path item %q must be an object", path)
		}
		pathItem, err := ResolvePathItem(document.Raw, pathItem)
		if err != nil {
			return nil, fmt.Errorf("path item %q: %w", path, err)
		}
		for _, method := range standardMethods {
			operation, ok := pathItem[method].(map[string]any)
			if !ok {
				continue
			}
			if method == "query" && versionLine != openapidoc.Version32 {
				return nil, fmt.Errorf("OpenAPI 3.2 feature at %s: query method is not available in OpenAPI %s.x", jsonPointer("paths", path, method), versionLine)
			}
			compiledOperation, err := buildOperation(document.Raw, path, strings.ToUpper(method), jsonPointer("paths", path, method), pathItem, operation)
			if err != nil {
				return nil, fmt.Errorf("operation %s %q: %w", strings.ToUpper(method), path, err)
			}
			result.Operations = append(result.Operations, compiledOperation)
		}
		additional, _ := pathItem["additionalOperations"].(map[string]any)
		if len(additional) > 0 && versionLine != openapidoc.Version32 {
			return nil, fmt.Errorf("OpenAPI 3.2 feature at %s: additionalOperations is not available in OpenAPI %s.x", jsonPointer("paths", path, "additionalOperations"), versionLine)
		}
		for _, method := range sortedKeys(additional) {
			operation, ok := additional[method].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("additional operation %q %q must be an object", method, path)
			}
			compiledOperation, err := buildOperation(document.Raw, path, method, jsonPointer("paths", path, "additionalOperations", method), pathItem, operation)
			if err != nil {
				return nil, fmt.Errorf("additional operation %s %q: %w", method, path, err)
			}
			result.Operations = append(result.Operations, compiledOperation)
		}
	}
	sort.SliceStable(result.Operations, func(i, j int) bool {
		if result.Operations[i].Path == result.Operations[j].Path {
			return result.Operations[i].Method < result.Operations[j].Method
		}
		return result.Operations[i].Path < result.Operations[j].Path
	})
	return result, nil
}

func readCompiledSchemas(raw map[string]any, version openapidoc.VersionLine) map[string]Schema {
	components, _ := raw["components"].(map[string]any)
	values, _ := components["schemas"].(map[string]any)
	result := make(map[string]Schema, len(values))
	dialect := defaultSchemaDialect(raw, version)
	base := documentResourceURI(raw)
	for _, name := range sortedKeys(values) {
		value := values[name]
		pointer := jsonPointer("components", "schemas", name)
		resource := base + "#" + pointer
		schemaDialect := dialect
		if object, ok := value.(map[string]any); ok {
			if identifier, ok := object["$id"].(string); ok && identifier != "" {
				resource = resolveSchemaResourceURI(base, identifier)
			}
			if declaredDialect, ok := object["$schema"].(string); ok && declaredDialect != "" {
				schemaDialect = declaredDialect
			}
		}
		result[name] = Schema{Name: name, Pointer: pointer, ResourceURI: resource, Dialect: schemaDialect, Value: value}
	}
	return result
}

func documentResourceURI(raw map[string]any) string {
	for _, key := range []string{"$self", "$id"} {
		if value, ok := raw[key].(string); ok && value != "" {
			return strings.TrimSuffix(value, "#")
		}
	}
	return "urn:openapi-sdkgen:document"
}

func defaultSchemaDialect(raw map[string]any, version openapidoc.VersionLine) string {
	if value, ok := raw["jsonSchemaDialect"].(string); ok && value != "" {
		return value
	}
	if version == openapidoc.Version30 {
		return "https://spec.openapis.org/oas/3.0/dialect/base"
	}
	return "https://spec.openapis.org/oas/3.1/dialect/base"
}

func resolveSchemaResourceURI(base, identifier string) string {
	if base == "" || strings.HasPrefix(base, "urn:") {
		return identifier
	}
	baseURI, baseErr := url.Parse(base)
	identifierURI, identifierErr := url.Parse(identifier)
	if baseErr != nil || identifierErr != nil {
		return identifier
	}
	return baseURI.ResolveReference(identifierURI).String()
}

func jsonPointer(parts ...string) string {
	encoded := make([]string, 0, len(parts))
	for _, part := range parts {
		encoded = append(encoded, escapeJSONPointerToken(part))
	}
	return "/" + strings.Join(encoded, "/")
}

func escapeJSONPointerToken(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

// ResolvePathItem resolves a local OpenAPI Path Item reference and applies
// sibling overrides. Path Item references may target any local JSON Pointer.
func ResolvePathItem(document, pathItem map[string]any) (map[string]any, error) {
	resolved, err := resolvePathItem(document, pathItem, make(map[string]bool))
	if err != nil {
		return nil, &ReferenceError{err: err}
	}
	return resolved, nil
}

// ReferenceError marks author-correctable Path Item reference failures.
type ReferenceError struct {
	err error
}

func (value *ReferenceError) Error() string { return value.err.Error() }
func (value *ReferenceError) Unwrap() error { return value.err }

// IsReferenceError reports whether an IR build failure belongs to reference
// resolution rather than general OpenAPI validation.
func IsReferenceError(err error) bool {
	var target *ReferenceError
	return errors.As(err, &target)
}

// IsLocalPathItemReference reports whether a Path Item reference uses a local
// JSON Pointer, including URI-fragment encoded pointers such as "#%2Fpaths".
func IsLocalPathItemReference(reference string) (bool, error) {
	if !strings.HasPrefix(reference, "#") {
		return false, nil
	}
	pointer, err := url.PathUnescape(strings.TrimPrefix(reference, "#"))
	if err != nil {
		return false, fmt.Errorf("invalid path item reference %q: invalid URI fragment escape", reference)
	}
	return strings.HasPrefix(pointer, "/"), nil
}

func resolvePathItem(document, pathItem map[string]any, resolving map[string]bool) (map[string]any, error) {
	reference, _ := pathItem["$ref"].(string)
	if reference == "" {
		return pathItem, nil
	}
	local, err := IsLocalPathItemReference(reference)
	if err != nil {
		return nil, err
	}
	if !local {
		return nil, fmt.Errorf("external path item reference %q is not supported", reference)
	}
	if resolving[reference] {
		return nil, fmt.Errorf("cyclic path item reference %q", reference)
	}
	resolving[reference] = true
	defer delete(resolving, reference)
	resolved, err := localPathItemReference(document, reference)
	if err != nil {
		return nil, err
	}
	resolved, err = resolvePathItem(document, resolved, resolving)
	if err != nil {
		return nil, err
	}
	merged := make(map[string]any, len(resolved)+len(pathItem))
	for key, value := range resolved {
		merged[key] = value
	}
	for key, value := range pathItem {
		if key != "$ref" {
			merged[key] = value
		}
	}
	return merged, nil
}

func localPathItemReference(document map[string]any, reference string) (map[string]any, error) {
	var value any = document
	pointer, err := url.PathUnescape(strings.TrimPrefix(reference, "#"))
	if err != nil {
		return nil, fmt.Errorf("invalid path item reference %q: invalid URI fragment escape", reference)
	}
	for _, token := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		name, err := jsonPointerToken(token)
		if err != nil {
			return nil, fmt.Errorf("invalid path item reference %q: %w", reference, err)
		}
		object, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unresolved path item reference %q", reference)
		}
		value, ok = object[name]
		if !ok {
			return nil, fmt.Errorf("unresolved path item reference %q", reference)
		}
	}
	pathItem, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unresolved path item reference %q", reference)
	}
	return pathItem, nil
}

func jsonPointerToken(token string) (string, error) {
	if strings.Contains(token, "/") {
		return "", fmt.Errorf("must target one object")
	}
	var output strings.Builder
	for index := 0; index < len(token); index++ {
		if token[index] != '~' {
			output.WriteByte(token[index])
			continue
		}
		if index+1 >= len(token) {
			return "", fmt.Errorf("invalid JSON Pointer escape")
		}
		index++
		switch token[index] {
		case '0':
			output.WriteByte('~')
		case '1':
			output.WriteByte('/')
		default:
			return "", fmt.Errorf("invalid JSON Pointer escape")
		}
	}
	return output.String(), nil
}

func buildOperation(document map[string]any, path, method, pointer string, pathItemRaw, raw map[string]any) (Operation, error) {
	operationPointer := "#" + pointer
	pathParameterOrder := templateParameters(path)
	parameters, err := readOperationParameters(document, pathItemRaw, raw, "#"+jsonPointer("paths", path), operationPointer, pathParameterOrder)
	if err != nil {
		return Operation{}, err
	}
	requestBody, err := readRequestBody(document, raw["requestBody"], operationPointer+"/requestBody")
	if err != nil {
		return Operation{}, err
	}
	responses, err := readResponses(document, raw["responses"], operationPointer+"/responses")
	if err != nil {
		return Operation{}, err
	}
	servers := effectiveServers(document, pathItemRaw, raw, path, operationPointer)
	securityValue, securityDeclared := raw["security"]
	if !securityDeclared {
		securityValue = document["security"]
	}
	security, err := readSecurityRequirements(securityValue)
	if err != nil {
		return Operation{}, err
	}
	return Operation{
		RouteKey:    method + " " + path,
		Pointer:     operationPointer,
		OperationID: stringValue(raw, "operationId"),
		Method:      method,
		Path:        path,
		Summary:     stringValue(raw, "summary"),
		Description: stringValue(raw, "description"),
		Tags:        stringSlice(raw["tags"]),
		Visibility:  stringValue(raw, "x-sdk-visibility"),
		Envelope:    stringValue(raw, "x-envelope"),
		Pagination:  stringValue(raw, "x-pagination"),
		Extensions: OperationExtensions{
			Envelope:   readStringExtension(raw, "x-envelope", operationPointer+"/x-envelope"),
			Pagination: readValueExtension(raw, "x-pagination", operationPointer+"/x-pagination"),
			Visibility: readStringExtension(raw, "x-sdk-visibility", operationPointer+"/x-sdk-visibility"),
		},
		PathParameterOrder: pathParameterOrder,
		Parameters:         parameters,
		RequestBody:        requestBody,
		Responses:          responses,
		Servers:            servers,
		Security:           security,
		SecurityDeclared:   securityDeclared,
		PathItemRaw:        pathItemRaw,
		Raw:                raw,
	}, nil
}

func readOperationParameters(document, pathItem, operation map[string]any, pathPointer, operationPointer string, pathOrder []string) ([]Parameter, error) {
	pathValues, _ := pathItem["parameters"].([]any)
	operationValues, _ := operation["parameters"].([]any)
	result := make([]Parameter, 0, len(pathValues)+len(operationValues))
	indices := make(map[string]int, len(pathValues)+len(operationValues))
	for _, source := range []struct {
		value   any
		pointer string
	}{
		{value: pathItem["parameters"], pointer: pathPointer + "/parameters"},
		{value: operation["parameters"], pointer: operationPointer + "/parameters"},
	} {
		values, _ := source.value.([]any)
		for index, value := range values {
			raw, _ := value.(map[string]any)
			pointer := source.pointer + "/" + strconv.Itoa(index)
			if reference, _ := raw["$ref"].(string); reference != "" {
				if resolvedPointer := localReferencePointer(reference); resolvedPointer != "" {
					pointer = resolvedPointer
				}
			}
			resolved, err := resolveReusableComponent(document, raw, "parameters", nil)
			if err != nil {
				return nil, err
			}
			name, _ := resolved["name"].(string)
			location, _ := resolved["in"].(string)
			if name == "" || location == "" {
				continue
			}
			style, _ := resolved["style"].(string)
			if style == "" {
				style = defaultParameterStyle(location)
			}
			explode, hasExplode := resolved["explode"].(bool)
			if !hasExplode {
				explode = style == "form"
			}
			content, err := readMediaTypes(document, resolved["content"])
			if err != nil {
				return nil, err
			}
			schema := resolved["schema"]
			contentType := ""
			if len(content) != 0 {
				contentType = content[0].ContentType
				schema = content[0].Schema
			}
			parameter := Parameter{
				Name:          name,
				Description:   stringValue(resolved, "description"),
				Location:      location,
				Style:         style,
				Explode:       explode,
				Required:      boolValue(resolved, "required"),
				Deprecated:    boolValue(resolved, "deprecated"),
				AllowReserved: boolValue(resolved, "allowReserved"),
				ContentType:   contentType,
				Content:       content,
				Schema:        schema,
				Raw:           resolved,
				Pointer:       pointer,
			}
			key := location + "\x00" + name
			if resultIndex, exists := indices[key]; exists {
				result[resultIndex] = parameter
			} else {
				indices[key] = len(result)
				result = append(result, parameter)
			}
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Location != "path" || result[j].Location != "path" {
			return false
		}
		return parameterPathIndex(pathOrder, result[i].Name) < parameterPathIndex(pathOrder, result[j].Name)
	})
	return result, nil
}

func readRequestBody(document map[string]any, value any, pointer string) (*RequestBody, error) {
	raw, ok := value.(map[string]any)
	if !ok {
		return nil, nil
	}
	if reference, _ := raw["$ref"].(string); reference != "" {
		if resolvedPointer := localReferencePointer(reference); resolvedPointer != "" {
			pointer = resolvedPointer
		}
	}
	resolved, err := resolveReusableComponent(document, raw, "requestBodies", nil)
	if err != nil {
		return nil, err
	}
	content, err := readMediaTypes(document, resolved["content"])
	if err != nil {
		return nil, err
	}
	return &RequestBody{
		Description: stringValue(resolved, "description"),
		Required:    boolValue(resolved, "required"),
		Content:     content,
		Raw:         resolved,
		Pointer:     pointer,
	}, nil
}

func readResponses(document map[string]any, value any, pointer string) ([]Response, error) {
	values, _ := value.(map[string]any)
	result := make([]Response, 0, len(values))
	for _, status := range sortedKeys(values) {
		if strings.HasPrefix(status, "x-") {
			continue
		}
		source, _ := values[status].(map[string]any)
		responsePointer := pointer + "/" + escapeJSONPointerToken(status)
		resolved, err := resolveReusableComponent(document, source, "responses", nil)
		if err != nil {
			return nil, err
		}
		content, err := readMediaTypes(document, resolved["content"])
		if err != nil {
			return nil, err
		}
		result = append(result, Response{
			Status:      status,
			Description: stringValue(resolved, "description"),
			Summary:     stringValue(resolved, "summary"),
			Content:     content,
			Raw:         resolved,
			SourceRaw:   source,
			Pointer:     responsePointer,
		})
	}
	return result, nil
}

func readMediaTypes(document map[string]any, value any) ([]MediaType, error) {
	values, _ := value.(map[string]any)
	result := make([]MediaType, 0, len(values))
	for _, contentType := range sortedKeys(values) {
		raw, _ := values[contentType].(map[string]any)
		resolved, err := resolveReusableComponent(document, raw, "mediaTypes", nil)
		if err != nil {
			return nil, err
		}
		result = append(result, MediaType{
			ContentType: contentType,
			Schema:      resolved["schema"],
			ItemSchema:  resolved["itemSchema"],
			Raw:         resolved,
		})
	}
	return result, nil
}

func resolveReusableComponent(document, object map[string]any, component string, resolving map[string]bool) (map[string]any, error) {
	if object == nil {
		return object, nil
	}
	reference, _ := object["$ref"].(string)
	if reference == "" {
		return object, nil
	}
	if resolving == nil {
		resolving = make(map[string]bool)
	}
	prefix := "#/components/" + component + "/"
	if !strings.HasPrefix(reference, prefix) {
		return nil, fmt.Errorf("external %s reference %q is not supported", component, reference)
	}
	if resolving[reference] {
		return nil, fmt.Errorf("cyclic %s reference %q", component, reference)
	}
	resolving[reference] = true
	defer delete(resolving, reference)
	components, _ := document["components"].(map[string]any)
	objects, _ := components[component].(map[string]any)
	token := strings.TrimPrefix(reference, prefix)
	if token == "" || strings.Contains(token, "/") {
		return nil, fmt.Errorf("%s reference %q must target one component", component, reference)
	}
	name, err := jsonPointerToken(token)
	if err != nil {
		return nil, fmt.Errorf("%s reference %q: %w", component, reference, err)
	}
	resolved, ok := objects[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unresolved %s reference %q", component, reference)
	}
	resolved, err = resolveReusableComponent(document, resolved, component, resolving)
	if err != nil {
		return nil, err
	}
	merged := make(map[string]any, len(resolved)+len(object))
	for key, value := range resolved {
		merged[key] = value
	}
	for key, value := range object {
		if key != "$ref" {
			merged[key] = value
		}
	}
	return merged, nil
}

func localReferencePointer(reference string) string {
	if !strings.HasPrefix(reference, "#") {
		return ""
	}
	decoded, err := url.PathUnescape(strings.TrimPrefix(reference, "#"))
	if err != nil || !strings.HasPrefix(decoded, "/") {
		return ""
	}
	return "#" + decoded
}

func defaultParameterStyle(location string) string {
	if location == "query" || location == "cookie" {
		return "form"
	}
	return "simple"
}

func parameterPathIndex(order []string, name string) int {
	for index, value := range order {
		if value == name {
			return index
		}
	}
	return len(order)
}

func readValueExtension(raw map[string]any, name, pointer string) ValueExtension {
	value, present := raw[name]
	return ValueExtension{Present: present, Raw: value, Pointer: pointer}
}

func readStringExtension(raw map[string]any, name, pointer string) StringExtension {
	value, present := raw[name]
	text, valid := value.(string)
	return StringExtension{
		Present: present,
		Valid:   present && valid,
		Value:   text,
		Raw:     value,
		Pointer: pointer,
	}
}

func effectiveServers(document, pathItem, operation map[string]any, path, operationPointer string) []Server {
	if value, exists := operation["servers"]; exists {
		return readServers(value, operationPointer+"/servers")
	}
	if value, exists := pathItem["servers"]; exists {
		return readServers(value, "#"+jsonPointer("paths", path, "servers"))
	}
	return readServers(document["servers"], "#/servers")
}

func readServers(value any, pointer string) []Server {
	items, _ := value.([]any)
	servers := make([]Server, 0, len(items))
	for index, item := range items {
		server, ok := item.(map[string]any)
		if !ok {
			continue
		}
		variablesRaw, _ := server["variables"].(map[string]any)
		variables := make([]ServerVariable, 0, len(variablesRaw))
		for _, name := range sortedKeys(variablesRaw) {
			variable, _ := variablesRaw[name].(map[string]any)
			variables = append(variables, ServerVariable{
				Name:        name,
				Default:     stringValue(variable, "default"),
				Enum:        stringSlice(variable["enum"]),
				Description: stringValue(variable, "description"),
			})
		}
		servers = append(servers, Server{
			URL:         stringValue(server, "url"),
			Description: stringValue(server, "description"),
			Variables:   variables,
			Pointer:     fmt.Sprintf("%s/%d", pointer, index),
			Raw:         server,
		})
	}
	return servers
}

func readSecurityRequirements(value any) ([]SecurityRequirement, error) {
	if value == nil {
		return nil, nil
	}
	values, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("security must be an array")
	}
	result := make([]SecurityRequirement, 0, len(values))
	for index, value := range values {
		requirement, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("security requirement %d must be an object", index)
		}
		schemes := make([]SecurityRequirementScheme, 0, len(requirement))
		for _, name := range sortedKeys(requirement) {
			rawScopes, ok := requirement[name].([]any)
			if !ok {
				return nil, fmt.Errorf("security scheme %q scopes must be an array", name)
			}
			scopes := make([]string, 0, len(rawScopes))
			for _, rawScope := range rawScopes {
				scope, ok := rawScope.(string)
				if !ok {
					return nil, fmt.Errorf("security scheme %q has a non-string scope", name)
				}
				scopes = append(scopes, scope)
			}
			sort.Strings(scopes)
			schemes = append(schemes, SecurityRequirementScheme{Name: name, Scopes: scopes})
		}
		result = append(result, SecurityRequirement{Schemes: schemes, Raw: requirement})
	}
	return result, nil
}

func readSecuritySchemes(raw map[string]any) map[string]SecurityScheme {
	components, _ := raw["components"].(map[string]any)
	values, _ := components["securitySchemes"].(map[string]any)
	result := make(map[string]SecurityScheme, len(values))
	for _, name := range sortedKeys(values) {
		scheme, ok := values[name].(map[string]any)
		if !ok {
			continue
		}
		result[name] = SecurityScheme{
			Name:              name,
			Type:              stringValue(scheme, "type"),
			Location:          stringValue(scheme, "in"),
			ParameterName:     stringValue(scheme, "name"),
			Scheme:            stringValue(scheme, "scheme"),
			BearerFormat:      stringValue(scheme, "bearerFormat"),
			Flows:             scheme["flows"],
			OpenIDConnectURL:  stringValue(scheme, "openIdConnectUrl"),
			OAuth2MetadataURL: stringValue(scheme, "oauth2MetadataUrl"),
			Deprecated:        boolValue(scheme, "deprecated"),
			Raw:               scheme,
		}
	}
	return result
}

func readComponentSchemas(raw map[string]any) map[string]map[string]any {
	components, _ := raw["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	result := make(map[string]map[string]any, len(schemas))
	for _, name := range sortedKeys(schemas) {
		if schema, ok := schemas[name].(map[string]any); ok {
			result[name] = schema
		}
	}
	return result
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func boolValue(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}

func stringSlice(value any) []string {
	values, _ := value.([]any)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func templateParameters(path string) []string {
	var result []string
	for {
		start := strings.IndexByte(path, '{')
		if start < 0 {
			return result
		}
		path = path[start+1:]
		end := strings.IndexByte(path, '}')
		if end < 0 {
			return result
		}
		result = append(result, path[:end])
		path = path[end+1:]
	}
}
