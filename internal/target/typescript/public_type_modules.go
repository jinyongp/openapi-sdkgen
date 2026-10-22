package typescript

import (
	"bytes"
	"fmt"
	"slices"
	"strings"
)

// Full clients already own the complete contract graph. Scoped/named clients
// retain their existing imports instead of acquiring a document-wide type map.
func usesPublicTypeModules(manifest Manifest, plan *semanticModulePlan) bool {
	return plan.selection == nil && usesRegistryModules(manifest, plan)
}

func routeTypeModulePath(registryPath string) string {
	return strings.Replace(strings.Replace(registryPath, "internal/client/registry-chunks/", "internal/routes/contracts/", 1), "/prepare-", "/types-", 1)
}

func emitRouteTypeModulesTo(manifest Manifest, plan *semanticModulePlan, resourceReachable map[string]bool, write func(Artifact) error) error {
	if !usesPublicTypeModules(manifest, plan) {
		return nil
	}
	items := registryItems(manifest)
	for _, module := range plan.registryModules {
		if module.kind != "prepare" {
			continue
		}
		artifact := routeTypeModulePath(module.path)
		entries := activeRegistryEntries(module, items)
		if len(entries) == 0 {
			continue
		}
		var out bytes.Buffer
		for _, contract := range []string{"Routes", "OperationRoutes", "RouteRequestSections", "ExactRawCalls", "ResourceRawCalls"} {
			fmt.Fprintf(&out, "export interface %s {\n", contract)
			for _, entry := range entries {
				item := items[entry.route]
				if contract == "OperationRoutes" && item.OperationID == "" {
					continue
				}
				specifier, err := operationModuleSpecifier(artifact, entry.route, plan)
				if err != nil {
					return err
				}
				key, value := entry.route, "import("+quoteTS(specifier)+").Contract"
				switch contract {
				case "Routes":
					emitOperationJSDoc(&out, "  ", item)
				case "OperationRoutes":
					key, value = item.OperationID, quoteTS(entry.route)
					emitOperationJSDoc(&out, "  ", item)
				case "RouteRequestSections":
					value = "import(" + quoteTS(specifier) + ").RequestInputs"
					writeJSDoc(&out, "  ", jsDocInline, fmt.Sprintf("Request sections for `%s`.", entry.route))
				case "ExactRawCalls":
					value = "import(" + quoteTS(specifier) + ").RawCall"
				case "ResourceRawCalls":
					value = "import(" + quoteTS(specifier) + ").ResourceRawCall"
				}
				fmt.Fprintf(&out, "  readonly %s: %s\n", quoteTS(key), value)
			}
			out.WriteString("}\n\n")
		}
		// Resolve ID-to-route indexing within the existing bounded contract part.
		// Global consumers reuse these contracts instead of relating two full maps.
		out.WriteString("export type Operations = { readonly [ID in keyof OperationRoutes]: Routes[OperationRoutes[ID]] }\n\n")
		for _, contract := range []string{"RouteSourcesBySection", "OperationIDsBySection", "ResourceRoutesBySection"} {
			fmt.Fprintf(&out, "export interface %s {\n", contract)
			for _, section := range requestInputSectionDescriptors {
				var keys []string
				for _, entry := range entries {
					item := items[entry.route]
					if !slices.Contains(item.InputSections, section.section) {
						continue
					}
					key := entry.route
					if contract == "OperationIDsBySection" {
						if item.OperationID == "" {
							continue
						}
						key = item.OperationID
					} else if contract == "ResourceRoutesBySection" && (section.section == inputSectionPath || item.Visibility != "public" || !resourceReachable[entry.route]) {
						continue
					}
					keys = append(keys, quoteTS(key))
				}
				value := "never"
				if len(keys) != 0 {
					value = strings.Join(keys, " | ")
				}
				fmt.Fprintf(&out, "  readonly %s: %s\n", section.sectionKey, value)
			}
			out.WriteString("}\n\n")
		}
		if err := write(Artifact{Path: artifact, Data: generatedSource(out.Bytes())}); err != nil {
			return err
		}
	}
	return nil
}

// The existing bounded contract parts own literal membership. Helpers combine
// those sets without mapping over every route again for each generic section.
func emitRouteSectionIndexes(out *bytes.Buffer, manifest Manifest, plan *semanticModulePlan) error {
	items := registryItems(manifest)
	parts := make(map[string][]string)
	index := 0
	for _, module := range plan.registryModules {
		if module.kind != "prepare" || len(activeRegistryEntries(module, items)) == 0 {
			continue
		}
		var imports []string
		for _, contract := range []string{"RouteSourcesBySection", "OperationIDsBySection", "ResourceRoutesBySection"} {
			alias := fmt.Sprintf("%sPart%d", contract, index)
			parts[contract] = append(parts[contract], alias)
			imports = append(imports, contract+" as "+alias)
		}
		if err := registryImport(out, plan, plan.fixed["route-helpers"], "type { "+strings.Join(imports, ", ")+" }", routeTypeModulePath(module.path)); err != nil {
			return err
		}
		index++
	}
	for _, contract := range []string{"RouteSourcesBySection", "OperationIDsBySection", "ResourceRoutesBySection"} {
		value := "never"
		if len(parts[contract]) != 0 {
			value = strings.Join(parts[contract], " | ")
		}
		fmt.Fprintf(out, "type %s = %s\n\n", contract, value)
	}
	return nil
}

func emitRoutePartInterfaces(out *bytes.Buffer, manifest Manifest, plan *semanticModulePlan, artifact string, exported bool, contracts ...string) error {
	items := registryItems(manifest)
	parts := make(map[string][]string)
	index := 0
	for _, module := range plan.registryModules {
		if module.kind != "prepare" || len(activeRegistryEntries(module, items)) == 0 {
			continue
		}
		var imports []string
		for _, contract := range contracts {
			alias := fmt.Sprintf("%sPart%d", contract, index)
			parts[contract] = append(parts[contract], alias)
			imports = append(imports, contract+" as "+alias)
		}
		if err := registryImport(out, plan, artifact, "type { "+strings.Join(imports, ", ")+" }", routeTypeModulePath(module.path)); err != nil {
			return err
		}
		index++
	}
	prefix := ""
	if exported {
		prefix = "export "
	}
	for _, contract := range contracts {
		if exported && contract == "Operations" {
			out.WriteString("\n/** Compatibility contracts keyed by exact explicit operation IDs. */\n")
		}
		fmt.Fprintf(out, "%sinterface %s extends %s {}\n", prefix, contract, strings.Join(parts[contract], ", "))
	}
	return nil
}

func emitChunkedRouteIndex(manifest Manifest, plan *semanticModulePlan) ([]byte, error) {
	var out bytes.Buffer
	if err := emitRoutePartInterfaces(&out, manifest, plan, plan.fixed["route-index"], true, "Routes", "OperationRoutes", "Operations"); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func emitDerivedClientTypes(plan *semanticModulePlan, links bool) ([]byte, error) {
	var out bytes.Buffer
	artifact := plan.fixed["client-types"]
	for _, im := range []struct{ clause, target string }{
		{"type { Routes, Operations, OperationRoutes }", plan.fixed["route-index"]},
		{"type { OperationMethod }", plan.fixed["route-helpers"]},
		{"type { OperationTypeIdentity, RouteTypeIdentity }", "internal/runtime/shared/identity.ts"},
		{"type { Surface as Resources }", plan.fixed["resource-index"]},
	} {
		if err := registryImport(&out, plan, artifact, im.clause, im.target); err != nil {
			return nil, err
		}
	}
	out.WriteString(`
type ClientRoutes = { readonly [Route in keyof Routes]: OperationMethod<Route> }
type ClientOperations = { readonly [ID in keyof OperationRoutes]: Operations[ID]["call"] & OperationTypeIdentity<OperationRoutes[ID], "exact"> }
`)
	if links {
		out.WriteString("type ClientLinks = { readonly [ID in keyof Operations as [Operations[ID][\"links\"]] extends [never] ? never : ID]: Operations[ID][\"links\"] & RouteTypeIdentity<OperationRoutes[ID]> }\n")
	}
	out.WriteString("/** Generated API client with exact route, operation-ID, and resource calls. */\nexport interface Client extends Resources {\n  readonly $routes: ClientRoutes\n  readonly $operations: ClientOperations\n")
	if links {
		out.WriteString("  readonly $links: ClientLinks\n")
	}
	out.WriteString("}\n")
	return []byte(generatedTypeImports(out.String())), nil
}
