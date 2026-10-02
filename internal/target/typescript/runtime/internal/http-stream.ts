import { createHTTPServices } from "./http-core.js";
import { decodeResponseStreamItems } from "./streaming.js";
import { jsonWireCodec } from "./wire-engine.js";
import type { HTTPStreamDecodeOptions } from "./http-types.js";
import type { MediaCodec, StreamCodec, WireResponseDefinition } from "./wire-engine.js";
import type { ClientOptions } from "./configuration.js";
import { TransportErrorCode, isAPIError } from "./runtime-support.js";
import { operationDiagnosticName } from "./runtime-support.js";
import type { OperationDefinition } from "./operation.js";
import type {
  OperationStream,
  RequestOptions,
  StreamResponseMetadata,
  RequestMetadata,
} from "./request.js";
import {
  applyOperationSecurity,
  assertReadableResponseHeaders,
  awaitAbortable,
  cancelResponseBody,
  cancelTrackedRequestBody,
  createAbortContext,
  isPromise,
  isReadableStream,
  normalizeMediaType,
  requestMetadata,
  selectResponseDefinition,
  serverError,
  transportError,
} from "./http-core.js";
import type { EncodedRequest, AbortContext } from "./http-types.js";
import type { WireCodec } from "./wire-engine.js";
import type { RequestExecutionServices } from "./http-types.js";
import type { HTTPCodecExtensions, StreamingRequestExecutionServices } from "./http-types.js";

/** Shares the existing stream lifecycle while selecting only the required framing and wire decoder. */
export function createOperationStreamService(
  getBase: () => RequestExecutionServices,
  decodeResponseStreamItems: NonNullable<HTTPCodecExtensions["decodeResponseStreamItems"]>,
  wire: WireCodec,
): StreamingRequestExecutionServices["createOperationStream"] {
  const { transformWireValue }: WireCodec = wire;
  const tolerantResponseTransformOptions: TolerantResponseTransformOptions = {
    unknownProperties: "preserve",
  } as const;
  function createOperationStream<Item>(
    baseURL: string | undefined,
    options: ClientOptions,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    streamCodecs: ReadonlyMap<string, StreamCodec>,
    fetchImplementation: typeof globalThis.fetch,
    operation: OperationDefinition,
    input: unknown,
    requestOptions: RequestOptions,
  ): OperationStream<Item> {
    const controller: AbortController = new AbortController();
    const externalSignal: AbortSignal | undefined = requestOptions.signal;
    let externalAbortCleanup: () => void = (): void => undefined;
    let source: AsyncIterator<Item> | undefined;
    let firstNext: Promise<IteratorResult<Item>> | undefined;
    let consumerClaimed: boolean = false;
    let terminal: boolean = false;
    let closePromise: Promise<void> | undefined;
    let responseMetadata: StreamResponseMetadata | undefined;
    let responseFailure: unknown;
    let responsePromise: Promise<StreamResponseMetadata> | undefined;
    let resolveResponse: ((metadata: StreamResponseMetadata) => void) | undefined;
    let rejectResponse: ((cause: unknown) => void) | undefined;

    const cleanupExternalAbort: () => void = (): void => {
      externalAbortCleanup();
      externalAbortCleanup = (): void => undefined;
    };

    const settleResponse: (metadata: StreamResponseMetadata) => void = (
      metadata: StreamResponseMetadata,
    ): void => {
      if (responseMetadata !== undefined) return;
      responseMetadata = metadata;
      resolveResponse?.(metadata);
    };

    const failResponse: (cause: unknown) => void = (cause: unknown): void => {
      if (responseMetadata !== undefined || responseFailure !== undefined) return;
      responseFailure = cause;
      rejectResponse?.(cause);
    };

    const observe: (result: Promise<IteratorResult<Item>>) => Promise<IteratorResult<Item>> = (
      result: Promise<IteratorResult<Item>>,
    ): Promise<IteratorResult<Item>> => {
      void result.then(
        (next: IteratorResult<Item, unknown>): void => {
          if (!next.done) return;
          terminal = true;
          cleanupExternalAbort();
        },
        (cause: unknown): void => {
          terminal = true;
          failResponse(cause);
          cleanupExternalAbort();
        },
      );
      return result;
    };

    const nextFrom: (iterator: AsyncIterator<Item>) => Promise<IteratorResult<Item>> = (
      iterator: AsyncIterator<Item>,
    ): Promise<IteratorResult<Item>> => {
      try {
        return observe(Promise.resolve(iterator.next()));
      } catch (cause: unknown) {
        return observe(Promise.reject(cause));
      }
    };

    const stopStarted: (reason?: unknown) => Promise<void> = (reason?: unknown): Promise<void> => {
      if (!controller.signal.aborted) {
        controller.abort(
          reason ?? new Error("OperationStream consumption ended before completion"),
        );
      }
      if (source === undefined) {
        cleanupExternalAbort();
        return Promise.resolve();
      }
      closePromise ??= (async (): Promise<undefined> => {
        firstNext = undefined;
        try {
          await source?.return?.();
        } catch {
          // Cancellation cleanup is best-effort; the stream's terminal result is already settled.
        } finally {
          terminal = true;
          cleanupExternalAbort();
        }
      })();
      return closePromise;
    };

    const ensureStarted: () => AsyncIterator<Item> = (): AsyncIterator<Item> => {
      if (source !== undefined) return source;
      if (!controller.signal.aborted && externalSignal !== undefined) {
        if (externalSignal.aborted) {
          controller.abort(externalSignal.reason);
        } else {
          const forwardAbort: () => void = (): void => {
            void stopStarted(externalSignal.reason);
          };
          externalSignal.addEventListener("abort", forwardAbort, { once: true });
          externalAbortCleanup = (): void =>
            externalSignal.removeEventListener("abort", forwardAbort);
        }
      }
      const effectiveRequestOptions: RequestOptions = {
        ...requestOptions,
        signal: controller.signal,
      };
      source = streamOperation<Item>(
        baseURL,
        options,
        codecs,
        streamCodecs,
        fetchImplementation,
        operation,
        input,
        effectiveRequestOptions,
        settleResponse,
      )[Symbol.asyncIterator]();
      firstNext = nextFrom(source);
      return source;
    };

    const advance: () => Promise<IteratorResult<Item>> = async (): Promise<
      IteratorResult<Item>
    > => {
      if (terminal) return { done: true, value: undefined as never };
      const iterator: AsyncIterator<Item, unknown, unknown> = ensureStarted();
      const pending: Promise<IteratorResult<Item, unknown>> = firstNext ?? nextFrom(iterator);
      firstNext = undefined;
      return pending;
    };

    const close: (reason?: unknown) => Promise<void> = async (reason?: unknown): Promise<void> => {
      if (terminal) {
        cleanupExternalAbort();
        return;
      }
      await stopStarted(reason);
    };

    const consume: () => AsyncIterableIterator<Item> =
      async function* (): AsyncIterableIterator<Item> {
        try {
          while (true) {
            const next: IteratorResult<Item, unknown> = await advance();
            if (next.done === true) return;
            yield next.value;
          }
        } finally {
          if (!terminal) await close();
        }
      };

    const claimConsumer: () => void = (): void => {
      if (consumerClaimed) throw new TypeError("OperationStream already has a consumer");
      consumerClaimed = true;
    };

    const getResponse: () => Promise<StreamResponseMetadata> =
      (): Promise<StreamResponseMetadata> => {
        ensureStarted();
        if (responseMetadata !== undefined) return Promise.resolve(responseMetadata);
        if (responseFailure !== undefined) return Promise.reject(responseFailure);
        responsePromise ??= new Promise<StreamResponseMetadata>(
          (
            resolve: (value: StreamResponseMetadata | PromiseLike<StreamResponseMetadata>) => void,
            reject: (reason?: unknown) => void,
          ): void => {
            resolveResponse = resolve;
            rejectResponse = reject;
            if (responseMetadata !== undefined) resolve(responseMetadata);
            else if (responseFailure !== undefined) reject(responseFailure);
          },
        );
        return responsePromise;
      };

    const stream: OperationStream<Item> = {
      get response(): Promise<StreamResponseMetadata> {
        return getResponse();
      },
      abort(reason?: unknown): void {
        void stopStarted(reason);
      },
      toReadableStream(): ReadableStream<Item> {
        claimConsumer();
        const iterator: AsyncIterableIterator<Item> = consume();
        return new ReadableStream<Item>(
          {
            async pull(readableController: ReadableStreamDefaultController<Item>): Promise<void> {
              try {
                const next: IteratorResult<Item, unknown> = await iterator.next();
                if (next.done === true) readableController.close();
                else readableController.enqueue(next.value);
              } catch (cause: unknown) {
                readableController.error(cause);
              }
            },
            async cancel(reason: unknown): Promise<void> {
              stream.abort(reason);
              await iterator.return?.();
            },
          },
          { highWaterMark: 0 },
        );
      },
      [Symbol.asyncIterator](): AsyncIterator<Item> {
        claimConsumer();
        return consume();
      },
    };
    return stream;
  }

  async function* streamOperation<Item>(
    baseURL: string | undefined,
    options: ClientOptions,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    streamCodecs: ReadonlyMap<string, StreamCodec>,
    fetchImplementation: typeof globalThis.fetch,
    operation: OperationDefinition,
    input: unknown,
    requestOptions: RequestOptions,
    onResponse: (metadata: StreamResponseMetadata) => void,
  ): AsyncIterable<Item> {
    const credentials: RequestCredentials | undefined =
      requestOptions.credentials ?? options.credentials;
    const timeoutMS: number | undefined = requestOptions.timeoutMS ?? options.timeoutMS;
    const abort: AbortContext = createAbortContext(requestOptions.signal, timeoutMS, true);
    let response: Response | undefined;
    let streamContentType: string | undefined;
    let requestBodyFailure: (() => unknown) | undefined;
    let requestBodyCancel: ((reason?: unknown) => Promise<void>) | undefined;
    let completed: boolean = false;
    try {
      let encoded: EncodedRequest;
      try {
        if (abort.signal?.aborted) throw abort.signal.reason;
        const effectiveRequestOptions: RequestOptions =
          abort.signal === undefined ? requestOptions : { ...requestOptions, signal: abort.signal };
        const pending: EncodedRequest | Promise<EncodedRequest> = getBase().encodeRequest(
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
        const secured: EncodedRequest | Promise<EncodedRequest> = applyOperationSecurity(
          options,
          operation,
          encoded,
          effectiveRequestOptions,
          credentials,
        );
        encoded = isPromise(secured) ? await awaitAbortable(secured, abort.signal) : secured;
        requestBodyFailure = encoded.bodyFailure;
        requestBodyCancel = encoded.bodyCancel;
      } catch (cause: unknown) {
        if (abort.timedOut() || abort.aborted()) throw cause;
        if (isAPIError(cause)) throw cause;
        throw transportError(
          TransportErrorCode.REQUEST_ENCODE_FAILED,
          `Failed to encode ${operationDiagnosticName(operation)} stream request`,
          cause,
        );
      }
      const init: RequestInit = {
        method: operation.method,
        headers: encoded.headers,
        ...(abort.signal === undefined ? {} : { signal: abort.signal }),
        ...(encoded.redirect === undefined ? {} : { redirect: encoded.redirect }),
      };
      if (encoded.body !== undefined) {
        init.body = encoded.body as BodyInit;
        if (isReadableStream(encoded.body))
          (init as RequestInit & RequestDuplexExtension).duplex = "half";
      }
      if (credentials !== undefined) init.credentials = credentials;
      if (abort.signal?.aborted) throw abort.signal.reason;
      assertReadableResponseHeaders(options.transport, operation);
      response = await awaitAbortable(fetchImplementation(encoded.url, init), abort.signal);
      const request: RequestMetadata = requestMetadata(response);
      if (!response.ok) {
        const decodedBody: unknown = await awaitAbortable(
          getBase().decodeResponse(operation, response, request, {
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
        const body: unknown = getBase().decodeResponseWireValue(operation, response, decodedBody);
        throw serverError(response, request, body);
      }
      const definition: WireResponseDefinition | undefined = selectResponseDefinition(
        operation,
        response,
        true,
      );
      if (definition?.itemSchema === undefined || response.body === null) {
        throw new TypeError(
          `response for ${operationDiagnosticName(operation)} is not a declared stream`,
        );
      }
      const contentType: string = response.headers.get("content-type") ?? definition.contentType;
      streamContentType = normalizeMediaType(contentType);
      onResponse({
        status: response.status,
        contentType: streamContentType,
        headers: response.headers,
        request,
      });
      const maxFrameBytes: number = resolveMaxStreamFrameBytes(
        requestOptions.maxStreamFrameBytes ?? options.maxStreamFrameBytes,
      );
      const streamCodec: StreamCodec<unknown, unknown> | undefined = resolveStreamCodec(
        contentType,
        requestOptions.streamCodec,
        streamCodecs,
      );
      for await (const value of decodeResponseStreamItems(response.body, {
        contentType,
        streamFraming: definition.streamFraming,
        itemSchema: definition.itemSchema,
        prefixSchemas: undefined,
        schemas: operation.outputSchemas ?? {},
        codecs,
        prefixEncoding: undefined,
        itemEncoding: definition.itemEncoding,
        maxFrameBytes,
        streamCodec,
        signal: abort.signal,
      })) {
        yield transformWireValue(
          value,
          definition.itemSchema,
          operation.outputSchemas ?? {},
          "decode",
          tolerantResponseTransformOptions,
        ) as Item;
      }
      completed = true;
    } catch (cause: unknown) {
      await cancelTrackedRequestBody(requestBodyCancel, cause);
      if (abort.timedOut()) {
        await cancelResponseBody(response, cause);
        throw transportError(
          TransportErrorCode.REQUEST_TIMEOUT,
          `Request timed out after ${timeoutMS}ms`,
          cause,
        );
      }
      if (abort.aborted()) {
        await cancelResponseBody(response, cause);
        throw transportError(TransportErrorCode.REQUEST_ABORTED, "Request was aborted", cause);
      }
      if (isAPIError(cause)) throw cause;
      const bodyFailure: unknown = requestBodyFailure?.();
      if (bodyFailure !== undefined) {
        throw transportError(
          TransportErrorCode.REQUEST_ENCODE_FAILED,
          `Failed to encode ${operationDiagnosticName(operation)} stream request body`,
          bodyFailure,
        );
      }
      if (response === undefined)
        throw transportError(TransportErrorCode.NETWORK_ERROR, "Network request failed", cause);
      throw transportError(
        TransportErrorCode.RESPONSE_DECODE_FAILED,
        `Failed to decode ${operationDiagnosticName(operation)} stream${streamContentType === undefined ? "" : ` (${streamContentType})`}`,
        cause,
      );
    } finally {
      if (!completed) abort.cancel(new Error("Stream consumption ended before completion"));
      await cancelTrackedRequestBody(requestBodyCancel);
      abort.cleanup();
    }
  }

  function resolveMaxStreamFrameBytes(value: number | undefined): number {
    const resolved: number = value ?? 1024 * 1024;
    if (!Number.isSafeInteger(resolved) || resolved <= 0)
      throw new TypeError("maxStreamFrameBytes must be a positive safe integer");
    return resolved;
  }

  function resolveStreamCodec(
    contentType: string,
    override: StreamCodec | undefined,
    defaults: ReadonlyMap<string, StreamCodec>,
  ): StreamCodec | undefined {
    return override ?? defaults.get(normalizeMediaType(contentType));
  }
  return createOperationStream;
}

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

type TolerantResponseTransformOptions = { readonly unknownProperties: "preserve" };

type RequestDuplexExtension = {
  duplex?: "half";
};
