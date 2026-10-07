package typescript

import (
	"fmt"

	schemaemit "openapi-sdkgen/internal/target/typescript/schema/emit"
	schemaplan "openapi-sdkgen/internal/target/typescript/schema/plan"
)

// This cache owns semantic nodes and exact dependencies, never an importing
// file's bindings. Shared references stop traversal in preparation and emission.
type schemaDescriptorPlan struct {
	node       *schemaplan.Node
	programs   map[*schemaplan.Node]*schemaRuntimeModule
	references map[*schemaplan.Node]*schemaRuntimeModule
	imports    map[string]schemaProgramImport
	properties bool
}

type operationSchemaPlan struct {
	definition string
	nodes      []*schemaplan.Node
}

func (runtime *schemaRuntimePlan) prepareDescriptor(node *schemaplan.Node, mode wirePropertiesMode, binding *schemaProgramBinding) (*schemaDescriptorPlan, error) {
	result := &schemaDescriptorPlan{node: node, programs: binding.programs, references: make(map[*schemaplan.Node]*schemaRuntimeModule), imports: make(map[string]schemaProgramImport)}
	_, err := schemaemit.Descriptor(node, schemaemit.DescriptorOptions{
		Literal: schemaDescriptorLiteral,
		Program: func(child *schemaplan.Node) (string, error) {
			program, exists := binding.programs[child]
			if !exists {
				return "", fmt.Errorf("unprepared descriptor program")
			}
			result.imports[program.path] = schemaProgramImport{path: program.path, name: "program0"}
			return "", nil
		},
		Reference: func(child *schemaplan.Node) (string, error) {
			shared, err := runtime.sharedDescriptor(child, mode, binding)
			if err != nil || shared == nil {
				return "", err
			}
			result.references[child] = shared
			result.imports[shared.path] = schemaProgramImport{path: shared.path, name: "schema"}
			return "{}", nil
		},
		Properties: func([]schemaemit.PropertyExpression) (string, error) {
			result.properties = true
			return "{}", nil
		},
	})
	return result, err
}

func (wire *wireRenderContext) renderDescriptor(plan *schemaDescriptorPlan) (string, error) {
	for _, dependency := range plan.imports {
		if err := wire.collectSchemaImport(dependency); err != nil {
			return "", err
		}
	}
	wire.usesProperties = wire.usesProperties || plan.properties
	if wire.collectOnly {
		return "{}", nil
	}
	return schemaemit.Descriptor(plan.node, schemaemit.DescriptorOptions{
		Literal: schemaDescriptorLiteral,
		Program: func(child *schemaplan.Node) (string, error) {
			program, exists := plan.programs[child]
			if !exists {
				return "", fmt.Errorf("unprepared descriptor program")
			}
			return wire.schemaImportName(schemaProgramImport{path: program.path, name: "program0"})
		},
		Reference: func(child *schemaplan.Node) (string, error) {
			if shared := plan.references[child]; shared != nil {
				return wire.schemaImportName(schemaProgramImport{path: shared.path, name: "schema"})
			}
			return "", nil
		},
		Properties: func(entries []schemaemit.PropertyExpression) (string, error) {
			properties := make([]runtimeProperty, 0, len(entries))
			for _, entry := range entries {
				properties = append(properties, runtimeProperty{key: entry.Name, value: entry.Expression})
			}
			return wire.propertyExpression(properties)
		},
	})
}

func (wire *wireRenderContext) collectSchemaImport(dependency schemaProgramImport) error {
	if wire.collectOnly {
		if wire.names != nil {
			return wire.names.request(schemaValueIdentifierKey(dependency.path, dependency.name))
		}
		return nil
	}
	if wire.programImports == nil {
		wire.programImports = make(map[localIdentifierKey]schemaProgramImport)
	}
	wire.programImports[schemaValueIdentifierKey(dependency.path, dependency.name)] = dependency
	return nil
}

func (wire *wireRenderContext) schemaImportName(dependency schemaProgramImport) (string, error) {
	return wire.names.resolve(schemaValueIdentifierKey(dependency.path, dependency.name))
}
