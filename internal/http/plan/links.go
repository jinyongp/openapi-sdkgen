package plan

import (
	"errors"
	"fmt"
	"net/url"
	"openapi-sdkgen/internal/compiler/ir"
	"openapi-sdkgen/internal/failure"
	"regexp"
	"sort"
	"strings"
)

type LinkDefinitionPointers struct{ Parameters, RequestBody string }
type ResponseLink struct {
	SourceOperation, TargetOperation ir.Operation
	Status, Name, ServerURL          string
	Definition                       any
	Object                           map[string]any
	Pointers                         LinkDefinitionPointers
}
type LinkPreparationFailure struct {
	SourceOperation ir.Operation
	Pointer, Name   string
	Err             error
	Blocking        bool
}

func (failure LinkPreparationFailure) Error() string {
	if failure.Err == nil {
		return "response link preparation failed"
	}
	return failure.Err.Error()
}
func newLinkFailure(source ir.Operation, pointer, name string, err error, blocking bool) LinkPreparationFailure {
	return LinkPreparationFailure{SourceOperation: source, Pointer: pointer, Name: name, Err: err, Blocking: blocking}
}

type LinkPlanner struct {
	Visible    map[string]bool
	Ownership  *OwnershipIndex
	Responses  func(*ir.Document, ir.Operation) ([]ir.Response, error)
	Definition func(ir.Operation, ir.Operation, map[string]any, LinkDefinitionPointers) (any, error)
}

func (planner *LinkPlanner) responses(document *ir.Document, operation ir.Operation) ([]ir.Response, error) {
	if planner.Responses != nil {
		return planner.Responses(document, operation)
	}
	return operation.Responses, nil
}
func operationRouteKey(operation ir.Operation) string {
	if operation.RouteKey != "" {
		return operation.RouteKey
	}
	return operation.Method + " " + operation.Path
}
func operationLabel(operation ir.Operation) string {
	if operation.OperationID != "" {
		return operation.OperationID
	}
	return operationRouteKey(operation)
}
func sortedLinkKeys(values map[string]any) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

var serverVariablePattern = regexp.MustCompile(`\{[^{}]+\}`)

func (planner *LinkPlanner) Prepare(document *ir.Document) ([]ResponseLink, []LinkPreparationFailure) {
	visible := planner.Visible
	ownership := planner.Ownership
	if ownership == nil {
		ownership = NewOwnershipIndex(document)
	}
	var result []ResponseLink
	var failures []LinkPreparationFailure
	for _, source := range document.Operations {
		sourceVisible := visible[operationRouteKey(source)]
		if !sourceVisible {
			continue
		}
		responses, err := planner.responses(document, source)
		if err != nil {
			failures = append(failures, newLinkFailure(source, source.Pointer+"/responses", "", fmt.Errorf("responses %s: %w", operationLabel(source), err), true))
			continue
		}
		for _, response := range responses {
			status := response.Status
			linksPointer, err := ComponentObjectFieldPointer(document, response.SourceRaw, "responses", response.Pointer, "links")
			if err != nil {
				failures = append(failures, newLinkFailure(source, response.Pointer+"/links", "", fmt.Errorf("response %s %s: %w", operationLabel(source), status, err), true))
				continue
			}
			links, _ := response.Raw["links"].(map[string]any)
			for _, name := range sortedLinkKeys(links) {
				linkPointer := linksPointer + "/" + PointerToken(name)
				if linkCapabilityRestricted(ownership.RestrictionsAt(document, linkPointer)) {
					continue
				}
				link, _ := links[name].(map[string]any)
				parametersPointer, err := ComponentObjectFieldPointer(document, link, "links", linkPointer, "parameters")
				if err != nil {
					failures = append(failures, newLinkFailure(source, linkPointer, name, fmt.Errorf("response link %s %s: %w", operationLabel(source), name, err), true))
					continue
				}
				requestBodyPointer, err := ComponentObjectFieldPointer(document, link, "links", linkPointer, "requestBody")
				if err != nil {
					failures = append(failures, newLinkFailure(source, linkPointer, name, fmt.Errorf("response link %s %s: %w", operationLabel(source), name, err), true))
					continue
				}
				operationRefPointer, err := ComponentObjectFieldPointer(document, link, "links", linkPointer, "operationRef")
				if err != nil {
					failures = append(failures, newLinkFailure(source, linkPointer, name, err, true))
					continue
				}
				link, err = ResolveComponentObject(document, link, "links")
				if err != nil {
					failures = append(failures, newLinkFailure(source, linkPointer, name, fmt.Errorf("response link %s %s: %w", operationLabel(source), name, err), true))
					continue
				}
				target, err := ownership.LinkTarget(link, operationRefPointer)
				if err != nil {
					failures = append(failures, newLinkFailure(source, linkPointer, name, fmt.Errorf("response link %s %s: %w", operationLabel(source), name, err), false))
					continue
				}
				targetVisible := visible[operationRouteKey(target)]
				if !targetVisible {
					reason := "unavailable"
					if target.Visibility == "hidden" {
						reason = "hidden"
					}
					failures = append(failures, newLinkFailure(source, linkPointer, name, fmt.Errorf("response link %s %s targets %s operation %q", operationLabel(source), name, reason, operationLabel(target)), false))
					continue
				}
				pointers := LinkDefinitionPointers{Parameters: parametersPointer, RequestBody: requestBodyPointer}
				var definition any
				if planner.Definition != nil {
					definition, err = planner.Definition(source, target, link, pointers)
				}
				if err != nil {
					if !strings.HasPrefix(err.Error(), "#") {
						err = fmt.Errorf("response link %s %s: %w", operationLabel(source), name, err)
					}
					failures = append(failures, newLinkFailure(source, linkPointer, name, err, false))
					continue
				}
				serverURL, err := LinkServerURL(link)
				serverSource := ""
				if provenance, found := document.LookupProvenance(operationRefPointer); found {
					serverSource = provenance.Primary.Source
				}
				if err == nil && serverURL == "" {
					operationRef, _ := link["operationRef"].(string)
					targetSource := ""
					if provenance, found := document.LookupProvenance(target.Pointer); found {
						targetSource = provenance.Primary.Source
					}
					rootSource := ""
					if provenance, found := document.LookupProvenance("#"); found {
						rootSource = provenance.Primary.Source
					}
					if (operationRef != "" && !strings.HasPrefix(operationRef, "#")) || (targetSource != "" && targetSource != rootSource) {
						serverSource = targetSource
						var server map[string]any
						values, exists := target.Raw["servers"].([]any)
						if !exists {
							values, exists = target.PathItemRaw["servers"].([]any)
						}
						if !exists {
							if provenance, found := document.LookupProvenance(target.Pointer); found {
								values = document.SourceServers[provenance.Primary.Source]
							}
						}
						if len(values) != 0 {
							server, _ = values[0].(map[string]any)
						}
						if len(server) != 0 {
							serverURL, err = LinkServerURL(map[string]any{"server": server})
						}
					}
				}
				if err == nil && serverURL != "" {
					if base, parseErr := url.Parse(serverSource); parseErr == nil && (base.Scheme == "http" || base.Scheme == "https") {
						if relative, parseErr := url.Parse(serverURL); parseErr == nil {
							serverURL = base.ResolveReference(relative).String()
						} else {
							err = parseErr
						}
					}
				}
				if err != nil {
					failures = append(failures, newLinkFailure(source, linkPointer, name, fmt.Errorf("response link %s %s: %w", operationLabel(source), name, err), false))
					continue
				}
				result = append(result, ResponseLink{
					SourceOperation: source, Status: status, Name: name, TargetOperation: target, Definition: definition, ServerURL: serverURL,
					Object: link, Pointers: pointers,
				})
			}
		}
	}
	sort.Slice(result, func(left, right int) bool {
		if operationRouteKey(result[left].SourceOperation) == operationRouteKey(result[right].SourceOperation) {
			if result[left].Name == result[right].Name {
				return result[left].Status < result[right].Status
			}
			return result[left].Name < result[right].Name
		}
		return operationRouteKey(result[left].SourceOperation) < operationRouteKey(result[right].SourceOperation)
	})
	return result, failures
}

func linkCapabilityRestricted(values []ir.SemanticRestriction) bool {
	for _, restriction := range values {
		if restriction.Scope == failure.ScopeCapability && restriction.Effect == failure.EffectOmitCapability {
			return true
		}
	}
	return false
}

func LinkServerURL(link map[string]any) (string, error) {
	server, _ := link["server"].(map[string]any)
	if len(server) == 0 {
		return "", nil
	}
	serverURL, _ := server["url"].(string)
	if serverURL == "" {
		return "", errors.New("Link Server Object has no URL")
	}
	variables, _ := server["variables"].(map[string]any)
	var missing string
	serverURL = serverVariablePattern.ReplaceAllStringFunc(serverURL, func(token string) string {
		name := strings.TrimSuffix(strings.TrimPrefix(token, "{"), "}")
		definition, _ := variables[name].(map[string]any)
		value, exists := definition["default"].(string)
		if !exists {
			missing = name
			return token
		}
		return url.PathEscape(value)
	})
	if missing != "" {
		return "", fmt.Errorf("Link Server variable %q has no default", missing)
	}
	return serverURL, nil
}
