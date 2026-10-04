import { createXMLCodec } from "./xml-codec.js";
import { fullWireHandlers } from "./wire-handlers.js";
import type { MediaCodec, WireCodec } from "./wire-types.js";
import { createWireCodec } from "./wire-engine.js";
import { decodeSchemaContent } from "./wire-engine.js";
import { isJSONMediaType, isXMLMediaType } from "./runtime-support.js";
import type { DynamicScope, WireSchema, WireSchemas, WireTransformOptions } from "./wire-types.js";

/** Retains the existing codec facade exports from ./wire-engine.js. */
export type * from "./wire-types.js";
/** Retains the existing codec facade exports from ./media-type.js. */
export { isJSONMediaType, isXMLMediaType } from "./runtime-support.js";

function decodeExtendedSchemaContent(
  value: string,
  mediaType: string,
  schema: WireSchema,
  components: WireSchemas,
): unknown {
  if (isXMLMediaType(mediaType)) return decodeXML(value, schema.contentSchema ?? {}, components);
  throw new TypeError(`unsupported contentMediaType ${mediaType}`);
}

/** Wire codec with XML content support, shared by generated execution plans. */
export const xmlWireCodec: WireCodec = /* @__PURE__ */ createWireCodec(
  (
    value: string,
    schema: WireSchema,
    components: WireSchemas,
    ignore: boolean | undefined,
  ): unknown => decodeSchemaContent(value, schema, components, ignore, decodeExtendedSchemaContent),
);

const xmlCodec: ReturnType<typeof createXMLCodec> = /* @__PURE__ */ createXMLCodec(
  xmlWireCodec,
  fullWireHandlers.dynamic,
);

/** Encodes XML through the shared full-capability wire codec. */
export const encodeXML: typeof xmlCodec.encodeXML = xmlCodec.encodeXML;
/** Decodes XML through the shared full-capability wire codec. */
export const decodeXML: typeof xmlCodec.decodeXML = xmlCodec.decodeXML;

/** Recursively maps a value between generated property names and wire names. */
export function transformWireValue(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions | undefined = undefined,
  dynamicScope: DynamicScope = [],
): unknown {
  return xmlWireCodec.transformWireValue(
    value,
    schema,
    components,
    direction,
    options,
    dynamicScope,
  );
}

/** Converts a validated JSON wire value into generated TypeScript property names. */
export function decodeWireValue(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
): unknown {
  return xmlWireCodec.decodeWireValue(value, schema, components);
}

/** Converts generated TypeScript property names into validated JSON wire names. */
export function encodeWireValue(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
): unknown {
  return xmlWireCodec.encodeWireValue(value, schema, components);
}

/** Validates a transformed wire value against its generated schema. */
export function validateWireValue(
  value: unknown,
  schema: WireSchema,
  components: WireSchemas,
  direction: "encode" | "decode",
  options: WireTransformOptions | undefined = undefined,
  dynamicScope: DynamicScope = [],
): void {
  xmlWireCodec.validateWireValue(value, schema, components, direction, options, dynamicScope);
}

/** Buffered media handlers shared by XML plans without importing request orchestration. */
interface BufferedXMLCodecExtensions {
  encodeRequestBody(
    contentType: string,
    value: unknown,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    schema: WireSchema | undefined,
    schemas: WireSchemas,
  ): BodyInit | Promise<BodyInit>;
  encodeXML: typeof encodeXML;
  decodeXML: typeof decodeXML;
}

/** Buffered media handlers shared by XML plans without importing request orchestration. */
export const bufferedXMLCodecExtensions: BufferedXMLCodecExtensions = {
  encodeRequestBody(
    contentType: string,
    value: unknown,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    schema: WireSchema | undefined,
    schemas: WireSchemas,
  ): BodyInit | Promise<BodyInit> {
    const normalized: string = contentType.toLowerCase();
    if (isJSONMediaType(normalized)) return JSON.stringify(value);
    if (isXMLMediaType(normalized)) return encodeXML(value, schema ?? {}, schemas);
    if (normalized.startsWith("text/")) return String(value);
    if (value instanceof Blob || value instanceof ArrayBuffer || ArrayBuffer.isView(value)) {
      return value as BodyInit;
    }
    const codec: MediaCodec<unknown> | undefined = codecs.get(
      contentType.split(";", 1)[0]?.trim().toLowerCase() ?? "",
    );
    if (codec?.encode === undefined) throw new TypeError("missing encode codec for " + contentType);
    return codec.encode(value, { contentType });
  },
  encodeXML,
  decodeXML,
};
