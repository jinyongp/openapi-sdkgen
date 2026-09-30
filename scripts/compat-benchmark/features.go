package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"openapi-sdkgen/internal/openapiwalk"

	yaml "go.yaml.in/yaml/v4"
)

var benchmarkFeatures = []string{
	"operation.query",
	"parameter.querystring",
	"document.media-types",
	"media.item-schema",
	"media.jsonl",
	"media.json-seq",
	"media.multipart-positional-encoding",
	"schema.discriminator-default-mapping",
	"security.oauth2-metadata",
	"security.device-authorization",
	"security.deprecated",
	"security.requirement-uri",
	"schema.xml-node-type",
	"oas.version.3.0",
	"oas.version.3.1",
	"oas.version.3.2",
	"reference.local",
	"reference.external",
	"operation.request-body",
	"operation.required-request-body",
	"operation.callbacks",
	"operation.servers",
	"document.webhooks",
	"response.links",
	"media.multipart",
	"media.url-encoded",
	"media.binary",
	"media.encoding",
	"security.api-key",
	"security.http",
	"security.oauth2",
	"security.openid-connect",
	"schema.allOf",
	"schema.oneOf",
	"schema.anyOf",
	"schema.not",
	"schema.discriminator",
	"schema.additional-properties.boolean",
	"schema.additional-properties.schema",
	"schema.type-array",
	"schema.const",
	"schema.null-type",
	"schema.prefix-items",
	"schema.unevaluated-properties",
}

func featureCatalog() []string {
	return append([]string(nil), benchmarkFeatures...)
}

func decodeFeatureInput(data []byte) (any, error) {
	var value any
	if json.Valid(data) {
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		return value, nil
	}
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func detectFeatures(value any) ([]string, string) {
	found := map[string]bool{}
	root, _ := value.(map[string]any)
	version, _ := root["openapi"].(string)
	switch {
	case strings.HasPrefix(version, "3.0"):
		found["oas.version.3.0"] = true
	case strings.HasPrefix(version, "3.1"):
		found["oas.version.3.1"] = true
	case strings.HasPrefix(version, "3.2"):
		found["oas.version.3.2"] = true
	}
	if webhooks, ok := root["webhooks"].(map[string]any); ok && len(webhooks) != 0 {
		found["document.webhooks"] = true
	}
	if components, _ := root["components"].(map[string]any); components != nil {
		if mediaTypes, _ := components["mediaTypes"].(map[string]any); len(mediaTypes) != 0 {
			found["document.media-types"] = true
		}
	}
	walkFeatures(value, nil, found)

	result := make([]string, 0, len(found))
	for _, feature := range benchmarkFeatures {
		if found[feature] {
			result = append(result, feature)
		}
	}
	return result, version
}

func walkFeatures(value any, path []string, found map[string]bool) {
	switch typed := value.(type) {
	case map[string]any:
		position := openapiwalk.StructuralPositionAt(path)
		context := position.Object

		if reference, ok := typed["$ref"].(string); ok && context != openapiwalk.ObjectUnknown {
			if strings.HasPrefix(reference, "#") {
				found["reference.local"] = true
			} else if reference != "" {
				found["reference.external"] = true
			}
		}

		switch context {
		case openapiwalk.ObjectOperation:
			if len(path) > 0 && path[len(path)-1] == "query" {
				found["operation.query"] = true
			}
			if requestBody, exists := typed["requestBody"]; exists {
				found["operation.request-body"] = true
				if body, ok := requestBody.(map[string]any); ok {
					if required, _ := body["required"].(bool); required {
						found["operation.required-request-body"] = true
					}
				}
			}
			if callbacks, ok := typed["callbacks"].(map[string]any); ok && len(callbacks) != 0 {
				found["operation.callbacks"] = true
			}
			if servers, ok := typed["servers"].([]any); ok && len(servers) != 0 {
				found["operation.servers"] = true
			}
		case openapiwalk.ObjectResponse:
			if links, ok := typed["links"].(map[string]any); ok && len(links) != 0 {
				found["response.links"] = true
			}
		case openapiwalk.ObjectMediaType:
			if _, exists := typed["itemSchema"]; exists {
				found["media.item-schema"] = true
			}
			if _, exists := typed["prefixEncoding"]; exists {
				found["media.multipart-positional-encoding"] = true
			}
			if _, exists := typed["itemEncoding"]; exists {
				found["media.multipart-positional-encoding"] = true
			}
			mediaType := ""
			if len(path) != 0 {
				mediaType = strings.ToLower(path[len(path)-1])
			}
			if strings.HasPrefix(mediaType, "multipart/") {
				found["media.multipart"] = true
			}
			if mediaType == "application/x-www-form-urlencoded" {
				found["media.url-encoded"] = true
			}
			if mediaType == "application/jsonl" {
				found["media.jsonl"] = true
			}
			if mediaType == "application/json-seq" {
				found["media.json-seq"] = true
			}
			if _, exists := typed["encoding"]; exists {
				found["media.encoding"] = true
			}
		case openapiwalk.ObjectSecurityScheme:
			if _, exists := typed["oauth2MetadataUrl"]; exists {
				found["security.oauth2-metadata"] = true
			}
			if deprecated, _ := typed["deprecated"].(bool); deprecated {
				found["security.deprecated"] = true
			}
			if flows, _ := typed["flows"].(map[string]any); flows != nil {
				if _, exists := flows["deviceAuthorization"]; exists {
					found["security.device-authorization"] = true
				}
			}
			typeName, _ := typed["type"].(string)
			switch typeName {
			case "apiKey":
				found["security.api-key"] = true
			case "http":
				found["security.http"] = true
			case "oauth2":
				found["security.oauth2"] = true
			case "openIdConnect":
				found["security.openid-connect"] = true
			}
		case openapiwalk.ObjectSchema:
			detectSchemaFeatures(typed, found)
		case openapiwalk.ObjectParameter:
			if typed["in"] == "querystring" {
				found["parameter.querystring"] = true
			}
		}
		if len(path) >= 2 && path[len(path)-2] == "security" && (len(path) == 2 || openapiwalk.ObjectContextAt(path[:len(path)-2]) == openapiwalk.ObjectOperation) {
			for name := range typed {
				if strings.HasPrefix(name, "https://") || strings.HasPrefix(name, "http://") {
					found["security.requirement-uri"] = true
				}
			}
		}

		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := typed[key]
			if openapiwalk.ReferenceChildOpaque(path, key, child) {
				continue
			}
			walkFeatures(child, append(path, key), found)
		}
	case []any:
		for index, item := range typed {
			walkFeatures(item, append(path, fmt.Sprintf("%d", index)), found)
		}
	}
}

func detectSchemaFeatures(schema map[string]any, found map[string]bool) {
	if discriminator, _ := schema["discriminator"].(map[string]any); discriminator != nil {
		if _, exists := discriminator["defaultMapping"]; exists {
			found["schema.discriminator-default-mapping"] = true
		}
	}
	if xml, _ := schema["xml"].(map[string]any); xml != nil {
		if _, exists := xml["nodeType"]; exists {
			found["schema.xml-node-type"] = true
		}
	}
	if _, exists := schema["allOf"]; exists {
		found["schema.allOf"] = true
	}
	if _, exists := schema["oneOf"]; exists {
		found["schema.oneOf"] = true
	}
	if _, exists := schema["anyOf"]; exists {
		found["schema.anyOf"] = true
	}
	if _, exists := schema["not"]; exists {
		found["schema.not"] = true
	}
	if _, exists := schema["discriminator"]; exists {
		found["schema.discriminator"] = true
	}
	if additional, exists := schema["additionalProperties"]; exists {
		switch additional.(type) {
		case bool:
			found["schema.additional-properties.boolean"] = true
		case map[string]any:
			found["schema.additional-properties.schema"] = true
		}
	}
	if _, exists := schema["const"]; exists {
		found["schema.const"] = true
	}
	if _, exists := schema["prefixItems"]; exists {
		found["schema.prefix-items"] = true
	}
	if _, exists := schema["unevaluatedProperties"]; exists {
		found["schema.unevaluated-properties"] = true
	}
	if schemaType, exists := schema["type"]; exists {
		switch item := schemaType.(type) {
		case []any:
			found["schema.type-array"] = true
			for _, entry := range item {
				if entry == "null" {
					found["schema.null-type"] = true
				}
			}
		case string:
			if item == "null" {
				found["schema.null-type"] = true
			}
		}
	}
	if format, _ := schema["format"].(string); strings.EqualFold(format, "binary") {
		found["media.binary"] = true
	}
}
