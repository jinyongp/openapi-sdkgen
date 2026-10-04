import { DelimitedTextFrames } from "../internal/framing-text.js";
import type { ServerCodecContext, InboundProtocolDecodeOptions } from "./runtime-types.js";
import { InboundRequestError } from "./runtime-errors.js";
import { awaitInboundAbortable } from "./runtime-shared.js";

export async function* decodeInboundJSONFrames(
  _codecContext: ServerCodecContext,
  body: ReadableStream<Uint8Array>,
  options: InboundProtocolDecodeOptions,
): AsyncIterable<unknown> {
  const { streamFraming, maxFrameBytes, signal }: InboundProtocolDecodeOptions = options;
  if (streamFraming !== "line-delimited-json" && streamFraming !== "json-sequence")
    throw new InboundRequestError(new Response("Unsupported Media Type", { status: 415 }));

  const reader: ReadableStreamDefaultReader<Uint8Array<ArrayBufferLike>> = body.getReader();
  const framing: DelimitedTextFrames = new DelimitedTextFrames(
    streamFraming === "json-sequence" ? "\u001e" : "\n",
    maxFrameBytes,
    (): never => {
      throw new InboundRequestError(
        new Response("Stream frame exceeds maxStreamFrameBytes", { status: 400 }),
      );
    },
  );
  const parse: (source: string) => unknown = (source: string): unknown => {
    try {
      return JSON.parse(source);
    } catch {
      throw new InboundRequestError(new Response("Invalid stream item", { status: 400 }));
    }
  };
  try {
    while (true) {
      const next: ReadableStreamReadResult<Uint8Array<ArrayBufferLike>> =
        await awaitInboundAbortable(reader.read(), signal);
      for (const frame of framing.push(next.value, next.done)) {
        const source: string = next.done
          ? frame.trim().replace(/^\u001e/, "")
          : streamFraming === "json-sequence"
            ? frame.trim()
            : frame.replace(/\r$/, "");
        if (source.trim() !== "") yield parse(source);
      }
      if (next.done) break;
    }
  } finally {
    try {
      await reader.cancel(signal.reason);
    } finally {
      reader.releaseLock();
    }
  }
}
