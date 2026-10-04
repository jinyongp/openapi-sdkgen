import type { OperationDefinition } from "../operation.js";
import type { RequestMetadata } from "../../shared/runtime-support.js";
import type { RequestOptions, RawResponse } from "../request.js";
import type {
  RequestContext,
  RequestExecutionServices,
  EncodedRequest,
  AbortContext,
  StreamingRequestInit,
} from "../http-types.js";
import type { BufferedRequestFunction } from "./request-execution-types.js";
import type { ResponseFailureMetadata } from "../http-execution-support.js";
import { TransportErrorCode, isAPIError, isRecord } from "../../shared/runtime-support.js";
import { operationDiagnosticName } from "../operation.js";
import {
  createAbortContext,
  isPromise,
  applyNoSecurity,
  isReadableStream,
  requestMetadata,
  responseContentType,
  transportErrorFromCause,
  cancelResponseBody,
  cancelTrackedRequestBody,
} from "../http-execution-support.js";
import { awaitAbortable } from "../../stream/stream-abort.js";
import { transportError } from "../../shared/runtime-support.js";

import type { RequestExecutionPolicy } from "./http-request-policy-types.js";
/** Canonical buffered executor preserves failures, raw metadata, deadlines and cancellation. */
export function createBufferedRequestCore(
  context: RequestContext,
  services: RequestExecutionServices,
  policy: RequestExecutionPolicy = {},
): BufferedRequestFunction {
  const { options, baseURL, fetchImplementation, codecs, streamCodecs }: RequestContext = context;
  const {
    encodeRequest,
    decodeResponse,
    decodeResponseHeaders,
    decodeResponseWireValue,
    createHTTPError,
  }: RequestExecutionServices = services;
  const execute: <Output>(
    operation: OperationDefinition,
    input?: unknown,
    requestOptions?: RequestOptions,
    raw?: boolean,
  ) => Promise<Output | RawResponse<Output>> = async <Output>(
    operation: OperationDefinition,
    input?: unknown,
    requestOptions: RequestOptions = {},
    raw: boolean = false,
  ): Promise<Output | RawResponse<Output>> => {
    const credentials: RequestCredentials | undefined =
      requestOptions.credentials ?? options.credentials;
    const timeoutMS: number | undefined = requestOptions.timeoutMS ?? options.timeoutMS;
    const abort: AbortContext = createAbortContext(requestOptions.signal, timeoutMS);
    let responseMetadata: ResponseFailureMetadata | undefined;
    let requestBodyFailure: (() => unknown) | undefined;
    let requestBodyCancel: ((reason?: unknown) => Promise<void>) | undefined;
    try {
      let encoded: EncodedRequest;
      try {
        if (abort.signal?.aborted) throw abort.signal.reason;
        const effectiveRequestOptions: RequestOptions =
          abort.signal === undefined ? requestOptions : { ...requestOptions, signal: abort.signal };
        const pending: EncodedRequest | Promise<EncodedRequest> = encodeRequest(
          baseURL,
          options,
          codecs,
          streamCodecs,
          operation,
          input,
          effectiveRequestOptions,
        );
        encoded = isPromise(pending) ? await awaitAbortable(pending, abort.signal) : pending;
        requestBodyFailure = encoded.bodyFailure;
        requestBodyCancel = encoded.bodyCancel;
        if (abort.signal?.aborted) throw abort.signal.reason;
        const secured: EncodedRequest | Promise<EncodedRequest> = (
          services.applyOperationSecurity ?? applyNoSecurity
        )(options, operation, encoded, effectiveRequestOptions, credentials);
        encoded = isPromise(secured) ? await awaitAbortable(secured, abort.signal) : secured;
        requestBodyFailure = encoded.bodyFailure;
        requestBodyCancel = encoded.bodyCancel;
      } catch (cause: unknown) {
        if (abort.timedOut() || abort.aborted()) throw cause;
        if (isAPIError(cause)) throw cause;
        throw transportError(
          TransportErrorCode.REQUEST_ENCODE_FAILED,
          `Failed to encode ${operationDiagnosticName(operation)} request`,
          cause,
        );
      }
      const init: StreamingRequestInit = {
        method: operation.method,
        headers: encoded.headers,
        ...(encoded.redirect === undefined ? {} : { redirect: encoded.redirect }),
      };
      if (encoded.body !== undefined) {
        init.body = encoded.body as BodyInit;
        if (isReadableStream(encoded.body)) init.duplex = "half";
      }
      if (abort.signal !== undefined) init.signal = abort.signal;
      if (credentials !== undefined) init.credentials = credentials;
      if (abort.signal?.aborted) throw abort.signal.reason;
      policy.assertResponseHeaders?.(options.transport, operation);
      const response: Response = await awaitAbortable(
        fetchImplementation(encoded.url, init),
        abort.signal,
      );
      const request: RequestMetadata = requestMetadata(response);
      responseMetadata = { request, status: response.status, response };
      if (raw && policy.readStreamingRaw !== undefined) {
        const early: RawResponse<unknown> | undefined = await policy.readStreamingRaw(
          operation,
          response,
          request,
          codecs,
          abort.signal,
        );
        if (early !== undefined) return early as RawResponse<Output>;
      }
      let body: unknown;
      try {
        const decodedBody: unknown = await awaitAbortable(
          decodeResponse(operation, response, request, {
            codecs,
            streamCodecs,
            ...(requestOptions.streamCodec === undefined
              ? {}
              : { streamCodec: requestOptions.streamCodec }),
            maxStreamFrameBytes: requestOptions.maxStreamFrameBytes ?? options.maxStreamFrameBytes,
            ...(abort.signal === undefined ? {} : { signal: abort.signal }),
          }),
          abort.signal,
        );
        body = decodeResponseWireValue(operation, response, decodedBody);
      } catch (cause: unknown) {
        throw transportErrorFromCause(
          TransportErrorCode.RESPONSE_DECODE_FAILED,
          "Failed to decode response body",
          cause,
          responseMetadata,
        );
      }
      if (!response.ok) {
        throw createHTTPError(operation, response, request, body);
      }
      const data: Output =
        operation.envelope === "data" && isRecord(body) && Object.hasOwn(body, "data")
          ? (body["data"] as Output)
          : (body as Output);
      if (!raw) return data;
      const contentType: string | undefined = responseContentType(response);
      let headerValues: Readonly<Record<string, unknown>>;
      try {
        headerValues = await awaitAbortable(
          decodeResponseHeaders(operation, response, codecs),
          abort.signal,
        );
      } catch (cause: unknown) {
        throw transportErrorFromCause(
          TransportErrorCode.RESPONSE_DECODE_FAILED,
          "Failed to decode response headers",
          cause,
          responseMetadata,
        );
      }
      return {
        status: response.status,
        ...(contentType === undefined ? {} : { contentType }),
        data: body as Output,
        headers: headerValues,
        request,
        response,
      };
    } catch (cause: unknown) {
      await cancelTrackedRequestBody(requestBodyCancel, cause);
      if (abort.timedOut()) {
        await cancelResponseBody(responseMetadata?.response, cause);
        throw transportErrorFromCause(
          TransportErrorCode.REQUEST_TIMEOUT,
          `Request timed out after ${timeoutMS}ms`,
          cause,
          responseMetadata,
        );
      }
      if (abort.aborted()) {
        await cancelResponseBody(responseMetadata?.response, cause);
        throw transportErrorFromCause(
          TransportErrorCode.REQUEST_ABORTED,
          "Request was aborted",
          cause,
          responseMetadata,
        );
      }
      if (isAPIError(cause)) throw cause;
      const bodyFailure: unknown = requestBodyFailure?.();
      if (bodyFailure !== undefined) {
        throw transportErrorFromCause(
          TransportErrorCode.REQUEST_ENCODE_FAILED,
          `Failed to encode ${operationDiagnosticName(operation)} request body stream`,
          bodyFailure,
          responseMetadata,
        );
      }
      throw transportError(TransportErrorCode.NETWORK_ERROR, "Network request failed", cause);
    } finally {
      await cancelTrackedRequestBody(requestBodyCancel);
      abort.cleanup();
    }
  };

  const request: <Output>(
    operation: OperationDefinition,
    input?: unknown,
    requestOptions?: RequestOptions,
  ) => Promise<Output> = <Output>(
    operation: OperationDefinition,
    input?: unknown,
    requestOptions?: RequestOptions,
  ): Promise<Output> => execute<Output>(operation, input, requestOptions, false) as Promise<Output>;
  const raw: <Output>(
    operation: OperationDefinition,
    input?: unknown,
    requestOptions?: RequestOptions,
  ) => Promise<RawResponse<Output>> = <Output>(
    operation: OperationDefinition,
    input?: unknown,
    requestOptions?: RequestOptions,
  ): Promise<RawResponse<Output>> =>
    execute<Output>(operation, input, requestOptions, true) as Promise<RawResponse<Output>>;

  return Object.assign(request, { raw });
}
