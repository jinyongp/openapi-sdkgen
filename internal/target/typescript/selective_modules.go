package typescript

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	pathpkg "path"
	"sort"
	"strings"
	"unicode/utf8"
)

const selectiveExecutionABI = 1

// Lookup names are shared with operationLookupFilename in the runtime. The
// exact kind and UTF-8 key are framed independently of filesystem characters.
func selectiveLookupArtifact(kind, key string) (string, error) {
	if (kind != "route" && kind != "operation") || !utf8.ValidString(key) {
		return "", fmt.Errorf("invalid selective lookup identity %q", kind)
	}
	digest := sha256.Sum256([]byte(kind + "\x00" + key))
	prefix := "o-"
	if kind == "route" {
		prefix = "r-"
	}
	encoded := hex.EncodeToString(digest[:])
	// Preserve the full digest while respecting the portable segment limit.
	return "selective/lookup/" + prefix + encoded[:16] + "/" + encoded[16:] + ".ts", nil
}

func selectiveStaticArtifact(module operationModulePlan) string {
	result := "selective/operations/" + strings.TrimPrefix(module.path, "internal/operations/")
	if len(result) <= maxArtifactPathBytes {
		return result
	}
	return pathpkg.Join("selective", "operations", "route-"+shortArtifactHash(module.routeKey), pathpkg.Base(module.path))
}

func (plan *semanticModulePlan) planSelective(manifest Manifest) error {
	for _, name := range []string{"index", "types", "all"} {
		plan.selective = append(plan.selective, artifactPathCandidate{identity: "selective " + name, base: "selective/" + name + ".ts"})
	}
	ids := make(map[string]string)
	for _, item := range manifest.Operations {
		ids[manifestRouteKey(item)] = item.OperationID
	}
	for _, module := range plan.operations {
		plan.selective = append(plan.selective, artifactPathCandidate{identity: "static reference " + module.routeKey, base: selectiveStaticArtifact(module)})
		for _, identity := range []struct{ kind, key string }{{"route", module.routeKey}, {"operation", ids[module.routeKey]}} {
			if identity.kind == "operation" && identity.key == "" {
				continue
			}
			artifact, err := selectiveLookupArtifact(identity.kind, identity.key)
			if err != nil {
				return err
			}
			plan.selective = append(plan.selective, artifactPathCandidate{identity: "lookup " + identity.kind + " " + identity.key, base: artifact})
		}
	}
	return nil
}

func hashSelectiveArtifact(digest hash.Hash, artifact Artifact) {
	// Length framing distinguishes every path/data pair without delimiter assumptions.
	fmt.Fprintf(digest, "%d:%s%d:", len(artifact.Path), artifact.Path, len(artifact.Data))
	_, _ = digest.Write(artifact.Data)
}

func emitSelectiveArtifactsTo(plan *sourcePlan, generation string, write func(Artifact) error) error {
	modules := plan.modules
	items := make(map[string]ManifestOperation, len(plan.manifest.Operations))
	for _, item := range plan.manifest.Operations {
		items[manifestRouteKey(item)] = item
	}
	placements := selectiveResourcePlacements(plan.resourceTree)
	linksBySource := make(map[string]map[string]bool)
	for _, link := range plan.links {
		source := operationRouteKey(link.SourceOperation)
		if linksBySource[source] == nil {
			linksBySource[source] = make(map[string]bool)
		}
		linksBySource[source][operationRouteKey(link.TargetOperation)] = true
	}
	for _, module := range modules.operations {
		item := items[module.routeKey]
		execution, exists := plan.executions[module.routeKey]
		if !exists {
			return fmt.Errorf("missing prepared selective execution %q", module.routeKey)
		}
		source, err := emitOperationExecutionProvider(modules, module, item, execution, generation, placements[module.routeKey], sortedStringKeys(linksBySource[module.routeKey]))
		if err != nil {
			return err
		}
		providerPath := operationExecutionArtifactPath(module)
		if err := write(Artifact{Path: providerPath, Data: generatedSource(source)}); err != nil {
			return err
		}
		for _, identity := range []struct{ kind, key string }{{"route", module.routeKey}, {"operation", item.OperationID}} {
			if identity.kind == "operation" && identity.key == "" {
				continue
			}
			artifact, err := selectiveLookupArtifact(identity.kind, identity.key)
			if err != nil {
				return err
			}
			specifier, err := modules.relativeModuleSpecifier(artifact, providerPath)
			if err != nil {
				return err
			}
			body := fmt.Sprintf("import { provider } from %s\nexport const entry = { abi: %d, generation: %s, kind: %s, key: %s, provider } as const\n", quoteTS(specifier), selectiveExecutionABI, quoteTS(generation), quoteTS(identity.kind), quoteTS(identity.key))
			if err := write(Artifact{Path: artifact, Data: generatedSource([]byte(body))}); err != nil {
				return err
			}
		}
		artifact := selectiveStaticArtifact(module)
		providerSpecifier, err := modules.relativeModuleSpecifier(artifact, providerPath)
		if err != nil {
			return err
		}
		loaderSpecifier, err := modules.relativeModuleSpecifier(artifact, "internal/runtime/operation-loader.ts")
		if err != nil {
			return err
		}
		body := fmt.Sprintf("import { provider } from %s\nimport { staticOperationReference } from %s\n/** Direct static reference for bundler-owned code splitting. */\nexport const operation = /* @__PURE__ */ staticOperationReference(provider)\n", quoteTS(providerSpecifier), quoteTS(loaderSpecifier))
		if err := write(Artifact{Path: artifact, Data: generatedSource([]byte(body))}); err != nil {
			return err
		}
	}
	types, err := emitSelectiveTypes(plan)
	if err != nil {
		return err
	}
	entry := fmt.Sprintf(`import { createOperationLoader } from "../internal/runtime/operation-loader.js"
import type { ClientOptions } from "../internal/runtime/configuration.js"
import type { SelectionInput } from "../internal/runtime/selection-types.js"
import type { PreparedOperations } from "../internal/runtime/operation-loader.js"
import type { Operations, RouteReferences, Client } from "./types.js"

const loader = /* @__PURE__ */ createOperationLoader({
  generation: %s,
  baseURL: new URL(import.meta.url),
  loadClient: () => import("../internal/runtime/selected-client.js"),
})
/** References keyed by exact operationId; accessing a key does not load its implementation. */
export const operations = loader.operations as Operations
/** References keyed by the exact METHOD and OpenAPI path template, including ID-less operations. */
export const routes = loader.routes as RouteReferences
/** Prepares only the selected operation code, without binding credentials or sending API requests. */
export function loadOperations<const Selection>(selection: Selection & SelectionInput<Selection>): Promise<PreparedOperations<Selection>> {
  return loader.loadOperations<Selection>(selection)
}
/** Synchronously binds prepared operations to an independent client configuration. */
export function createClient<Selection>(options: ClientOptions & { readonly operations: PreparedOperations<Selection> }): Client<Selection> {
  return loader.createClient(options) as Client<Selection>
}
export type * from "./types.js"
export type { PreparedOperations } from "../internal/runtime/operation-loader.js"
export type { OperationReference, OperationSelection } from "../internal/runtime/selection-types.js"
export type { ClientOptions } from "../internal/runtime/configuration.js"
export { OperationPreparationError } from "../internal/runtime/operation-loader.js"
`, quoteTS(generation))
	all, err := emitSelectiveNames(plan)
	if err != nil {
		return err
	}
	for _, artifact := range []Artifact{
		{Path: "selective/index.ts", Data: generatedSource([]byte(entry))},
		{Path: "selective/types.ts", Data: generatedSource(types)},
		{Path: "selective/all.ts", Data: generatedSource(all)},
	} {
		if err := write(artifact); err != nil {
			return err
		}
	}
	return nil
}

func emitSelectiveTypes(plan *sourcePlan) ([]byte, error) {
	var output bytes.Buffer
	output.WriteString("import type { OperationReference, GuaranteedSelection, PossibleSelection, SelectedOperationCalls } from \"../internal/runtime/selection-types.js\"\n\n")
	output.WriteString("/** Full development-time contract; never a runtime operation registry. */\nexport interface RouteCalls {\n")
	items := make(map[string]ManifestOperation)
	for _, item := range plan.manifest.Operations {
		items[manifestRouteKey(item)] = item
	}
	for _, module := range plan.modules.operations {
		specifier, err := plan.modules.relativeModuleSpecifier("selective/types.ts", module.path)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "  readonly %s: import(%s).ExactCall\n", quoteTS(module.routeKey), quoteTS(specifier))
	}
	output.WriteString("}\nexport type RouteKey = keyof RouteCalls\n\nexport interface OperationRoutes {\n")
	for _, module := range plan.modules.operations {
		item := items[module.routeKey]
		if item.OperationID != "" {
			fmt.Fprintf(&output, "  readonly %s: %s\n", quoteTS(item.OperationID), quoteTS(module.routeKey))
		}
	}
	output.WriteString("}\n\nexport interface OperationLinkRoutes {\n")
	linkIDs := make(map[string]string)
	for _, link := range plan.links {
		route := operationRouteKey(link.SourceOperation)
		if item, exists := items[route]; exists && item.OperationID != "" {
			linkIDs[item.OperationID] = route
		}
	}
	linkNames := make([]string, 0, len(linkIDs))
	for id := range linkIDs {
		linkNames = append(linkNames, id)
	}
	sort.Strings(linkNames)
	for _, id := range linkNames {
		fmt.Fprintf(&output, "  readonly %s: %s\n", quoteTS(id), quoteTS(linkIDs[id]))
	}
	output.WriteString(`}
export type Operations = { readonly [ID in keyof OperationRoutes]: OperationReference<OperationRoutes[ID]> }
export type RouteReferences = { readonly [Route in RouteKey]: OperationReference<Route> }
type G<S> = GuaranteedSelection<S, RouteKey>
type P<S> = PossibleSelection<S, RouteKey> & RouteKey
// Compute membership before entering document-wide mapped types. Re-evaluating
// the feature tree for every operation makes dense selections quadratic.
type SelectedIDs<Guaranteed extends RouteKey, Possible extends RouteKey> = {
  readonly [ID in keyof OperationRoutes as OperationRoutes[ID] extends Guaranteed ? ID : never]: RouteCalls[OperationRoutes[ID]]
} & {
  readonly [ID in keyof OperationRoutes as OperationRoutes[ID] extends Exclude<Possible, Guaranteed> ? ID : never]?: RouteCalls[OperationRoutes[ID]]
}
type LinksFor<Route extends RouteKey> = RouteCalls[Route] extends { readonly links: infer Links } ? Links : never
type SelectedLinkIDs<Guaranteed extends RouteKey, Possible extends RouteKey> = {
  readonly [ID in keyof OperationLinkRoutes as OperationLinkRoutes[ID] extends Guaranteed ? ID : never]: LinksFor<OperationLinkRoutes[ID]>
} & {
  readonly [ID in keyof OperationLinkRoutes as OperationLinkRoutes[ID] extends Exclude<Possible, Guaranteed> ? ID : never]?: LinksFor<OperationLinkRoutes[ID]>
}
`)
	resources, err := emitSelectedResourceTypes(plan.document, plan.modules, plan.resourceTree)
	if err != nil {
		return nil, err
	}
	output.Write(resources)
	output.WriteString("\n/** Only the selected routes and their collision-resolved resource paths. */\nexport type Client<Selection> = {\n  readonly $routes: SelectedOperationCalls<Selection, RouteCalls>\n  readonly $operations: SelectedIDs<G<Selection>, P<Selection>>\n} & SelectedResources<G<Selection>, P<Selection>> & Member<\"$links\", OperationLinkRoutes[keyof OperationLinkRoutes], G<Selection>, P<Selection>, SelectedLinkIDs<G<Selection>, P<Selection>>>\n")
	return output.Bytes(), nil
}

func emitSelectiveNames(plan *sourcePlan) ([]byte, error) {
	var routes, ids []string
	for _, item := range plan.manifest.Operations {
		if item.Visibility == "hidden" {
			continue
		}
		routes = append(routes, manifestRouteKey(item))
		if item.OperationID != "" {
			ids = append(ids, item.OperationID)
		}
	}
	sort.Strings(routes)
	sort.Strings(ids)
	encode := func(values []string) string {
		if values == nil {
			values = []string{}
		}
		data, _ := json.Marshal(values)
		return string(data)
	}
	body := `import { operations as operationReferences, routes as routeReferences } from "./index.js"
import type { Operations, RouteReferences } from "./types.js"
/** Names-only opt-in runtime enumeration; no operation execution modules are imported. */
`
	body += "export const operations = /* @__PURE__ */ Object.freeze(Object.fromEntries((" + encode(ids) + " as const).map(key => [key, operationReferences[key]]))) as Operations\n"
	body += "export const routes = /* @__PURE__ */ Object.freeze(Object.fromEntries((" + encode(routes) + " as const).map(key => [key, routeReferences[key]]))) as RouteReferences\n"
	return []byte(body), nil
}
