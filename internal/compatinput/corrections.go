package compatinput

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	compiler "openapi-sdkgen/internal/compiler"
	"openapi-sdkgen/internal/openapiwalk"
)

const (
	DigitalOceanTagsV1      = "digitalocean-tags-v1"
	SpoonacularParametersV1 = "spoonacular-parameter-bodies-v1"
)

func SupportedKind(kind string) bool {
	return kind == OperationReferencesV1 || kind == DigitalOceanTagsV1 || kind == SpoonacularParametersV1
}

// BundleWithKind preserves the original source and applies only the explicitly
// selected provider correction. The manifest pins the complete resulting bytes.
func BundleWithKind(input, kind string) ([]byte, []operationContract, error) {
	if !SupportedKind(kind) {
		return nil, nil, fmt.Errorf("unsupported input preparation %q", kind)
	}
	data, contracts, err := Bundle(input)
	if err != nil || kind == OperationReferencesV1 {
		return data, contracts, err
	}
	abs, err := filepath.Abs(input)
	if err != nil {
		return nil, nil, err
	}
	document, err := compiler.CompileInputWithOptions("-", compiler.CompileOptions{InputBase: abs, InputReader: bytes.NewReader(data)})
	if err != nil {
		return nil, nil, err
	}
	switch kind {
	case DigitalOceanTagsV1:
		err = correctTagPatterns(document.Raw)
	case SpoonacularParametersV1:
		err = removeParameterBodies(document.Raw)
	}
	if err != nil {
		return nil, nil, err
	}
	state := &bundler{root: filepath.Dir(abs), entry: abs, active: map[string]bool{}, document: document.Raw}
	contracts, err = state.contracts(document.Raw)
	if err != nil {
		return nil, nil, err
	}
	data, err = json.Marshal(document.Raw)
	return append(data, '\n'), contracts, err
}

func correctTagPatterns(root map[string]any) error {
	count := 0
	var visit func(any, []string)
	visit = func(value any, path []string) {
		switch object := value.(type) {
		case map[string]any:
			if openapiwalk.ObjectContextAt(path) == openapiwalk.ObjectSchema && object["pattern"] == `^[a-zA-Z0-9_\-\:]+$` {
				// A colon is literal in this class; escaping it is invalid with u.
				object["pattern"] = `^[a-zA-Z0-9_\-:]+$`
				count++
			}
			for key, child := range object {
				if !openapiwalk.ReferenceChildOpaque(path, key, child) {
					visit(child, append(path, key))
				}
			}
		case []any:
			for index, child := range object {
				visit(child, append(path, fmt.Sprint(index)))
			}
		}
	}
	visit(root, nil)
	if count == 0 {
		return fmt.Errorf("DigitalOcean tag correction did not match the pinned pattern")
	}
	return nil
}

func containedObject(root map[string]any, value any) (map[string]any, error) {
	seen := map[string]bool{}
	for {
		object, _ := value.(map[string]any)
		reference, _ := object["$ref"].(string)
		if reference == "" {
			return object, nil
		}
		if !strings.HasPrefix(reference, "#/") || seen[reference] {
			return nil, fmt.Errorf("provider correction requires contained acyclic references")
		}
		seen[reference] = true
		var err error
		parsed, err := url.Parse(reference)
		if err != nil {
			return nil, err
		}
		value, err = pointer(root, parsed.Fragment)
		if err != nil {
			return nil, err
		}
	}
}

func parameterShape(value any) string {
	object, _ := value.(map[string]any)
	copy := make(map[string]any, len(object))
	for key, child := range object {
		switch key {
		case "description", "example", "examples", "title", "deprecated", "externalDocs":
		default:
			copy[key] = child
		}
	}
	data, _ := json.Marshal(copy)
	return string(data)
}

// The pinned Spoonacular source repeats path/query parameters in an invalid
// empty-media body. Remove only fields already represented by those parameters.
func removeParameterBodies(root map[string]any) error {
	paths, _ := root["paths"].(map[string]any)
	count := 0
	for path, raw := range paths {
		item, _ := raw.(map[string]any)
		for _, method := range []string{"get", "post", "put", "patch", "delete", "head", "options", "trace"} {
			operation, _ := item[method].(map[string]any)
			body, _ := operation["requestBody"].(map[string]any)
			content, _ := body["content"].(map[string]any)
			placeholder, exists := content[""]
			if !exists {
				continue
			}
			parameters := map[string]map[string]any{}
			for _, owner := range []map[string]any{item, operation} {
				entries, _ := owner["parameters"].([]any)
				for _, entry := range entries {
					parameter, err := containedObject(root, entry)
					if err != nil {
						return err
					}
					if parameter["in"] == "path" || parameter["in"] == "query" {
						name, _ := parameter["name"].(string)
						parameters[name] = parameter
					}
				}
			}
			media, _ := placeholder.(map[string]any)
			schema, _ := media["schema"].(map[string]any)
			properties, _ := schema["properties"].(map[string]any)
			if len(media) != 1 || schema["type"] != "object" || len(properties) == 0 && len(content) == 1 {
				return fmt.Errorf("empty media is not a redundant parameter body at %s %s", method, path)
			}
			for key := range schema {
				switch key {
				case "type", "properties", "required", "description", "title":
				default:
					return fmt.Errorf("empty-media body has additional constraint %q at %s %s", key, method, path)
				}
			}
			for name, property := range properties {
				parameter := parameters[name]
				if parameter == nil || parameterShape(property) != parameterShape(parameter["schema"]) {
					return fmt.Errorf("empty-media field %q has no equivalent path/query parameter at %s %s", name, method, path)
				}
			}
			required, _ := schema["required"].([]any)
			for _, raw := range required {
				name, _ := raw.(string)
				if properties[name] == nil || parameters[name] == nil || parameters[name]["required"] != true {
					return fmt.Errorf("empty-media required field %q is not a required parameter at %s %s", name, method, path)
				}
			}
			delete(content, "")
			if len(content) == 0 {
				delete(operation, "requestBody")
			}
			count++
		}
	}
	if count == 0 {
		return fmt.Errorf("Spoonacular parameter body correction matched no empty media")
	}
	return nil
}
