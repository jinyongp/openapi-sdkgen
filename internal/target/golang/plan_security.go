package golang

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
	httpplan "openapi-sdkgen/internal/http/plan"
)

type securityRequirementPlan struct{ ID, Name, Definition string }

func prepareOperationSecurity(document *ir.Document, operation ir.Operation) ([]securityRequirementPlan, error) {
	requirements := operation.Security
	if operation.Parameters == nil {
		value, exists := operation.Raw["security"]
		if !exists {
			value = document.Raw["security"]
		}
		var err error
		requirements, err = ir.ReadSecurityRequirements(document.Raw, value, document.OpenAPIVersionLine == "3.2")
		if err != nil {
			return nil, err
		}
	}
	result := make([]securityRequirementPlan, 0, len(requirements))
	ids := httpplan.SecurityRequirementIDs(requirements)
	used := make(map[string]bool)
	for index, requirement := range requirements {
		definitions := make([]string, 0, len(requirement.Schemes))
		for _, requested := range requirement.Schemes {
			if requested.ResolutionError != "" {
				return nil, fmt.Errorf("%s", requested.ResolutionError)
			}
			scheme, found := document.SecuritySchemes[requested.Name]
			if !found {
				var err error
				scheme, err = ir.ResolveSecurityScheme(document.Raw, requested.Name, document.OpenAPIVersionLine == "3.2")
				if err != nil {
					return nil, err
				}
			}
			if scheme.ResolutionError != "" {
				return nil, fmt.Errorf("%s", scheme.ResolutionError)
			}
			switch scheme.Type {
			case "apiKey":
				if scheme.ParameterName == "" || scheme.Location != "header" && scheme.Location != "query" && scheme.Location != "cookie" {
					return nil, fmt.Errorf("invalid apiKey security scheme %q", requested.Name)
				}
			case "http":
				if scheme.Scheme == "" {
					return nil, fmt.Errorf("HTTP security scheme %q has no scheme", requested.Name)
				}
			case "oauth2", "openIdConnect", "mutualTLS":
			default:
				return nil, fmt.Errorf("unsupported security scheme %q", requested.Name)
			}
			fields := []string{"Name:" + strconv.Quote(requested.Name), "Type:" + strconv.Quote(scheme.Type), "Location:" + strconv.Quote(scheme.Location), "ParameterName:" + strconv.Quote(scheme.ParameterName), "Scheme:" + strconv.Quote(strings.ToLower(scheme.Scheme)), "BearerFormat:" + strconv.Quote(scheme.BearerFormat), "OpenIDConnectURL:" + strconv.Quote(scheme.OpenIDConnectURL), "OAuth2MetadataURL:" + strconv.Quote(scheme.OAuth2MetadataURL), fmt.Sprintf("Deprecated:%t", scheme.Deprecated)}
			scopes := make([]string, 0, len(requested.Scopes))
			for _, scope := range requested.Scopes {
				scopes = append(scopes, strconv.Quote(scope))
			}
			fields = append(fields, "Scopes:[]string{"+strings.Join(scopes, ",")+"}")
			if scheme.Flows != nil {
				data, err := json.Marshal(scheme.Flows)
				if err != nil {
					return nil, err
				}
				fields = append(fields, "Flows:securityMetadata("+strconv.Quote(string(data))+")")
			}
			definitions = append(definitions, "{"+strings.Join(fields, ",")+"}")
		}
		id := ids[index]
		name, err := reservePublicName(used, id, fmt.Sprintf("%s/security/%d", operation.Pointer, index), "")
		if err != nil {
			return nil, err
		}
		result = append(result, securityRequirementPlan{ID: id, Name: name, Definition: "{ID:" + strconv.Quote(id) + ",Schemes:[]types.SecuritySchemeDefinition{" + strings.Join(definitions, ",") + "}}"})
	}
	return result, nil
}

func operationServers(servers []ir.Server) string {
	entries := make([]string, 0, len(servers))
	for _, server := range servers {
		variables := make([]string, 0, len(server.Variables))
		for _, variable := range server.Variables {
			values := make([]string, 0, len(variable.Enum))
			for _, value := range variable.Enum {
				values = append(values, strconv.Quote(value))
			}
			variables = append(variables, "{Name:"+strconv.Quote(variable.Name)+",Default:"+strconv.Quote(variable.Default)+",Enum:[]string{"+strings.Join(values, ",")+"}}")
		}
		entries = append(entries, "{ID:"+strconv.Quote(server.Pointer)+",URL:"+strconv.Quote(server.URL)+",Variables:[]types.ServerVariable{"+strings.Join(variables, ",")+"}}")
	}
	return "[]types.ServerDefinition{" + strings.Join(entries, ",") + "}"
}
