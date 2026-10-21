package typescript

import (
	"bytes"
	"fmt"
	"strings"
)

const selectedResourceChunkSize = 128
const selectedResourceHelpersPath = "internal/selection/resource-types.ts"

type selectedResourceTypePlan struct {
	root  *resourceNode
	nodes []*resourceNode
	ids   map[*resourceNode]int
}

func selectedResourceTypeModulePath(index int) string {
	return fmt.Sprintf("internal/selection/resources/part-%06d.ts", index)
}

func prepareSelectedResourceTypeModules(plan *semanticModulePlan, root *resourceNode) {
	if len(plan.registryModules) == 0 {
		return
	}
	result := &selectedResourceTypePlan{root: root, ids: make(map[*resourceNode]int)}
	var collect func(*resourceNode)
	collect = func(node *resourceNode) {
		result.ids[node] = len(result.nodes)
		result.nodes = append(result.nodes, node)
		for _, name := range sortedResourceChildNames(node) {
			collect(node.children[name])
		}
		if node.parameterChild != nil {
			collect(node.parameterChild)
		}
	}
	collect(root)
	if len(result.nodes) <= selectedResourceChunkSize {
		return
	}
	plan.selectedResourceTypes = result
	plan.selective = append(plan.selective, artifactPathCandidate{identity: "selected resource helpers", base: selectedResourceHelpersPath})
	for index := 0; index < len(result.nodes); index += selectedResourceChunkSize {
		path := selectedResourceTypeModulePath(index / selectedResourceChunkSize)
		plan.selective = append(plan.selective, artifactPathCandidate{identity: "selected resource types " + path, base: path})
	}
}

func emitSelectedResourceFacade(plan *semanticModulePlan, artifact string) ([]byte, error) {
	var output bytes.Buffer
	if err := registryImport(&output, plan, artifact, "type { Member }", selectedResourceHelpersPath); err != nil {
		return nil, err
	}
	specifier, err := plan.relativeModuleSpecifier(artifact, selectedResourceTypeModulePath(0))
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&output, "export type SelectedResources<G extends RouteKey, P extends RouteKey> = import(%s).Node0<G, P>\n", quoteTS(specifier))
	return output.Bytes(), nil
}

func emitSelectedResourceTypeModulesTo(plan *sourcePlan, write func(Artifact) error) error {
	prepared := plan.modules.selectedResourceTypes
	if plan.selection != nil || prepared == nil || prepared.root != plan.resourceTree {
		return nil
	}
	var helper bytes.Buffer
	if err := registryImport(&helper, plan.modules, selectedResourceHelpersPath, "type { Routes }", plan.modules.fixed["route-index"]); err != nil {
		return err
	}
	helper.WriteString("export type RouteKey = keyof Routes\n")
	helper.WriteString(strings.NewReplacer("type Member<", "export type Member<", "type WithCall<", "export type WithCall<", "type SelectedMembers<", "export type SelectedMembers<").Replace(selectedResourceTypeHelpers))
	if err := write(Artifact{Path: selectedResourceHelpersPath, Data: generatedSource(helper.Bytes())}); err != nil {
		return err
	}
	for start := 0; start < len(prepared.nodes); start += selectedResourceChunkSize {
		path := selectedResourceTypeModulePath(start / selectedResourceChunkSize)
		nodes := prepared.nodes[start:min(start+selectedResourceChunkSize, len(prepared.nodes))]
		source, err := emitSelectedResourceTypeNodes(plan.document, plan.modules, prepared.root, path, nodes)
		if err != nil {
			return err
		}
		if err := write(Artifact{Path: path, Data: generatedSource(source)}); err != nil {
			return err
		}
	}
	return nil
}
