import type { MediaCodec, WireSchema, WireSchemas, WireBodyDefinition } from "./wire-types.js";
import type { BodyEncoders, HTTPBodyEncoder } from "./http-media-types.js";
import { isJSONMediaType, isXMLMediaType } from "./runtime-support.js";
import { normalizeMediaType } from "./http-execution-support.js";
/** Dispatches to the request body encoders present in this composition. */
export function createBodyEncoder(encoders: BodyEncoders): HTTPBodyEncoder {
  return (
    contentType: string,
    value: unknown,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    schema: WireSchema | undefined,
    schemas: WireSchemas,
    definition: WireBodyDefinition | undefined,
    multipartHeaders: Readonly<Record<string, HeadersInit>> | undefined,
    multipartContentTypes: Readonly<Record<string, string>> | undefined,
  ): BodyInit | Promise<BodyInit> => {
    const normalized: string = normalizeMediaType(contentType);
    const handler: HTTPBodyEncoder | undefined = isJSONMediaType(normalized)
      ? encoders.json
      : normalized === "application/x-www-form-urlencoded"
        ? encoders.form
        : normalized.startsWith("multipart/")
          ? encoders.multipart
          : isXMLMediaType(normalized)
            ? encoders.xml
            : normalized.startsWith("text/")
              ? encoders.text
              : value instanceof Blob || value instanceof ArrayBuffer || ArrayBuffer.isView(value)
                ? encoders.binary
                : encoders.custom;
    if (handler === undefined) throw new TypeError(`missing encode codec for ${contentType}`);
    return handler(
      contentType,
      value,
      codecs,
      schema,
      schemas,
      definition,
      multipartHeaders,
      multipartContentTypes,
    );
  };
}
