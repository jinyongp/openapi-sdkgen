import type { WireCodec } from "../../schema/wire-types.js";
import type { WireResponseDefinition } from "./http-response-types.js";
import type { MediaCodec } from "../../media/media-codec-types.js";
import type { StreamCodec } from "../../stream/stream-protocol-types.js";
import type { OperationDefinition } from "../operation.js";
import type { RequestMetadata } from "../../shared/runtime-support.js";
import type { HTTPCodecExtensions } from "../../media/media-service-types.js";
import type { RequestExecutionServices, ResponseDecodeOptions } from "../http-types.js";
import { isXMLMediaType } from "../../shared/runtime-support.js";
import { createHTTPErrorFactory } from "./http-response-error.js";
import { decodeBufferedResponseValue, responseDecodeFailure } from "./http-response-body.js";
import {
  emptyResponseHeaders,
  tolerantResponseTransformOptions,
  transformPreparedResponse,
} from "./http-response-wire.js";
import {
  selectResponseDefinition,
  responseContentType,
  requireHTTPHook,
  resolveStreamCodec,
  resolveMaxStreamFrameBytes,
} from "../http-execution-support.js";
import { normalizeMediaType } from "../../shared/runtime-support.js";

import type { BufferedResponseServices } from "./http-response-service-types.js";

/** Buffered response decoding and error provenance shared by narrow and general requests. */
export function createResponseServices(
  wire: WireCodec,
  extensions: HTTPCodecExtensions,
  decodeResponseHeaders: RequestExecutionServices["decodeResponseHeaders"] = emptyResponseHeaders,
): BufferedResponseServices {
  const { transformWireValue }: WireCodec = wire;

  function decodeResponseWireValue(
    operation: OperationDefinition,
    response: Response,
    value: unknown,
  ): unknown {
    const definition: WireResponseDefinition | undefined = selectResponseDefinition(
      operation,
      response,
      true,
    );
    if (
      definition !== undefined &&
      isXMLMediaType(definition.contentType) &&
      typeof value === "string"
    ) {
      value = requireHTTPHook(extensions.decodeXML)(
        value,
        definition.schema,
        operation.outputSchemas ?? {},
      );
    }
    const contentType: string | undefined = responseContentType(response);
    if (
      definition !== undefined &&
      definition.streamFraming === undefined &&
      typeof value === "string" &&
      contentType?.startsWith("text/") &&
      !isXMLMediaType(contentType)
    ) {
      try {
        // Keep valid string representations, including string/number unions.
        return transformWireValue(
          value,
          definition.schema,
          operation.outputSchemas ?? {},
          "decode",
          tolerantResponseTransformOptions,
        );
      } catch (cause: unknown) {
        let scalar: unknown;
        try {
          scalar = JSON.parse(value);
        } catch {
          throw cause;
        }
        // Text scalar bodies must contain one complete value. Objects and
        // arrays retain their media-specific decoding and validation paths.
        if (scalar !== null && typeof scalar !== "number" && typeof scalar !== "boolean")
          throw cause;
        value = scalar;
      }
    }
    return transformPreparedResponse(wire, definition, operation, value);
  }

  async function decodeResponse(
    operation: OperationDefinition,
    response: Response,
    request: RequestMetadata,
    options: ResponseDecodeOptions,
  ): Promise<unknown> {
    if (response.status === 204 || response.status === 205) return undefined;
    const contentType: string | undefined = responseContentType(response);
    if (contentType === undefined || response.body === null) return undefined;
    try {
      const definition: WireResponseDefinition | undefined = selectResponseDefinition(
        operation,
        response,
        true,
      );
      const completeSequential: boolean = definition?.streamFraming !== undefined;
      if (completeSequential && definition !== undefined) {
        const values: unknown[] = [];
        const streamCodec: StreamCodec<unknown, unknown> | undefined = resolveStreamCodec(
          contentType,
          options.streamCodec,
          options.streamCodecs,
        );
        for await (const value of requireHTTPHook(extensions.decodeResponseStreamItems)(
          response.body,
          {
            contentType: response.headers.get("content-type") ?? contentType,
            streamFraming: definition.streamFraming,
            itemSchema: definition.schema.items ?? {},
            prefixSchemas: definition.schema.prefixItems,
            schemas: operation.outputSchemas ?? {},
            codecs: options.codecs,
            prefixEncoding: definition.prefixEncoding,
            itemEncoding: definition.itemEncoding,
            maxFrameBytes: resolveMaxStreamFrameBytes(options.maxStreamFrameBytes),
            streamCodec,
            signal: options.signal,
          },
        )) {
          values.push(value);
        }
        return values;
      }
      if (normalizeMediaType(contentType).startsWith("multipart/") && definition !== undefined) {
        return requireHTTPHook(extensions.decodeMultipartResponse)(
          response.body,
          response.headers.get("content-type") ?? contentType,
          definition,
          operation.outputSchemas ?? {},
          options.codecs,
        );
      }
      if (definition?.binary === true) {
        const codec: MediaCodec<unknown> | undefined = options.codecs.get(contentType);
        return codec?.decode === undefined
          ? response.body
          : await codec.decode(response, { contentType });
      }
      return await decodeBufferedResponseValue(response, options.codecs);
    } catch (cause: unknown) {
      throw responseDecodeFailure(response, request, cause);
    }
  }

  return {
    decodeResponse,
    decodeResponseHeaders,
    decodeResponseWireValue,
    createHTTPError: createHTTPErrorFactory(wire),
  };
}
