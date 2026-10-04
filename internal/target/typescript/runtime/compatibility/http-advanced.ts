import type { WireCodec, WireSchema, WireSchemas } from "../schema/wire-types.js";
import type { MediaCodec } from "../media/media-codec-types.js";
import type { AdvancedHTTPServices, RequestExecutionServices } from "../http/http-types.js";
import type { HTTPStreamDecodeOptions } from "../media/media-service-types.js";
import { createOperationStreamService } from "../http/http-stream-core.js";
import { createXMLCodec } from "../media/xml/xml-codec.js";
import { fullWireHandlers } from "./wire-handlers.js";
import { createHeaderContentDecoder } from "../http/response/http-header-content.js";
import { createMultipartRequestServices } from "../media/http-multipart-request.js";
import { createMultipartResponseServices } from "../media/http-multipart-response.js";
import { createRequestStreamServices } from "../http/request/http-request-stream.js";
import { createTextRequestEncoder } from "../http/request/http-request-text-stream.js";
import { encodeJSONStreamItem } from "../http/request/http-request-json-frame.js";
import { encodeSSERequestItem } from "../http/request/http-request-sse-frame.js";
import { createBodyEncoder } from "../media/http-body.js";
import { encodeJSONBody } from "../media/http-body-json.js";
import { encodeFormBody } from "../media/http-body-form.js";
import { encodeTextBody } from "../media/http-body-text.js";
import { encodeBinaryBody } from "../media/http-body-binary.js";
import { encodeCustomBody } from "../media/http-body-custom.js";
import { decodeResponseStreamItems as decodeFrames } from "./streaming.js";
/** Full-capability compatibility composition; selected providers use the same individual factories. */
export function createAdvancedHTTPServices(
  getBase: () => RequestExecutionServices,
  wire: WireCodec,
): AdvancedHTTPServices {
  const xml: ReturnType<typeof createXMLCodec> = createXMLCodec(wire, fullWireHandlers.dynamic);
  const header: ReturnType<typeof createHeaderContentDecoder> = createHeaderContentDecoder(
    wire,
    xml,
  );
  const multipartRequest: ReturnType<typeof createMultipartRequestServices> =
    createMultipartRequestServices(wire, xml, header);
  const multipartResponse: ReturnType<typeof createMultipartResponseServices> =
    createMultipartResponseServices(wire, xml, header);
  const streams: ReturnType<typeof createRequestStreamServices> = createRequestStreamServices(
    wire,
    {
      "line-delimited-json": createTextRequestEncoder(
        (value: unknown): string => `${encodeJSONStreamItem(value)}\n`,
      ),
      "json-sequence": createTextRequestEncoder(
        (value: unknown): string => `\u001e${encodeJSONStreamItem(value)}\n`,
      ),
      sse: createTextRequestEncoder(encodeSSERequestItem),
    },
    multipartRequest.encodeStream,
  );
  const encodeRequestBody: AdvancedHTTPServices["encodeRequestBody"] = createBodyEncoder({
    json: encodeJSONBody,
    form: encodeFormBody,
    multipart: multipartRequest.encodeBody,
    text: encodeTextBody,
    binary: encodeBinaryBody,
    custom: encodeCustomBody,
    xml: (
      _contentType: string,
      value: unknown,
      _codecs: ReadonlyMap<string, MediaCodec<unknown>>,
      schema: WireSchema | undefined,
      schemas: WireSchemas,
    ): BodyInit => xml.encodeXML(value, schema ?? {}, schemas),
  });
  const decodeResponseStreamItems: AdvancedHTTPServices["decodeResponseStreamItems"] = (
    body: ReadableStream<Uint8Array>,
    options: HTTPStreamDecodeOptions,
  ): AsyncIterable<unknown> =>
    decodeFrames(body, {
      contentType: options.contentType,
      streamFraming: options.streamFraming,
      maxFrameBytes: options.maxFrameBytes,
      streamCodec: options.streamCodec,
      signal: options.signal,
      ...(options.streamFraming === "multipart"
        ? {
            multipartFrames: (): AsyncIterable<unknown> =>
              multipartResponse.decodeStreamItems(body, options),
          }
        : {}),
    });
  return {
    ...xml,
    ...streams,
    encodeRequestBody,
    decodeResponseStreamItems,
    decodeMultipartResponse: multipartResponse.decodeMultipartResponse,
    createOperationStream: createOperationStreamService(getBase, decodeResponseStreamItems, wire),
  };
}
