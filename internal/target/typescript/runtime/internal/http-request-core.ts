import type { WireResponseDefinition } from "./wire-types.js";
import type { OperationDefinition } from "./operation.js";
import type { RequestMetadata, RequestOptions, OperationStream, RawResponse } from "./request.js";
import type {
  RequestContext,
  RequestExecutionServices,
  StreamingRequestExecutionServices,
  EncodedRequest,
  AbortContext,
  StreamingRequestInit,
} from "./http-types.js";
import type { BufferedRequestFunction, RequestFunction } from "./callables.js";
import type { ResponseFailureMetadata } from "./http-execution-support.js";
import {
  TransportErrorCode,
  isAPIError,
  isRecord,
  operationDiagnosticName,
} from "./runtime-support.js";
import {
  createAbortContext,
  isPromise,
  awaitAbortable,
  applyNoSecurity,
  isReadableStream,
  assertReadableResponseHeaders,
  requestMetadata,
  selectResponseDefinition,
  responseContentType,
  transportError,
  transportErrorFromCause,
  cancelResponseBody,
  cancelTrackedRequestBody,
} from "./http-execution-support.js";
/** Binds normalized client state to only the request services this operation needs. */
export function createRequestCore(
  context: RequestContext,
  services: StreamingRequestExecutionServices,
): RequestFunction;
/** Binds normalized client state to the request services required by an operation. */
export function createRequestCore(
  context: RequestContext,
  services: RequestExecutionServices,
): BufferedRequestFunction;
/** Binds normalized client state to the request services required by an operation. */
export function createRequestCore(
  context: RequestContext,
  services: RequestExecutionServices & Partial<StreamingRequestExecutionServices>,
): BufferedRequestFunction | RequestFunction {
  const { options, baseURL, fetchImplementation, codecs, streamCodecs }: RequestContext = context;
  const {
    encodeRequest,
    decodeResponse,
    decodeResponseHeaders,
    decodeResponseWireValue,
    createHTTPError,
    createOperationStream,
  }: RequestExecutionServices & Partial<StreamingRequestExecutionServices> = services;
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
      assertReadableResponseHeaders(options.transport, operation);
      const response: Response = await awaitAbortable(
        fetchImplementation(encoded.url, init),
        abort.signal,
      );
      const request: RequestMetadata = requestMetadata(response);
      responseMetadata = { request, status: response.status, response };
      const responseDefinition: WireResponseDefinition | undefined = selectResponseDefinition(
        operation,
        response,
        true,
      );
      if (
        raw &&
        response.ok &&
        (responseDefinition?.itemSchema !== undefined ||
          responseDefinition?.streamFraming !== undefined)
      ) {
        const contentType: string | undefined = responseContentType(response);
        let headerValues: Readonly<Record<string, unknown>>;
        try {
          headerValues = await awaitAbortable(
            decodeResponseHeaders(operation, response, codecs),
            abort.signal,
          );
        } catch (cause: unknown) {
          await response.body?.cancel().catch((): undefined => undefined);
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
          data: undefined as Output,
          headers: headerValues,
          request,
          response,
        };
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
  if (createOperationStream === undefined) return Object.assign(request, { raw });
  const stream: <Item>(
    operation: OperationDefinition,
    input?: unknown,
    requestOptions?: RequestOptions,
  ) => OperationStream<Item> = <Item>(
    operation: OperationDefinition,
    input?: unknown,
    requestOptions: RequestOptions = {},
  ): OperationStream<Item> =>
    createOperationStream<Item>(
      baseURL,
      options,
      codecs,
      streamCodecs,
      fetchImplementation,
      operation,
      input,
      requestOptions,
    );
  return Object.assign(request, { raw, stream });
}
