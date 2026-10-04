import type { WireSchema, WireSchemas } from "./wire-types.js";
/** Canonical XML encoding and decoding hooks using the caller's wire schemas. */
export interface XMLCodec {
  encodeXML(value: unknown, schema: WireSchema, schemas: WireSchemas): string;
  decodeXML(value: string, schema: WireSchema, schemas: WireSchemas): unknown;
}
