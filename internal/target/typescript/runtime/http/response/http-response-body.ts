import type { MediaCodec } from "../../media/media-codec-types.js";
import type { RequestMetadata } from "../../shared/runtime-support.js";
import { APIError, TransportErrorCode, isJSONMediaType } from "../../shared/runtime-support.js";
import { responseContentType } from "../http-execution-support.js";
/** Preserves fallback decoding for actual, including undeclared, response media. */
export async function decodeBufferedResponseValue(
  response: Response,
  codecs: ReadonlyMap<string, MediaCodec<unknown>>,
): Promise<unknown> {
  const contentType: string = responseContentType(response)!;
  if (isJSONMediaType(contentType)) {
    return await response.json();
  }
  if (contentType.startsWith("text/") || contentType.includes("xml")) {
    return await response.text();
  }
  if (isBinaryMediaType(contentType)) return response.body;
  const codec: MediaCodec<unknown> | undefined = codecs.get(contentType);
  if (codec?.decode === undefined) throw new TypeError(`missing decode codec for ${contentType}`);
  return await codec.decode(response, { contentType });
}
function isBinaryMediaType(contentType: string): boolean {
  return (
    contentType === "application/octet-stream" ||
    contentType.startsWith("image/") ||
    contentType.startsWith("audio/") ||
    contentType.startsWith("video/")
  );
}
/** Wraps body decoding failures with response and request metadata. */
export function responseDecodeFailure(
  response: Response,
  request: RequestMetadata,
  cause: unknown,
): APIError {
  return new APIError({
    code: TransportErrorCode.RESPONSE_DECODE_FAILED,
    message: "Failed to decode response body",
    request,
    status: response.status,
    response,
    cause,
  });
}
