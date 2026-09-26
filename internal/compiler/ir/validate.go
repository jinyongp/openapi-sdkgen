package ir

import (
	"fmt"
	"strings"

	openapidoc "openapi-sdkgen/internal/compiler/openapi"
)

// ValidationFinding is one author-input prerequisite failure owned by IR
// construction. Reference and target-owned failures intentionally do not
// appear here.
type ValidationFinding struct {
	Pointer string
	Err     error
}

func (value ValidationFinding) Error() string {
	if value.Err == nil {
		return ""
	}
	return value.Err.Error()
}

// ValidatePrerequisites collects independent author-input failures that must be
// absent before canonical IR construction. Build uses the same validation
// kernel so fail-fast and collect mode cannot drift.
func ValidatePrerequisites(document *openapidoc.Document) []ValidationFinding {
	if document == nil {
		return []ValidationFinding{{Pointer: "#", Err: fmt.Errorf("OpenAPI document is nil")}}
	}

	versionLine := document.Version
	if versionLine == "" {
		detected, err := openapidoc.DetectVersionLine(stringValue(document.Raw, "openapi"))
		if err != nil {
			return []ValidationFinding{{Pointer: "#/openapi", Err: err}}
		}
		versionLine = detected
	}

	var result []ValidationFinding
	result = append(result, securityValidationFindings(document.Raw["security"], "#/security", "root security")...)

	paths, err := validatedPathsObject(document.Raw)
	if err != nil {
		return append(result, ValidationFinding{Pointer: "#/paths", Err: err})
	}

	for _, path := range sortedKeys(paths) {
		if strings.HasPrefix(path, "x-") {
			continue
		}
		pathPointer := "#" + jsonPointer("paths", path)
		pathItem, err := validatedPathItemObject(paths, path)
		if err != nil {
			result = append(result, ValidationFinding{Pointer: pathPointer, Err: err})
			continue
		}

		// Path Item reference validity is reference-phase owned. If resolution
		// fails, the reference analyzer/Build path remains authoritative.
		resolved, err := ResolvePathItem(document.Raw, pathItem)
		if err != nil {
			continue
		}
		for _, method := range standardMethods {
			operation, ok := resolved[method].(map[string]any)
			if !ok {
				continue
			}
			if method == "query" && versionLine != openapidoc.Version32 {
				continue
			}
			if security, declared := operation["security"]; declared {
				pointer := "#" + jsonPointer("paths", path, method)
				result = append(result, securityValidationFindings(
					security,
					pointer+"/security",
					fmt.Sprintf("operation %s %q", strings.ToUpper(method), path),
				)...)
			}
		}

		additional, _ := resolved["additionalOperations"].(map[string]any)
		if len(additional) > 0 && versionLine != openapidoc.Version32 {
			continue
		}
		for _, method := range sortedKeys(additional) {
			operation, err := validatedAdditionalOperationObject(additional, method, path)
			pointer := "#" + jsonPointer("paths", path, "additionalOperations", method)
			if err != nil {
				result = append(result, ValidationFinding{Pointer: pointer, Err: err})
				continue
			}
			if security, declared := operation["security"]; declared {
				result = append(result, securityValidationFindings(
					security,
					pointer+"/security",
					fmt.Sprintf("additional operation %s %q", method, path),
				)...)
			}
		}
	}

	return result
}

func validatedPathsObject(raw map[string]any) (map[string]any, error) {
	if value, exists := raw["paths"]; exists {
		paths, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("OpenAPI paths must be an object")
		}
		return paths, nil
	}
	return map[string]any{}, nil
}

func validatedPathItemObject(paths map[string]any, path string) (map[string]any, error) {
	pathItem, ok := paths[path].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("path item %q must be an object", path)
	}
	return pathItem, nil
}

func validatedAdditionalOperationObject(additional map[string]any, method, path string) (map[string]any, error) {
	operation, ok := additional[method].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("additional operation %q %q must be an object", method, path)
	}
	return operation, nil
}

type securityValidationIssue struct {
	suffix string
	err    error
}

func securityValidationIssues(value any) []securityValidationIssue {
	if value == nil {
		return nil
	}
	values, ok := value.([]any)
	if !ok {
		return []securityValidationIssue{{err: fmt.Errorf("security must be an array")}}
	}

	var result []securityValidationIssue
	for index, rawRequirement := range values {
		requirement, ok := rawRequirement.(map[string]any)
		if !ok {
			result = append(result, securityValidationIssue{
				suffix: fmt.Sprintf("/%d", index),
				err:    fmt.Errorf("security requirement %d must be an object", index),
			})
			continue
		}
		for _, name := range sortedKeys(requirement) {
			schemeSuffix := fmt.Sprintf("/%d/%s", index, escapeJSONPointerToken(name))
			rawScopes, ok := requirement[name].([]any)
			if !ok {
				result = append(result, securityValidationIssue{
					suffix: schemeSuffix,
					err:    fmt.Errorf("security scheme %q scopes must be an array", name),
				})
				continue
			}
			for scopeIndex, rawScope := range rawScopes {
				if _, ok := rawScope.(string); ok {
					continue
				}
				result = append(result, securityValidationIssue{
					suffix: fmt.Sprintf("%s/%d", schemeSuffix, scopeIndex),
					err:    fmt.Errorf("security scheme %q has a non-string scope", name),
				})
			}
		}
	}
	return result
}

func securityValidationFindings(value any, pointer, prefix string) []ValidationFinding {
	issues := securityValidationIssues(value)
	result := make([]ValidationFinding, 0, len(issues))
	for _, issue := range issues {
		result = append(result, ValidationFinding{
			Pointer: pointer + issue.suffix,
			Err:     fmt.Errorf("%s: %w", prefix, issue.err),
		})
	}
	return result
}
