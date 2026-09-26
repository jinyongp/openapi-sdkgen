package openapi

import (
	"fmt"
	"sort"
	"strings"
)

// versionValidation is the single validation kernel used by both fail-fast
// and collecting callers. Collect mode records every independently reachable
// version feature finding while preserving the same deterministic traversal.
type versionValidation struct {
	collect  bool
	findings []*VersionFeatureError
}

func validateVersionSpecificFeatures(raw map[string]any, version VersionLine) error {
	validation := &versionValidation{}
	return validation.validateVersionSpecificFeatures(raw, version)
}

// CollectVersionFeatureErrors returns every deterministic version-feature
// violation reachable in one traversal. The returned errors preserve the same
// rule, pointer, and ordering used by the fail-fast adapter.
func CollectVersionFeatureErrors(raw map[string]any, version VersionLine) []*VersionFeatureError {
	validation := &versionValidation{collect: true}
	_ = validation.validateVersionSpecificFeatures(raw, version)
	return append([]*VersionFeatureError(nil), validation.findings...)
}

func (validation *versionValidation) reject(path, detail string) error {
	value := &VersionFeatureError{Pointer: path, Detail: detail}
	validation.findings = append(validation.findings, value)
	if validation.collect {
		return nil
	}
	return value
}

// validateVersionSpecificFeatures rejects syntax introduced after the source
// document's declared minor line. A later-version construct must never be
// silently lowered as though an older document had declared it.
func (validation *versionValidation) validateVersionSpecificFeatures(raw map[string]any, version VersionLine) error {
	if err := validation.validateVersionedDocumentFields(raw, version); err != nil {
		return err
	}
	if version == Version30 {
		if _, exists := raw["paths"]; !exists {
			if err := validation.reject("#/paths", "paths is required by OpenAPI 3.0"); err != nil {
				return err
			}
		}
		for _, key := range []string{"webhooks", "jsonSchemaDialect"} {
			if _, exists := raw[key]; exists {
				if err := validation.reject(pointer(key), key+" requires OpenAPI 3.1 or later"); err != nil {
					return err
				}
			}
		}
		info, _ := raw["info"].(map[string]any)
		if _, exists := info["summary"]; exists {
			if err := validation.reject("#/info/summary", "info.summary requires OpenAPI 3.1 or later"); err != nil {
				return err
			}
		}
		license, _ := info["license"].(map[string]any)
		if _, exists := license["identifier"]; exists {
			if err := validation.reject("#/info/license/identifier", "license.identifier requires OpenAPI 3.1 or later"); err != nil {
				return err
			}
		}
		components, _ := raw["components"].(map[string]any)
		if _, exists := components["pathItems"]; exists {
			if err := validation.reject("#/components/pathItems", "components.pathItems requires OpenAPI 3.1 or later"); err != nil {
				return err
			}
		}
		securitySchemes, _ := components["securitySchemes"].(map[string]any)
		for _, name := range sortedKeys(securitySchemes) {
			scheme, _ := securitySchemes[name].(map[string]any)
			if scheme["type"] == "mutualTLS" {
				if err := validation.reject(pointer("components", "securitySchemes", name, "type"), "mutualTLS requires OpenAPI 3.1 or later"); err != nil {
					return err
				}
			}
		}
	}
	if version != Version32 {
		if _, exists := raw["$self"]; exists {
			if err := validation.reject("#/$self", "$self requires OpenAPI 3.2"); err != nil {
				return err
			}
		}
		components, _ := raw["components"].(map[string]any)
		if _, exists := components["mediaTypes"]; exists {
			if err := validation.reject("#/components/mediaTypes", "components.mediaTypes requires OpenAPI 3.2"); err != nil {
				return err
			}
		}
	}
	if err := validation.validateVersionedPaths(raw, version); err != nil {
		return err
	}
	if version == Version30 {
		if err := validation.validateOpenAPI30ReferenceObjects(raw, "#"); err != nil {
			return err
		}
	}
	return validation.validateVersionedSchemas(raw, version)
}

func (validation *versionValidation) validateVersionedDocumentFields(raw map[string]any, version VersionLine) error {
	if err := validation.validateServerFields(raw["servers"], "#/servers", version); err != nil {
		return err
	}
	if err := validation.validateTagFields(raw["tags"], "#/tags", version); err != nil {
		return err
	}
	components, _ := raw["components"].(map[string]any)
	if err := validation.validateSecuritySchemeFields(components["securitySchemes"], "#/components/securitySchemes", version); err != nil {
		return err
	}
	if err := validation.validateExamplesFields(components["examples"], "#/components/examples", version); err != nil {
		return err
	}
	return nil
}

func (validation *versionValidation) validateSecuritySchemeFields(value any, path string, version VersionLine) error {
	if version == Version32 {
		return nil
	}
	schemes, _ := value.(map[string]any)
	for _, name := range sortedKeys(schemes) {
		scheme, _ := schemes[name].(map[string]any)
		for _, key := range []string{"oauth2MetadataUrl", "deprecated"} {
			if _, exists := scheme[key]; exists {
				if err := validation.reject(pointerFrom(path, name, key), "security scheme "+key+" requires OpenAPI 3.2"); err != nil {
					return err
				}
			}
		}
		flows, _ := scheme["flows"].(map[string]any)
		if flow, exists := flows["deviceAuthorization"]; exists {
			if _, ok := flow.(map[string]any); ok {
				if err := validation.reject(pointerFrom(path, name, "flows", "deviceAuthorization"), "deviceAuthorization OAuth flow requires OpenAPI 3.2"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (validation *versionValidation) validateServerFields(value any, path string, version VersionLine) error {
	if version == Version32 {
		return nil
	}
	servers, _ := value.([]any)
	for index, value := range servers {
		if err := validation.validateServerObjectFields(value, pointerFrom(path, fmt.Sprint(index)), version); err != nil {
			return err
		}
	}
	return nil
}

func (validation *versionValidation) validateServerObjectFields(value any, path string, version VersionLine) error {
	if version == Version32 {
		return nil
	}
	server, _ := value.(map[string]any)
	if _, exists := server["name"]; exists {
		if err := validation.reject(pointerFrom(path, "name"), "server.name requires OpenAPI 3.2"); err != nil {
			return err
		}
	}
	return nil
}

func (validation *versionValidation) validateTagFields(value any, path string, version VersionLine) error {
	if version == Version32 {
		return nil
	}
	tags, _ := value.([]any)
	for index, value := range tags {
		tag, _ := value.(map[string]any)
		for _, key := range []string{"summary", "parent", "kind"} {
			if _, exists := tag[key]; exists {
				if err := validation.reject(pointerFrom(path, fmt.Sprint(index), key), "tag."+key+" requires OpenAPI 3.2"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (validation *versionValidation) validateExamplesFields(value any, path string, version VersionLine) error {
	if version == Version32 {
		return nil
	}
	examples, _ := value.(map[string]any)
	for _, name := range sortedKeys(examples) {
		example, _ := examples[name].(map[string]any)
		for _, key := range []string{"dataValue", "serializedValue"} {
			if _, exists := example[key]; exists {
				if err := validation.reject(pointerFrom(path, name, key), "Example Object "+key+" requires OpenAPI 3.2"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (validation *versionValidation) validateVersionedPaths(raw map[string]any, version VersionLine) error {
	paths, _ := raw["paths"].(map[string]any)
	for _, path := range sortedKeys(paths) {
		if strings.HasPrefix(path, "x-") {
			continue
		}
		pathItem, _ := paths[path].(map[string]any)
		if err := validation.validateVersionedPathItem(pathItem, pointer("paths", path), version); err != nil {
			return err
		}
	}
	webhooks, _ := raw["webhooks"].(map[string]any)
	for _, name := range sortedKeys(webhooks) {
		pathItem, _ := webhooks[name].(map[string]any)
		if err := validation.validateVersionedPathItem(pathItem, pointer("webhooks", name), version); err != nil {
			return err
		}
	}
	components, _ := raw["components"].(map[string]any)
	pathItems, _ := components["pathItems"].(map[string]any)
	for _, name := range sortedKeys(pathItems) {
		item, _ := pathItems[name].(map[string]any)
		if err := validation.validateVersionedPathItem(item, pointer("components", "pathItems", name), version); err != nil {
			return err
		}
	}
	if err := validation.validateVersionedCallbacks(components["callbacks"], pointer("components", "callbacks"), version); err != nil {
		return err
	}
	parameters, _ := components["parameters"].(map[string]any)
	for _, name := range sortedKeys(parameters) {
		if err := validation.validateQuerystringParameter(parameters[name], pointer("components", "parameters", name), version); err != nil {
			return err
		}
	}
	return nil
}

func (validation *versionValidation) validateVersionedPathItem(pathItem map[string]any, path string, version VersionLine) error {
	if err := validation.validateServerFields(pathItem["servers"], pointerFrom(path, "servers"), version); err != nil {
		return err
	}
	if version != Version32 {
		if _, exists := pathItem["query"]; exists {
			if err := validation.reject(pointerFrom(path, "query"), "query requires OpenAPI 3.2"); err != nil {
				return err
			}
		}
		if _, exists := pathItem["additionalOperations"]; exists {
			if err := validation.reject(pointerFrom(path, "additionalOperations"), "additionalOperations requires OpenAPI 3.2"); err != nil {
				return err
			}
		}
	}
	if err := validation.validateQuerystringParameters(pathItem["parameters"], pointerFrom(path, "parameters"), version); err != nil {
		return err
	}
	for _, method := range sortedKeys(pathItem) {
		operation, _ := pathItem[method].(map[string]any)
		if operation == nil {
			continue
		}
		if err := validation.validateQuerystringParameters(operation["parameters"], pointerFrom(path, method, "parameters"), version); err != nil {
			return err
		}
		if err := validation.validateServerFields(operation["servers"], pointerFrom(path, method, "servers"), version); err != nil {
			return err
		}
		if err := validation.validateTagFields(operation["tags"], pointerFrom(path, method, "tags"), version); err != nil {
			return err
		}
		if err := validation.validateVersionedCallbacks(operation["callbacks"], pointerFrom(path, method, "callbacks"), version); err != nil {
			return err
		}
	}
	return nil
}

func (validation *versionValidation) validateVersionedCallbacks(value any, path string, version VersionLine) error {
	callbacks, _ := value.(map[string]any)
	for _, name := range sortedKeys(callbacks) {
		callback, _ := callbacks[name].(map[string]any)
		for _, expression := range sortedKeys(callback) {
			pathItem, _ := callback[expression].(map[string]any)
			if err := validation.validateVersionedPathItem(pathItem, pointerFrom(path, name, expression), version); err != nil {
				return err
			}
		}
	}
	return nil
}

func (validation *versionValidation) validateQuerystringParameters(value any, path string, version VersionLine) error {
	if version == Version32 {
		return nil
	}
	parameters, _ := value.([]any)
	for index, value := range parameters {
		if err := validation.validateQuerystringParameter(value, pointerFrom(path, fmt.Sprint(index)), version); err != nil {
			return err
		}
	}
	return nil
}

func (validation *versionValidation) validateQuerystringParameter(value any, path string, version VersionLine) error {
	if version == Version32 {
		return nil
	}
	parameter, _ := value.(map[string]any)
	if style, _ := parameter["style"].(string); style == "cookie" {
		if err := validation.reject(pointerFrom(path, "style"), "cookie parameter style requires OpenAPI 3.2"); err != nil {
			return err
		}
	}
	if parameter["in"] == "querystring" {
		if err := validation.reject(pointerFrom(path, "in"), "querystring parameters require OpenAPI 3.2"); err != nil {
			return err
		}
	}
	return nil
}

func (validation *versionValidation) validateVersionedSchemas(raw map[string]any, version VersionLine) error {
	components, _ := raw["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	for _, name := range sortedKeys(schemas) {
		if err := validation.validateSchemaVersion(schemas[name], pointer("components", "schemas", name), version); err != nil {
			return err
		}
	}
	return validation.validateSchemaValues(raw, "#", version)
}

func (validation *versionValidation) validateSchemaValues(value any, path string, version VersionLine) error {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(typed) {
			if strings.HasPrefix(key, "x-") {
				continue
			}
			if key == "examples" {
				if err := validation.validateExamplesFields(typed[key], pointerFrom(path, key), version); err != nil {
					return err
				}
				continue
			}
			if isLiteralOpenAPIValue(key) {
				// Example values and Schema annotations are arbitrary instance
				// data. Their keys must not be interpreted as OpenAPI or JSON
				// Schema syntax while checking the surrounding document version.
				continue
			}
			item := typed[key]
			child := pointerFrom(path, key)
			if path == "#/components" && key == "schemas" {
				// Component schemas are traversed as Schema Objects above. Their
				// arbitrary JSON Schema vocabulary is not Media Type syntax.
				continue
			}
			if key == "content" {
				if err := validation.validateMediaTypeFeatures(item, child, version); err != nil {
					return err
				}
				continue
			}
			if key == "server" {
				if err := validation.validateServerObjectFields(item, child, version); err != nil {
					return err
				}
			}
			if key == "responses" {
				if err := validation.validateResponseFields(item, child, version); err != nil {
					return err
				}
				continue
			}
			if key == "schema" {
				if err := validation.validateSchemaVersion(item, child, version); err != nil {
					return err
				}
				continue
			}
			if err := validation.validateSchemaValues(item, child, version); err != nil {
				return err
			}
		}
	case []any:
		for index, item := range typed {
			if err := validation.validateSchemaValues(item, pointerFrom(path, fmt.Sprint(index)), version); err != nil {
				return err
			}
		}
	}
	return nil
}

func (validation *versionValidation) validateResponseFields(value any, path string, version VersionLine) error {
	responses, _ := value.(map[string]any)
	for _, status := range sortedKeys(responses) {
		response, _ := responses[status].(map[string]any)
		if version != Version32 {
			if _, exists := response["summary"]; exists {
				if err := validation.reject(pointerFrom(path, status, "summary"), "response.summary requires OpenAPI 3.2"); err != nil {
					return err
				}
			}
		}
		if err := validation.validateSchemaValues(response, pointerFrom(path, status), version); err != nil {
			return err
		}
	}
	return nil
}

func (validation *versionValidation) validateMediaTypeFeatures(value any, path string, version VersionLine) error {
	mediaTypes, _ := value.(map[string]any)
	for _, mediaType := range sortedKeys(mediaTypes) {
		media, _ := mediaTypes[mediaType].(map[string]any)
		mediaPath := pointerFrom(path, mediaType)
		for _, key := range []string{"itemSchema", "prefixEncoding", "itemEncoding"} {
			if _, exists := media[key]; exists && version != Version32 {
				if err := validation.reject(pointerFrom(mediaPath, key), key+" requires OpenAPI 3.2"); err != nil {
					return err
				}
			}
		}
		for _, key := range sortedKeys(media) {
			if strings.HasPrefix(key, "x-") {
				continue
			}
			if key == "examples" {
				if err := validation.validateExamplesFields(media[key], pointerFrom(mediaPath, key), version); err != nil {
					return err
				}
				continue
			}
			if isLiteralOpenAPIValue(key) {
				continue
			}
			item := media[key]
			child := pointerFrom(mediaPath, key)
			if key == "schema" || key == "itemSchema" {
				if err := validation.validateSchemaVersion(item, child, version); err != nil {
					return err
				}
				continue
			}
			if err := validation.validateSchemaValues(item, child, version); err != nil {
				return err
			}
		}
	}
	return nil
}

func (validation *versionValidation) validateSchemaVersion(value any, path string, version VersionLine) error {
	if version == Version32 {
		return validation.validateOpenAPI32XMLCompatibility(value, path)
	}
	if version == Version31 {
		return validation.validateOpenAPI32SchemaFields(value, path)
	}
	if version != Version30 {
		return nil
	}
	if err := validation.validateOpenAPI32SchemaFields(value, path); err != nil {
		return err
	}
	schema, _ := value.(map[string]any)
	for _, key := range []string{"exclusiveMaximum", "exclusiveMinimum"} {
		if value, exists := schema[key]; exists {
			if _, isBoolean := value.(bool); !isBoolean {
				if err := validation.reject(pointerFrom(path, key), key+" must be boolean in OpenAPI 3.0"); err != nil {
					return err
				}
			}
		}
	}
	if _, isTypeArray := schema["type"].([]any); isTypeArray {
		if err := validation.reject(pointerFrom(path, "type"), "type arrays require OpenAPI 3.1 or later"); err != nil {
			return err
		}
	}
	for _, key := range sortedKeys(schema) {
		if openAPI31SchemaKeywords[key] {
			if err := validation.reject(pointerFrom(path, key), key+" requires OpenAPI 3.1 or later"); err != nil {
				return err
			}
		}
	}
	return validation.validateSchemaVersionChildren(schema, path)
}

func (validation *versionValidation) validateOpenAPI32SchemaFields(value any, path string) error {
	schema, _ := value.(map[string]any)
	if discriminator, _ := schema["discriminator"].(map[string]any); discriminator != nil {
		if _, exists := discriminator["defaultMapping"]; exists {
			if err := validation.reject(pointerFrom(path, "discriminator", "defaultMapping"), "discriminator.defaultMapping requires OpenAPI 3.2"); err != nil {
				return err
			}
		}
	}
	if xml, _ := schema["xml"].(map[string]any); xml != nil {
		if _, exists := xml["nodeType"]; exists {
			if err := validation.reject(pointerFrom(path, "xml", "nodeType"), "xml.nodeType requires OpenAPI 3.2"); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"additionalProperties", "contains", "contentSchema", "else", "if", "items", "not", "propertyNames", "then", "unevaluatedItems", "unevaluatedProperties"} {
		if nested, exists := schema[key]; exists {
			if err := validation.validateOpenAPI32SchemaFields(nested, pointerFrom(path, key)); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf", "prefixItems"} {
		values, _ := schema[key].([]any)
		for index, nested := range values {
			if err := validation.validateOpenAPI32SchemaFields(nested, pointerFrom(path, key, fmt.Sprint(index))); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"$defs", "dependentSchemas", "patternProperties", "properties"} {
		values, _ := schema[key].(map[string]any)
		for _, name := range sortedKeys(values) {
			if err := validation.validateOpenAPI32SchemaFields(values[name], pointerFrom(path, key, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (validation *versionValidation) validateOpenAPI32XMLCompatibility(value any, path string) error {
	schema, _ := value.(map[string]any)
	if xml, _ := schema["xml"].(map[string]any); xml != nil {
		if _, hasNodeType := xml["nodeType"]; hasNodeType {
			for _, legacy := range []string{"attribute", "wrapped"} {
				if _, exists := xml[legacy]; exists {
					if err := validation.reject(pointerFrom(path, "xml", legacy), "xml."+legacy+" must not be present when xml.nodeType is present in OpenAPI 3.2"); err != nil {
						return err
					}
				}
			}
		}
	}
	for _, key := range []string{"additionalProperties", "contains", "contentSchema", "else", "if", "items", "not", "propertyNames", "then", "unevaluatedItems", "unevaluatedProperties"} {
		if nested, exists := schema[key]; exists {
			if err := validation.validateOpenAPI32XMLCompatibility(nested, pointerFrom(path, key)); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf", "prefixItems"} {
		values, _ := schema[key].([]any)
		for index, nested := range values {
			if err := validation.validateOpenAPI32XMLCompatibility(nested, pointerFrom(path, key, fmt.Sprint(index))); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"$defs", "dependentSchemas", "patternProperties", "properties"} {
		values, _ := schema[key].(map[string]any)
		for _, name := range sortedKeys(values) {
			if err := validation.validateOpenAPI32XMLCompatibility(values[name], pointerFrom(path, key, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (validation *versionValidation) validateOpenAPI30ReferenceObjects(value any, path string) error {
	switch typed := value.(type) {
	case map[string]any:
		if reference, _ := typed["$ref"].(string); reference != "" {
			if !isPathItemReferencePath(path) {
				for _, key := range sortedKeys(typed) {
					if key != "$ref" && !strings.HasPrefix(key, "x-") {
						if err := validation.reject(pointerFrom(path, key), "Reference Object siblings require OpenAPI 3.1 or later"); err != nil {
							return err
						}
					}
				}
			}
		}
		for _, key := range sortedKeys(typed) {
			if strings.HasPrefix(key, "x-") {
				continue
			}
			if isLiteralOpenAPIValue(key) {
				continue
			}
			if err := validation.validateOpenAPI30ReferenceObjects(typed[key], pointerFrom(path, key)); err != nil {
				return err
			}
		}
	case []any:
		for index, item := range typed {
			if err := validation.validateOpenAPI30ReferenceObjects(item, pointerFrom(path, fmt.Sprint(index))); err != nil {
				return err
			}
		}
	}
	return nil
}

func isPathItemReferencePath(path string) bool {
	segments := strings.Split(strings.TrimPrefix(path, "#/"), "/")
	if len(segments) == 2 && (segments[0] == "paths" || segments[0] == "webhooks") {
		return true
	}
	if len(segments) == 3 && segments[0] == "components" && segments[1] == "pathItems" {
		return true
	}
	for index := len(segments) - 1; index >= 0; index-- {
		segment := segments[index]
		if segment == "callbacks" {
			// A callback map's direct value is a Path Item. Any deeper segment is
			// an operation, response, or another nested object and uses a Reference
			// Object, whose 3.0 siblings must be rejected.
			return len(segments) == index+3
		}
	}
	return false
}

func isLiteralOpenAPIValue(key string) bool {
	switch key {
	case "const", "default", "enum", "example", "value", "dataValue", "serializedValue":
		return true
	default:
		return false
	}
}

func (validation *versionValidation) validateSchemaVersionChildren(schema map[string]any, path string) error {
	for _, key := range []string{"additionalProperties", "contains", "contentSchema", "else", "if", "items", "not", "propertyNames", "then", "unevaluatedItems", "unevaluatedProperties"} {
		if value, exists := schema[key]; exists {
			if err := validation.validateSchemaVersion(value, pointerFrom(path, key), Version30); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf", "prefixItems"} {
		values, _ := schema[key].([]any)
		for index, value := range values {
			if err := validation.validateSchemaVersion(value, pointerFrom(path, key, fmt.Sprint(index)), Version30); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"dependentSchemas", "patternProperties", "properties"} {
		values, _ := schema[key].(map[string]any)
		for _, name := range sortedKeys(values) {
			if err := validation.validateSchemaVersion(values[name], pointerFrom(path, key, name), Version30); err != nil {
				return err
			}
		}
	}
	return nil
}

var openAPI31SchemaKeywords = map[string]bool{
	"$anchor": true, "$comment": true, "$defs": true, "$dynamicAnchor": true,
	"$dynamicRef": true, "$id": true, "$schema": true, "$vocabulary": true,
	"const": true, "contains": true, "contentEncoding": true, "contentMediaType": true,
	"contentSchema": true, "dependentRequired": true, "dependentSchemas": true,
	"else": true, "examples": true, "if": true, "maxContains": true, "minContains": true,
	"patternProperties": true, "prefixItems": true, "propertyNames": true, "then": true,
	"unevaluatedItems": true, "unevaluatedProperties": true,
}

type VersionFeatureError struct {
	Pointer string
	Detail  string
}

func (value *VersionFeatureError) Error() string {
	return fmt.Sprintf("OpenAPI version feature at %s: %s", value.Pointer, value.Detail)
}

func (value *VersionFeatureError) DiagnosticPointer() string   { return value.Pointer }
func (value *VersionFeatureError) CompatibilityRule() string   { return "COMP-VERSION-003" }
func (value *VersionFeatureError) CompatibilityAction() string { return "reject" }

func pointer(parts ...string) string { return pointerFrom("#", parts...) }

func pointerFrom(path string, parts ...string) string {
	for _, part := range parts {
		path += "/" + strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
	}
	return path
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
