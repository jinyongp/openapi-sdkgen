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

// Each descriptor and directed type has one shared module. Consumers import the
// required projection directly, without an intermediate component facade.
func emitSchemaProjectionLeaf(document *ir.Document, plan *semanticModulePlan, schema schemaModulePlan, direction projection) ([]byte, error) {
	artifact := plan.schemaProjectionPath(schema.name, direction)
	export := "Input"
	usedWire := schema.inputWire
	if direction == projectionOutput {
		export, usedWire = "Output", schema.outputWire
	}
	value := componentSchemaValue(document, schema.name)
	projected := schema
	projected.path = artifact
	projections, err := renderSchemaProjections(document, plan, projected, value, direction)
	if err != nil {
		return nil, err
	}
	typeSource := projections.input
	if direction == projectionOutput {
		typeSource = projections.output
	}
	var output bytes.Buffer
	for _, declaration := range projections.imports {
		output.WriteString(declaration)
		output.WriteByte('\n')
	}
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
