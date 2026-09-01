package typescript

import (
	"bytes"
	"fmt"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

func emitOperationArtifactsTo(document *ir.Document, manifest Manifest, plan *semanticModulePlan, tree *resourceNode, resourceReachable map[string]bool, links []generatedLink, streams []generatedStream, write func(Artifact) error) error {
	if plan == nil {
		return fmt.Errorf("internal TypeScript target: prepared plan has no semantic modules")
	}
	items := make(map[string]ManifestOperation, len(manifest.Operations))
	for _, item := range manifest.Operations {
		items[manifestRouteKey(item)] = item
	}
	if tree == nil {
		return fmt.Errorf("internal TypeScript target: prepared resource tree is nil")
	}
	linksBySource := make(map[string][]generatedLink)
	for _, link := range links {
		route := operationRouteKey(link.SourceOperation)
		linksBySource[route] = append(linksBySource[route], link)
	}
	streamsByRoute := make(map[string]generatedStream, len(streams))
	for _, stream := range streams {
		streamsByRoute[operationRouteKey(stream.Operation)] = stream
	}

	for _, module := range plan.operations {
		item, exists := items[module.routeKey]
		if !exists {
			return fmt.Errorf("operation module %q has no manifest operation", module.routeKey)
		}
		operation := item.compiled
		stream, hasStream := streamsByRoute[module.routeKey]
		source, err := emitOperationLeaf(document, plan, module, operation, item, resourceReachable[module.routeKey], linksBySource[module.routeKey], stream, hasStream)
		if err != nil {
			return fmt.Errorf("emit operation module %q: %w", module.routeKey, err)
		}
		if err := write(Artifact{Path: module.path, Data: generatedSource(source)}); err != nil {
			return err
		}
	}
	return nil
}

func emitOperationLeaf(document *ir.Document, plan *semanticModulePlan, module operationModulePlan, operation ir.Operation, item ManifestOperation, resourceReachable bool, links []generatedLink, stream generatedStream, hasStream bool) ([]byte, error) {
	runtimeCallables, err := plan.relativeModuleSpecifier(module.path, "internal/runtime/callables.ts")
	if err != nil {
		return nil, err
	}
	runtimeCodecs, err := plan.relativeModuleSpecifier(module.path, "internal/runtime/codecs.ts")
	if err != nil {
		return nil, err
	}
	runtimeErrors, err := plan.relativeModuleSpecifier(module.path, "internal/runtime/errors.ts")
	if err != nil {
		return nil, err
	}
	runtimeRequest, err := plan.relativeModuleSpecifier(module.path, "internal/runtime/request.ts")
	if err != nil {
		return nil, err
	}
	runtimeIdentity, err := plan.relativeModuleSpecifier(module.path, "internal/runtime/identity.ts")
	if err != nil {
		return nil, err
	}
	schemaIndex, err := plan.relativeModuleSpecifier(module.path, plan.fixed["schema-index"])
	if err != nil {
		return nil, err
	}
	errorCatalog, err := plan.relativeModuleSpecifier(module.path, "internal/errors.ts")
	if err != nil {
		return nil, err
	}
	routeHelpers, err := plan.relativeModuleSpecifier(module.path, plan.fixed["route-helpers"])
	if err != nil {
		return nil, err
	}
	var body bytes.Buffer
	if err := emitOperationTypes(&body, document, operation, item); err != nil {
		return nil, err
	}
	bodySource, err := localizeOperationTypeSource(body.String(), module, plan)
	if err != nil {
		return nil, err
	}

	operationName := operationTypeName(module.routeKey)
	inputType := "never"
	if len(item.InputTypes) > 0 {
		inputType = operationName + "Input"
	}
	resourceInputType := inputType
	if len(item.PathParameterOrder) > 0 {
		resourceInputType = operationName + "ResourceInput"
	}
	resourceCallType := "never"
	if item.Visibility == "public" && resourceReachable {
		resourceCallType = operationName + "ResourceCall"
	}

	paginationType := "never"
	if operation.PaginationPlan != nil {
		itemType, err := operationItemTypeForScope(document, operation, typeRenderContract)
		if err != nil {
			return nil, err
		}
		paginationType = paginationFunctionType(item, itemType, item.optionsRequired)
	}
	linkGroups := linkGroupsForSource(links, module.routeKey)
	linksType, err := routeLinkGroupsType(document, linkGroups)
	if err != nil {
		return nil, err
	}
	streamType := "never"
	if hasStream {
		streamType, err = streamFunctionType(document, stream)
		if err != nil {
			return nil, err
		}
	}
	for _, capabilityType := range []*string{&paginationType, &linksType, &streamType} {
		if *capabilityType == "never" {
			continue
		}
		*capabilityType, err = localizeOperationTypeSource(*capabilityType, module, plan)
		if err != nil {
			return nil, err
		}
	}
	capabilityTypes := []struct {
		field string
		value string
	}{
		{field: "paginate", value: paginationType},
		{field: "links", value: linksType},
		{field: "stream", value: streamType},
	}
	exactCallType := operationName + "Call"
	for _, capability := range capabilityTypes {
		if capability.value != "never" {
			exactCallType = "(" + exactCallType + ") & { readonly " + capability.field + ": " + publicCapabilityType(capability.field) + "<RouteKey> }"
		}
	}
	if resourceCallType != "never" && len(item.PathParameterOrder) == 0 {
		for _, capability := range capabilityTypes {
			if capability.value != "never" {
				resourceCallType = "(" + resourceCallType + ") & { readonly " + capability.field + ": " + publicCapabilityType(capability.field) + "<RouteKey> }"
			}
		}
	}

	var output bytes.Buffer
	callableImports := "bindOperation, type RequestFunction"
	if hasStream {
		callableImports = "bindOperation, bindStreamOperation, type RequestFunction"
	}
	fmt.Fprintf(&output, "import { %s } from %s\n", callableImports, quoteTS(runtimeCallables))
	fmt.Fprintf(&output, "import type { WireSchemas } from %s\n", quoteTS(runtimeCodecs))
	fmt.Fprintf(&output, "import type { TransportError } from %s\n", quoteTS(runtimeErrors))
	fmt.Fprintf(&output, "import type { BinaryBody, RawResponseFor, RequestOptions } from %s\n", quoteTS(runtimeRequest))
	fmt.Fprintf(&output, "import type { OperationTypeIdentity } from %s\n", quoteTS(runtimeIdentity))
	fmt.Fprintf(&output, "import type { LinkCalls, OperationRawCall, PaginateCall, ResourceRawCapability, RouteInput, RouteOptions, RouteOutput, RouteRawResponse, RouteResourceInput, StreamCall } from %s\n", quoteTS(routeHelpers))
	fmt.Fprintf(&output, "import type * as ContractSchemas from %s\n", quoteTS(schemaIndex))
	fmt.Fprintf(&output, "import type * as Errors from %s\n", quoteTS(errorCatalog))
	if operation.PaginationPlan != nil {
		runtimePagination, err := plan.relativeModuleSpecifier(module.path, "internal/runtime/pagination.ts")
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "import { createPaginator, type PaginateInput } from %s\n", quoteTS(runtimePagination))
	}
	if linksType != "never" {
		runtimeLinks, err := plan.relativeModuleSpecifier(module.path, "internal/runtime/links.ts")
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "import { mergeLinkInput, resolveLinkInput, type LinkInvocation, type RequiredLinkInvocation } from %s\n", quoteTS(runtimeLinks))
		fmt.Fprintf(&output, "import type { APIError } from %s\n", quoteTS(runtimeErrors))
	}
	output.WriteByte('\n')
	fmt.Fprintf(&output, "export type RouteKey = %s\n", quoteTS(module.routeKey))
	output.WriteByte('\n')
	output.WriteString(bodySource)

	output.WriteString("export interface RequestInputs {\n")
	for _, input := range item.InputTypes {
		descriptor, err := requestInputSection(operationName, input)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "  readonly %s: %s\n", descriptor.sectionKey, input)
	}
	output.WriteString("}\n\n")
	fmt.Fprintf(&output, "export type Input = %s\n", inputType)
	fmt.Fprintf(&output, "export type ResourceInput = %s\n", resourceInputType)
	fmt.Fprintf(&output, "export type Options = %sOptions\n", operationName)
	fmt.Fprintf(&output, "export type Output = %sOutput\n", operationName)
	fmt.Fprintf(&output, "export type Error = %s\n", item.renderError(typeRenderContract))
	fmt.Fprintf(&output, "export type RawResponse = %sRawResponse\n", operationName)
	fmt.Fprintf(&output, "export type BaseCall = %sCall\n", operationName)
	fmt.Fprintf(&output, "export type RawCall = %sRawCall\n", operationName)
	fmt.Fprintf(&output, "export type ResourceBaseCall = %s\n", map[bool]string{true: operationName + "ResourceCall", false: "never"}[item.Visibility == "public"])
	fmt.Fprintf(&output, "export type ResourceRawCall = %s\n", map[bool]string{true: operationName + "ResourceRawCall", false: "never"}[item.Visibility == "public"])
	fmt.Fprintf(&output, "export type Pagination = %s\n", paginationType)
	fmt.Fprintf(&output, "export type Links = %s\n", linksType)
	fmt.Fprintf(&output, "export type Stream = %s\n", streamType)
	fmt.Fprintf(&output, "export type ExactCall = (%s) & OperationTypeIdentity<RouteKey, \"exact\">\n", exactCallType)
	fmt.Fprintf(&output, "export type ResourceCall = %s\n\n", resourceCallType)
	output.WriteString("export interface Contract {\n")
	output.WriteString("  readonly input: Input\n")
	output.WriteString("  readonly resourceInput: ResourceInput\n")
	output.WriteString("  readonly options: Options\n")
	output.WriteString("  readonly output: Output\n")
	output.WriteString("  readonly error: Error\n")
	output.WriteString("  readonly rawResponse: RawResponse\n")
	output.WriteString("  readonly call: ExactCall\n")
	output.WriteString("  readonly resourceCall: ResourceCall\n")
	output.WriteString("  readonly pagination: Pagination\n")
	output.WriteString("  readonly links: Links\n")
	output.WriteString("  readonly stream: Stream\n")
	output.WriteString("}\n\n")

	definition, err := operationDefinition(document, operation, item)
	if err != nil {
		return nil, err
	}
	definition, err = localizeOperationTypeSource(definition, module, plan)
	if err != nil {
		return nil, err
	}
	hasInput := len(item.InputTypes) > 0
	inputOptional := hasInput && !item.prepared.inputRequired
	output.WriteString("/** Binds this operation's immutable definition to one request executor. */\n")
	output.WriteString("export function bindBase(request: RequestFunction, inputSchemas?: WireSchemas, outputSchemas?: WireSchemas): BaseCall {\n")
	fmt.Fprintf(&output, "  return bindOperation<Input, Output, Options, RawResponse>(request, %s, %t, %t) as BaseCall\n", definition, hasInput, inputOptional)
	output.WriteString("}\n")

	if operation.PaginationPlan != nil {
		itemType, err := operationItemTypeForScope(document, operation, typeRenderContract)
		if err != nil {
			return nil, err
		}
		runtimePlan, err := paginationRuntimePlanExpression(*operation.PaginationPlan)
		if err != nil {
			return nil, err
		}
		output.WriteString("\n/** Creates this operation's paginator from its single base call. */\n")
		output.WriteString("export function bindPagination(base: BaseCall): Pagination {\n")
		fmt.Fprintf(&output, "  return createPaginator<%s, Input, unknown, %s, %s, %s, Options, %t>((input, requestOptions) => base.raw(input, requestOptions).then((response) => response.data), %s)\n", itemType, quoteTS(operation.PaginationPlan.Mode), quoteTS(operation.PaginationPlan.Request.Cursor), quoteTS(operation.PaginationPlan.Request.Offset), item.optionsRequired, runtimePlan)
		output.WriteString("}\n")
	}
	if linksType != "never" {
		factory, err := emitOperationLinkFactory(document, plan, module, links, linkGroups)
		if err != nil {
			return nil, err
		}
		output.Write(factory)
	}
	if hasStream {
		streamItemType := strings.ReplaceAll(stream.ItemType, "Contract.", "ContractSchemas.")
		output.WriteString("\n/** Creates this operation's separately bound streaming callable. */\n")
		output.WriteString("export function bindStream(request: RequestFunction, inputSchemas?: WireSchemas, outputSchemas?: WireSchemas): Stream {\n")
		fmt.Fprintf(&output, "  return bindStreamOperation<Input, %s, Options>(request, %s, %t, %t) as Stream\n", streamItemType, definition, hasInput, inputOptional)
		output.WriteString("}\n")
	}

	localized := strings.ReplaceAll(output.String(), "Contract.", "ContractSchemas.")
	localized, err = localizeOperationSchemaReferences(localized, module, plan, schemaIndex)
	if err != nil {
		return nil, err
	}
	return []byte(localized), nil
}

func emitOperationLinkFactory(document *ir.Document, plan *semanticModulePlan, module operationModulePlan, links []generatedLink, groups []generatedLinkGroup) ([]byte, error) {
	var output bytes.Buffer
	output.WriteString("\n/** Completed exact callables required by this operation's response links. */\n")
	output.WriteString("export interface LinkTargets {\n")
	targets := make(map[string]bool)
	for _, link := range links {
		targets[operationRouteKey(link.TargetOperation)] = true
	}
	for _, route := range sortedStringKeys(targets) {
		path, exists := plan.operationByRoute[route]
		if !exists {
			return nil, fmt.Errorf("link target %q has no operation module", route)
		}
		specifier, err := plan.relativeModuleSpecifier(module.path, path)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "  readonly %s: import(%s).Contract[\"call\"]\n", quoteTS(route), quoteTS(specifier))
	}
	output.WriteString("}\n\n")
	output.WriteString("/** Creates this operation's response-link container. */\n")
	output.WriteString("export function bindLinks(targets: LinkTargets): Links {\n")
	var body bytes.Buffer
	if err := emitLinkValuesForGroups(&body, document, links, groups); err != nil {
		return nil, err
	}
	bodySource := body.String()
	for route := range targets {
		bodySource = strings.ReplaceAll(bodySource, operationValueName(route), "targets["+quoteTS(route)+"]")
	}
	bodySource, err := localizeOperationTypeSource(bodySource, module, plan)
	if err != nil {
		return nil, err
	}
	output.WriteString(strings.ReplaceAll(bodySource, "Contract.", "ContractSchemas."))
	value, err := routeLinkGroupsValue(groups)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&output, "  return %s\n", value)
	output.WriteString("}\n")
	return output.Bytes(), nil
}

func publicCapabilityType(field string) string {
	switch field {
	case "paginate":
		return "PaginateCall"
	case "links":
		return "LinkCalls"
	case "stream":
		return "StreamCall"
	default:
		return "never"
	}
}

var operationLocalSlots = map[string]string{
	`"input"`:         "Input",
	`"resourceInput"`: "ResourceInput",
	`"options"`:       "Options",
	`"output"`:        "Output",
	`"error"`:         "Error",
	`"rawResponse"`:   "RawResponse",
	`"call"`:          "ExactCall",
	`"resourceCall"`:  "ResourceCall",
	`"pagination"`:    "Pagination",
	`"links"`:         "Links",
	`"stream"`:        "Stream",
}

func localizeOperationTypeSource(source string, module operationModulePlan, plan *semanticModulePlan) (string, error) {
	const prefix = "Routes["
	cursor := 0
	search := 0
	var output strings.Builder
	for search < len(source) {
		relative := strings.Index(source[search:], prefix)
		if relative < 0 {
			break
		}
		start := search + relative
		nextSearch := start + len(prefix)
		routeEnd, ok := quotedTokenEnd(source, nextSearch)
		if !ok || !strings.HasPrefix(source[routeEnd:], "][") {
			search = nextSearch
			continue
		}
		slotStart := routeEnd + 2
		slotEnd, ok := quotedTokenEnd(source, slotStart)
		if !ok || slotEnd >= len(source) || source[slotEnd] != ']' {
			search = nextSearch
			continue
		}
		route, routeExists := plan.operationByQuotedRoute[source[nextSearch:routeEnd]]
		local, slotExists := operationLocalSlots[source[slotStart:slotEnd]]
		if !routeExists || !slotExists {
			search = slotEnd + 1
			continue
		}
		replacement := local
		if route != module.routeKey {
			specifier, err := plan.relativeModuleSpecifier(module.path, plan.operationByRoute[route])
			if err != nil {
				return "", err
			}
			replacement = "import(" + quoteTS(specifier) + ").Contract[" + source[slotStart:slotEnd] + "]"
		}
		if output.Len() == 0 {
			output.Grow(len(source))
		}
		output.WriteString(source[cursor:start])
		output.WriteString(replacement)
		cursor = slotEnd + 1
		search = cursor
	}
	if output.Len() == 0 {
		return source, nil
	}
	output.WriteString(source[cursor:])
	return output.String(), nil
}

type operationSchemaReference struct {
	start       int
	end         int
	name        string
	export      string
	replacement string
}

func localizeOperationSchemaReferences(source string, module operationModulePlan, plan *semanticModulePlan, schemaIndexSpecifier string) (string, error) {
	const namespace = "ContractSchemas"
	const referencePrefix = namespace + ".Component"
	var occurrences []operationSchemaReference
	counts := make(map[string]int)
	search := 0
	for search < len(source) {
		relative := strings.Index(source[search:], referencePrefix)
		if relative < 0 {
			break
		}
		start := search + relative
		if operationReferenceInDoc(source, start) {
			occurrences = append(occurrences, operationSchemaReference{start: start, end: start + len(namespace), replacement: "Contract"})
			search = start + len(referencePrefix)
			continue
		}
		remainder := source[start+len(referencePrefix):]
		export := ""
		prefixBytes := 0
		switch {
		case strings.HasPrefix(remainder, "Input<"):
			export = "Input"
			prefixBytes = len("Input<")
		case strings.HasPrefix(remainder, "Output<"):
			export = "Output"
			prefixBytes = len("Output<")
		default:
			search = start + len(referencePrefix)
			continue
		}
		nameStart := start + len(referencePrefix) + prefixBytes
		nameEnd, ok := quotedTokenEnd(source, nameStart)
		if !ok || nameEnd >= len(source) || source[nameEnd] != '>' {
			search = nameStart
			continue
		}
		name, exists := plan.schemaByQuotedName[source[nameStart:nameEnd]]
		if !exists {
			search = nameEnd + 1
			continue
		}
		key := name + "\x00" + export
		counts[key]++
		occurrences = append(occurrences, operationSchemaReference{start: start, end: nameEnd + 1, name: name, export: export})
		search = nameEnd + 1
	}

	imports := make([]string, 0)
	replacements := make(map[string]string, len(counts))
	for _, schema := range plan.schemas {
		if !schema.publicProjection {
			continue
		}
		for _, export := range []string{"Input", "Output"} {
			key := schema.name + "\x00" + export
			count := counts[key]
			if count == 0 {
				continue
			}
			specifier, err := plan.relativeModuleSpecifier(module.path, schema.path)
			if err != nil {
				return "", err
			}
			replacement := "import(" + quoteTS(specifier) + ")." + export
			if count > 1 {
				alias := stablePrivateIdentifier("schema-type", schema.name+"\x00"+export)
				imports = append(imports, "import type { "+export+" as "+alias+" } from "+quoteTS(specifier))
				replacement = alias
			}
			replacements[key] = replacement
		}
	}
	var output strings.Builder
	output.Grow(len(source))
	cursor := 0
	for _, occurrence := range occurrences {
		output.WriteString(source[cursor:occurrence.start])
		replacement := occurrence.replacement
		if replacement == "" {
			replacement = replacements[occurrence.name+"\x00"+occurrence.export]
		}
		output.WriteString(replacement)
		cursor = occurrence.end
	}
	output.WriteString(source[cursor:])
	source = output.String()
	namespaceImport := "import type * as ContractSchemas from " + quoteTS(schemaIndexSpecifier) + "\n"
	replacement := ""
	if len(imports) > 0 {
		replacement = strings.Join(imports, "\n") + "\n"
	}
	source = strings.Replace(source, namespaceImport, replacement, 1)
	if strings.Contains(source, "ContractSchemas.") {
		return "", fmt.Errorf("operation %q retains an unplanned schema registry reference", module.routeKey)
	}
	return source, nil
}

func quotedTokenEnd(source string, start int) (int, bool) {
	if start >= len(source) || source[start] != '"' {
		return 0, false
	}
	for index := start + 1; index < len(source); index++ {
		switch source[index] {
		case '\\':
			index++
		case '"':
			return index + 1, true
		}
	}
	return 0, false
}

func operationReferenceInDoc(source string, offset int) bool {
	lineStart := strings.LastIndex(source[:offset], "\n") + 1
	return strings.HasPrefix(strings.TrimSpace(source[lineStart:offset]), "*")
}
