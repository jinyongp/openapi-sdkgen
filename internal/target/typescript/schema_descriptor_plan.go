package typescript

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	schemaemit "openapi-sdkgen/internal/target/typescript/schema/emit"
	schemaplan "openapi-sdkgen/internal/target/typescript/schema/plan"
)

// Descriptor sharing retains complete semantic data, including reference names.
// Algorithm sharing is independent and cannot erase contract-specific values.
func (runtime *schemaRuntimePlan) sharedDescriptor(node *schemaplan.Node, mode wirePropertiesMode, binding *schemaProgramBinding) (*schemaRuntimeModule, error) {
	// Both descriptor constructors preserve the same exact identity mapping.
	// A shared owner chooses one representation for every importing surface.
	mode = wirePropertiesConstructed
	identity := runtime.descriptorKeys[node]
	if identity == "" || runtime.owners[node] == nil && runtime.descriptorUses[identity] < 2 {
		return nil, nil
	}
	key := fmt.Sprintf("descriptor:%d:%s", mode, identity)
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
			wire.programImports[program.path] = schemaProgramImport{path: program.path, alias: program.alias}
			return program.alias + "_program0", nil
		},
		Reference: func(child *schemaplan.Node) (string, error) {
			shared, err := runtime.sharedDescriptor(child, mode, binding)
			if err != nil || shared == nil {
				return "", err
			}
			return wire.importDescriptor(shared), nil
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
	module.alias = "__sdkgen_D" + strings.ReplaceAll(contentKey, "-", "$")
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

func (wire *wireRenderContext) importDescriptor(module *schemaRuntimeModule) string {
	if wire.programImports == nil {
		wire.programImports = make(map[string]schemaProgramImport)
	}
	wire.programImports[module.path] = schemaProgramImport{path: module.path, alias: module.alias, name: "schema"}
	return module.alias + "_schema"
}
