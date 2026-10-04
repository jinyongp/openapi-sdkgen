package typescript

import (
	"fmt"
	"path"
)

func prepareRuntimeArtifactPlan(plan *sourcePlan) error {
	if err := prepareSchemaRuntimePlan(plan); err != nil {
		return err
	}
	if plan.includeServer {
		if err := prepareServerSchemaRuntimePlan(plan); err != nil {
			return err
		}
	}
	root := plan
	if plan.root != nil {
		root = plan.root
	}
	clientFeatures := clientRuntimeFeatures(root.runtimeFeatures, root.manifest)
	root.modules.runtimeComposition = prepareRuntimeComposition(clientFeatures, len(root.streams) > 0)
	roots := []string{"objects.ts", "configuration.ts", "wire-types.ts", "errors.ts", "http-errors.ts", "constants.ts", "links-types.ts", "callables.ts", "pagination-types.ts", "request.ts", "security.ts", "transport.ts", "selected-client.ts", "named-client.ts", "selection.ts", "selection-types.ts", "contract-types.ts", "wire-properties.ts", "operation.ts"}
	roots = append(roots, "media-codec-types.ts", "stream-protocol-types.ts")
	for _, module := range plan.schemaPrograms.modules {
		roots = append(roots, module.dependencies...)
	}
	for _, feature := range plan.runtimeFeatures.shared {
		if feature == "callable.links" {
			roots = append(roots, "links.ts")
		}
		if feature == "callable.pagination" {
			roots = append(roots, "pagination.ts")
		}
	}
	compositionRoots := make(map[string]bool, len(roots))
	for _, root := range roots {
		compositionRoots[root] = true
	}
	add := func(composition runtimeComposition) {
		for _, dependency := range composition.imports {
			name := path.Base(dependency.path)
			if !compositionRoots[name] {
				roots = append(roots, name)
				compositionRoots[name] = true
			}
		}
	}
	add(root.modules.runtimeComposition)
	for _, execution := range plan.executions {
		add(execution.composition)
	}
	clientArtifacts, err := runtimeArtifactClosure(roots)
	if err != nil {
		return err
	}
	plan.runtimeArtifacts = clientArtifacts
	if plan.includeServer {
		for _, module := range plan.serverSchemaPrograms.modules {
			roots = append(roots, module.dependencies...)
		}
		roots = append(roots, "program-codec.ts")
		plan.serverRuntimeFiles = append([]string(nil), serverRuntimeTemplateNames...)
		// Generic server helpers accept arbitrary schemas and are an explicit
		// full-capability compatibility root, separate from generated routers.
		roots = append(roots, "codecs.ts", "schema-query.ts", "framing-text.ts", "framing-multipart.ts", "stream-sse.ts", "request.ts", "objects.ts")
		plan.serverCompositions = make(map[string][]byte)
		for _, kind := range []string{"callback", "webhook"} {
			features := make(runtimeFeatureSet)
			for _, root := range plan.runtimeFeatures.inbound {
				if root.kind == kind {
					for _, feature := range root.features {
						features[feature] = true
					}
				}
			}
			plan.serverCompositions[kind] = prepareServerComposition(features.sorted())
			for _, dependency := range prepareWireHandlers(features.sorted()).imports {
				roots = append(roots, path.Base(dependency.path))
			}
		}
	}
	if plan.includeServer {
		all, err := runtimeArtifactClosure(roots)
		if err != nil {
			return err
		}
		client := make(map[string]bool)
		for _, template := range clientArtifacts {
			client[template.source] = true
		}
		plan.serverRuntimeArtifacts = nil
		for _, template := range all {
			if !client[template.source] {
				plan.serverRuntimeArtifacts = append(plan.serverRuntimeArtifacts, template)
			}
		}
	}
	return nil
}

func runtimeArtifactClosure(roots []string) ([]runtimeTemplateArtifact, error) {
	seen := make(map[string]bool)
	queue := append([]string(nil), roots...)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] {
			continue
		}
		dependencies, ok := runtimeTemplateDependencies[name]
		if !ok {
			return nil, fmt.Errorf("unplanned runtime template %q", name)
		}
		seen[name] = true
		queue = append(queue, dependencies...)
	}
	var artifacts []runtimeTemplateArtifact
	for _, template := range runtimeTemplateArtifacts {
		if seen[template.source] {
			artifacts = append(artifacts, template)
		}
	}
	if len(artifacts) != len(seen) {
		return nil, fmt.Errorf("runtime template inventory does not cover the prepared closure")
	}
	return artifacts, nil
}
