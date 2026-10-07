package typescript

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	schemaemit "openapi-sdkgen/internal/target/typescript/schema/emit"
	schemaplan "openapi-sdkgen/internal/target/typescript/schema/plan"
)

// Descriptor sharing retains complete semantic data, including reference names.
// Algorithm sharing is independent and cannot erase contract-specific values.
func compactSchemaDescriptor(node *schemaplan.Node) bool {
	if len(node.Fields) == 0 || len(node.Fields) == 1 && (node.Fields[0].Name == "reference" || node.Fields[0].Name == "types" || node.Fields[0].Name == "boolean") {
		return true
	}
	return false
}

func (runtime *schemaRuntimePlan) sharedDescriptor(node *schemaplan.Node, mode wirePropertiesMode, binding *schemaProgramBinding) (*schemaRuntimeModule, error) {
	// Both descriptor constructors preserve the same exact identity mapping.
	// A shared owner chooses one representation for every importing surface.
	mode = wirePropertiesConstructed
	identity := runtime.descriptorKeys[node]
	if identity == "" || runtime.owners[node] == nil && runtime.descriptorUses[identity] < 2 {
		return nil, nil
	}
	key := identity
	if module, exists := runtime.descriptorModules[key]; exists {
		return module, nil
	}
	if runtime.sealed {
		return nil, fmt.Errorf("unprepared shared schema descriptor")
	}
	module := &schemaRuntimeModule{path: "internal/schema-descriptors/shared/schema.ts", node: node}
	wire := newWireRenderContext(mode)
	wire.programImports = make(map[string]schemaProgramImport)
	expression, err := schemaemit.Descriptor(node, schemaemit.DescriptorOptions{
		Literal: schemaDescriptorLiteral,
		Program: func(child *schemaplan.Node) (string, error) {
			program, exists := binding.programs[child]
			if !exists {
				return "", fmt.Errorf("unprepared shared descriptor program")
			}
			dependency := schemaProgramImport{path: program.path, name: "program0"}
			if err := wire.collectSchemaImport(dependency); err != nil {
				return "", err
			}
			return wire.schemaImportName(dependency)
		},
		Reference: func(child *schemaplan.Node) (string, error) {
			shared, err := runtime.sharedDescriptor(child, mode, binding)
			if err != nil || shared == nil {
				return "", err
			}
			return wire.importDescriptor(shared)
		},
		Properties: func(entries []schemaemit.PropertyExpression) (string, error) {
			properties := make([]runtimeProperty, 0, len(entries))
			for _, entry := range entries {
				properties = append(properties, runtimeProperty{key: entry.Name, value: entry.Expression})
			}
			return wire.propertyExpression(properties)
		},
	})
	if err != nil {
		return nil, err
	}
	imports, err := wire.programImportSource(module.path)
	if err != nil {
		return nil, err
	}
	types, err := relativeModuleSpecifier(module.path, runtimeTemplatePath("wire-types.ts"))
	if err != nil {
		return nil, err
	}
	source := fmt.Sprintf("import type { WireSchema } from %s\n%s", quoteTS(types), imports)
	module.dependencies = []string{"wire-types.ts"}
	if wire.usesProperties && mode == wirePropertiesConstructed {
		properties, err := relativeModuleSpecifier(module.path, runtimeTemplatePath("wire-properties.ts"))
		if err != nil {
			return nil, err
		}
		source += fmt.Sprintf("import { wireProperties as __sdkgen_Properties } from %s\n", quoteTS(properties))
		module.dependencies = append(module.dependencies, "wire-properties.ts")
	}
	module.source = []byte(source + "export const schema: WireSchema = " + expression + "\n")
	sum := sha256.Sum256(module.source)
	contentKey := base64.RawURLEncoding.EncodeToString(sum[:])
	module.path = "internal/schema-descriptors/shared/schema_" + contentKey + ".ts"
	// The same semantic contract may have different dependency layouts in
	// client and server plans. Only byte-identical emitted data shares a file.
	if existing, exists := runtime.modules["descriptor:"+contentKey]; exists {
		module = existing
	} else {
		runtime.modules["descriptor:"+contentKey] = module
	}
	runtime.descriptorModules[key] = module
	return module, nil
}

func (wire *wireRenderContext) importDescriptor(module *schemaRuntimeModule) (string, error) {
	dependency := schemaProgramImport{path: module.path, name: "schema"}
	if err := wire.collectSchemaImport(dependency); err != nil {
		return "", err
	}
	if wire.collectOnly {
		return "{}", nil
	}
	return wire.schemaImportName(dependency)
}
