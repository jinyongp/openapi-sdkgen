package typescript

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

var (
	serverVariablePattern       = regexp.MustCompile(`\{[^{}]+\}`)
	linkRequestParameterPattern = regexp.MustCompile(`^\$request\.(path|query|header|cookie)\.([^#]+)(#.*)?$`)
	exactResponseStatusPattern  = regexp.MustCompile(`^[1-5][0-9][0-9]$`)
	rangeResponseStatusPattern  = regexp.MustCompile(`^[1-5][Xx][Xx]$`)
)

type generatedLink struct {
	SourceOperation       ir.Operation
	Status                string
	Name                  string
	TargetOperation       ir.Operation
	Definition            string
	ServerURL             string
	SourceHasInput        bool
	TargetHasInput        bool
	TargetOptionsRequired bool
}

func generatedLinks(document *ir.Document, manifest Manifest) ([]generatedLink, error) {
	result, failures := generatedLinksDiagnostics(document, manifest)
	if len(failures) != 0 {
		return nil, failures[0]
	}
	return result, nil
}

func generatedLinksDiagnostics(document *ir.Document, manifest Manifest) ([]generatedLink, []error) {
	visible := map[string]ManifestOperation{}
	for _, operation := range manifest.Operations {
		if operation.Visibility != "hidden" {
			visible[manifestRouteKey(operation)] = operation
		}
	}
	byID := make(map[string]ir.Operation, len(document.Operations))
	byPathMethod := make(map[string]ir.Operation, len(document.Operations))
	for _, operation := range document.Operations {
		if operation.OperationID != "" {
			byID[operation.OperationID] = operation
		}
		byPathMethod[operationPathMethodKey(operation.Path, operation.Method)] = operation
	}
	var result []generatedLink
	var failures []error
	for _, source := range document.Operations {
		sourcePlan, sourceVisible := visible[operationRouteKey(source)]
		if !sourceVisible {
			continue
		}
		responses, _ := source.Raw["responses"].(map[string]any)
		for _, status := range sortedAnyKeys(responses) {
			response, _ := responses[status].(map[string]any)
			responseSourcePointer := responsePointer(source, status)
			linksPointer, err := componentObjectFieldPointer(document, response, "responses", responseSourcePointer, "links")
			if err != nil {
				failures = append(failures, fmt.Errorf("response %s %s: %w", operationLabel(source), status, err))
				continue
			}
			resolved, err := resolveComponentObject(document, response, "responses")
			if err != nil {
				failures = append(failures, fmt.Errorf("response %s %s: %w", operationLabel(source), status, err))
				continue
			}
			links, _ := resolved["links"].(map[string]any)
			for _, name := range sortedAnyKeys(links) {
				linkPointer := linksPointer + "/" + escapePointerToken(name)
				link, _ := links[name].(map[string]any)
				parametersPointer, err := componentObjectFieldPointer(document, link, "links", linkPointer, "parameters")
				if err != nil {
					failures = append(failures, fmt.Errorf("response link %s %s: %w", operationLabel(source), name, err))
					continue
				}
				requestBodyPointer, err := componentObjectFieldPointer(document, link, "links", linkPointer, "requestBody")
				if err != nil {
					failures = append(failures, fmt.Errorf("response link %s %s: %w", operationLabel(source), name, err))
					continue
				}
				link, err = resolveComponentObject(document, link, "links")
				if err != nil {
					failures = append(failures, fmt.Errorf("response link %s %s: %w", operationLabel(source), name, err))
					continue
				}
				target, err := linkTargetOperation(byID, byPathMethod, link)
				if err != nil {
					failures = append(failures, fmt.Errorf("response link %s %s: %w", operationLabel(source), name, err))
					continue
				}
				targetPlan, targetVisible := visible[operationRouteKey(target)]
				if !targetVisible {
					failures = append(failures, fmt.Errorf("response link %s %s targets hidden operation %q", operationLabel(source), name, operationLabel(target)))
					continue
				}
				definition, err := linkDefinition(sourcePlan.prepared.parameters, targetPlan.prepared.parameters, link, linkDefinitionPointers{
					parameters:  parametersPointer,
					requestBody: requestBodyPointer,
				})
				if err != nil {
					if strings.HasPrefix(err.Error(), "#") {
						failures = append(failures, err)
					} else {
						failures = append(failures, fmt.Errorf("response link %s %s: %w", operationLabel(source), name, err))
					}
					continue
				}
				serverURL, err := linkServerURL(link)
				if err != nil {
					failures = append(failures, fmt.Errorf("response link %s %s: %w", operationLabel(source), name, err))
					continue
				}
				result = append(result, generatedLink{
					SourceOperation: source, Status: status, Name: name, TargetOperation: target, Definition: definition, ServerURL: serverURL,
					SourceHasInput: len(sourcePlan.InputTypes) != 0, TargetHasInput: len(targetPlan.InputTypes) != 0,
					TargetOptionsRequired: targetPlan.optionsRequired,
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

func responsePointer(source ir.Operation, status string) string {
	pointer := source.Pointer
	if pointer == "" {
		pointer = "#/paths/" + escapePointerToken(source.Path) + "/" + strings.ToLower(source.Method)
	}
	return pointer + "/responses/" + escapePointerToken(status)
}

func operationPathMethodKey(path, method string) string {
	return path + "\x00" + strings.ToUpper(method)
}

func linkServerURL(link map[string]any) (string, error) {
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

func linkTargetOperation(byID, byPathMethod map[string]ir.Operation, link map[string]any) (ir.Operation, error) {
	if operationID, _ := link["operationId"].(string); operationID != "" {
		operation, ok := byID[operationID]
		if !ok {
			return ir.Operation{}, fmt.Errorf("operationId %q does not name a generated operation", operationID)
		}
		return operation, nil
	}
	operationRef, _ := link["operationRef"].(string)
	if !strings.HasPrefix(operationRef, "#/paths/") {
		return ir.Operation{}, fmt.Errorf("requires operationId or a local operationRef")
	}
	tokens := strings.Split(strings.TrimPrefix(operationRef, "#/"), "/")
	if len(tokens) != 3 || tokens[0] != "paths" {
		return ir.Operation{}, fmt.Errorf("operationRef %q must target one path operation", operationRef)
	}
	path, err := linkJSONPointerToken(tokens[1])
	if err != nil {
		return ir.Operation{}, err
	}
	method, err := linkJSONPointerToken(tokens[2])
	if err != nil {
		return ir.Operation{}, err
	}
	if operation, exists := byPathMethod[operationPathMethodKey(path, method)]; exists {
		return operation, nil
	}
	return ir.Operation{}, fmt.Errorf("operationRef %q does not name a generated operation", operationRef)
}

type linkDefinitionPointers struct {
	parameters  string
	requestBody string
}

func linkDefinition(sourceParameters, parameters []operationParameter, link map[string]any, pointers linkDefinitionPointers) (string, error) {
	byName := map[string][]operationParameter{}
	for _, parameter := range parameters {
		byName[parameter.Name] = append(byName[parameter.Name], parameter)
	}
	var assignments []string
	values, _ := link["parameters"].(map[string]any)
	for _, name := range sortedAnyKeys(values) {
		valuePointer := pointers.parameters + "/" + escapePointerToken(name)
		matches := byName[name]
		if len(matches) != 1 {
			return "", withSourcePointer(valuePointer, "parameter %q matches %d target parameters", name, len(matches))
		}
		location := matches[0].Location
		switch location {
		case "header":
			location = "headerParams"
		case "cookie":
			location = "cookieParams"
		case "querystring":
			location = "querystring"
		}
		value, err := linkValueLiteral(values[name], sourceParameters)
		if err != nil {
			return "", withSourcePointer(valuePointer, "%v", err)
		}
		assignments = append(assignments, "{ location: "+quoteTS(location)+", property: "+quoteTS(matches[0].Property)+", value: "+value+" }")
	}
	fields := []string{}
	if len(assignments) != 0 {
		fields = append(fields, "parameters: ["+strings.Join(assignments, ", ")+"]")
	}
	if body, exists := link["requestBody"]; exists {
		value, err := linkValueLiteral(body, sourceParameters)
		if err != nil {
			return "", withSourcePointer(pointers.requestBody, "%v", err)
		}
		fields = append(fields, "requestBody: "+value)
	}
	return "{ " + strings.Join(fields, ", ") + " }", nil
}

func linkValueLiteral(value any, sourceParameters []operationParameter) (string, error) {
	if expression, ok := value.(string); ok {
		var err error
		value, err = linkRequestParameterExpression(expression, sourceParameters)
		if err != nil {
			return "", err
		}
	}
	encoded, err := runtimeJSONExpression(value)
	if err != nil {
		return "", fmt.Errorf("encode Link runtime value: %w", err)
	}
	return encoded, nil
}

func linkRequestParameterExpression(expression string, sourceParameters []operationParameter) (any, error) {
	matches := linkRequestParameterPattern.FindStringSubmatch(expression)
	if matches == nil {
		return expression, nil
	}
	location := matches[1]
	name := matches[2]
	for _, parameter := range sourceParameters {
		if parameter.Location != location || parameter.Name != name {
			continue
		}
		section := location
		if location == "header" {
			section = "headerParams"
		} else if location == "cookie" {
			section = "cookieParams"
		}
		pointer := ""
		if suffix := matches[3]; suffix != "" {
			pointer = strings.TrimPrefix(suffix, "#")
		}
		return map[string]any{"x-sdkgen-link-request-parameter": map[string]any{"section": section, "property": parameter.Property, "pointer": pointer}}, nil
	}
	return nil, fmt.Errorf("request runtime expression %q references unknown source %s parameter %q", expression, location, name)
}

func linkJSONPointerToken(token string) (string, error) {
	var output strings.Builder
	for index := 0; index < len(token); index++ {
		if token[index] != '~' {
			output.WriteByte(token[index])
			continue
		}
		if index+1 >= len(token) {
			return "", fmt.Errorf("invalid JSON Pointer escape")
		}
		index++
		switch token[index] {
		case '0':
			output.WriteByte('~')
		case '1':
			output.WriteByte('/')
		default:
			return "", fmt.Errorf("invalid JSON Pointer escape")
		}
	}
	return output.String(), nil
}

func emitLinkInterface(output *bytes.Buffer, document *ir.Document, links []generatedLink) error {
	if len(links) == 0 {
		return nil
	}
	output.WriteString("  /** OpenAPI response links grouped by source operation. */\n")
	output.WriteString("  readonly $links: {\n")
	for _, source := range linkSourceOperations(links) {
		if source.OperationID == "" {
			continue
		}
		fmt.Fprintf(output, "    readonly %s: LinkCalls<%s>\n", quoteTS(source.OperationID), quoteTS(operationRouteKey(source)))
	}
	output.WriteString("  }\n")
	return nil
}

type generatedLinkGroup struct {
	SourceOperation ir.Operation
	Name            string
	Links           []generatedLink
}

func linkGroupsForSource(links []generatedLink, routeKey string) []generatedLinkGroup {
	byName := map[string][]generatedLink{}
	var source ir.Operation
	for _, link := range links {
		if operationRouteKey(link.SourceOperation) != routeKey {
			continue
		}
		source = link.SourceOperation
		byName[link.Name] = append(byName[link.Name], link)
	}
	names := sortedAnyKeys(mapStringAny(byName))
	result := make([]generatedLinkGroup, 0, len(names))
	for _, name := range names {
		items := byName[name]
		sort.Slice(items, func(left, right int) bool { return items[left].Status < items[right].Status })
		result = append(result, generatedLinkGroup{SourceOperation: source, Name: name, Links: items})
	}
	return result
}

func mapStringAny[T any](values map[string]T) map[string]any {
	result := make(map[string]any, len(values))
	for key := range values {
		result[key] = nil
	}
	return result
}

func linkGroupContract(document *ir.Document, group generatedLinkGroup) (string, error) {
	_ = document
	sourceInput := linkPreparedInputType(operationRouteKey(group.SourceOperation), len(group.Links) != 0 && group.Links[0].SourceHasInput)
	targetInputs := map[string]bool{}
	targetOptions := map[string]bool{}
	targetOutputs := map[string]bool{}
	requiresOptions := false
	statusMembers := make([]string, 0, len(group.Links))
	for _, link := range group.Links {
		input := linkPreparedInputType(operationRouteKey(link.TargetOperation), link.TargetHasInput)
		output := operationSlotType(operationRouteKey(link.TargetOperation), "output")
		targetInputs[input] = true
		targetOptions[operationSlotType(operationRouteKey(link.TargetOperation), "options")] = true
		targetOutputs[output] = true
		targetRequiresOptions := link.TargetOptionsRequired
		if targetRequiresOptions {
			requiresOptions = true
		}
		statusProperty, err := linkStatusProperty(link.Status)
		if err != nil {
			return "", err
		}
		statusMembers = append(statusMembers, "readonly "+statusProperty+": (response: "+operationSlotType(operationRouteKey(group.SourceOperation), "rawResponse")+" | APIError, "+linkInvocationParameter(input, operationSlotType(operationRouteKey(link.TargetOperation), "options"), sourceInput, targetRequiresOptions)+") => Promise<"+output+">")
	}
	return "{ (response: " + operationSlotType(operationRouteKey(group.SourceOperation), "rawResponse") + " | APIError, " + linkInvocationParameter(sortedStringSet(targetInputs), sortedStringIntersection(targetOptions), sourceInput, requiresOptions) + "): Promise<" + sortedStringSet(targetOutputs) + ">; readonly byStatus: { " + strings.Join(statusMembers, "; ") + " } }", nil
}

func linkInvocationParameter(input, options, sourceInput string, required bool) string {
	if required {
		return "invocation: RequiredLinkInvocation<" + input + ", " + options + ", " + sourceInput + ">"
	}
	return "invocation?: LinkInvocation<" + input + ", " + options + ", " + sourceInput + ">"
}

func linkPreparedInputType(route string, hasInput bool) string {
	if !hasInput {
		return "never"
	}
	return operationSlotType(route, "input")
}

func sortedStringSet(values map[string]bool) string {
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	return strings.Join(items, " | ")
}

func sortedStringIntersection(values map[string]bool) string {
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	return strings.Join(items, " & ")
}

func linkStatusProperty(status string) (string, error) {
	if status == "default" {
		return "statusDefault", nil
	}
	if exactResponseStatusPattern.MatchString(status) {
		return "status" + status, nil
	}
	if rangeResponseStatusPattern.MatchString(status) {
		return "status" + string(status[0]) + "XX", nil
	}
	return "", fmt.Errorf("unsupported Link response status %q", status)
}

func emitLinkValues(output *bytes.Buffer, document *ir.Document, links []generatedLink) error {
	if len(links) == 0 {
		return nil
	}
	groups := make([]generatedLinkGroup, 0)
	for _, source := range linkSourceOperations(links) {
		groups = append(groups, linkGroupsForSource(links, operationRouteKey(source))...)
	}
	return emitLinkValuesForGroups(output, document, links, groups)
}

func emitLinkValuesForGroups(output *bytes.Buffer, document *ir.Document, links []generatedLink, groups []generatedLinkGroup) error {
	for _, link := range links {
		name, err := generatedLinkVariableName(link)
		if err != nil {
			return err
		}
		targetProperty := operationValueName(operationRouteKey(link.TargetOperation))
		targetHasInput := link.TargetHasInput
		targetInput := operationSlotType(operationRouteKey(link.TargetOperation), "input")
		targetOptions := operationSlotType(operationRouteKey(link.TargetOperation), "options")
		sourceRawResponse := operationSlotType(operationRouteKey(link.SourceOperation), "rawResponse")
		sourceInput := linkPreparedInputType(operationRouteKey(link.SourceOperation), link.SourceHasInput)
		targetOutput := operationSlotType(operationRouteKey(link.TargetOperation), "output")
		invocationType := "LinkInvocation"
		invocationDefault := " = {}"
		targetRequiresOptions := link.TargetOptionsRequired
		if targetRequiresOptions {
			invocationType = "RequiredLinkInvocation"
			invocationDefault = ""
		}
		options := "invocation.options"
		if link.ServerURL != "" {
			options = "{ ...(invocation.options ?? {}), baseURL: new URL(" + quoteTS(link.ServerURL) + ", response.response.url).href }"
		}
		if !targetHasInput {
			fmt.Fprintf(output, "  const %s = (response: %s | APIError, invocation: %s<never, %s, %s>%s): Promise<%s> => { resolveLinkInput<never>(response, %s, invocation.sourceInput); return %s(%s) }\n", name, sourceRawResponse, invocationType, targetOptions, sourceInput, invocationDefault, targetOutput, link.Definition, targetProperty, options)
			continue
		}
		fmt.Fprintf(output, "  const %s = (response: %s | APIError, invocation: %s<%s, %s, %s>%s): Promise<%s> => %s(mergeLinkInput(resolveLinkInput<%s>(response, %s, invocation.sourceInput), invocation.input), %s)\n", name, sourceRawResponse, invocationType, targetInput, targetOptions, sourceInput, invocationDefault, targetOutput, targetProperty, targetInput, link.Definition, options)
	}
	for _, group := range groups {
		if err := emitLinkGroupValue(output, document, group); err != nil {
			return err
		}
	}
	return nil
}

func emitLinkGroupValue(output *bytes.Buffer, document *ir.Document, group generatedLinkGroup) error {
	variable, err := generatedLinkGroupVariableName(group)
	if err != nil {
		return err
	}
	contract, err := linkGroupContract(document, group)
	if err != nil {
		return err
	}
	sourceInput := linkPreparedInputType(operationRouteKey(group.SourceOperation), len(group.Links) != 0 && group.Links[0].SourceHasInput)
	targetInputs := map[string]bool{}
	targetOptions := map[string]bool{}
	targetOutputs := map[string]bool{}
	requiresOptions := false
	for _, link := range group.Links {
		input := linkPreparedInputType(operationRouteKey(link.TargetOperation), link.TargetHasInput)
		output := operationSlotType(operationRouteKey(link.TargetOperation), "output")
		targetInputs[input] = true
		targetOptions[operationSlotType(operationRouteKey(link.TargetOperation), "options")] = true
		targetOutputs[output] = true
		targetRequiresOptions := link.TargetOptionsRequired
		if targetRequiresOptions {
			requiresOptions = true
		}
	}
	invocationType := "LinkInvocation"
	invocationDefault := " = {}"
	if requiresOptions {
		invocationType = "RequiredLinkInvocation"
		invocationDefault = ""
	}
	fmt.Fprintf(output, "  const %s: %s = Object.assign(async (response: %s | APIError, invocation: %s<%s, %s, %s>%s): Promise<%s> => {\n", variable, contract, operationSlotType(operationRouteKey(group.SourceOperation), "rawResponse"), invocationType, sortedStringSet(targetInputs), sortedStringIntersection(targetOptions), sourceInput, invocationDefault, sortedStringSet(targetOutputs))
	for _, link := range group.Links {
		if link.Status == "default" {
			continue
		}
		condition, err := linkStatusCondition(link.Status)
		if err != nil {
			return err
		}
		leaf, err := generatedLinkVariableName(link)
		if err != nil {
			return err
		}
		fmt.Fprintf(output, "    if (%s) return await %s(response, invocation as never)\n", condition, leaf)
	}
	for _, link := range group.Links {
		if link.Status != "default" {
			continue
		}
		leaf, err := generatedLinkVariableName(link)
		if err != nil {
			return err
		}
		fmt.Fprintf(output, "    return await %s(response, invocation as never)\n", leaf)
	}
	fmt.Fprintf(output, "    throw new TypeError(%s)\n  }, { byStatus: {\n", quoteTS("no Link Object named "+group.Name+" matches response status"))
	for _, link := range group.Links {
		property, err := linkStatusProperty(link.Status)
		if err != nil {
			return err
		}
		leaf, err := generatedLinkVariableName(link)
		if err != nil {
			return err
		}
		fmt.Fprintf(output, "    %s: %s,\n", property, leaf)
	}
	output.WriteString("  } })\n")
	return nil
}

func linkStatusCondition(status string) (string, error) {
	if exactResponseStatusPattern.MatchString(status) {
		return "response.status === " + status, nil
	}
	if rangeResponseStatusPattern.MatchString(status) {
		return "Math.floor(response.status / 100) === " + string(status[0]), nil
	}
	return "", fmt.Errorf("unsupported Link response status %q", status)
}

func emitLinkReturnValue(output *bytes.Buffer, links []generatedLink) error {
	if len(links) == 0 {
		return nil
	}
	sources := make([]runtimeProperty, 0)
	for _, source := range linkSourceOperations(links) {
		if source.OperationID == "" {
			continue
		}
		groups := make([]runtimeProperty, 0)
		for _, group := range linkGroupsForSource(links, operationRouteKey(source)) {
			variable, err := generatedLinkGroupVariableName(group)
			if err != nil {
				return err
			}
			groups = append(groups, runtimeProperty{key: group.Name, value: variable})
		}
		sources = append(sources, runtimeProperty{key: source.OperationID, value: runtimeObjectExpression(groups)})
	}
	fmt.Fprintf(output, "    $links: %s as unknown as Client[\"$links\"],\n", runtimeObjectExpression(sources))
	return nil
}

func linkSourceOperations(links []generatedLink) []ir.Operation {
	seen := map[string]ir.Operation{}
	for _, link := range links {
		seen[operationRouteKey(link.SourceOperation)] = link.SourceOperation
	}
	result := make([]ir.Operation, 0, len(seen))
	for _, operation := range seen {
		result = append(result, operation)
	}
	sort.Slice(result, func(left, right int) bool { return operationRouteKey(result[left]) < operationRouteKey(result[right]) })
	return result
}

func linksForSource(links []generatedLink, routeKey string) []generatedLink {
	result := make([]generatedLink, 0)
	for _, link := range links {
		if operationRouteKey(link.SourceOperation) == routeKey {
			result = append(result, link)
		}
	}
	return result
}

func routeLinksType(document *ir.Document, links []generatedLink, routeKey string) (string, error) {
	return routeLinkGroupsType(document, linkGroupsForSource(links, routeKey))
}

func routeLinkGroupsType(document *ir.Document, groups []generatedLinkGroup) (string, error) {
	if len(groups) == 0 {
		return "never", nil
	}
	members := make([]string, 0, len(groups))
	for _, group := range groups {
		contract, err := linkGroupContract(document, group)
		if err != nil {
			return "", err
		}
		members = append(members, "readonly "+quoteTS(group.Name)+": "+contract)
	}
	return "{ " + strings.Join(members, "; ") + " }", nil
}

func routeLinksValue(links []generatedLink, routeKey string) (string, error) {
	return routeLinkGroupsValue(linkGroupsForSource(links, routeKey))
}

func routeLinkGroupsValue(groups []generatedLinkGroup) (string, error) {
	if len(groups) == 0 {
		return "", nil
	}
	values := make([]runtimeProperty, 0, len(groups))
	for _, group := range groups {
		variable, err := generatedLinkGroupVariableName(group)
		if err != nil {
			return "", err
		}
		values = append(values, runtimeProperty{key: group.Name, value: variable})
	}
	return runtimeObjectExpression(values), nil
}

func generatedLinkVariableName(link generatedLink) (string, error) {
	_, err := linkStatusProperty(link.Status)
	if err != nil {
		return "", err
	}
	return stablePrivateIdentifier("link-value", operationRouteKey(link.SourceOperation)+"\x00"+link.Name+"\x00"+link.Status), nil
}

func generatedLinkGroupVariableName(group generatedLinkGroup) (string, error) {
	return stablePrivateIdentifier("link-group-value", operationRouteKey(group.SourceOperation)+"\x00"+group.Name), nil
}
