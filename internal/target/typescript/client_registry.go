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
	callables, err := plan.relativeModuleSpecifier(artifact, "internal/runtime/callables.ts")
	if err != nil {
		return nil, err
	}
	codecs, err := plan.relativeModuleSpecifier(artifact, "internal/runtime/codecs.ts")
	if err != nil {
		return nil, err
	}
	objects, err := plan.relativeModuleSpecifier(artifact, "internal/runtime/objects.ts")
	if err != nil {
		return nil, err
	}
	routes, err := plan.relativeModuleSpecifier(artifact, plan.fixed["route-index"])
	if err != nil {
		return nil, err
	}

	var output bytes.Buffer
	fmt.Fprintf(&output, "import { assignCallableProperties, type RequestFunction } from %s\n", quoteTS(callables))
	fmt.Fprintf(&output, "import type { WireSchemas } from %s\n", quoteTS(codecs))
	fmt.Fprintf(&output, "import { defineOwnDataProperty } from %s\n", quoteTS(objects))
	if plan.selection == nil {
		fmt.Fprintf(&output, "import type { Routes } from %s\n", quoteTS(routes))
	} else {
		output.WriteString("/** Private contracts including response-Link execution dependencies. */\ntype Routes = {\n")
		for _, operation := range manifest.Operations {
			if operation.Visibility == "hidden" {
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
	output.WriteString("export function createCallableRegistry(request: RequestFunction, inputSchemas?: WireSchemas, outputSchemas?: WireSchemas): CallableRegistry {\n")
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" {
			continue
		}
		route := manifestRouteKey(operation)
		names := factories[route]
		base := factories[route].baseValue
		fmt.Fprintf(&output, "  const %s = %s(request, inputSchemas, outputSchemas)\n", base, names.base)
		if names.pagination != "" {
			fmt.Fprintf(&output, "  const %s = %s(%s)\n", factories[route].paginationValue, names.pagination, base)
		}
		if names.stream != "" {
			fmt.Fprintf(&output, "  const %s = %s(request, inputSchemas, outputSchemas)\n", factories[route].streamValue, names.stream)
		}
	}
	output.WriteString("  const completed = {} as { -readonly [Route in keyof Routes]: Routes[Route][\"call\"] }\n")
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" {
			continue
		}
		route := manifestRouteKey(operation)
		if factories[route].links != "" {
			fmt.Fprintf(&output, "  const %s = %s(completed)\n", factories[route].linksValue, factories[route].links)
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
			fmt.Fprintf(&output, "  const %s = assignCallableProperties(%s, %s) as unknown as Routes[%s][\"call\"]\n", factories[route].value, value, runtimeObjectExpression(properties), quoteTS(route))
		} else {
			fmt.Fprintf(&output, "  const %s = %s as Routes[%s][\"call\"]\n", factories[route].value, value, quoteTS(route))
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
	if plan.selection != nil {
		output.WriteString("  const publicRoutes = {} as CallableRegistry[\"routes\"]\n")
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
	if plan.selection == nil {
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
		source = strings.Replace(source, "assignCallableProperties, ", "", 1)
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
