package typescript

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

func emitOperationArtifactsTo(document *ir.Document, manifest Manifest, plan *semanticModulePlan, executions map[string]operationExecutionPlan, tree *resourceNode, resourceReachable map[string]bool, links []generatedLink, streams []generatedStream, write func(Artifact) error) error {
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
	contractTypes, err := plan.relativeModuleSpecifier(module.path, "internal/runtime/contract-types.ts")
	if err != nil {
		return nil, err
	}
	names, err := operationLocalIdentifiers(module, item, links)
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

	operationName := operationLocalTypePrefix
	inputType := "never"
	if len(item.InputSections) > 0 {
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
	resourceStreamType := "never"
	if hasStream {
		streamType, err = streamFunctionType(document, stream)
		if err != nil {
			return nil, err
		}
		resourceStreamType, err = resourceStreamFunctionType(document, stream)
		if err != nil {
			return nil, err
		}
	}
	for _, capabilityType := range []*string{&paginationType, &linksType, &streamType, &resourceStreamType} {
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
	if resourceCallType != "never" {
		for _, capability := range capabilityTypes {
			if capability.value == "never" {
				continue
			}
			if len(item.PathParameterOrder) > 0 {
				if capability.field == "stream" && resourceStreamType != "never" {
					resourceCallType = "(" + resourceCallType + ") & { readonly stream: ResourceStream }"
				}
				continue
			}
			resourceCallType = "(" + resourceCallType + ") & { readonly " + capability.field + ": " + publicCapabilityType(capability.field) + "<RouteKey> }"
		}
	}

	wire := newWireRenderContext(wirePropertiesConstructed)
	definition, err := wire.operationDefinition(document, operation, item)
	if err != nil {
		return nil, err
	}
	definition, err = localizeOperationTypeSource(definition, module, plan)
	if err != nil {
		return nil, err
	}

	var output strings.Builder
	output.Grow(len(bodySource) + 4096)
	callableImports := "bindGeneratedOperation, type BufferedRequestFunction"
	if hasStream {
		callableImports = "bindGeneratedOperation, bindStreamOperation, type BufferedRequestFunction, type RequestFunction"
	}
	if wire.usesProperties {
		callableImports += ", createWireProperties as __sdkgen_Properties"
	}
	fmt.Fprintf(&output, "import { %s } from %s\n", callableImports, quoteTS(runtimeCallables))
	fmt.Fprintf(&output, "import type { WireSchemas } from %s\n", quoteTS(runtimeCodecs))
	fmt.Fprintf(&output, "import type { TransportError } from %s\n", quoteTS(runtimeErrors))
	fmt.Fprintf(&output, "import type { BinaryBody, OperationStream, RawResponseFor, RequestOptions, StreamSource } from %s\n", quoteTS(runtimeRequest))
	fmt.Fprintf(&output, "import type { OperationTypeIdentity, RouteTypeIdentity } from %s\n", quoteTS(runtimeIdentity))
	fmt.Fprintf(&output, "import type { OperationPublicType, OperationResourceRawCapability } from %s\n", quoteTS(contractTypes))
	fmt.Fprintf(&output, "import type * as ContractSchemas from %s\n", quoteTS(schemaIndex))
	if plan.splitSchemaProjections {
		// Named operations have exact code/detail contracts independent of the
		// root SDK's selected error catalog. Keep its public catalog scoped.
		if strings.Contains(item.renderError(typeRenderContract), "Errors.ServerError<") {
			fmt.Fprintf(&output, "declare namespace Errors { export type ServerError<Code extends string, Details> = import(%s).APIError<Code, Details> }\n", quoteTS(runtimeErrors))
		}
	} else {
		fmt.Fprintf(&output, "import type * as Errors from %s\n", quoteTS(errorCatalog))
	}
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
	output.WriteString("type ResourceRawMethod<Route> = ResourceRawCall & RouteTypeIdentity<Route>\n")
	output.WriteString("interface ResourceRawCapability<Route> { readonly raw: ResourceRawMethod<Route> }\n")
	output.WriteByte('\n')
	output.WriteString(bodySource)

	output.WriteString("export interface RequestInputs {\n")
	for _, section := range item.InputSections {
		descriptor, err := requestInputSection(section)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "  readonly %s: %s\n", descriptor.sectionKey, operationName+descriptor.suffix)
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
	fmt.Fprintf(&output, "export type ResourceStream = %s\n", resourceStreamType)
	fmt.Fprintf(&output, "export type ExactCall = (%s) & OperationTypeIdentity<RouteKey, \"exact\">\n", exactCallType)
	fmt.Fprintf(&output, "export type ResourceCall = %s\n\n", resourceCallType)
	output.WriteString("export type ResourceMethod<Route extends RouteKey = RouteKey> = ResourceCall & RouteTypeIdentity<Route> & OperationTypeIdentity<Route, \"resource\">\n\n")
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

	hasInput := len(item.InputSections) > 0
	inputOptional := hasInput && !item.prepared.inputRequired
	output.WriteString("/** Binds this operation's immutable definition to one request executor. */\n")
	output.WriteString("export function bindBase(request: BufferedRequestFunction, inputSchemas?: WireSchemas, outputSchemas?: WireSchemas): BaseCall {\n")
	fmt.Fprintf(&output, "  return bindGeneratedOperation(request, %s, %t, %t) as BaseCall\n", definition, hasInput, inputOptional)
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
	if hasStream {
		streamItemType := qualifyOperationSchemaContractReferences(stream.ItemType)
		defaultAccept := "undefined"
		if len(stream.Plan.streamMediaTypes) > 0 {
			defaultAccept = quoteTS(stream.Plan.streamMediaTypes[0])
		}
		output.WriteString("\n/** Creates this operation's streaming capability. */\n")
		output.WriteString("export function bindStream(request: RequestFunction, inputSchemas?: WireSchemas, outputSchemas?: WireSchemas): Stream {\n")
		fmt.Fprintf(&output, "  return bindStreamOperation<Input, %s, Options>(request, %s, %t, %t, %s) as Stream\n", streamItemType, definition, hasInput, inputOptional, defaultAccept)
		output.WriteString("}\n")
	}

	localized := qualifyOperationSchemaContractReferences(output.String())
	localized, err = localizeOperationSchemaReferences(localized, module, plan, schemaIndex, names)
	if err != nil {
		return nil, err
	}
	result := []byte(localized)
	if linksType != "never" {
		// The Link factory's types are exact operation slots. It introduces no
		// schema import requests and resolves group bindings only after freeze.
		factory, err := emitOperationLinkFactory(document, plan, module, links, linkGroups, names)
		if err != nil {
			return nil, err
		}
		result = append(result, factory...)
	}
	localizedHelpers, err := localizeOperationHelperTypes(string(result), module, plan)
	if err != nil {
		return nil, err
	}
	return []byte(localizedHelpers), nil
}

func emitOperationLinkFactory(document *ir.Document, plan *semanticModulePlan, module operationModulePlan, links []generatedLink, groups []generatedLinkGroup, names *localIdentifierPlan) ([]byte, error) {
	if names == nil || !names.frozen || names.owner != module.path {
		return nil, fmt.Errorf("Link factory for %q requires its frozen artifact identifier plan", module.routeKey)
	}
	var output bytes.Buffer
	output.WriteString("\n/** Completed exact callables required by this operation's response links. */\n")
	output.WriteString("export interface LinkTargets {\n")
	targets := make(map[string]bool)
	targetInputs := make(map[string]generatedLink)
	for _, link := range links {
		route := operationRouteKey(link.TargetOperation)
		targets[route] = true
		targetInputs[route] = link
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
	output.WriteString("/** Invokes a resolved Link target without exposing an unimplemented callable surface. */\n")
	output.WriteString("export type LinkInvoker = (route: keyof LinkTargets, args: readonly unknown[]) => Promise<unknown>\n\n")
	output.WriteString("/** Creates this operation's response-link container from ready or lazy targets. */\n")
	output.WriteString("export function bindLinks(targets: LinkTargets | LinkInvoker): Links {\n")
	output.WriteString("  const invoke: LinkInvoker = typeof targets === \"function\" ? targets : (route, args) => Reflect.apply(targets[route], targets, args) as Promise<unknown>\n")
	var body bytes.Buffer
	targetReference := func(route string) (string, error) {
		if !targets[route] {
			return "", fmt.Errorf("operation %q has no declared Link target %q", module.routeKey, route)
		}
		specifier, err := plan.relativeModuleSpecifier(module.path, plan.operationByRoute[route])
		if err != nil {
			return "", err
		}
		// Retain target argument checking at the generated boundary without
		// pretending a deferred call already has raw/stream/helper properties.
		target := targetInputs[route]
		qualified := "import(" + quoteTS(specifier) + ")."
		optional := "?"
		if target.TargetOptionsRequired {
			optional = ""
		}
		arguments := "options" + optional + ": " + qualified + "Options"
		if target.TargetHasInput {
			arguments = "input: " + qualified + "Input, " + arguments
		}
		return "((...args: [" + arguments + "]) => invoke(" + quoteTS(route) + ", args) as Promise<" + qualified + "Output>)", nil
	}
	if err := emitLinkValuesForGroups(&body, document, links, groups, targetReference, names); err != nil {
		return nil, err
	}
	bodySource, err := localizeOperationTypeSource(body.String(), module, plan)
	if err != nil {
		return nil, err
	}
	output.WriteString(bodySource)
	value, err := routeLinkGroupsValue(groups, names)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&output, "  return %s as Links\n", value)
	output.WriteString("}\n")
	return output.Bytes(), nil
}

func qualifyOperationSchemaContractReferences(source string) string {
	replacer := strings.NewReplacer(
		"Contract.ComponentInput<", "ContractSchemas.ComponentInput<",
		"Contract.ComponentOutput<", "ContractSchemas.ComponentOutput<",
	)
	return replacer.Replace(source)
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

type operationSchemaReferenceKey struct {
	name   string
	export string
}

func localizeOperationSchemaReferences(source string, module operationModulePlan, plan *semanticModulePlan, schemaIndexSpecifier string, names *localIdentifierPlan) (string, error) {
	if names == nil || names.owner != module.path {
		return "", fmt.Errorf("operation %q requires its artifact identifier owner", module.routeKey)
	}
	const namespace = "ContractSchemas"
	const referencePrefix = namespace + ".Component"
	var occurrences []operationSchemaReference
	counts := make(map[operationSchemaReferenceKey]int)
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
		key := operationSchemaReferenceKey{name: name, export: export}
		counts[key]++
		occurrences = append(occurrences, operationSchemaReference{start: start, end: nameEnd + 1, name: name, export: export})
		search = nameEnd + 1
	}

	keys := make([]operationSchemaReferenceKey, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].name != keys[j].name {
			return keys[i].name < keys[j].name
		}
		return keys[i].export < keys[j].export
	})
	for _, key := range keys {
		path, exists := plan.schemaByName[key.name]
		if !exists || path == "" {
			return "", fmt.Errorf("operation %q has no planned %s projection for component %q", module.routeKey, key.export, key.name)
		}
		path = plan.schemaProjectionPath(key.name, projection(strings.ToLower(key.export)))
		if counts[key] > 1 {
			if err := names.request(typeImportIdentifierKey(path, key.export)); err != nil {
				return "", err
			}
		}
	}
	if err := names.freeze(); err != nil {
		return "", err
	}
	imports := make([]string, 0)
	replacements := make(map[operationSchemaReferenceKey]string, len(counts))
	for _, key := range keys {
		path := plan.schemaProjectionPath(key.name, projection(strings.ToLower(key.export)))
		specifier, err := plan.relativeModuleSpecifier(module.path, path)
		if err != nil {
			return "", err
		}
		replacement := "import(" + quoteTS(specifier) + ")." + key.export
		if counts[key] > 1 {
			alias, err := names.resolve(typeImportIdentifierKey(path, key.export))
			if err != nil {
				return "", err
			}
			imports = append(imports, "import type { "+key.export+" as "+alias+" } from "+quoteTS(specifier))
			replacement = alias
		}
		replacements[key] = replacement
	}
	var output strings.Builder
	output.Grow(len(source))
	cursor := 0
	for _, occurrence := range occurrences {
		output.WriteString(source[cursor:occurrence.start])
		replacement := occurrence.replacement
		if replacement == "" {
			key := operationSchemaReferenceKey{name: occurrence.name, export: occurrence.export}
			var exists bool
			replacement, exists = replacements[key]
			if !exists {
				return "", fmt.Errorf("operation %q has no planned schema reference for %q projection %q", module.routeKey, key.name, key.export)
			}
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
