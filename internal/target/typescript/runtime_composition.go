package typescript

import (
	"bytes"
	"fmt"
	"strings"
)

// A composition contains only preparation-owned imports and factory calls.
// Every algorithm remains in one canonical runtime template.
type runtimeComposition struct {
	imports      []runtimeHandlerImport
	declarations []string
	servicesType string
	wireTypes    []string
	httpTypes    []string
}

func prepareRuntimeComposition(features []runtimeFeature, streaming bool) runtimeComposition {
	result := runtimeComposition{servicesType: "RequestExecutionServices", wireTypes: []string{"WireCodec"}, httpTypes: []string{"RequestExecutionServices"}}
	imports := make(map[string]map[string]bool)
	add := func(source string, names ...string) {
		source = strings.TrimSuffix(strings.TrimPrefix(source, "internal/runtime/"), ".ts")
		if imports[source] == nil {
			imports[source] = make(map[string]bool)
		}
		for _, name := range names {
			imports[source][name] = true
		}
	}
	has := func(feature string) bool {
		for _, value := range features {
			if string(value) == feature {
				return true
			}
		}
		return false
	}
	prefix := func(value string) bool {
		for _, feature := range features {
			if strings.HasPrefix(string(feature), value) {
				return true
			}
		}
		return false
	}
	media := func(kind string) bool {
		for _, feature := range features {
			if strings.HasSuffix(string(feature), ".media."+kind) {
				return true
			}
		}
		return false
	}
	open := media("open")
	multipartRequest := open || has("request.media.multipart")
	multipartResponse := open || has("response.media.multipart")
	xmlNeeded := open || media("xml") || prefix("request.open-part-media") || prefix("response.open-part-media")
	wire := prepareWireHandlers(features)
	for _, dependency := range wire.imports {
		add(dependency.path, dependency.names...)
	}
	add("internal/runtime/wire-core.ts", "createWireCodec")
	fields := append([]string(nil), wire.fields...)
	if xmlNeeded && has("schema.contentSchema") {
		result.wireTypes = append(result.wireTypes, "WireSchema", "WireSchemas")
		for index, field := range fields {
			if field == "decodeContent: decodeSchemaContent" {
				fields[index] = "decodeContent: (value: string, schema: WireSchema, schemas: WireSchemas, ignore: boolean | undefined): unknown => decodeSchemaContent(value, schema, schemas, ignore, (source: string, media: string, contract: WireSchema, components: WireSchemas): unknown => { if (isXMLMediaType(media)) return xml.decodeXML(source, contract.contentSchema ?? {}, components); throw new TypeError(`unsupported contentMediaType ${media}`) })"
			}
		}
		add("internal/runtime/runtime-support.ts", "isXMLMediaType")
	}
	result.declarations = append(result.declarations, "const wire: WireCodec = /* @__PURE__ */ createWireCodec({ "+strings.Join(fields, ", ")+" })")
	xml := "undefined"
	extensions := []string{}
	extensionSources := []string{}
	if xmlNeeded {
		add("internal/runtime/xml-codec.ts", "createXMLCodec")
		dynamic := "undefined"
		if has("schema.dynamic") {
			dynamic = "{ extend: extendDynamicScope, resolve: resolveDynamicReference }"
		}
		result.declarations = append(result.declarations, "const xml: ReturnType<typeof createXMLCodec> = /* @__PURE__ */ createXMLCodec(wire, "+dynamic+")")
		xml = "xml"
		extensionSources = append(extensionSources, "xml")
	}
	if multipartRequest || multipartResponse {
		add("internal/runtime/http-header-content.ts", "createHeaderContentDecoder")
		result.declarations = append(result.declarations, "const header: ReturnType<typeof createHeaderContentDecoder> = /* @__PURE__ */ createHeaderContentDecoder(wire, "+xml+")")
	}
	if multipartRequest {
		add("internal/runtime/http-multipart-request.ts", "createMultipartRequestServices")
		result.declarations = append(result.declarations, "const multipartRequest: ReturnType<typeof createMultipartRequestServices> = /* @__PURE__ */ createMultipartRequestServices(wire, "+xml+", header)")
	}
	if multipartResponse {
		add("internal/runtime/http-multipart-response.ts", "createMultipartResponseServices")
		result.declarations = append(result.declarations, "const multipartResponse: ReturnType<typeof createMultipartResponseServices> = /* @__PURE__ */ createMultipartResponseServices(wire, "+xml+", header)")
		extensions = append(extensions, "decodeMultipartResponse: multipartResponse.decodeMultipartResponse")
	}
	encoders := []string{}
	for _, kind := range []string{"json", "form", "text", "binary", "custom"} {
		if !open && !has("request.media."+kind) {
			continue
		}
		name := "encode" + strings.ToUpper(kind[:1]) + kind[1:] + "Body"
		if kind == "json" {
			name = "encodeJSONBody"
		}
		add("internal/runtime/http-body-"+kind+".ts", name)
		encoders = append(encoders, kind+": "+name)
	}
	if multipartRequest {
		encoders = append(encoders, "multipart: multipartRequest.encodeBody")
	}
	if open || has("request.media.xml") {
		result.wireTypes = append(result.wireTypes, "WireSchema", "WireSchemas", "MediaCodec")
		encoders = append(encoders, "xml: (_media: string, value: unknown, _codecs: ReadonlyMap<string, MediaCodec<unknown>>, schema: WireSchema | undefined, schemas: WireSchemas): BodyInit => xml.encodeXML(value, schema ?? {}, schemas)")
	}
	add("internal/runtime/http-body.ts", "createBodyEncoder")
	extensions = append(extensions, "encodeRequestBody: /* @__PURE__ */ createBodyEncoder({ "+strings.Join(encoders, ", ")+" })")
	requestFrames := []string{}
	for _, kind := range []string{"line-delimited-json", "json-sequence", "sse"} {
		if !prefix("request.framing.") || !(has("request.framing.complete."+kind) || has("request.framing.incremental."+kind)) {
			continue
		}
		add("internal/runtime/http-request-text-stream.ts", "createTextRequestEncoder")
		encoder := "encodeSSERequestItem"
		if kind == "sse" {
			add("internal/runtime/http-request-sse-frame.ts", encoder)
		} else {
			add("internal/runtime/http-request-json-frame.ts", "encodeJSONStreamItem")
			encoder = "(value: unknown): string => `"
			if kind == "json-sequence" {
				encoder += "\\u001e"
			}
			encoder += "${encodeJSONStreamItem(value)}\\n`"
		}
		requestFrames = append(requestFrames, quoteTS(kind)+": /* @__PURE__ */ createTextRequestEncoder("+encoder+")")
	}
	if prefix("request.framing.") {
		add("internal/runtime/http-request-stream.ts", "createRequestStreamServices")
		multipart := "undefined"
		if multipartRequest {
			multipart = "multipartRequest.encodeStream"
		}
		extensionSources = append(extensionSources, "/* @__PURE__ */ createRequestStreamServices(wire, { "+strings.Join(requestFrames, ", ")+" }, "+multipart+")")
	}
	if prefix("response.framing.") || streaming {
		result.httpTypes = append(result.httpTypes, "HTTPCodecExtensions", "HTTPStreamDecodeOptions")
		add("internal/runtime/stream-core.ts", "createStreamDecoder")
		protocols := []string{}
		for _, kind := range []string{"line-delimited-json", "json-sequence", "sse"} {
			if !has("response.framing.complete."+kind) && !has("response.framing.incremental."+kind) {
				continue
			}
			name := "decodeJSONStreamItems"
			file := "stream-json"
			if kind == "sse" {
				name = "decodeSSEStreamFrames"
				file = "stream-sse"
			}
			add("internal/runtime/"+file+".ts", name)
			protocols = append(protocols, quoteTS(kind)+": "+name)
		}
		result.declarations = append(result.declarations, "const decodeFrames: ReturnType<typeof createStreamDecoder> = /* @__PURE__ */ createStreamDecoder({ "+strings.Join(protocols, ", ")+" })")
		extra := ""
		if multipartResponse {
			extra = ", ...(options.streamFraming === \"multipart\" ? { multipartFrames: (): AsyncIterable<unknown> => multipartResponse.decodeStreamItems(body, options) } : {})"
		}
		result.declarations = append(result.declarations, "const decodeStreams: NonNullable<HTTPCodecExtensions[\"decodeResponseStreamItems\"]> = (body: ReadableStream<Uint8Array>, options: HTTPStreamDecodeOptions): AsyncIterable<unknown> => decodeFrames(body, { contentType: options.contentType, streamFraming: options.streamFraming, maxFrameBytes: options.maxFrameBytes, streamCodec: options.streamCodec, signal: options.signal"+extra+" })")
		extensions = append(extensions, "decodeResponseStreamItems: decodeStreams")
	}
	httpFactory := "createBasicHTTPServices"
	httpTemplate := "http-basic"
	if has("http.general") || streaming || prefix("request.framing.") || prefix("response.framing.") || media("open") || media("xml") || media("multipart") || media("form") || media("text") || media("binary") || media("custom") {
		httpFactory = "createHTTPServices"
		httpTemplate = "http-services"
	}
	add("internal/runtime/"+httpTemplate+".ts", httpFactory)
	httpArguments := ""
	if httpFactory == "createBasicHTTPServices" && has("http.query") {
		add("internal/runtime/http-query.ts", "createQueryEncoder")
		httpArguments = ", /* @__PURE__ */ createQueryEncoder(wire)"
	}
	security := prepareSecurityHandlers(features)
	suffix := ""
	if len(security.fields) > 0 {
		add("internal/runtime/http-security.ts", "createOperationSecurity")
		for _, dependency := range security.imports {
			add(dependency.path, dependency.names...)
		}
		suffix = ", applyOperationSecurity: /* @__PURE__ */ createOperationSecurity({ " + strings.Join(security.fields, ", ") + " })"
	}
	extensionValue := "{ " + strings.Join(extensions, ", ") + " }"
	if len(extensionSources) > 0 {
		// A discarded pure call still evaluates its arguments. Object spread
		// would retain the source factory for possible getter effects, whereas
		// these compiler-owned factories return ordinary data properties.
		extensionValue = "/* @__PURE__ */ Object.assign({}, " + strings.Join(extensionSources, ", ") + ", " + extensionValue + ")"
	}
	result.declarations = append(result.declarations, "const baseServices: RequestExecutionServices = /* @__PURE__ */ Object.assign(/* @__PURE__ */ "+httpFactory+"(wire, "+extensionValue+httpArguments+"), {"+strings.TrimPrefix(suffix, ", ")+"})")
	services := "baseServices"
	if streaming {
		result.servicesType = "StreamingRequestExecutionServices"
		result.httpTypes = append(result.httpTypes, "StreamingRequestExecutionServices")
		add("internal/runtime/http-stream-core.ts", "createOperationStreamService")
		services = "/* @__PURE__ */ Object.assign({}, baseServices, { createOperationStream: /* @__PURE__ */ createOperationStreamService((): RequestExecutionServices => baseServices, decodeStreams, wire) })"
	}
	result.declarations = append(result.declarations, "const services: "+result.servicesType+" = "+services)
	result.imports = freezeRuntimeHandlerImports(imports)
	return result
}

func emitRuntimeComposition(output *bytes.Buffer, composition runtimeComposition, importFrom func(string, string) error) error {
	if len(composition.wireTypes) > 0 {
		if err := importFrom("type { "+strings.Join(uniqueStrings(composition.wireTypes), ", ")+" }", "internal/runtime/wire-types.ts"); err != nil {
			return err
		}
	}
	if err := importFrom("type { "+strings.Join(uniqueStrings(composition.httpTypes), ", ")+" }", "internal/runtime/http-types.ts"); err != nil {
		return err
	}
	for _, dependency := range composition.imports {
		if err := importFrom("{ "+strings.Join(dependency.names, ", ")+" }", dependency.path); err != nil {
			return err
		}
	}
	for _, declaration := range composition.declarations {
		fmt.Fprintln(output, "\n"+declaration)
	}
	return nil
}
