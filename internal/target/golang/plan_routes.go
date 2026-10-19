package golang

import (
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

func routeMethodName(operation ir.Operation) string {
	var name strings.Builder
	name.WriteString(exportedName(strings.ToLower(operation.Method)))
	for _, part := range strings.Split(strings.Trim(operation.Path, "/"), "/") {
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && strings.Count(part, "{") == 1 && strings.Count(part, "}") == 1 {
			parameter := strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
			if len(parameter) >= 2 && strings.EqualFold(parameter[len(parameter)-2:], "id") {
				parameter = "ID"
			}
			name.WriteString("By")
			name.WriteString(exportedName(parameter))
		} else {
			name.WriteString(exportedName(part))
		}
	}
	return name.String()
}
