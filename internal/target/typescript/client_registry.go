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
	fmt.Fprintf(&output, "import type { Routes } from %s\n", quoteTS(routes))

	type factoryNames struct {
		base       string
		pagination string
		links      string
		stream     string
	}
	factories := make(map[string]factoryNames)
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
		names := factoryNames{base: stablePrivateIdentifier("base-factory", route)}
		imports := []string{"bindBase as " + names.base}
		if operationsByRoute[route].PaginationPlan != nil {
			names.pagination = stablePrivateIdentifier("pagination-factory", route)
			imports = append(imports, "bindPagination as "+names.pagination)
		}
		if linksBySource[route] {
			names.links = stablePrivateIdentifier("links-factory", route)
			imports = append(imports, "bindLinks as "+names.links)
		}
		if streamsByRoute[route] {
			names.stream = stablePrivateIdentifier("stream-factory", route)
			imports = append(imports, "bindStream as "+names.stream)
		}
		factories[route] = names
		fmt.Fprintf(&output, "import { %s } from %s\n", strings.Join(imports, ", "), quoteTS(specifier))
	}
	output.WriteByte('\n')

	output.WriteString("/** Complete callable maps assembled once for one client instance. */\n")
	output.WriteString("export interface CallableRegistry {\n")
	output.WriteString("  readonly routes: {\n")
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" {
			continue
		}
		route := manifestRouteKey(operation)
		fmt.Fprintf(&output, "    readonly %s: Routes[%s][\"call\"]\n", quoteTS(route), quoteTS(route))
	}
	output.WriteString("  }\n")
	output.WriteString("  readonly operations: {\n")
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" || operation.OperationID == "" {
			continue
		}
		route := manifestRouteKey(operation)
		fmt.Fprintf(&output, "    readonly %s: Routes[%s][\"call\"]\n", quoteTS(operation.OperationID), quoteTS(route))
	}
	output.WriteString("  }\n")
	output.WriteString("  readonly links: {\n")
	for _, source := range linkSourceOperations(links) {
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
		base := operationBaseValueName(route)
		fmt.Fprintf(&output, "  const %s = %s(request, inputSchemas, outputSchemas)\n", base, names.base)
		if names.pagination != "" {
			fmt.Fprintf(&output, "  const %s = %s(%s)\n", operationPaginationValueName(route), names.pagination, base)
		}
		if names.stream != "" {
			fmt.Fprintf(&output, "  const %s = %s(request, inputSchemas, outputSchemas)\n", stablePrivateIdentifier("stream-value", route), names.stream)
		}
	}
	output.WriteString("  const completed = {} as { -readonly [Route in keyof Routes]: Routes[Route][\"call\"] }\n")
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" {
			continue
		}
		route := manifestRouteKey(operation)
		if factories[route].links != "" {
			fmt.Fprintf(&output, "  const %s = %s(completed)\n", operationLinksValueName(route), factories[route].links)
		}
	}
	for _, operation := range manifest.Operations {
		if operation.Visibility == "hidden" {
			continue
		}
		route := manifestRouteKey(operation)
		properties := make([]runtimeProperty, 0, 3)
		if factories[route].pagination != "" {
			properties = append(properties, runtimeProperty{key: "paginate", value: operationPaginationValueName(route)})
		}
		if factories[route].links != "" {
			properties = append(properties, runtimeProperty{key: "links", value: operationLinksValueName(route)})
		}
		if factories[route].stream != "" {
			properties = append(properties, runtimeProperty{key: "stream", value: stablePrivateIdentifier("stream-value", route)})
		}
		value := operationBaseValueName(route)
		if len(properties) > 0 {
			fmt.Fprintf(&output, "  const %s = assignCallableProperties(%s, %s) as unknown as Routes[%s][\"call\"]\n", operationValueName(route), value, runtimeObjectExpression(properties), quoteTS(route))
		} else {
			fmt.Fprintf(&output, "  const %s = %s as Routes[%s][\"call\"]\n", operationValueName(route), value, quoteTS(route))
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
		routeValues = append(routeValues, runtimeProperty{key: route, value: operationValueName(route)})
		if operation.OperationID != "" {
			operationValues = append(operationValues, runtimeProperty{key: operation.OperationID, value: operationValueName(route)})
			if factories[route].links != "" {
				linkValues = append(linkValues, runtimeProperty{key: operation.OperationID, value: operationLinksValueName(route)})
			}
		}
	}
	for _, values := range [][]runtimeProperty{routeValues, operationValues, linkValues} {
		sort.SliceStable(values, func(left, right int) bool { return values[left].key < values[right].key })
	}
	for _, property := range routeValues {
		fmt.Fprintf(&output, "  defineOwnDataProperty(completed as Record<string, unknown>, %s, %s)\n", quoteTS(property.key), property.value)
	}
	for _, property := range operationValues {
		fmt.Fprintf(&output, "  defineOwnDataProperty(operations, %s, %s)\n", quoteTS(property.key), property.value)
	}
	for _, property := range linkValues {
		fmt.Fprintf(&output, "  defineOwnDataProperty(linkCalls, %s, %s)\n", quoteTS(property.key), property.value)
	}
	output.WriteString("  return {\n")
	output.WriteString("    routes: completed,\n")
	output.WriteString("    operations: operations as CallableRegistry[\"operations\"],\n")
	output.WriteString("    links: linkCalls as CallableRegistry[\"links\"],\n")
	output.WriteString("  }\n")
	output.WriteString("}\n")
	return output.Bytes(), nil
}

func operationLinksValueName(route string) string {
	return stablePrivateIdentifier("operation-links-value", route)
}
