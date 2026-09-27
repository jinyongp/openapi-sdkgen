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
			if _, exists := typed["encoding"]; exists {
				found["media.encoding"] = true
			}
		case openapiwalk.ObjectSecurityScheme:
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
