package plan

import (
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/compiler/naming"
	"strings"
)

func ResourcePathParts(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func ResourcePathPart(part string) (string, bool, bool) {
	if !strings.ContainsAny(part, "{}") {
		return part, false, true
	}
	if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") &&
		strings.Count(part, "{") == 1 && strings.Count(part, "}") == 1 && len(part) > 2 {
		return strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}"), true, true
	}
	return "", false, false
}

// resourceTerminalName keeps literal path segments as namespaces. For example,
// POST /auth/login becomes api.auth.login.post(), rather than api.auth.login().
func ResourceTerminal(operation ir.Operation, parts []string) (string, error) {
	terminal, err := resourceTerminalName(operation, parts)
	if err != nil {
		return "", err
	}
	if len(parts) == 0 {
		return terminal, nil
	}
	last := parts[len(parts)-1]
	if strings.HasPrefix(last, "{") {
		return terminal, nil
	}
	property, err := naming.Property(last)
	if err != nil {
		return "", err
	}
	if terminal != property {
		return terminal, nil
	}
	return ResourceMethodTerminal(operation.Method)
}

func ResourceMethodTerminal(method string) (string, error) {
	switch method {
	case "GET":
		return "get", nil
	case "POST":
		return "post", nil
	case "PUT":
		return "replace", nil
	case "PATCH":
		return "patch", nil
	case "DELETE":
		return "delete", nil
	case "QUERY":
		return "query", nil
	default:
		return naming.Property(strings.ToLower(method))
	}
}

func resourceTerminalName(operation ir.Operation, parts []string) (string, error) {
	last := ""
	if len(parts) > 0 {
		last = parts[len(parts)-1]
	}
	if isResourceTerminalAction(operation, parts, len(parts)-1) {
		return naming.Property(last)
	}
	lowerID := strings.ToLower(operation.OperationID)
	switch operation.Method {
	case "GET":
		if strings.HasPrefix(lowerID, "list") || strings.HasPrefix(lowerID, "search") {
			return "list", nil
		}
		return "get", nil
	case "POST":
		if strings.HasPrefix(lowerID, "create") {
			return "create", nil
		}
		return "post", nil
	case "PUT":
		return "replace", nil
	case "PATCH":
		return "patch", nil
	case "DELETE":
		return "delete", nil
	case "QUERY":
		return "query", nil
	default:
		return naming.Property(strings.ToLower(operation.Method))
	}
}

func isResourceTerminalAction(operation ir.Operation, parts []string, index int) bool {
	if index < 0 || index != len(parts)-1 || operation.Method == "GET" || len(parts) < 2 {
		return false
	}
	last := parts[index]
	if last == "" || strings.HasPrefix(last, "{") {
		return false
	}
	return strings.Contains(operation.OperationID, strings.ToUpper(last[:1])+last[1:]) || !strings.HasPrefix(strings.ToLower(operation.OperationID), "create")
}
