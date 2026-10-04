import type { ServerCodecContext, InboundProtocolDecodeOptions } from "./runtime-types.js";
import { InboundRequestError } from "./runtime-errors.js";
import { decodeSSEStreamItems } from "../stream/stream-sse.js";
export async function* decodeInboundSSEFrames(
  _context: ServerCodecContext,
  body: ReadableStream<Uint8Array>,
  options: InboundProtocolDecodeOptions,
): AsyncIterable<unknown> {
  try {
    yield* decodeSSEStreamItems(body, options.maxFrameBytes, options.signal);
  } catch (cause: unknown) {
    if (options.signal.aborted) throw cause;
    throw new InboundRequestError(
      new Response(
        cause instanceof Error && cause.message.includes("frame exceeds")
          ? "Stream frame exceeds maxStreamFrameBytes"
          : "Invalid SSE stream frame",
        { status: 400 },
      ),
    );
  }
}
