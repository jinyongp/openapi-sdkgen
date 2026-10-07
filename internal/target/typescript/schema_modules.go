package typescript

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"openapi-sdkgen/internal/compiler/ir"
)

type renderedSchemaProjections struct {
	imports []string
	input   string
	output  string
}

func emitSchemaProjectionJSDoc(output *bytes.Buffer, summary string, deprecated bool) {
	if !deprecated {
		fmt.Fprintf(output, "/** %s */\n", summary)
		return
	}
	fmt.Fprintf(output, "/**\n * %s\n * @deprecated This OpenAPI schema is deprecated.\n */\n", summary)
}

func emitSchemaArtifactsTo(document *ir.Document, plan *semanticModulePlan, write func(Artifact) error) ([]byte, error) {
	return emitSchemaArtifactsWithRegistriesTo(document, plan, plan, plan, write)
}

func emitSchemaArtifactsWithRegistriesTo(document *ir.Document, plan, index, registry *semanticModulePlan, write func(Artifact) error) ([]byte, error) {
	if plan == nil {
		return nil, fmt.Errorf("internal TypeScript target: prepared plan has no semantic modules")
	}
	for _, schema := range plan.schemas {
		if plan.splitSchemaProjections {
			for _, direction := range []projection{projectionInput, projectionOutput} {
				path := plan.schemaProjectionPath(schema.name, direction)
				source, err := emitSchemaProjectionLeaf(document, plan, schema, direction)
				if err != nil {
					return nil, err
				}
				if err := write(Artifact{Path: path, Data: generatedSource(source)}); err != nil {
					return nil, err
				}
			}
			continue
		}
		source, err := emitSchemaLeaf(document, plan, schema)
		if err != nil {
			return nil, err
		}
		if err := write(Artifact{Path: schema.path, Data: generatedSource(source)}); err != nil {
			return nil, err
		}
	}
	indexSource, err := emitSchemaIndex(document, index)
	if err != nil {
		return nil, err
	}
	wireSource, err := emitSchemaWireRegistry(registry)
	if err != nil {
		return nil, err
	}
	if err := write(Artifact{Path: plan.fixed["schema-index"], Data: generatedSource(indexSource)}); err != nil {
		return nil, err
	}
	if err := write(Artifact{Path: plan.fixed["schema-wire"], Data: generatedSource(wireSource)}); err != nil {
		return nil, err
	}
	return indexSource, nil
}

func emitSchemaLeaf(document *ir.Document, plan *semanticModulePlan, schema schemaModulePlan) ([]byte, error) {
	value := componentSchemaValue(document, schema.name)
	names := newLocalIdentifierPlan(schema.path)
	wire := newWireRenderContext(wirePropertiesLiteral)
	wire.schemaPrograms, wire.names, wire.collectOnly = plan.schemaPrograms, names, true
	for _, direction := range []projection{projectionInput, projectionOutput} {
		if direction == projectionInput && !schema.inputWire || direction == projectionOutput && !schema.outputWire {
			continue
		}
		if _, err := wire.wireSchemaDescriptorForDocument(document, value, direction); err != nil {
			return nil, err
		}
	}
	var projections renderedSchemaProjections
	var err error
	if schema.publicProjection {
		projections, err = renderSchemaProjectionsWithNames(document, plan, schema, value, names)
		if err != nil {
			return nil, fmt.Errorf("component %s projections: %w", schema.name, err)
		}
	} else {
		if err := names.reserve("Input", "Output", "WireSchema", "WireProperty", "inputWireSchema", "outputWireSchema"); err != nil {
			return nil, err
		}
		if err := names.freeze(); err != nil {
			return nil, err
		}
	}
	wire.collectOnly = false
	inputDescriptor := ""
	if schema.inputWire {
		inputDescriptor, err = wire.wireSchemaDescriptorForDocument(document, value, projectionInput)
		if err != nil {
			return nil, fmt.Errorf("component %s input wire schema: %w", schema.name, err)
		}
	}
	outputDescriptor := ""
	if schema.outputWire {
		outputDescriptor, err = wire.wireSchemaDescriptorForDocument(document, value, projectionOutput)
		if err != nil {
			return nil, fmt.Errorf("component %s output wire schema: %w", schema.name, err)
		}
	}

	var output bytes.Buffer
	programImports, err := wire.programImportSource(schema.path)
	if err != nil {
		return nil, err
	}
	output.WriteString(programImports)
	if schema.inputWire || schema.outputWire {
		imports := "WireSchema"
		if wire.usesProperties {
			imports += ", WireProperty"
		}
		fmt.Fprintf(&output, "import type { %s } from \"../runtime/schema/wire-types.js\"\n", imports)
	}
	for _, declaration := range projections.imports {
		output.WriteString(declaration)
		output.WriteByte('\n')
	}
	if output.Len() > 0 {
		output.WriteByte('\n')
	}
	if schema.publicProjection {
		deprecated := schemaIsAlwaysDeprecated(document, value)
		emitSchemaProjectionJSDoc(&output, "Request/input projection.", deprecated)
		fmt.Fprintf(&output, "export type Input = %s\n\n", projections.input)
		emitSchemaProjectionJSDoc(&output, "Response/output projection.", deprecated)
		fmt.Fprintf(&output, "export type Output = %s\n", projections.output)
		if schema.inputWire || schema.outputWire {
			output.WriteByte('\n')
		}
	}
	if schema.inputWire {
		output.WriteString("/** Runtime wire descriptor for the input projection. */\n")
		fmt.Fprintf(&output, "export const inputWireSchema: WireSchema = %s\n", inputDescriptor)
		if schema.outputWire {
			output.WriteByte('\n')
		}
	}
	if schema.outputWire {
		output.WriteString("/** Runtime wire descriptor for the output projection. */\n")
		fmt.Fprintf(&output, "export const outputWireSchema: WireSchema = %s\n", outputDescriptor)
	}
	return output.Bytes(), nil
}

func renderSchemaProjections(document *ir.Document, plan *semanticModulePlan, schema schemaModulePlan, value any, directions ...projection) (renderedSchemaProjections, error) {
	return renderSchemaProjectionsWithNames(document, plan, schema, value, newLocalIdentifierPlan(schema.path), directions...)
}

func renderSchemaProjectionsWithNames(document *ir.Document, plan *semanticModulePlan, schema schemaModulePlan, value any, names *localIdentifierPlan, directions ...projection) (renderedSchemaProjections, error) {
	if len(directions) == 0 {
		directions = []projection{projectionInput, projectionOutput}
	}
	if err := names.reserve("Input", "Output", "WireSchema", "WireProperty", "inputWireSchema", "outputWireSchema"); err != nil {
		return renderedSchemaProjections{}, err
	}
	uses := make([]typeReferenceUse, 0)
	var expected []schemaProjectionReference
	sequence := 0
	var referenceErr error
	countReference := func(name string, direction projection) string {
		expected = append(expected, schemaProjectionReference{name: name, direction: direction})
		if name == schema.name {
			if direction == projectionInput {
				return "Input"
			}
			return "Output"
		}
		path, exists := plan.schemaByName[name]
		if !exists {
			if referenceErr == nil {
				referenceErr = fmt.Errorf("component reference %q has no schema owner", name)
			}
			return "unknown"
		}
		path = plan.schemaProjectionPath(name, direction)
		exportName := "Output"
		if direction == projectionInput {
			exportName = "Input"
		}
		key := fmt.Sprintf("%08d", sequence)
		sequence++
		uses = append(uses, typeReferenceUse{key: key, modulePath: path, exportName: exportName})
		return "unknown"
	}
	countScope := typeRenderModule(schema.name, countReference)
	for _, direction := range directions {
		if _, err := schemaTypeForScope(document, value, direction, countScope); err != nil {
			return renderedSchemaProjections{}, err
		}
	}
	if referenceErr != nil {
		return renderedSchemaProjections{}, referenceErr
	}
	planned, err := planTypeReferences(plan, schema.path, uses, names)
	if err != nil {
		return renderedSchemaProjections{}, err
	}
	byKey := make(map[string]plannedTypeReference, len(planned))
	for _, reference := range planned {
		byKey[reference.key] = reference
	}

	sequence = 0
	referenceErr = nil
	replay := schemaReferenceReplay{owner: schema.path, expected: expected}
	renderReference := func(name string, direction projection) string {
		if err := replay.observe(name, direction); err != nil {
			if referenceErr == nil {
				referenceErr = err
			}
			return "unknown" // The enclosing render fails; this is never published.
		}
		if name == schema.name {
			if direction == projectionInput {
				return "Input"
			}
			return "Output"
		}
		key := fmt.Sprintf("%08d", sequence)
		sequence++
		reference, exists := byKey[key]
		if !exists {
			if referenceErr == nil {
				referenceErr = fmt.Errorf("component reference %q was not planned", name)
			}
			return "unknown"
		}
		exportName := "Output"
		if direction == projectionInput {
			exportName = "Input"
		}
		if reference.modulePath != plan.schemaProjectionPath(name, direction) || reference.exportName != exportName {
			if referenceErr == nil {
				referenceErr = fmt.Errorf("component reference %q (%s) resolves to a different planned target in %q", name, exportName, schema.path)
			}
			return "unknown"
		}
		if reference.inline {
			return "import(" + quoteTS(reference.specifier) + ")." + reference.exportName
		}
		return reference.alias
	}
	renderScope := typeRenderModule(schema.name, renderReference)
	var input, output string
	for _, direction := range directions {
		rendered, err := schemaTypeForScope(document, value, direction, renderScope)
		if err != nil {
			return renderedSchemaProjections{}, err
		}
		if direction == projectionInput {
			input = rendered
		} else {
			output = rendered
		}
	}
	if referenceErr != nil {
		return renderedSchemaProjections{}, referenceErr
	}
	if err := replay.finish(); err != nil {
		return renderedSchemaProjections{}, err
	}
	if sequence != len(uses) {
		return renderedSchemaProjections{}, fmt.Errorf("rendered %d component references, planned %d", sequence, len(uses))
	}

	importsByAlias := make(map[string]plannedTypeReference)
	for _, reference := range planned {
		if !reference.inline {
			importsByAlias[reference.alias] = reference
		}
	}
	aliases := make([]string, 0, len(importsByAlias))
	for alias := range importsByAlias {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	imports := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		reference := importsByAlias[alias]
		imports = append(imports, "import type { "+reference.exportName+" as "+alias+" } from "+quoteTS(reference.specifier))
	}
	return renderedSchemaProjections{imports: imports, input: input, output: output}, nil
}

func emitSchemaIndex(document *ir.Document, plan *semanticModulePlan) ([]byte, error) {
	var output bytes.Buffer
	output.WriteString("/** Component types keyed by exact OpenAPI schema names. */\n")
	output.WriteString("export interface Components {\n")
	for _, schema := range plan.schemas {
		if !schema.publicProjection {
			continue
		}
		value := componentSchemaValue(document, schema.name)
		if object, ok := value.(map[string]any); ok {
			emitSchemaValueJSDoc(&output, document, "  ", object, "OpenAPI component `"+sanitizeComment(schema.name)+"`.")
		} else {
			fmt.Fprintf(&output, "  /** OpenAPI component `%s`. */\n", sanitizeComment(schema.name))
		}
		inputSpecifier, err := plan.relativeModuleSpecifier(plan.fixed["schema-index"], plan.schemaProjectionPath(schema.name, projectionInput))
		if err != nil {
			return nil, fmt.Errorf("component %s registry reference: %w", schema.name, err)
		}
		outputSpecifier, err := plan.relativeModuleSpecifier(plan.fixed["schema-index"], plan.schemaProjectionPath(schema.name, projectionOutput))
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "  readonly %s: {\n", quoteTS(schema.name))
		output.WriteString("    /** Request/input projection. */\n")
		fmt.Fprintf(&output, "    readonly input: import(%s).Input\n", quoteTS(inputSpecifier))
		output.WriteString("    /** Response/output projection. */\n")
		fmt.Fprintf(&output, "    readonly output: import(%s).Output\n", quoteTS(outputSpecifier))
		output.WriteString("  }\n")
	}
	output.WriteString("}\n\n")
	output.WriteString("/** Input projection for an exact OpenAPI component schema name. */\n")
	output.WriteString("export type ComponentInput<Name extends keyof Components> = Components[Name][\"input\"]\n")
	output.WriteString("/** Output projection for an exact OpenAPI component schema name. */\n")
	output.WriteString("export type ComponentOutput<Name extends keyof Components> = Components[Name][\"output\"]\n")
	return output.Bytes(), nil
}

func emitSchemaWireRegistry(plan *semanticModulePlan) ([]byte, error) {
	names, err := planSchemaWireIdentifiers(plan)
	if err != nil {
		return nil, err
	}
	var output bytes.Buffer
	output.WriteString("import type { WireSchemas } from \"../runtime/schema/wire-types.js\"\n")
	inputProperties := make([]runtimeProperty, 0, len(plan.schemas))
	outputProperties := make([]runtimeProperty, 0, len(plan.schemas))
	for _, schema := range plan.schemas {
		if !schema.inputWire && !schema.outputWire {
			continue
		}
		if plan.splitSchemaProjections {
			for _, direction := range []projection{projectionInput, projectionOutput} {
				used, identifier := schema.inputWire, names[schema.name].input
				if direction == projectionOutput {
					used, identifier = schema.outputWire, names[schema.name].output
				}
				if !used {
					continue
				}
				specifier, err := plan.relativeModuleSpecifier(plan.fixed["schema-wire"], plan.schemaProjectionPath(schema.name, direction))
				if err != nil {
					return nil, err
				}
				fmt.Fprintf(&output, "import { %sWireSchema as %s } from %s\n", direction, identifier, quoteTS(specifier))
				property := runtimeProperty{key: schema.name, value: identifier}
				if direction == projectionInput {
					inputProperties = append(inputProperties, property)
				} else {
					outputProperties = append(outputProperties, property)
				}
			}
			continue
		}
		specifier, err := plan.relativeModuleSpecifier(plan.fixed["schema-wire"], schema.path)
		if err != nil {
			return nil, fmt.Errorf("component %s wire registry reference: %w", schema.name, err)
		}
		imports := make([]string, 0, 2)
		if schema.inputWire {
			identifier := names[schema.name].input
			imports = append(imports, "inputWireSchema as "+identifier)
			inputProperties = append(inputProperties, runtimeProperty{key: schema.name, value: identifier})
		}
		if schema.outputWire {
			identifier := names[schema.name].output
			imports = append(imports, "outputWireSchema as "+identifier)
			outputProperties = append(outputProperties, runtimeProperty{key: schema.name, value: identifier})
		}
		fmt.Fprintf(&output, "import { %s } from %s\n", strings.Join(imports, ", "), quoteTS(specifier))
	}
	output.WriteByte('\n')
	output.WriteString("/** Input component wire schemas keyed by exact OpenAPI names. */\n")
	fmt.Fprintf(&output, "export const inputSchemas: WireSchemas = %s\n\n", runtimeObjectExpression(inputProperties))
	output.WriteString("/** Output component wire schemas keyed by exact OpenAPI names. */\n")
	fmt.Fprintf(&output, "export const outputSchemas: WireSchemas = %s\n", runtimeObjectExpression(outputProperties))
	return output.Bytes(), nil
}
