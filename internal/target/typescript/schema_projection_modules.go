package typescript

import (
	"bytes"
	"fmt"
	"path"

	"openapi-sdkgen/internal/compiler/ir"
)

func (plan *semanticModulePlan) schemaProjectionPath(name string, direction projection) string {
	owner := plan.schemaByName[name]
	if !plan.splitSchemaProjections || owner == "" {
		return owner
	}
	return path.Join("internal", "schema-projections", string(direction), path.Base(owner))
}

func (plan *semanticModulePlan) planSchemaProjections() error {
	plan.splitSchemaProjections = true
	for _, schema := range plan.schemas {
		for _, direction := range []projection{projectionInput, projectionOutput} {
			plan.selective = append(plan.selective, artifactPathCandidate{
				identity: "schema projection " + schema.name + " " + string(direction),
				base:     plan.schemaProjectionPath(schema.name, direction),
			})
		}
	}
	return plan.validate()
}

// Component owners remain stable facades for the ordinary SDK. Each descriptor
// and directed type lives in one shared projection module, so native imports of
// an output provider do not evaluate another client's input-only payload.
func emitSchemaProjectionFacade(plan *semanticModulePlan, schema schemaModulePlan) ([]byte, error) {
	var output bytes.Buffer
	for _, direction := range []projection{projectionInput, projectionOutput} {
		specifier, err := plan.relativeModuleSpecifier(schema.path, plan.schemaProjectionPath(schema.name, direction))
		if err != nil {
			return nil, err
		}
		name := "Input"
		wire := schema.inputWire
		if direction == projectionOutput {
			name, wire = "Output", schema.outputWire
		}
		fmt.Fprintf(&output, "export type { %s } from %s\n", name, quoteTS(specifier))
		if wire {
			fmt.Fprintf(&output, "export { %sWireSchema } from %s\n", direction, quoteTS(specifier))
		}
	}
	return output.Bytes(), nil
}

func emitSchemaProjectionLeaf(document *ir.Document, plan *semanticModulePlan, schema schemaModulePlan, direction projection) ([]byte, error) {
	artifact := plan.schemaProjectionPath(schema.name, direction)
	export := "Input"
	usedWire := schema.inputWire
	if direction == projectionOutput {
		export, usedWire = "Output", schema.outputWire
	}
	value := componentSchemaValue(document, schema.name)
	var referenceErr error
	reference := func(name string, projection projection) string {
		typeName := "Input"
		if projection == projectionOutput {
			typeName = "Output"
		}
		if name == schema.name && projection == direction {
			return export
		}
		owner := plan.schemaProjectionPath(name, projection)
		if owner == "" {
			referenceErr = fmt.Errorf("component %q has no %s projection owner", name, projection)
			return "unknown"
		}
		specifier, err := plan.relativeModuleSpecifier(artifact, owner)
		if err != nil {
			referenceErr = err
			return "unknown"
		}
		return "import(" + quoteTS(specifier) + ")." + typeName
	}
	typeSource, err := schemaTypeForScope(document, value, direction, typeRenderModule(schema.name, reference))
	if err != nil {
		return nil, err
	}
	if referenceErr != nil {
		return nil, referenceErr
	}
	var output bytes.Buffer
	if usedWire {
		types, err := plan.relativeModuleSpecifier(artifact, "internal/runtime/codecs.ts")
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "import type { WireSchema, WireProperty } from %s\n", quoteTS(types))
	}
	emitSchemaProjectionJSDoc(&output, export+" projection.", schemaIsAlwaysDeprecated(document, value))
	fmt.Fprintf(&output, "export type %s = %s\n", export, typeSource)
	if usedWire {
		wire := newWireRenderContext(wirePropertiesLiteral)
		descriptor, err := wire.wireSchemaDescriptorForDocument(document, value, direction)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "\nexport const %sWireSchema: WireSchema = %s\n", direction, descriptor)
	}
	return output.Bytes(), nil
}
