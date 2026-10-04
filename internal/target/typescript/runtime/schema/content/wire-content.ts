import type { WireSchema, WireSchemas } from "../wire-types.js";
import { isJSONMediaType } from "../../shared/runtime-support.js";
/** Decodes common schema content, delegating extended media only when explicitly connected. */
export function decodeSchemaContent(
  value: string,
  schema: WireSchema,
  components: WireSchemas,
  ignoreContentMediaType: boolean = false,
  decodeExtended?: (
    value: string,
    mediaType: string,
    schema: WireSchema,
    components: WireSchemas,
  ) => unknown,
): unknown {
  let decoded: string = value;
  const encoding: string | undefined = schema.contentEncoding?.toLowerCase();
  if (encoding === "base64" || encoding === "base64url") {
    try {
      const normalized: string =
        encoding === "base64url" ? value.replaceAll("-", "+").replaceAll("_", "/") : value;
      decoded = new TextDecoder().decode(
        Uint8Array.from(atob(normalized), (character: string): number => character.charCodeAt(0)),
      );
    } catch (cause: unknown) {
      throw new TypeError(`contentEncoding ${schema.contentEncoding} cannot decode the value`, {
        cause,
      });
    }
  } else if (
    encoding !== undefined &&
    encoding !== "7bit" &&
    encoding !== "8bit" &&
    encoding !== "binary"
  ) {
    throw new TypeError(`unsupported contentEncoding ${schema.contentEncoding}`);
  }
  const mediaType: string | undefined = ignoreContentMediaType
    ? undefined
    : schema.contentMediaType;
  if (mediaType === undefined || mediaType === "" || mediaType.toLowerCase().startsWith("text/"))
    return decoded;
  if (isJSONMediaType(mediaType)) {
    try {
      return JSON.parse(decoded);
    } catch (cause: unknown) {
      throw new TypeError(`contentMediaType ${mediaType} cannot decode JSON`, { cause });
    }
  }
  if (decodeExtended !== undefined) return decodeExtended(decoded, mediaType, schema, components);
  throw new TypeError(`unsupported contentMediaType ${mediaType}`);
}
