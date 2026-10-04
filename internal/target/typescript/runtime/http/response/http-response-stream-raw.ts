import type { RequestExecutionServices } from "../http-types.js";
import type { StreamingRawResponseReader } from "../request/http-request-policy-types.js";
import type { OperationDefinition } from "../operation.js";
import type { RequestMetadata } from "../../shared/runtime-support.js";
import type { MediaCodec } from "../../media/media-codec-types.js";
import type { RawResponse } from "../request.js";
import { TransportErrorCode } from "../../shared/runtime-support.js";
import {
  selectResponseDefinition,
  responseContentType,
  transportErrorFromCause,
} from "../http-execution-support.js";
import { awaitAbortable } from "../../stream/stream-abort.js";
import type { ResponseFailureMetadata } from "../http-execution-support.js";
/** Raw sequential response metadata leaves the framed body unread. */
export function createStreamingRawResponseReader(
  decodeResponseHeaders: RequestExecutionServices["decodeResponseHeaders"],
): StreamingRawResponseReader {
  return async (
    operation: OperationDefinition,
    response: Response,
    request: RequestMetadata,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    signal: AbortSignal | undefined,
  ): Promise<RawResponse<unknown> | undefined> => {
    const responseDefinition: ReturnType<typeof selectResponseDefinition> =
      selectResponseDefinition(operation, response, true);
    if (
      !response.ok ||
      (responseDefinition?.itemSchema === undefined &&
        responseDefinition?.streamFraming === undefined)
    )
      return undefined;
    const responseMetadata: ResponseFailureMetadata = {
      request,
      status: response.status,
      response,
    };
    const contentType: string | undefined = responseContentType(response);
    let headerValues: Readonly<Record<string, unknown>>;
    try {
      headerValues = await awaitAbortable(
        decodeResponseHeaders(operation, response, codecs),
        signal,
      );
    } catch (cause: unknown) {
      await response.body?.cancel().catch((): undefined => undefined);
      throw transportErrorFromCause(
        TransportErrorCode.RESPONSE_DECODE_FAILED,
        "Failed to decode response headers",
        cause,
        responseMetadata,
      );
    }
    return {
      status: response.status,
      ...(contentType === undefined ? {} : { contentType }),
      data: undefined as unknown,
      headers: headerValues,
      request,
      response,
    };
  };
}
