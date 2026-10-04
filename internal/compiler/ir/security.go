package ir

import (
	"fmt"
	"net/url"
	"strings"
)

// ReadSecurityRequirements is the common IR kernel for compiler and synthetic
// target entry points. Resolution failures retain their established ownership.
func ReadSecurityRequirements(document map[string]any, value any, allowURI bool) ([]SecurityRequirement, error) {
	values, err := readSecurityRequirements(value)
	if err != nil {
		return nil, err
	}
	return ResolveSecurityRequirements(document, values, allowURI), nil
}

// ResolveSecurityRequirements preserves raw declarations and resolves credential
// identities in the IR. Unsupported external URIs remain target-owned errors.
func ResolveSecurityRequirements(document map[string]any, values []SecurityRequirement, allowURI bool) []SecurityRequirement {
	if values == nil {
		return nil
	}
	result := make([]SecurityRequirement, len(values))
	for index, requirement := range values {
		result[index] = SecurityRequirement{Raw: requirement.Raw, Schemes: make([]SecurityRequirementScheme, len(requirement.Schemes))}
		for schemeIndex, requested := range requirement.Schemes {
			requested.ResolutionError = ""
			resolved, err := ResolveSecurityScheme(document, requested.Name, allowURI)
			if err != nil {
				requested.ResolutionError = err.Error()
			} else if resolved.Name != requested.Name {
				requested.Reference = requested.Name
				requested.Name = resolved.Name
			}
			result[index].Schemes[schemeIndex] = requested
		}
	}
	return result
}

// ResolveSecurityScheme gives declared component names precedence over URI keys.
// Fragment decoding happens once, before the existing strict JSON Pointer kernel.
// It resolves local aliases without loading another document.
func ResolveSecurityScheme(document map[string]any, key string, allowURI bool) (SecurityScheme, error) {
	components, _ := document["components"].(map[string]any)
	schemes, _ := components["securitySchemes"].(map[string]any)
	value, declared := schemes[key]
	name := key
	if !declared {
		if !allowURI {
			return SecurityScheme{}, fmt.Errorf("unknown security scheme %q", key)
		}
		var err error
		value, name, err = securitySchemeReference(document, key)
		if err != nil {
			return SecurityScheme{}, err
		}
	}
	source, ok := value.(map[string]any)
	if !ok {
		return SecurityScheme{}, fmt.Errorf("security scheme %q must be an object", key)
	}
	resolved, err := resolveSecuritySchemeObject(document, source, make(map[string]bool))
	if err != nil {
		return SecurityScheme{}, fmt.Errorf("security scheme %q: %w", key, err)
	}
	kind := stringValue(resolved, "type")
	switch kind {
	case "apiKey", "http", "oauth2", "openIdConnect", "mutualTLS":
	default:
		return SecurityScheme{}, fmt.Errorf("security scheme %q does not target a Security Scheme Object", key)
	}
	return SecurityScheme{
		Name: name, Type: kind, Location: stringValue(resolved, "in"), ParameterName: stringValue(resolved, "name"),
		Scheme: stringValue(resolved, "scheme"), BearerFormat: stringValue(resolved, "bearerFormat"),
		Flows: resolved["flows"], OpenIDConnectURL: stringValue(resolved, "openIdConnectUrl"),
		OAuth2MetadataURL: stringValue(resolved, "oauth2MetadataUrl"), Deprecated: boolValue(resolved, "deprecated"),
		Raw: resolved, SourceRaw: source,
	}, nil
}

func resolveSecuritySchemeObject(document, value map[string]any, seen map[string]bool) (map[string]any, error) {
	reference, exists := value["$ref"]
	if !exists {
		return value, nil
	}
	uri, ok := reference.(string)
	if !ok || uri == "" {
		return nil, fmt.Errorf("Security Scheme $ref must be a non-empty string")
	}
	if seen[uri] {
		return nil, fmt.Errorf("cyclic Security Scheme reference %q", uri)
	}
	seen[uri] = true
	target, _, err := securitySchemeReference(document, uri)
	if err != nil {
		return nil, err
	}
	object, ok := target.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Security Scheme reference %q must target an object", uri)
	}
	return resolveSecuritySchemeObject(document, object, seen)
}

func securitySchemeReference(document map[string]any, reference string) (any, string, error) {
	if !strings.HasPrefix(reference, "#") {
		return nil, "", fmt.Errorf("external Security Scheme URI %q is not supported", reference)
	}
	pointer, err := url.PathUnescape(strings.TrimPrefix(reference, "#"))
	if err != nil {
		return nil, "", fmt.Errorf("invalid Security Scheme URI %q: invalid URI fragment escape", reference)
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, "", fmt.Errorf("Security Scheme URI %q must use a JSON Pointer", reference)
	}
	// The generic object lookup already enforces strict ~0/~1 escapes and absence.
	value, err := localPathItemReference(document, reference)
	if err != nil {
		return nil, "", fmt.Errorf("invalid Security Scheme URI %q: %w", reference, err)
	}
	name := reference
	tokens := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
	if len(tokens) == 3 && tokens[0] == "components" && tokens[1] == "securitySchemes" {
		name, err = jsonPointerToken(tokens[2])
		if err != nil {
			return nil, "", err
		}
	}
	return value, name, nil
}
