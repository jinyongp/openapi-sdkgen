package typescript

import (
	"fmt"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

type operationSecurityRequirement struct {
	id      string
	schemes []ir.SecurityRequirementScheme
}

func operationSecurityRequirements(document *ir.Document, operation ir.Operation) ([]operationSecurityRequirement, bool, error) {
	if operation.Parameters != nil {
		return plannedSecurityRequirements(operation.Security), len(operation.Security) > 0, nil
	}
	return syntheticOperationSecurityRequirements(document, operation)
}

func plannedSecurityRequirements(values []ir.SecurityRequirement) []operationSecurityRequirement {
	result := make([]operationSecurityRequirement, 0, len(values))
	ids := make(map[string]int, len(values))
	for _, value := range values {
		names := make([]string, 0, len(value.Schemes))
		for _, scheme := range value.Schemes {
			name := scheme.Name
			if scheme.Reference != "" {
				name = scheme.Reference
			}
			names = append(names, name)
		}
		id := "anonymous"
		if len(names) > 0 {
			id = strings.Join(names, "__")
		}
		baseID := id
		if count := ids[baseID]; count > 0 {
			id = fmt.Sprintf("%s__%d", baseID, count+1)
		}
		ids[baseID]++
		result = append(result, operationSecurityRequirement{id: id, schemes: append([]ir.SecurityRequirementScheme(nil), value.Schemes...)})
	}
	return result
}

func syntheticOperationSecurityRequirements(document *ir.Document, operation ir.Operation) ([]operationSecurityRequirement, bool, error) {
	value, exists := operation.Raw["security"]
	if !exists {
		value, exists = document.Raw["security"]
	}
	if !exists {
		return nil, false, nil
	}
	allowURI := document.OpenAPIVersionLine == "3.2" || strings.HasPrefix(stringMapValue(document.Raw, "openapi"), "3.2.")
	requirements, err := ir.ReadSecurityRequirements(document.Raw, value, allowURI)
	if err != nil {
		return nil, false, err
	}
	if len(requirements) == 0 {
		return nil, false, nil
	}
	return plannedSecurityRequirements(requirements), true, nil
}

func operationRequiresSecuritySelection(document *ir.Document, operation ir.Operation) (bool, error) {
	requirements, _, err := operationSecurityRequirements(document, operation)
	if err != nil {
		return false, err
	}
	return len(requirements) > 1, nil
}

// operationSecurityDefinition lowers an operation's effective OpenAPI Security
// Requirement Object. An absent operation field inherits the root field;
// explicit `security: []` disables that inheritance.
func operationSecurityDefinition(document *ir.Document, operation ir.Operation) (string, bool, error) {
	requirements, hasSecurity, err := operationSecurityRequirements(document, operation)
	if err != nil || !hasSecurity {
		return "", hasSecurity, err
	}
	entries := make([]string, 0, len(requirements))
	for index, requirement := range requirements {
		definitions := make([]string, 0, len(requirement.schemes))
		for _, requested := range requirement.schemes {
			if requested.ResolutionError != "" {
				return "", false, fmt.Errorf("%s", requested.ResolutionError)
			}
			scheme, ok := securityScheme(document, requested.Name)
			if !ok {
				return "", false, fmt.Errorf("security requirement %d references unknown scheme %q", index, requested.Name)
			}
			definition, err := securitySchemeDefinition(scheme, requested.Scopes)
			if err != nil {
				return "", false, err
			}
			definitions = append(definitions, definition)
		}
		entries = append(entries, "{ id: "+quoteTS(requirement.id)+", schemes: ["+strings.Join(definitions, ", ")+"] }")
	}
	return "[" + strings.Join(entries, ", ") + "]", true, nil
}

func securityScheme(document *ir.Document, name string) (ir.SecurityScheme, bool) {
	if document.SecuritySchemes != nil {
		scheme, ok := document.SecuritySchemes[name]
		if ok {
			return scheme, true
		}
	}
	allowURI := document.OpenAPIVersionLine == "3.2" || strings.HasPrefix(stringMapValue(document.Raw, "openapi"), "3.2.")
	scheme, err := ir.ResolveSecurityScheme(document.Raw, name, allowURI)
	return scheme, err == nil
}

func securitySchemeDefinition(scheme ir.SecurityScheme, scopes []string) (string, error) {
	if scheme.ResolutionError != "" {
		return "", fmt.Errorf("%s", scheme.ResolutionError)
	}
	name := scheme.Name
	kind := scheme.Type
	if kind == "" {
		return "", fmt.Errorf("security scheme %q is missing type", name)
	}
	fields := []string{"name: " + quoteTS(name), "type: " + quoteTS(kind)}
	switch kind {
	case "apiKey":
		if scheme.Location != "header" && scheme.Location != "query" && scheme.Location != "cookie" {
			return "", fmt.Errorf("apiKey security scheme %q has unsupported location %q", name, scheme.Location)
		}
		if scheme.ParameterName == "" {
			return "", fmt.Errorf("apiKey security scheme %q is missing name", name)
		}
		fields = append(fields, "location: "+quoteTS(scheme.Location), "parameterName: "+quoteTS(scheme.ParameterName))
	case "http":
		if scheme.Scheme == "" {
			return "", fmt.Errorf("http security scheme %q is missing scheme", name)
		}
		fields = append(fields, "scheme: "+quoteTS(strings.ToLower(scheme.Scheme)))
	case "oauth2", "openIdConnect", "mutualTLS":
		// Flow/discovery metadata is preserved in metadata.js. Runtime credential
		// application only needs the standard scheme kind and requested scopes.
	default:
		return "", fmt.Errorf("security scheme %q has unsupported type %q", name, kind)
	}
	if scheme.BearerFormat != "" {
		fields = append(fields, "bearerFormat: "+quoteTS(scheme.BearerFormat))
	}
	if scheme.Flows != nil {
		encoded, err := runtimeJSONExpression(scheme.Flows)
		if err != nil {
			return "", fmt.Errorf("security scheme %q flows: %w", name, err)
		}
		fields = append(fields, "flows: "+encoded)
	}
	if scheme.OpenIDConnectURL != "" {
		fields = append(fields, "openIdConnectUrl: "+quoteTS(scheme.OpenIDConnectURL))
	}
	if scheme.OAuth2MetadataURL != "" {
		fields = append(fields, "oauth2MetadataUrl: "+quoteTS(scheme.OAuth2MetadataURL))
	}
	if scheme.Deprecated {
		fields = append(fields, "deprecated: true")
	}
	if len(scopes) > 0 {
		values := make([]string, 0, len(scopes))
		for _, scope := range scopes {
			values = append(values, quoteTS(scope))
		}
		sort.Strings(values)
		fields = append(fields, "scopes: ["+strings.Join(values, ", ")+"]")
	}
	return "{ " + strings.Join(fields, ", ") + " }", nil
}

func inboundSecurityRequirements(document *ir.Document, value any) ([]ir.SecurityRequirement, error) {
	allowURI := document.OpenAPIVersionLine == "3.2" || strings.HasPrefix(stringMapValue(document.Raw, "openapi"), "3.2.")
	requirements, err := ir.ReadSecurityRequirements(document.Raw, value, allowURI)
	if err != nil {
		return nil, err
	}
	for _, requirement := range requirements {
		for _, requested := range requirement.Schemes {
			if requested.ResolutionError != "" {
				return nil, fmt.Errorf("%s", requested.ResolutionError)
			}
			scheme, exists := securityScheme(document, requested.Name)
			if !exists {
				return nil, fmt.Errorf("unknown security scheme %q", requested.Name)
			}
			if _, err := securitySchemeDefinition(scheme, requested.Scopes); err != nil {
				return nil, err
			}
		}
	}
	return requirements, nil
}

func stringMapValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func rootSecurityValue(document *ir.Document) any {
	if document.SecuritySchemes != nil {
		if document.Security == nil {
			return nil
		}
		return securityRequirementsValue(document.Security)
	}
	return document.Raw["security"]
}

func securityRequirementsValue(requirements []ir.SecurityRequirement) []any {
	values := make([]any, 0, len(requirements))
	for _, requirement := range requirements {
		if requirement.Raw != nil {
			values = append(values, requirement.Raw)
			continue
		}
		value := make(map[string]any, len(requirement.Schemes))
		for _, scheme := range requirement.Schemes {
			scopes := make([]any, 0, len(scheme.Scopes))
			for _, scope := range scheme.Scopes {
				scopes = append(scopes, scope)
			}
			value[scheme.Name] = scopes
		}
		values = append(values, value)
	}
	return values
}

func securitySchemesValue(document *ir.Document) map[string]any {
	if document.SecuritySchemes != nil {
		values := make(map[string]any, len(document.SecuritySchemes))
		for name, scheme := range document.SecuritySchemes {
			values[name] = scheme.Raw
		}
		return values
	}
	components, _ := document.Raw["components"].(map[string]any)
	values, _ := components["securitySchemes"].(map[string]any)
	return values
}
