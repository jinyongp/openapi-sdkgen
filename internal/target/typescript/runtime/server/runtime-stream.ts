import type { StreamFraming } from "../internal/wire-types.js";
import type {
  InboundFrameDecoder,
  InboundProtocolDecodeOptions,
  ServerCodecContext,
} from "./runtime-types.js";

import { InboundRequestError } from "./runtime-errors.js";
export async function* decodeInboundBuiltInStreamFrames(
  codecContext: ServerCodecContext,
  body: ReadableStream<Uint8Array>,
  options: InboundProtocolDecodeOptions,
): AsyncIterable<unknown> {
  const kind: StreamFraming | undefined = options.streamFraming;
  const decoder: InboundFrameDecoder | undefined =
    kind === undefined || kind === "custom" ? undefined : codecContext.frames?.[kind];
  if (decoder === undefined)
    throw new InboundRequestError(new Response("Unsupported Media Type", { status: 415 }));
  yield* decoder(codecContext, body, options);
}
