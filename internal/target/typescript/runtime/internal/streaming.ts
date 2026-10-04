export type { StreamDecodeOptions } from "./stream-types.js";
export { parseStreamJSON } from "./stream-json.js";
import { createStreamDecoder } from "./stream-core.js";
import { decodeJSONStreamItems } from "./stream-json.js";
import { decodeSSEStreamItems } from "./stream-sse.js";
import type { StreamDecodeOptions, StreamFrameDecoder } from "./stream-types.js";
export const decodeResponseStreamItems: StreamFrameDecoder = /* @__PURE__ */ createStreamDecoder({
  "line-delimited-json": decodeJSONStreamItems,
  "json-sequence": decodeJSONStreamItems,
  sse: (body: ReadableStream<Uint8Array>, options: StreamDecodeOptions): AsyncIterable<unknown> =>
    decodeSSEStreamItems(body, options.maxFrameBytes, options.signal),
});
