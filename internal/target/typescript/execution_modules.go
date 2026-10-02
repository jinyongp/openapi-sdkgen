package typescript

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Execution companions deliberately do not sit on the full client import path.
// Their paths reuse the already collision-resolved operation artifact stems.
func operationExecutionArtifactPath(module operationModulePlan) string {
	return "internal/executions/" + strings.TrimPrefix(module.path, "internal/operations/")
}

func emitOperationExecutionProvider(plan *semanticModulePlan, module operationModulePlan, item ManifestOperation, execution operationExecutionPlan, generation string, placements []selectiveResourcePlacement, linkedTargets []string) ([]byte, error) {
	artifact := operationExecutionArtifactPath(module)
	var output bytes.Buffer
	importFrom := func(clause, target string) error {
		specifier, err := plan.relativeModuleSpecifier(artifact, target)
		if err != nil {
			return err
		}
		fmt.Fprintf(&output, "import %s from %s\n", clause, quoteTS(specifier))
		return nil
	}
	if err := importFrom("type { RequestContext }", "internal/runtime/http-types.ts"); err != nil {
		return nil, err
	}
	if execution.inputBundle == "" && len(execution.inputSchemas) > 0 || execution.outputBundle == "" && len(execution.outputSchemas) > 0 {
		if err := importFrom("type { WireSchemas }", "internal/runtime/wire-engine.ts"); err != nil {
			return nil, err
		}
	}
	binders := []string{"bindBase", "type BaseCall"}
	if len(linkedTargets) > 0 {
		binders = append(binders, "bindLinks")
		if err := importFrom("type { OperationExecutionProvider }", "internal/runtime/operation-loader.ts"); err != nil {
			return nil, err
		}
	}
	callType := "BaseCall"
	if execution.hasStream {
		binders = append(binders, "bindStream", "type Stream")
		callType += " & { readonly stream: Stream }"
	}
	if item.compiled.PaginationPlan != nil {
		binders = append(binders, "bindPagination", "type Pagination")
		callType += " & { readonly paginate: Pagination }"
	}
	if err := importFrom("{ "+strings.Join(binders, ", ")+" }", module.path); err != nil {
		return nil, err
	}
	core := "{ createRequestCore }"
	if execution.profile == executionJSON || execution.profile == executionBufferedXML {
		core = "{ createRequestCore, createHTTPServices }"
	}
	if err := importFrom(core, "internal/runtime/http-core.ts"); err != nil {
		return nil, err
	}
	switch execution.profile {
	case executionJSON:
		if err := importFrom("{ jsonWireCodec }", "internal/runtime/wire-engine.ts"); err != nil {
			return nil, err
		}
	case executionBufferedXML:
		if err := importFrom("{ xmlWireCodec, bufferedXMLCodecExtensions }", "internal/runtime/codecs.ts"); err != nil {
			return nil, err
		}
	case executionJSONStream:
		if err := importFrom("{ jsonResponseStreamServices as services }", "internal/runtime/http-stream.ts"); err != nil {
			return nil, err
		}
	case executionGeneral:
		if err := importFrom("{ fullRequestServices as services }", "internal/runtime/http-codecs.ts"); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unrecognized execution profile %q", execution.profile)
	}

	input, out := make([]string, 0, len(execution.inputSchemas)), make([]string, 0, len(execution.outputSchemas))
	index := 0
	for _, group := range []struct {
		names      []string
		projection string
		entries    *[]string
		bundle     string
		local      string
	}{
		{execution.inputSchemas, "inputWireSchema", &input, execution.inputBundle, "inputSchemas"},
		{execution.outputSchemas, "outputWireSchema", &out, execution.outputBundle, "outputSchemas"},
	} {
		if group.bundle != "" {
			if err := importFrom("{ schemas as "+group.local+" }", group.bundle); err != nil {
				return nil, err
			}
			continue
		}
		for _, name := range group.names {
			schemaPath, exists := plan.schemaByName[name]
			if !exists {
				return nil, fmt.Errorf("execution provider %q references unplanned schema %q", module.routeKey, name)
			}
			schemaPath = plan.schemaProjectionPath(name, projection(strings.TrimSuffix(group.projection, "WireSchema")))
			alias := fmt.Sprintf("schema%d", index)
			index++
			if err := importFrom("{ "+group.projection+" as "+alias+" }", schemaPath); err != nil {
				return nil, err
			}
			*group.entries = append(*group.entries, "["+quoteTS(name)+", "+alias+"]")
		}
	}
	output.WriteByte('\n')
	if execution.profile == executionJSON {
		output.WriteString("const services = /* @__PURE__ */ createHTTPServices(jsonWireCodec, { encodeRequestBody(_contentType, value) { return JSON.stringify(value) } })\n")
	} else if execution.profile == executionBufferedXML {
		output.WriteString("const services = /* @__PURE__ */ createHTTPServices(xmlWireCodec, bufferedXMLCodecExtensions)\n")
	}
	inputArg, outputArg := "undefined", "undefined"
	if execution.inputBundle != "" {
		inputArg = "inputSchemas"
	}
	if execution.outputBundle != "" {
		outputArg = "outputSchemas"
	}
	if len(input) > 0 {
		fmt.Fprintf(&output, "const inputSchemas: WireSchemas = /* @__PURE__ */ Object.fromEntries([%s])\n", strings.Join(input, ", "))
		inputArg = "inputSchemas"
	}
	if len(out) > 0 {
		fmt.Fprintf(&output, "const outputSchemas: WireSchemas = /* @__PURE__ */ Object.fromEntries([%s])\n", strings.Join(out, ", "))
		outputArg = "outputSchemas"
	}
	output.WriteString("\n/** Compiler-owned base execution provider; no client configuration is cached here. */\n")
	output.WriteString("export const provider = /* @__PURE__ */ Object.freeze({\n")
	fmt.Fprintf(&output, "  abi: %d, generation: %s,\n", selectiveExecutionABI, quoteTS(generation))
	if len(placements) > 0 {
		data, err := json.Marshal(placements)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&output, "  resources: %s,\n", data)
	}
	fmt.Fprintf(&output, "  route: %s,\n", quoteTS(module.routeKey))
	if item.compiled.OperationID != "" {
		fmt.Fprintf(&output, "  operationID: %s,\n", quoteTS(item.compiled.OperationID))
	}
	fmt.Fprintf(&output, "  profile: %s,\n", quoteTS(string(execution.profile)))
	fmt.Fprintf(&output, "  bind(context: RequestContext): %s {\n", callType)
	output.WriteString("    const request = createRequestCore(context, services)\n")
	fmt.Fprintf(&output, "    const base = bindBase(request, %s, %s)\n", inputArg, outputArg)
	var capabilities []string
	if execution.hasStream {
		capabilities = append(capabilities, fmt.Sprintf("stream: bindStream(request, %s, %s)", inputArg, outputArg))
	}
	if item.compiled.PaginationPlan != nil {
		capabilities = append(capabilities, "paginate: bindPagination(base)")
	}
	if len(capabilities) == 0 {
		output.WriteString("    return base\n")
	} else {
		fmt.Fprintf(&output, "    return Object.assign(base, { %s })\n", strings.Join(capabilities, ", "))
	}
	output.WriteString("  },\n")
	if len(linkedTargets) > 0 {
		output.WriteString("  bindLinks,\n  linkTargets: {\n")
		for _, route := range linkedTargets {
			target, exists := plan.operationByRoute[route]
			if !exists {
				return nil, fmt.Errorf("Link target %q has no execution module", route)
			}
			specifier, err := plan.relativeModuleSpecifier(artifact, operationExecutionArtifactPath(operationModulePlan{path: target}))
			if err != nil {
				return nil, err
			}
			// Literal dynamic imports let native ESM and bundlers defer the same
			// target. An explicit return type bounds cyclic Link provider types.
			fmt.Fprintf(&output, "    %s: (): Promise<OperationExecutionProvider> => import(%s).then(module => module.provider),\n", quoteTS(route), quoteTS(specifier))
		}
		output.WriteString("  },\n")
	}
	output.WriteString("} as const)\n")
	return output.Bytes(), nil
}
