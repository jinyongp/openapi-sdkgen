export { createOperationStreamService } from "../http/http-stream-core.js";
import { createOperationStreamService } from "../http/http-stream-core.js";
import { createHTTPServices } from "./http-core.js";
import { decodeResponseStreamItems } from "./streaming.js";
import { jsonWireCodec } from "./wire-engine.js";
import type { HTTPStreamDecodeOptions } from "../media/media-service-types.js";
import type {
  RequestExecutionServices,
  StreamingRequestExecutionServices,
} from "../http/http-types.js";
function decodeJSONResponseItems(
  body: ReadableStream<Uint8Array>,
  options: HTTPStreamDecodeOptions,
): AsyncIterable<unknown> {
  return decodeResponseStreamItems(body, {
    contentType: options.contentType,
    streamFraming: options.streamFraming,
    maxFrameBytes: options.maxFrameBytes,
    streamCodec: options.streamCodec,
    signal: options.signal,
  });
}

function createJSONResponseStreamServices(): StreamingRequestExecutionServices {
  const base: RequestExecutionServices = createHTTPServices(jsonWireCodec, {
    encodeRequestBody(_contentType: string, value: unknown): BodyInit {
      return JSON.stringify(value);
    },
    decodeResponseStreamItems: decodeJSONResponseItems,
  });
  return {
    ...base,
    createOperationStream: createOperationStreamService(
      (): RequestExecutionServices => base,
      decodeJSONResponseItems,
      jsonWireCodec,
    ),
  };
}

/** JSON-bodied, non-XML plans exposing non-multipart response streams and buffered sequential responses. */
export const jsonResponseStreamServices: StreamingRequestExecutionServices =
  /* @__PURE__ */ createJSONResponseStreamServices();
