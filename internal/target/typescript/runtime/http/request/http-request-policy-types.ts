import type { OperationDefinition } from "../operation.js";
import type { RequestMetadata } from "../../shared/runtime-support.js";
import type { Transport } from "../../shared/transport.js";
import type { MediaCodec } from "../../media/media-codec-types.js";
import type { RawResponse } from "../request.js";
/** Reads sequential raw response metadata without consuming the framed body. */
export type StreamingRawResponseReader = (
  operation: OperationDefinition,
  response: Response,
  request: RequestMetadata,
  codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  signal: AbortSignal | undefined,
) => Promise<RawResponse<unknown> | undefined>;
/** Optional response-header and streaming-raw policies for the buffered executor. */
export interface RequestExecutionPolicy {
  readonly assertResponseHeaders?: (
    transport: Transport | undefined,
    operation: OperationDefinition,
  ) => void;
  readonly readStreamingRaw?: StreamingRawResponseReader;
}
