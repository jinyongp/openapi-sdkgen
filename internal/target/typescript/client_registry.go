package typescript

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

func emitClientRegistry(document *ir.Document, manifest Manifest, plan *semanticModulePlan, links []generatedLink, streams []generatedStream) ([]byte, error) {
	if plan == nil {
		return nil, fmt.Errorf("internal TypeScript target: prepared plan has no semantic modules")
	}
	artifact := plan.fixed["client-registry"]
	callables, err := plan.relativeModuleSpecifier(artifact, "internal/runtime/http/request/request-execution-types.ts")
	if err != nil {
		return nil, err
	}
	codecs, err := plan.relativeModuleSpecifier(artifact, "internal/runtime/schema/wire-types.ts")
	if err != nil {
		return nil, err
	}
	objects, err := plan.relativeModuleSpecifier(artifact, "internal/runtime/shared/objects.ts")
	if err != nil {
		return nil, err
	}
	routes, err := plan.relativeModuleSpecifier(artifact, plan.fixed["route-index"])
	if err != nil {
		return nil, err
	}
	// Only private execution dependencies require a wider contract and a
	// separate public map. An explicit selection can have the same two scopes.
	hasPrivateRoutes := false
	for _, operation := range manifest.Operations {
		if operation.Visibility != "hidden" && operation.dependencyOnly {
			hasPrivateRoutes = true
			break
		}
	}

	var output bytes.Buffer
	requestType := "BufferedRequestFunction"
	if len(streams) > 0 {
		requestType = "RequestFunction"
	}
	fmt.Fprintf(&output, "import type { %s } from %s\n", requestType, quoteTS(callables))
	properties, err := plan.relativeModuleSpecifier(artifact, "internal/runtime/client/callable-properties.ts")
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&output, "import { assignCallableProperties } from %s\n", quoteTS(properties))
	fmt.Fprintf(&output, "import type { WireSchemas } from %s\n", quoteTS(codecs))
	fmt.Fprintf(&output, "import { defineOwnDataProperty } from %s\n", quoteTS(objects))
	if !hasPrivateRoutes {
		fmt.Fprintf(&output, "import type { Routes } from %s\n", quoteTS(routes))
	} else {
		fmt.Fprintf(&output, "/** Public contracts extended with private response-Link execution dependencies. */\ntype Routes = import(%s).Routes & {\n", quoteTS(routes))
		for _, operation := range manifest.Operations {
			if operation.Visibility == "hidden" || !operation.dependencyOnly {
				continue
			}
			route := manifestRouteKey(operation)
			specifier, err := operationModuleSpecifier(artifact, route, plan)
			if err != nil {
				return nil, err
			}
			fmt.Fprintf(&output, "  readonly %s: import(%s).Contract\n", quoteTS(route), quoteTS(specifier))
		}
		output.WriteString("}\n")
	}

	operationsByRoute := make(map[string]ir.Operation, len(document.Operations))
	for _, operation := range document.Operations {
		operationsByRoute[operationRouteKey(operation)] = operation
	}
	linksBySource := make(map[string]bool)
	for _, link := range links {
		linksBySource[operationRouteKey(link.SourceOperation)] = true
	}
	streamsByRoute := make(map[string]bool, len(streams))
	for _, stream := range streams {
		streamsByRoute[operationRouteKey(stream.Operation)] = true
	}
	factories, err := planRegistryIdentifiers(artifact, manifest, operationsByRoute, linksBySource, streamsByRoute)
	if err != nil {
		return nil, err
	}
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" {
			continue
		}
		route := manifestRouteKey(operation)
		path, exists := plan.operationByRoute[route]
		if !exists {
			return nil, fmt.Errorf("route %q has no operation module", route)
		}
		specifier, err := plan.relativeModuleSpecifier(artifact, path)
		if err != nil {
			return nil, err
		}
		names := factories[route]
		imports := []string{"bindBase as " + names.base}
		if operationsByRoute[route].PaginationPlan != nil {
			imports = append(imports, "bindPagination as "+names.pagination)
		}
		if linksBySource[route] {
			imports = append(imports, "bindLinks as "+names.links)
		}
		if streamsByRoute[route] {
			imports = append(imports, "bindStream as "+names.stream)
		}
		fmt.Fprintf(&output, "import { %s } from %s\n", strings.Join(imports, ", "), quoteTS(specifier))
	}
	output.WriteByte('\n')

	output.WriteString("/** Complete callable maps assembled once for one client instance. */\n")
	output.WriteString("export interface CallableRegistry {\n")
	output.WriteString("  readonly routes: {\n")
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" || operation.dependencyOnly {
			continue
		}
		route := manifestRouteKey(operation)
		fmt.Fprintf(&output, "    readonly %s: Routes[%s][\"call\"]\n", quoteTS(route), quoteTS(route))
	}
	output.WriteString("  }\n")
	output.WriteString("  readonly operations: {\n")
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" || operation.dependencyOnly || operation.OperationID == "" {
			continue
		}
		route := manifestRouteKey(operation)
		fmt.Fprintf(&output, "    readonly %s: Routes[%s][\"call\"]\n", quoteTS(operation.OperationID), quoteTS(route))
	}
	output.WriteString("  }\n")
	output.WriteString("  readonly links: {\n")
	for _, source := range linkSourceOperations(plan.selection.publicLinks(links)) {
		if source.OperationID == "" {
			continue
		}
		route := operationRouteKey(source)
		fmt.Fprintf(&output, "    readonly %s: Routes[%s][\"links\"]\n", quoteTS(source.OperationID), quoteTS(route))
	}
	output.WriteString("  }\n")
	output.WriteString("}\n\n")

	output.WriteString("/** Binds and decorates every generated operation exactly once. */\n")
	fmt.Fprintf(&output, "export function createCallableRegistry(request: %s, inputSchemas?: WireSchemas, outputSchemas?: WireSchemas): CallableRegistry {\n", requestType)
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" {
			continue
		}
		route := manifestRouteKey(operation)
		names := factories[route]
		base := factories[route].baseValue
		emitTypedConstant(&output, "  ", base, "ReturnType<typeof "+names.base+">", names.base+"(request, inputSchemas, outputSchemas)")
		if names.pagination != "" {
			emitTypedConstant(&output, "  ", factories[route].paginationValue, "ReturnType<typeof "+names.pagination+">", names.pagination+"("+base+")")
		}
		if names.stream != "" {
			emitTypedConstant(&output, "  ", factories[route].streamValue, "ReturnType<typeof "+names.stream+">", names.stream+"(request, inputSchemas, outputSchemas)")
		}
	}
	emitTypedConstant(&output, "  ", "completed", "{ -readonly [Route in keyof Routes]: Routes[Route][\"call\"] }", "{} as { -readonly [Route in keyof Routes]: Routes[Route][\"call\"] }")
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" {
			continue
		}
		route := manifestRouteKey(operation)
		if factories[route].links != "" {
			emitTypedConstant(&output, "  ", factories[route].linksValue, "ReturnType<typeof "+factories[route].links+">", factories[route].links+"(completed)")
		}
	}
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" {
			continue
		}
		route := manifestRouteKey(operation)
		properties := make([]runtimeProperty, 0, 3)
		if factories[route].pagination != "" {
			properties = append(properties, runtimeProperty{key: "paginate", value: factories[route].paginationValue})
		}
		if factories[route].links != "" {
			properties = append(properties, runtimeProperty{key: "links", value: factories[route].linksValue})
		}
		if factories[route].stream != "" {
			properties = append(properties, runtimeProperty{key: "stream", value: factories[route].streamValue})
		}
		value := factories[route].baseValue
		if len(properties) > 0 {
			emitTypedConstant(&output, "  ", factories[route].value, "Routes["+quoteTS(route)+"][\"call\"]", "assignCallableProperties("+value+", "+runtimeObjectExpression(properties)+") as unknown as Routes["+quoteTS(route)+"][\"call\"]")
		} else {
			emitTypedConstant(&output, "  ", factories[route].value, "Routes["+quoteTS(route)+"][\"call\"]", value+" as Routes["+quoteTS(route)+"][\"call\"]")
		}
	}
	output.WriteString("  const operations: Record<string, unknown> = {}\n")
	output.WriteString("  const linkCalls: Record<string, unknown> = {}\n")
	routeValues := make([]runtimeProperty, 0, len(manifest.Operations))
	operationValues := make([]runtimeProperty, 0, len(manifest.Operations))
	linkValues := make([]runtimeProperty, 0, len(links))
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" {
			continue
		}
		route := manifestRouteKey(operation)
		routeValues = append(routeValues, runtimeProperty{key: route, value: factories[route].value})
		if operation.dependencyOnly {
			continue
		}
		if operation.OperationID != "" {
			operationValues = append(operationValues, runtimeProperty{key: operation.OperationID, value: factories[route].value})
			if factories[route].links != "" {
				linkValues = append(linkValues, runtimeProperty{key: operation.OperationID, value: factories[route].linksValue})
			}
		}
	}
	for _, values := range [][]runtimeProperty{routeValues, operationValues, linkValues} {
		sort.SliceStable(values, func(left, right int) bool { return values[left].key < values[right].key })
	}
	for _, property := range routeValues {
		fmt.Fprintf(&output, "  defineOwnDataProperty(completed as Record<string, unknown>, %s, %s)\n", quoteTS(property.key), property.value)
	}
	if hasPrivateRoutes {
		emitTypedConstant(&output, "  ", "publicRoutes", "CallableRegistry[\"routes\"]", "{} as CallableRegistry[\"routes\"]")
		for _, property := range routeValues {
			if plan.selection.direct[property.key] {
				fmt.Fprintf(&output, "  defineOwnDataProperty(publicRoutes as Record<string, unknown>, %s, %s)\n", quoteTS(property.key), property.value)
			}
		}
	}
	for _, property := range operationValues {
		fmt.Fprintf(&output, "  defineOwnDataProperty(operations, %s, %s)\n", quoteTS(property.key), property.value)
	}
	for _, property := range linkValues {
		fmt.Fprintf(&output, "  defineOwnDataProperty(linkCalls, %s, %s)\n", quoteTS(property.key), property.value)
	}
	output.WriteString("  return {\n")
	if !hasPrivateRoutes {
		output.WriteString("    routes: completed,\n")
	} else {
		output.WriteString("    routes: publicRoutes,\n")
	}
	output.WriteString("    operations: operations as CallableRegistry[\"operations\"],\n")
	output.WriteString("    links: linkCalls as CallableRegistry[\"links\"],\n")
	output.WriteString("  }\n")
	output.WriteString("}\n")
	source := output.String()
	used := generatedIdentifiers(source)
	if used["assignCallableProperties"] == 1 {
		source = strings.Replace(source, "import { assignCallableProperties } from "+quoteTS(properties)+"\n", "", 1)
	}
	if used["defineOwnDataProperty"] == 1 {
		source = strings.Replace(source, "import { defineOwnDataProperty } from "+quoteTS(objects)+"\n", "", 1)
	}
	for _, parameter := range []string{"request", "inputSchemas", "outputSchemas"} {
		if used[parameter] == 1 {
			source = strings.Replace(source, parameter+":", "_"+parameter+":", 1)
			source = strings.Replace(source, parameter+"?:", "_"+parameter+"?:", 1)
		}
	}
	return []byte(generatedTypeImports(source)), nil
}
