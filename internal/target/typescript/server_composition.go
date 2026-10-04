package typescript

import (
	"bytes"
	"fmt"
	"strings"
)

func prepareServerComposition(features []runtimeFeature) []byte {
	var output bytes.Buffer
	output.WriteString("import { createWireCodec } from '../internal/runtime/wire-core.js'\nimport type { WireCodec } from '../internal/runtime/wire-types.js'\nimport type { ServerCodecContext, ServerBound } from './runtime-types.js'\nimport { bindServerCodec } from './runtime-codecs.js'\nexport type * from './runtime-types.js'\nexport { InboundRequestError } from './runtime-errors.js'\nexport { matchInboundRoute, normalizeInboundMediaCodecs, normalizeInboundStreamCodecs } from './runtime-shared.js'\nexport { requiresInboundAuthentication, collectInboundSecurityCandidates } from './runtime-authentication.js'\n")
	wire := prepareWireHandlers(features)
	for _, dependency := range wire.imports {
		fmt.Fprintf(&output, "import { %s } from '../%s'\n", strings.Join(dependency.names, ", "), strings.TrimSuffix(dependency.path, ".ts")+".js")
	}
	fields := append([]string(nil), wire.fields...)
	has := func(value string) bool {
		for _, feature := range features {
			if string(feature) == value {
				return true
			}
		}
		return false
	}
	contains := func(value string) bool {
		for _, feature := range features {
			if strings.Contains(string(feature), value) {
				return true
			}
		}
		return false
	}
	xml := contains(".media.xml") || contains(".media.open") || contains(".open-part-media")
	if xml && has("schema.contentSchema") {
		output.WriteString("import type { WireSchema, WireSchemas } from '../internal/runtime/wire-types.js'\nimport { isXMLMediaType } from '../internal/runtime/runtime-support.js'\n")
		for index, field := range fields {
			if field == "decodeContent: decodeSchemaContent" {
				fields[index] = "decodeContent: (value: string, schema: WireSchema, schemas: WireSchemas, ignore: boolean | undefined): unknown => decodeSchemaContent(value, schema, schemas, ignore, (source: string, media: string, contract: WireSchema, components: WireSchemas): unknown => { if(isXMLMediaType(media)) return xml.decodeXML(source, contract.contentSchema ?? {}, components); throw new TypeError(`unsupported contentMediaType ${media}`) })"
			}
		}
	}
	fmt.Fprintf(&output, "const wire: WireCodec = /* @__PURE__ */ createWireCodec({ %s })\n", strings.Join(fields, ", "))
	context := []string{"wire"}
	if xml {
		output.WriteString("import { decodeLegacyXML } from './runtime-legacy-xml.js'\n")
		context = append(context, "decodeLegacyXML")
		dynamic := "undefined"
		if has("schema.dynamic") {
			dynamic = "{ extend: extendDynamicScope, resolve: resolveDynamicReference }"
		}
		output.WriteString("import { createXMLCodec } from '../internal/runtime/xml-codec.js'\n")
		fmt.Fprintf(&output, "const xml: ReturnType<typeof createXMLCodec> = /* @__PURE__ */ createXMLCodec(wire, %s)\n", dynamic)
		context = append(context, "xml")
	}
	// The form hook is shared with parameter decoding, including content params.
	output.WriteString("import { decodeInboundFormValue } from './runtime-parameters.js'\n")
	context = append(context, "decodeFormValue: decodeInboundFormValue")
	frames := []string{}
	for _, kind := range []string{"line-delimited-json", "json-sequence", "sse", "multipart"} {
		if !contains(".framing.complete."+kind) && !contains(".framing.incremental."+kind) {
			continue
		}
		name, file := "decodeInboundJSONFrames", "json-stream"
		if kind == "sse" {
			name, file = "decodeInboundSSEFrames", "sse-stream"
		}
		if kind == "multipart" {
			name, file = "decodeInboundMultipartStream", "multipart"
		}
		if kind != "json-sequence" || !contains(".framing.complete.line-delimited-json") && !contains(".framing.incremental.line-delimited-json") {
			fmt.Fprintf(&output, "import { %s } from './runtime-%s.js'\n", name, file)
		}
		frames = append(frames, quoteTS(kind)+": "+name)
	}
	if len(frames) > 0 {
		context = append(context, "frames: { "+strings.Join(frames, ", ")+" }")
	}
	fmt.Fprintf(&output, "const codecContext: ServerCodecContext = { %s }\n", strings.Join(context, ", "))
	for _, binding := range []struct{ name, source string }{{"decodeInboundParameters", "parameters"}, {"decodeInboundBody", "body"}, {"responseFromHandler", "response"}} {
		fmt.Fprintf(&output, "import { %s as %sImpl } from './runtime-%s.js'\nexport const %s: ServerBound<typeof %sImpl> = /* @__PURE__ */ bindServerCodec(codecContext, %sImpl)\n", binding.name, binding.name, binding.source, binding.name, binding.name, binding.name)
	}
	return []byte(output.String())
}
