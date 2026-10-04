import type { StreamDecodeOptions } from "./stream-types.js";
import { DelimitedTextFrames } from "./framing/framing-text.js";
import { awaitAbortable } from "./stream-abort.js";
export async function* decodeJSONStreamItems(
  body: ReadableStream<Uint8Array>,
  options: StreamDecodeOptions,
): AsyncIterable<unknown> {
  const { contentType, streamFraming, maxFrameBytes, signal }: StreamDecodeOptions = options;
  if (streamFraming !== "line-delimited-json" && streamFraming !== "json-sequence")
    throw new TypeError(`missing stream protocol for ${contentType}`);
  const framing: DelimitedTextFrames = new DelimitedTextFrames(
    streamFraming === "json-sequence" ? "\u001e" : "\n",
    maxFrameBytes,
    (): never => {
      throw new TypeError(`stream frame exceeds ${maxFrameBytes} bytes`);
    },
  );
  const reader: ReadableStreamDefaultReader<Uint8Array<ArrayBufferLike>> = body.getReader();
  try {
    while (true) {
      const { done, value }: ReadableStreamReadResult<Uint8Array<ArrayBufferLike>> =
        await awaitAbortable(reader.read(), signal);
      for (const frame of framing.push(value, done)) {
        const source: string = done
          ? frame.trim().replace(/^\u001e/, "")
          : streamFraming === "json-sequence"
            ? frame.trim()
            : frame.replace(/\r$/, "");
        if (source.trim() !== "") yield parseStreamJSON(source);
      }
      if (done) break;
    }
  } finally {
    try {
      await reader.cancel();
    } finally {
      reader.releaseLock();
    }
  }
}

export function parseStreamJSON(value: string): unknown {
  try {
    return JSON.parse(value);
  } catch {
    throw new TypeError("stream item is not valid JSON");
  }
}
