package typescript

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type executionSchemaModule struct {
	path       string
	projection string
	names      []string
	delta      []string
	blocks     []string
	ordinals   []int
	block      bool
}

const executionSchemaMergePath = "internal/execution-schemas/merge.ts"

func executionSchemaModulePath(projection string, names []string) string {
	digest := sha256.New()
	fmt.Fprintf(digest, "%d:%s", len(projection), projection)
	for _, name := range names {
		fmt.Fprintf(digest, "%d:%s", len(name), name)
	}
	encoded := fmt.Sprintf("%x", digest.Sum(nil))
	return "internal/execution-schemas/" + encoded[:16] + "/" + projection + "-" + encoded[16:] + ".ts"
}

// Repeated closures share their exact maps. Large distinct closures share only
// projected blocks that are subsets of every importing operation's dependencies.
func prepareExecutionSchemaModules(modules *semanticModulePlan, executions map[string]operationExecutionPlan) ([]executionSchemaModule, error) {
	groups := make(map[string]executionSchemaModule)
	uses := make(map[string]int)
	for _, module := range modules.operations {
		execution := executions[module.routeKey]
		for _, group := range []struct {
			projection string
			names      []string
		}{{"input", execution.inputSchemas}, {"output", execution.outputSchemas}} {
			if len(group.names) < 2 {
				continue
			}
			path := executionSchemaModulePath(group.projection, group.names)
			groups[path] = executionSchemaModule{path: path, projection: group.projection, names: group.names}
			uses[path]++
		}
	}
	// Group descriptors by their exact set of consuming closures. This takes
	// linear space in the closure incidence, rather than comparing every pair.
	// Every block is therefore a subset of each importing closure by construction.
	paths := make([]string, 0, len(groups))
	for path := range groups {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var result []executionSchemaModule
	for _, projection := range []string{"input", "output"} {
		consumers := make(map[string][]int)
		for index, path := range paths {
			group := groups[path]
			if group.projection == projection && len(group.names) >= 64 {
				for _, name := range group.names {
					consumers[name] = append(consumers[name], index)
				}
			}
		}
		names := make([]string, 0, len(consumers))
		for name := range consumers {
			names = append(names, name)
		}
		sort.Strings(names)
		incidence := make(map[string][]string)
		membership := make(map[string][]int)
		ordinal := make(map[string]int, len(names))
		for index, name := range names {
			ordinal[name] = index
			indices := consumers[name]
			if len(indices) < 2 {
				continue
			}
			var key strings.Builder
			for _, index := range indices {
				key.WriteString(strconv.Itoa(index))
				key.WriteByte(',')
			}
			identity := key.String()
			incidence[identity] = append(incidence[identity], name)
			membership[identity] = indices
		}
		shared := make(map[string]bool)
		for identity, names := range incidence {
			// Small blocks cost more imports and merge bookkeeping than they save.
			if len(names) < 64 {
				continue
			}
			path := executionSchemaModulePath(projection+"-block", names)
			block := executionSchemaModule{path: path, projection: projection, names: names, block: true}
			for _, name := range names {
				block.ordinals = append(block.ordinals, ordinal[name])
				shared[name] = true
			}
			result = append(result, block)
			for _, index := range membership[identity] {
				group := groups[paths[index]]
				group.blocks = append(group.blocks, path)
				groups[paths[index]] = group
			}
		}
		for _, path := range paths {
			group := groups[path]
			if group.projection != projection || len(group.blocks) == 0 {
				continue
			}
			sort.Strings(group.blocks)
			// Keep the full names for the canonical closure identity and the
			// execution plan. Only emission uses the unshared delta.
			for _, name := range group.names {
				if !shared[name] {
					group.delta = append(group.delta, name)
					group.ordinals = append(group.ordinals, ordinal[name])
				}
			}
			groups[path] = group
		}
	}
	for _, path := range paths {
		if uses[path] > 1 || len(groups[path].blocks) > 0 {
			result = append(result, groups[path])
		}
	}
	if len(result) > 0 {
		for _, module := range result {
			if module.block {
				result = append(result, executionSchemaModule{path: executionSchemaMergePath})
				break
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].path < result[j].path })
	for _, module := range result {
		modules.selective = append(modules.selective, artifactPathCandidate{identity: "execution schemas " + module.path, base: module.path})
	}
	for _, module := range modules.operations {
		execution := executions[module.routeKey]
		if len(execution.inputSchemas) > 1 {
			if path := executionSchemaModulePath("input", execution.inputSchemas); uses[path] > 1 || len(groups[path].blocks) > 0 {
				execution.inputBundle = path
				execution.inputSchemas = groups[path].names
			}
		}
		if len(execution.outputSchemas) > 1 {
			if path := executionSchemaModulePath("output", execution.outputSchemas); uses[path] > 1 || len(groups[path].blocks) > 0 {
				execution.outputBundle = path
				execution.outputSchemas = groups[path].names
			}
		}
		executions[module.routeKey] = execution
	}
	// Validate all added execution artifacts together after runtime composition
	// preparation, rather than rescanning the complete module graph twice.
	return result, nil
}

func emitExecutionSchemaModule(plan *semanticModulePlan, module executionSchemaModule) ([]byte, error) {
	if module.path == executionSchemaMergePath {
		return []byte(executionSchemaMergeSource), nil
	}
	var output bytes.Buffer
	types, err := plan.relativeModuleSpecifier(module.path, "internal/runtime/schema/wire-types.ts")
	if err != nil {
		return nil, err
	}
	if !module.block {
		fmt.Fprintf(&output, "import type { WireSchemas } from %s\n", quoteTS(types))
	}
	var blocks []string
	if module.block || len(module.blocks) > 0 {
		specifier, err := plan.relativeModuleSpecifier(module.path, executionSchemaMergePath)
		if err != nil {
			return nil, err
		}
		if module.block {
			fmt.Fprintf(&output, "import type { SchemaBlock } from %s\n", quoteTS(specifier))
		} else {
			imports := "mergeSchemas"
			if len(module.delta) > 0 {
				imports += ", type SchemaBlock"
			}
			fmt.Fprintf(&output, "import { %s } from %s\n", imports, quoteTS(specifier))
		}
		for index, path := range module.blocks {
			specifier, err := plan.relativeModuleSpecifier(module.path, path)
			if err != nil {
				return nil, err
			}
			alias := fmt.Sprintf("block%d", index)
			fmt.Fprintf(&output, "import { entries as %s } from %s\n", alias, quoteTS(specifier))
			blocks = append(blocks, alias)
		}
	}
	entries := make([]string, 0, len(module.names))
	ordinalIndex := 0
	names := module.names
	if len(module.blocks) > 0 {
		names = module.delta
	}
	for index, name := range names {
		path, exists := plan.schemaByName[name]
		if !exists {
			return nil, fmt.Errorf("execution schema module %q references unplanned schema %q", module.path, name)
		}
		path = plan.schemaProjectionPath(name, projection(module.projection))
		specifier, err := plan.relativeModuleSpecifier(module.path, path)
		if err != nil {
			return nil, err
		}
		alias := fmt.Sprintf("schema%d", index)
		fmt.Fprintf(&output, "import { %sWireSchema as %s } from %s\n", module.projection, alias, quoteTS(specifier))
		entry := "[" + quoteTS(name) + ", " + alias + "]"
		if module.block || len(module.blocks) > 0 {
			entry = "[" + strconv.Itoa(module.ordinals[ordinalIndex]) + ", " + quoteTS(name) + ", " + alias + "]"
			ordinalIndex++
		}
		entries = append(entries, entry)
	}
	if module.block {
		fmt.Fprintf(&output, "\nexport const entries: SchemaBlock = [%s]\n", strings.Join(entries, ", "))
		return output.Bytes(), nil
	}
	if len(module.blocks) > 0 {
		if len(entries) > 0 {
			fmt.Fprintf(&output, "\nconst delta: SchemaBlock = [%s]\n", strings.Join(entries, ", "))
			blocks = append(blocks, "delta")
		}
		fmt.Fprintf(&output, "\nexport const schemas: WireSchemas = /* @__PURE__ */ mergeSchemas([%s])\n", strings.Join(blocks, ", "))
		return output.Bytes(), nil
	}
	fmt.Fprintf(&output, "\nexport const schemas: WireSchemas = /* @__PURE__ */ Object.fromEntries([%s])\n", strings.Join(entries, ", "))
	return output.Bytes(), nil
}
