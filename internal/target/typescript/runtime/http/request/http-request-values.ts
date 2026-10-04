import type { WireBodyDefinition } from "../../media/media-contract-types.js";
import { mediaTypeMatches, mediaTypeMatchScore } from "../http-execution-support.js";
/** Forwards canonical path safety checks for the general request helper. */
export { assertSafeOperationPath } from "./http-request-path-safety.js";
/** Forwards canonical raw-header and reserved-header handling. */
export { appendRawHeaders, setHeader } from "./http-request-headers.js";
/** Forwards canonical request input checks and parameter lookup. */
export { rejectUndefinedArrayValues, findParameterByProperty } from "./http-request-input.js";
/** Forwards the general schema path serializer. */
export { serializeSchemaPathParameter } from "./http-path.js";
/** Forwards schema and structured-sort parameter encoding. */
export { createParameterEncoder } from "./http-parameter-sort.js";
/** Forwards general server-variable and deployment URL resolution. */
export { resolveOperationBaseURL } from "./http-server-variables.js";
/** Selects the declared request-body contract matching its media type. */
export function selectRequestBodyDefinition(
  bodies: readonly WireBodyDefinition[],
  contentType: string,
): WireBodyDefinition | undefined {
  return bodies
    .filter((body: WireBodyDefinition): boolean => mediaTypeMatches(body.contentType, contentType))
    .sort(
      (left: WireBodyDefinition, right: WireBodyDefinition): number =>
        mediaTypeMatchScore(right.contentType, contentType) -
        mediaTypeMatchScore(left.contentType, contentType),
    )[0];
}
