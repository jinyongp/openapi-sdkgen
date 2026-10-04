package typescript

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
)

// This generated client layer shares assembly code, never service instances,
// request contexts, schema closures, or runtime algorithms.
type executionCompositionModule struct {
	path        string
	composition runtimeComposition
}

func executionCompositionIdentity(features []runtimeFeature, streaming bool) string {
	ordered := append([]runtimeFeature(nil), features...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	digest := sha256.New()
	fmt.Fprintf(digest, "execution-composition-v1:%t:", streaming)
	for _, feature := range ordered {
		fmt.Fprintf(digest, "%d:%s", len(feature), feature)
	}
	return fmt.Sprintf("%x", digest.Sum(nil))
}

func prepareExecutionCompositionModules(modules *semanticModulePlan, executions map[string]operationExecutionPlan) ([]executionCompositionModule, error) {
	uses := make(map[string]int)
	groups := make(map[string]runtimeComposition)
	for _, module := range modules.operations {
		composition := executions[module.routeKey].composition
		uses[composition.identity]++
		groups[composition.identity] = composition
	}
	identities := make([]string, 0, len(groups))
	for identity := range groups {
		identities = append(identities, identity)
	}
	sort.Strings(identities)
	shared := make(map[string]string)
	var result []executionCompositionModule
	for _, identity := range identities {
		if identity == "" || uses[identity] < 2 {
			continue
		}
		artifact := "internal/execution-compositions/" + identity[:16] + "/services-" + identity[16:] + ".ts"
		composition := groups[identity]
		composition.sharedPath = ""
		composition.callerWireTypes = nil
		result = append(result, executionCompositionModule{path: artifact, composition: composition})
		modules.selective = append(modules.selective, artifactPathCandidate{identity: "execution composition " + identity, base: artifact})
		shared[identity] = artifact
	}
	for _, module := range modules.operations {
		execution := executions[module.routeKey]
		execution.composition.sharedPath = shared[execution.composition.identity]
		executions[module.routeKey] = execution
	}
	if err := modules.validate(); err != nil {
		return nil, err
	}
	return result, nil
}

func emitExecutionCompositionModule(plan *semanticModulePlan, module executionCompositionModule) ([]byte, error) {
	var imports bytes.Buffer
	var declarations bytes.Buffer
	if err := emitRuntimeComposition(&declarations, module.composition, func(clause, target string) error {
		specifier, err := plan.relativeModuleSpecifier(module.path, target)
		if err != nil {
			return err
		}
		fmt.Fprintf(&imports, "import %s from %s\n", clause, quoteTS(specifier))
		return nil
	}); err != nil {
		return nil, err
	}
	imports.WriteString("\n/** The executor selected by this prepared HTTP contract. */\nexport { createRequestCore }\n")
	fmt.Fprintf(&imports, "\n/** Creates independent services for one execution provider; accepts no client state. */\nexport function createExecutionServices(): %s {\n", module.composition.servicesType)
	for _, line := range bytes.Split(declarations.Bytes(), []byte("\n")) {
		if len(line) > 0 {
			imports.WriteString("  ")
			imports.Write(line)
		}
		imports.WriteByte('\n')
	}
	imports.WriteString("  return services\n}\n")
	return imports.Bytes(), nil
}
