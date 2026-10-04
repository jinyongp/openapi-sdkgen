package typescript

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

type executionSchemaModule struct {
	path       string
	projection string
	names      []string
}

func executionSchemaModulePath(projection string, names []string) string {
	digest := sha256.New()
	fmt.Fprintf(digest, "%d:%s", len(projection), projection)
	for _, name := range names {
		fmt.Fprintf(digest, "%d:%s", len(name), name)
	}
	encoded := fmt.Sprintf("%x", digest.Sum(nil))
	return "internal/execution-schemas/" + encoded[:16] + "/" + projection + "-" + encoded[16:] + ".ts"
}

// Repeated closures share exactly their own projected dependencies. Distinct
// closures remain separate, preserving the selected operation's schema boundary.
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
	var result []executionSchemaModule
	paths := make([]string, 0, len(groups))
	for path := range groups {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if uses[path] > 1 {
			result = append(result, groups[path])
			modules.selective = append(modules.selective, artifactPathCandidate{identity: "execution schemas " + path, base: path})
		}
	}
	for _, module := range modules.operations {
		execution := executions[module.routeKey]
		if path := executionSchemaModulePath("input", execution.inputSchemas); uses[path] > 1 {
			execution.inputBundle = path
			execution.inputSchemas = groups[path].names
		}
		if path := executionSchemaModulePath("output", execution.outputSchemas); uses[path] > 1 {
			execution.outputBundle = path
			execution.outputSchemas = groups[path].names
		}
		executions[module.routeKey] = execution
	}
	if err := modules.validate(); err != nil {
		return nil, err
	}
	return result, nil
}

func emitExecutionSchemaModule(plan *semanticModulePlan, module executionSchemaModule) ([]byte, error) {
	var output bytes.Buffer
	types, err := plan.relativeModuleSpecifier(module.path, "internal/runtime/wire-types.ts")
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&output, "import type { WireSchemas } from %s\n", quoteTS(types))
	entries := make([]string, 0, len(module.names))
	for index, name := range module.names {
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
		entries = append(entries, "["+quoteTS(name)+", "+alias+"]")
	}
	fmt.Fprintf(&output, "\nexport const schemas: WireSchemas = /* @__PURE__ */ Object.fromEntries([%s])\n", strings.Join(entries, ", "))
	return output.Bytes(), nil
}
