package typescript

import (
	"bytes"
	"fmt"
	"strings"
)

func emitNamedClientArtifactsTo(shared *sourcePlan, generation string, write func(Artifact) error) error {
	for _, client := range shared.clients {
		view := client.view
		base := "clients/" + client.name + "/"
		types, err := emitSelectiveTypesAt(view, base+"types.ts", true)
		if err != nil {
			return err
		}
		schemas, err := emitNamedClientSchemaTypes(view, base+"types.ts")
		if err != nil {
			return err
		}
		types = append(types, schemas...)
		var entry bytes.Buffer
		var aliases []string
		for _, module := range view.modules.operations {
			if !view.selection.direct[module.routeKey] {
				continue
			}
			specifier, err := view.modules.relativeModuleSpecifier(base+"index.ts", operationExecutionArtifactPath(module))
			if err != nil {
				return err
			}
			alias := fmt.Sprintf("provider%d", len(aliases))
			aliases = append(aliases, alias)
			fmt.Fprintf(&entry, "import { provider as %s } from %s\n", alias, quoteTS(specifier))
		}
		factory, err := view.modules.relativeModuleSpecifier(base+"index.ts", "internal/runtime/client/named-client.ts")
		if err != nil {
			return err
		}
		options, err := view.modules.relativeModuleSpecifier(base+"index.ts", "internal/runtime/http/configuration.ts")
		if err != nil {
			return err
		}
		fmt.Fprintf(&entry, "import { createNamedClientFactory } from %s\nimport type { ClientOptions } from %s\nimport type { Client } from \"./types.js\"\n\n", quoteTS(factory), quoteTS(options))
		emitTypedConstant(&entry, "", "bind", "ReturnType<typeof createNamedClientFactory>", "/* @__PURE__ */ createNamedClientFactory(["+strings.Join(aliases, ", ")+"], "+quoteTS(generation)+")")
		entry.WriteString("/** Creates an independently configured client for this entry's selected APIs. */\nexport function createClient(options: ClientOptions): Client {\n  return bind(options) as Client\n}\nexport type * from \"./types.js\"\n")
		fmt.Fprintf(&entry, "export type { ClientOptions } from %s\n", quoteTS(options))
		for _, artifact := range []Artifact{{Path: base + "index.ts", Data: generatedSource(entry.Bytes())}, {Path: base + "types.ts", Data: generatedSource(types)}} {
			if err := write(artifact); err != nil {
				return err
			}
		}
	}
	return nil
}

func emitNamedClientSchemaTypes(view *sourcePlan, artifact string) ([]byte, error) {
	input, output := make(map[string]bool), make(map[string]bool)
	for _, item := range view.manifest.Operations {
		execution := view.executions[manifestRouteKey(item)]
		for _, name := range execution.inputSchemas {
			input[name] = true
		}
		for _, name := range execution.outputSchemas {
			output[name] = true
		}
	}
	var source bytes.Buffer
	source.WriteString("\n/** Component projections used by this client's API contracts. */\nexport interface Components {\n")
	names := make(map[string]bool)
	for name := range input {
		names[name] = true
	}
	for name := range output {
		names[name] = true
	}
	for _, name := range sortedStringKeys(names) {
		fmt.Fprintf(&source, "  readonly %s: {\n", quoteTS(name))
		for _, direction := range []projection{projectionInput, projectionOutput} {
			used, export := input[name], "Input"
			if direction == projectionOutput {
				used, export = output[name], "Output"
			}
			value := "never"
			if used {
				specifier, err := view.modules.relativeModuleSpecifier(artifact, view.modules.schemaProjectionPath(name, direction))
				if err != nil {
					return nil, err
				}
				value = "import(" + quoteTS(specifier) + ")." + export
			}
			fmt.Fprintf(&source, "    readonly %s: %s\n", direction, value)
		}
		source.WriteString("  }\n")
	}
	source.WriteString("}\nexport type ComponentInput<Name extends keyof Components> = Components[Name][\"input\"]\nexport type ComponentOutput<Name extends keyof Components> = Components[Name][\"output\"]\n")
	return source.Bytes(), nil
}
