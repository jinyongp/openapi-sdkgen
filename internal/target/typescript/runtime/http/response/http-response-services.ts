import type { WireCodec, WireTransformOptions } from "../../schema/wire-types.js";
import type { WireResponseDefinition } from "./http-response-types.js";
import type { MediaCodec } from "../../media/media-codec-types.js";
import type { StreamCodec } from "../../stream/stream-protocol-types.js";
import type { OperationDefinition } from "../operation.js";
import type { RequestMetadata } from "../../shared/runtime-support.js";
import type { HTTPCodecExtensions } from "../../media/media-service-types.js";
import type { RequestExecutionServices, ResponseDecodeOptions } from "../http-types.js";
import {
  APIError,
  TransportErrorCode,
  isJSONMediaType,
  isXMLMediaType,
} from "../../shared/runtime-support.js";
import { registerHTTPError } from "./http-errors.js";
import {
  selectResponseDefinition,
  responseContentType,
  requireHTTPHook,
  serverError,
  resolveStreamCodec,
  resolveMaxStreamFrameBytes,
} from "../http-execution-support.js";
import { normalizeMediaType } from "../../shared/runtime-support.js";

type ResponseServices = Pick<
  RequestExecutionServices,
  "decodeResponse" | "decodeResponseHeaders" | "decodeResponseWireValue" | "createHTTPError"
>;
async function emptyResponseHeaders(): Promise<Readonly<Record<string, unknown>>> {
  return Object.create(null) as Record<string, unknown>;
}
/** Buffered response decoding and error provenance shared by narrow and general requests. */
export function createResponseServices(
  wire: WireCodec,
  extensions: HTTPCodecExtensions,
  decodeResponseHeaders: RequestExecutionServices["decodeResponseHeaders"] = emptyResponseHeaders,
): ResponseServices {
  const { transformWireValue }: WireCodec = wire;
  const tolerantResponseTransformOptions: WireTransformOptions = { unknownProperties: "preserve" };

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
    return definition === undefined
      ? value
      : transformWireValue(
          value,
          definition.schema,
          operation.outputSchemas ?? {},
          "decode",
          tolerantResponseTransformOptions,
        );
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
      if (isJSONMediaType(contentType)) {
        return await response.json();
      }
      if (contentType.startsWith("text/") || contentType.includes("xml")) {
        return await response.text();
      }
      if (isBinaryMediaType(contentType)) return response.body;
      const codec: MediaCodec<unknown> | undefined = options.codecs.get(contentType);
      if (codec?.decode === undefined)
        throw new TypeError(`missing decode codec for ${contentType}`);
      return await codec.decode(response, { contentType });
    } catch (cause: unknown) {
      throw new APIError({
        code: TransportErrorCode.RESPONSE_DECODE_FAILED,
        message: "Failed to decode response body",
        request,
        status: response.status,
        response,
        cause,
      });
    }
  }

  function isBinaryMediaType(contentType: string): boolean {
    return (
      contentType === "application/octet-stream" ||
      contentType.startsWith("image/") ||
      contentType.startsWith("audio/") ||
      contentType.startsWith("video/")
    );
  }

  function createHTTPError(
    operation: OperationDefinition,
    response: Response,
    request: RequestMetadata,
    data: unknown,
  ): APIError {
    const definition: WireResponseDefinition | undefined = selectResponseDefinition(
      operation,
      response,
      true,
    );
    const error: APIError = serverError(
      response,
      request,
      data,
      definition?.contentType === "" ? undefined : definition?.contentType,
    );
    if (definition === undefined) return error;
    return registerHTTPError(error, operation.route, (current: unknown): void => {
      if (response.ok || selectResponseDefinition(operation, response, true) !== definition)
        throw new TypeError("HTTP response no longer matches its declared representation");
      if (definition.contentType === "") {
        if (current !== undefined) throw new TypeError("bodyless HTTP response has a body");
      } else {
        // Validate mapped public names through the canonical output encoder,
        // including decoded content schemas and recursive/composed mappings.
        transformWireValue(current, definition.schema, operation.outputSchemas ?? {}, "encode");
      }
    });
  }

  return { decodeResponse, decodeResponseHeaders, decodeResponseWireValue, createHTTPError };
}
