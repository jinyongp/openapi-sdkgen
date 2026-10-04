import type { WireCodec, WireSchema, WireSchemas } from "../../schema/wire-types.js";
import type { MediaCodec } from "../../media/media-codec-types.js";
import {
  isJSONMediaType,
  isXMLMediaType,
  defineOwnDataProperty,
} from "../../shared/runtime-support.js";
import { normalizeMediaType } from "../../shared/runtime-support.js";
import type { HTTPHeaderDecoder } from "../../media/http-media-types.js";
import type { XMLCodec } from "../../media/xml/xml-types.js";
import { decodeSimpleWireHeader } from "../../schema/schema-query.js";
/** Shares content and simple-header decoding across HTTP and multipart parts. */
export function createHeaderContentDecoder(
  wire: WireCodec,
  xml: Pick<XMLCodec, "decodeXML"> | undefined,
): HTTPHeaderDecoder {
  const { validateWireValue }: WireCodec = wire;
  function decodeXML(value: string, schema: WireSchema, schemas: WireSchemas): unknown {
    if (xml === undefined) throw new TypeError("required HTTP execution hook is missing");
    return xml.decodeXML(value, schema, schemas);
  }
  async function decodeResponseHeaderValue(
    name: string,
    value: string,
    schema: WireSchema,
    contentType: string | undefined,
    explode: boolean | undefined,
    schemas: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    role: "response" | "multipart" = "response",
  ): Promise<unknown> {
    if (contentType !== undefined) {
      const decoded: unknown = decodeHeaderContent(name, value, contentType);
      if (
        isJSONMediaType(contentType) ||
        normalizeMediaType(contentType) === "application/x-www-form-urlencoded"
      )
        return decoded;
      if (isXMLMediaType(contentType)) return decodeXML(value, schema, schemas);
      if (!normalizeMediaType(contentType).startsWith("text/")) {
        const codec: MediaCodec<unknown> | undefined = codecs.get(normalizeMediaType(contentType));
        if (codec?.decodeParameter === undefined)
          throw new TypeError(`missing decodeParameter codec for ${role} header ${name}`);
        return codec.decodeParameter(value, { contentType });
      }
      value = decoded as string;
    }
    return decodeSimpleWireHeader(
      value,
      schema,
      schemas,
      explode ?? false,
      (candidate: unknown, contract: WireSchema): void =>
        validateWireValue(candidate, contract, schemas, "decode"),
      "converted",
      `${role} header ${name}`,
    );
  }

  function decodeHeaderContent(name: string, value: string, contentType: string): unknown {
    if (isJSONMediaType(contentType)) {
      try {
        return JSON.parse(value);
      } catch (cause: unknown) {
        throw new TypeError(`response header ${name} is not valid ${contentType}`, { cause });
      }
    }
    if (normalizeMediaType(contentType) === "application/x-www-form-urlencoded") {
      const result: Record<string, string | string[]> = Object.create(null) as Record<
        string,
        string | string[]
      >;
      for (const [key, item] of new URLSearchParams(value)) {
        const previous: string | string[] | undefined = result[key];
        defineOwnDataProperty(
          result,
          key,
          previous === undefined
            ? item
            : Array.isArray(previous)
              ? [...previous, item]
              : [previous, item],
        );
      }
      return result;
    }
    return value;
  }

  return decodeResponseHeaderValue;
}
