import type { WireCodec } from "../../schema/wire-types.js";
import type { QueryEncoder } from "../http-types.js";
import { createParameterEncoder } from "./http-parameter-sort.js";
import { composeQueryParameterEncoder, createPreparedQueryEncoder } from "./http-query-core.js";
import type { ContentParameterEncoder, QueryParameterEncoder } from "./http-query-core.js";
export {
  assertQueryEmptyValueAllowed,
  appendQueryValue,
  serializeQuery,
} from "./http-query-core.js";
/** Creates the general query/header encoder with optional content encoding. */
export function createQueryParameterEncoder(
  wire: WireCodec,
  encodeContent?: ContentParameterEncoder,
): QueryParameterEncoder {
  return composeQueryParameterEncoder(createParameterEncoder(wire), encodeContent);
}
/** Creates the general query encoder with schema and structured-sort support. */
export function createQueryEncoder(wire: WireCodec): QueryEncoder {
  return createPreparedQueryEncoder(createParameterEncoder(wire));
}
