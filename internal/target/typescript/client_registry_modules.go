package typescript

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

const registryChunkSize = 256
const registryChunkThreshold = 512
const registryContractsPath = "internal/client/registry-contracts.ts"

type registryEntry struct{ key, route string }
type registryModulePlan struct {
	path, kind string
	entries    []registryEntry
}

func prepareRegistryModules(plan *semanticModulePlan, manifest Manifest, size int) {
	var routes, ids []registryEntry
	for _, item := range manifest.Operations {
		if item.Visibility == "hidden" {
			continue
		}
		route := manifestRouteKey(item)
		routes = append(routes, registryEntry{route, route})
		if item.OperationID != "" {
			ids = append(ids, registryEntry{item.OperationID, route})
		}
	}
	if len(routes) <= registryChunkThreshold {
		return
	}
	plan.selective = append(plan.selective, artifactPathCandidate{identity: "client registry contracts", base: registryContractsPath})
	for _, group := range []struct {
		kind    string
		entries []registryEntry
	}{{"prepare", routes}, {"ids", ids}} {
		sort.Slice(group.entries, func(i, j int) bool { return group.entries[i].key < group.entries[j].key })
		for start := 0; start < len(group.entries); start += size {
			end := min(start+size, len(group.entries))
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(group.kind+"\x00"+group.entries[start].key)))
			path := "internal/client/registry-chunks/" + digest[:16] + "/" + group.kind + "-" + digest[16:] + ".ts"
			plan.registryModules = append(plan.registryModules, registryModulePlan{path: path, kind: group.kind, entries: group.entries[start:end]})
			plan.selective = append(plan.selective, artifactPathCandidate{identity: "client registry " + path, base: path})
			if group.kind == "prepare" {
				part := routeTypeModulePath(path)
				plan.selective = append(plan.selective, artifactPathCandidate{identity: "route contracts " + part, base: part})
			}
		}
	}
}

func usesRegistryModules(manifest Manifest, plan *semanticModulePlan) bool {
	count := 0
	for _, item := range manifest.Operations {
		if item.Visibility != "hidden" {
			count++
		}
	}
	return count > registryChunkThreshold && len(plan.registryModules) > 0
}

func registryItems(manifest Manifest) map[string]ManifestOperation {
	items := make(map[string]ManifestOperation, len(manifest.Operations))
	for _, item := range manifest.Operations {
		if item.Visibility != "hidden" {
			items[manifestRouteKey(item)] = item
		}
	}
	return items
}

func activeRegistryEntries(module registryModulePlan, items map[string]ManifestOperation) []registryEntry {
	var result []registryEntry
	for _, entry := range module.entries {
		item, exists := items[entry.route]
		if exists && (module.kind == "prepare" || !item.dependencyOnly && item.OperationID == entry.key) {
			result = append(result, entry)
		}
	}
	return result
}

func registryImport(output *bytes.Buffer, plan *semanticModulePlan, artifact, clause, target string) error {
	specifier, err := plan.relativeModuleSpecifier(artifact, target)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "import %s from %s\n", clause, quoteTS(specifier))
	return nil
}

func registryLinkSources(links []generatedLink) map[string]bool {
	result := make(map[string]bool)
	for _, link := range links {
		result[operationRouteKey(link.SourceOperation)] = true
	}
	return result
}

func registryEntryHasLinks(entries []registryEntry, sources map[string]bool) bool {
	for _, entry := range entries {
		if sources[entry.route] {
			return true
		}
	}
	return false
}

func registryHasPrivate(items map[string]ManifestOperation) bool {
	for _, item := range items {
		if item.dependencyOnly {
			return true
		}
	}
	return false
}

func emitChunkedRegistry(manifest Manifest, plan *semanticModulePlan, links []generatedLink, streams []generatedStream) ([]byte, error) {
	var out bytes.Buffer
	artifact := plan.fixed["client-registry"]
	requestType := "BufferedRequestFunction"
	if len(streams) > 0 {
		requestType = "RequestFunction"
	}
	for _, im := range []struct{ clause, target string }{
		{"type { " + requestType + " }", "internal/runtime/http/request/request-execution-types.ts"},
		{"type { WireSchemas }", "internal/runtime/schema/wire-types.ts"},
		{"type { CallableRegistry, CompletedRoutes, PreparedRegistryChunk, RegistryRegistrar }", registryContractsPath},
	} {
		if err := registryImport(&out, plan, artifact, im.clause, im.target); err != nil {
			return nil, err
		}
	}
	contract, err := plan.relativeModuleSpecifier(artifact, registryContractsPath)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&out, "export type { CallableRegistry } from %s\n", quoteTS(contract))
	items := registryItems(manifest)
	sources := registryLinkSources(links)
	var prepare, operations, linkCalls []string
	for _, module := range plan.registryModules {
		entries := activeRegistryEntries(module, items)
		if len(entries) == 0 {
			continue
		}
		if module.kind == "prepare" {
			alias := fmt.Sprintf("prepare%d", len(prepare))
			if err := registryImport(&out, plan, artifact, "{ prepare as "+alias+" }", module.path); err != nil {
				return nil, err
			}
			prepare = append(prepare, alias+"(request, inputSchemas, outputSchemas)")
		} else {
			alias := fmt.Sprintf("registerOperations%d", len(operations))
			clause := "registerOperations as " + alias
			operations = append(operations, alias)
			if registryEntryHasLinks(entries, sources) {
				alias := fmt.Sprintf("registerLinks%d", len(linkCalls))
				clause += ", registerLinks as " + alias
				linkCalls = append(linkCalls, alias)
			}
			if err := registryImport(&out, plan, artifact, "{ "+clause+" }", module.path); err != nil {
				return nil, err
			}
		}
	}
	fmt.Fprintf(&out, "\nexport function createCallableRegistry(request: %s, inputSchemas?: WireSchemas, outputSchemas?: WireSchemas): CallableRegistry {\n", requestType)
	fmt.Fprintf(&out, "  const chunks: PreparedRegistryChunk[] = [%s]\n", strings.Join(prepare, ", "))
	out.WriteString("  const completed: CompletedRoutes = {} as CompletedRoutes\n  for (const chunk of chunks) chunk.bindLinks?.(completed)\n  for (const chunk of chunks) chunk.decorate()\n  for (const chunk of chunks) chunk.register(completed)\n")
	routeValue := "completed"
	if registryHasPrivate(items) {
		out.WriteString("  const publicRoutes: CallableRegistry[\"routes\"] = {} as CallableRegistry[\"routes\"]\n  for (const chunk of chunks) chunk.expose?.(publicRoutes)\n")
		routeValue = "publicRoutes"
	}
	out.WriteString("  const operations: Record<string, unknown> = {}\n  const linkCalls: Record<string, unknown> = {}\n")
	fmt.Fprintf(&out, "  const operationRegistrars: RegistryRegistrar[] = [%s]\n  for (const register of operationRegistrars) register(completed, operations)\n", strings.Join(operations, ", "))
	if len(linkCalls) > 0 {
		fmt.Fprintf(&out, "  const linkRegistrars: RegistryRegistrar[] = [%s]\n  for (const register of linkRegistrars) register(completed, linkCalls)\n", strings.Join(linkCalls, ", "))
	}
	fmt.Fprintf(&out, "  return { routes: %s, operations: operations as CallableRegistry[\"operations\"], links: linkCalls as CallableRegistry[\"links\"] }\n}\n", routeValue)
	return out.Bytes(), nil
}

func emitRegistryContracts(manifest Manifest, plan *semanticModulePlan) ([]byte, error) {
	var out bytes.Buffer
	if err := registryImport(&out, plan, registryContractsPath, "type { Routes as PublicRoutes, OperationRoutes }", plan.fixed["route-index"]); err != nil {
		return nil, err
	}
	out.WriteString("interface PrivateRoutes {\n")
	for _, item := range manifest.Operations {
		if item.Visibility == "hidden" || !item.dependencyOnly {
			continue
		}
		route := manifestRouteKey(item)
		specifier, err := operationModuleSpecifier(registryContractsPath, route, plan)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&out, "  readonly %s: import(%s).Contract\n", quoteTS(route), quoteTS(specifier))
	}
	out.WriteString(`}
export type RegistryRoutes = PublicRoutes & PrivateRoutes
export type CompletedRoutes = { -readonly [Route in keyof RegistryRoutes]: RegistryRoutes[Route]["call"] }
type PublicCalls = { readonly [Route in keyof PublicRoutes]: PublicRoutes[Route]["call"] }
type OperationCalls = { readonly [ID in keyof OperationRoutes]: PublicRoutes[OperationRoutes[ID]]["call"] }
type LinkCalls = { readonly [ID in keyof OperationRoutes as [PublicRoutes[OperationRoutes[ID]]["links"]] extends [never] ? never : ID]: PublicRoutes[OperationRoutes[ID]]["links"] }
export interface CallableRegistry { readonly routes: PublicCalls; readonly operations: OperationCalls; readonly links: LinkCalls }
export interface PreparedRegistryChunk {
  bindLinks?(completed: CompletedRoutes): void
  decorate(): void
  register(completed: CompletedRoutes): void
  expose?(routes: CallableRegistry["routes"]): void
}
export type RegistryRegistrar = (completed: CompletedRoutes, values: Record<string, unknown>) => void
`)
	return out.Bytes(), nil
}

func emitRegistryModulesTo(document *ir.Document, manifest Manifest, plan *semanticModulePlan, links []generatedLink, streams []generatedStream, write func(Artifact) error) error {
	if !usesRegistryModules(manifest, plan) {
		return nil
	}
	source, err := emitRegistryContracts(manifest, plan)
	if err != nil {
		return err
	}
	if err := write(Artifact{Path: registryContractsPath, Data: generatedSource(source)}); err != nil {
		return err
	}
	items := registryItems(manifest)
	sources := registryLinkSources(links)
	operations := make(map[string]ir.Operation, len(document.Operations))
	for _, operation := range document.Operations {
		operations[operationRouteKey(operation)] = operation
	}
	streamRoutes := make(map[string]bool)
	for _, stream := range streams {
		streamRoutes[operationRouteKey(stream.Operation)] = true
	}
	factories, err := planRegistryIdentifiers(plan.fixed["client-registry"], manifest, operations, sources, streamRoutes)
	if err != nil {
		return err
	}
	for _, module := range plan.registryModules {
		module.entries = activeRegistryEntries(module, items)
		if len(module.entries) == 0 {
			continue
		}
		var source []byte
		if module.kind == "prepare" {
			source, err = emitRegistryPrepare(plan, module, factories, registryHasPrivate(items), len(streams) > 0)
		} else {
			source, err = emitRegistryIDs(plan, module, sources)
		}
		if err != nil {
			return err
		}
		if err := write(Artifact{Path: module.path, Data: generatedSource(source)}); err != nil {
			return err
		}
	}
	return nil
}

func emitRegistryPrepare(plan *semanticModulePlan, module registryModulePlan, factories map[string]registryIdentifierNames, private, streaming bool) ([]byte, error) {
	var out bytes.Buffer
	requestType := "BufferedRequestFunction"
	if streaming {
		requestType = "RequestFunction"
	}
	contracts := "PreparedRegistryChunk, CompletedRoutes, RegistryRoutes"
	var public []registryEntry
	if private {
		for _, entry := range module.entries {
			if plan.selection.direct[entry.route] {
				public = append(public, entry)
			}
		}
	}
	if len(public) > 0 {
		contracts += ", CallableRegistry"
	}
	for _, im := range []struct{ clause, target string }{
		{"type { " + requestType + " }", "internal/runtime/http/request/request-execution-types.ts"},
		{"type { WireSchemas }", "internal/runtime/schema/wire-types.ts"},
		{"type { " + contracts + " }", registryContractsPath},
		{"{ defineOwnDataProperty }", "internal/runtime/shared/objects.ts"},
	} {
		if err := registryImport(&out, plan, module.path, im.clause, im.target); err != nil {
			return nil, err
		}
	}
	hasLinks, hasCapabilities := false, false
	for _, entry := range module.entries {
		names := factories[entry.route]
		imports := []string{"bindBase as " + names.base}
		for _, capability := range []struct{ export, name string }{{"bindPagination", names.pagination}, {"bindLinks", names.links}, {"bindStream", names.stream}} {
			if capability.name != "" {
				imports = append(imports, capability.export+" as "+capability.name)
				hasCapabilities = true
			}
		}
		hasLinks = hasLinks || names.links != ""
		if err := registryImport(&out, plan, module.path, "{ "+strings.Join(imports, ", ")+" }", plan.operationByRoute[entry.route]); err != nil {
			return nil, err
		}
	}
	if hasCapabilities {
		if err := registryImport(&out, plan, module.path, "{ assignCallableProperties }", "internal/runtime/client/callable-properties.ts"); err != nil {
			return nil, err
		}
	}
	fmt.Fprintf(&out, "\nexport function prepare(request: %s, inputSchemas?: WireSchemas, outputSchemas?: WireSchemas): PreparedRegistryChunk {\n", requestType)
	for _, entry := range module.entries {
		names := factories[entry.route]
		emitTypedConstant(&out, "  ", names.baseValue, "ReturnType<typeof "+names.base+">", names.base+"(request, inputSchemas, outputSchemas)")
		if names.pagination != "" {
			emitTypedConstant(&out, "  ", names.paginationValue, "ReturnType<typeof "+names.pagination+">", names.pagination+"("+names.baseValue+")")
		}
		if names.stream != "" {
			emitTypedConstant(&out, "  ", names.streamValue, "ReturnType<typeof "+names.stream+">", names.stream+"(request, inputSchemas, outputSchemas)")
		}
		if names.links != "" {
			fmt.Fprintf(&out, "  let %s: ReturnType<typeof %s>\n", names.linksValue, names.links)
		}
		fmt.Fprintf(&out, "  let %s: RegistryRoutes[%s][\"call\"]\n", names.value, quoteTS(entry.route))
	}
	out.WriteString("  const chunk: PreparedRegistryChunk = {\n")
	if hasLinks {
		out.WriteString("    bindLinks(completed: CompletedRoutes): void {\n")
		for _, entry := range module.entries {
			names := factories[entry.route]
			if names.links != "" {
				fmt.Fprintf(&out, "      %s = %s(completed)\n", names.linksValue, names.links)
			}
		}
		out.WriteString("    },\n")
	}
	out.WriteString("    decorate(): void {\n")
	for _, entry := range module.entries {
		names := factories[entry.route]
		var properties []runtimeProperty
		for _, capability := range []struct{ key, factory, value string }{{"paginate", names.pagination, names.paginationValue}, {"links", names.links, names.linksValue}, {"stream", names.stream, names.streamValue}} {
			if capability.factory != "" {
				properties = append(properties, runtimeProperty{key: capability.key, value: capability.value})
			}
		}
		value := names.baseValue
		if len(properties) > 0 {
			value = "assignCallableProperties(" + value + ", " + runtimeObjectExpression(properties) + ") as unknown"
		}
		fmt.Fprintf(&out, "      %s = %s as RegistryRoutes[%s][\"call\"]\n", names.value, value, quoteTS(entry.route))
	}
	out.WriteString("    },\n    register(completed: CompletedRoutes): void {\n")
	for _, entry := range module.entries {
		fmt.Fprintf(&out, "      defineOwnDataProperty(completed as Record<string, unknown>, %s, %s)\n", quoteTS(entry.route), factories[entry.route].value)
	}
	out.WriteString("    },\n")
	if len(public) > 0 {
		// The public key set is obtained from the prepared selection, never from
		// the wider private Link dependency map.
		out.WriteString("    expose(routes: CallableRegistry[\"routes\"]): void {\n")
		for _, entry := range public {
			fmt.Fprintf(&out, "      defineOwnDataProperty(routes as Record<string, unknown>, %s, %s)\n", quoteTS(entry.route), factories[entry.route].value)
		}
		out.WriteString("    },\n")
	}
	out.WriteString("  }\n  return chunk\n}\n")
	return out.Bytes(), nil
}

func emitRegistryIDs(plan *semanticModulePlan, module registryModulePlan, sources map[string]bool) ([]byte, error) {
	var out bytes.Buffer
	for _, im := range []struct{ clause, target string }{{"type { CompletedRoutes }", registryContractsPath}, {"{ defineOwnDataProperty }", "internal/runtime/shared/objects.ts"}} {
		if err := registryImport(&out, plan, module.path, im.clause, im.target); err != nil {
			return nil, err
		}
	}
	out.WriteString("\nexport function registerOperations(completed: CompletedRoutes, values: Record<string, unknown>): void {\n")
	for _, entry := range module.entries {
		fmt.Fprintf(&out, "  defineOwnDataProperty(values, %s, completed[%s])\n", quoteTS(entry.key), quoteTS(entry.route))
	}
	out.WriteString("}\n")
	if registryEntryHasLinks(module.entries, sources) {
		out.WriteString("\nexport function registerLinks(completed: CompletedRoutes, values: Record<string, unknown>): void {\n")
		for _, entry := range module.entries {
			if sources[entry.route] {
				fmt.Fprintf(&out, "  defineOwnDataProperty(values, %s, completed[%s].links)\n", quoteTS(entry.key), quoteTS(entry.route))
			}
		}
		out.WriteString("}\n")
	}
	return out.Bytes(), nil
}
