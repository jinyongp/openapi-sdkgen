import type { MediaCodec, StreamCodec, WireResponseDefinition } from "./wire-types.js";
import type { ClientOptions } from "./configuration.js";
import type { OperationDefinition } from "./operation.js";
import type { RequestMetadata } from "./request.js";
import type { Transport } from "./transport.js";
import type {
  AbortContext,
  RequestContext,
  OperationRequestOptions,
  EncodedRequest,
} from "./http-types.js";
import { APIError, TransportErrorCode, isAPIError, isRecord } from "./runtime-support.js";
import type { TransportError } from "./runtime-support.js";
/** Checks transport capabilities required by declared response headers. */
export function assertReadableResponseHeaders(
  transport: Transport | undefined,
  operation: OperationDefinition,
): void {
  const readable: true | readonly string[] | undefined =
    transport?.capabilities?.readableResponseHeaders;
  for (const response of operation.responses ?? []) {
    for (const header of response.headers ?? []) {
      if (!header.required || header.name.toLowerCase() !== "set-cookie") continue;
      if (
        readable === true ||
        (Array.isArray(readable) &&
          readable.some((name: string): boolean => name.toLowerCase() === "set-cookie"))
      )
        continue;
      throw transportError(
        TransportErrorCode.TRANSPORT_CAPABILITY_REQUIRED,
        "Reading required Set-Cookie response headers requires a capable transport",
        undefined,
      );
    }
  }
}

/** Finalizes a request body source while preserving the original request failure. */
export async function cancelTrackedRequestBody(
  cancel: ((reason?: unknown) => Promise<void>) | undefined,
  reason?: unknown,
): Promise<void> {
  if (cancel === undefined) return;
  await cancel(reason).catch((): undefined => undefined);
}

/** Selects the most specific declared response for the actual status and media type. */
export function selectResponseDefinition(
  operation: OperationDefinition,
  response: Response,
  requireMediaMatch: boolean,
): WireResponseDefinition | undefined {
  const contentType: string | undefined = responseContentType(response);
  return operation.responses
    ?.filter((item: WireResponseDefinition): boolean => {
      if (!statusMatches(item.status, response.status)) return false;
      if (!requireMediaMatch) return true;
      return contentType === undefined
        ? item.contentType === ""
        : mediaTypeMatches(item.contentType, contentType);
    })
    .sort((left: WireResponseDefinition, right: WireResponseDefinition): number => {
      const statusDifference: number =
        statusMatchScore(right.status, response.status) -
        statusMatchScore(left.status, response.status);
      if (statusDifference !== 0) return statusDifference;
      return (
        mediaTypeMatchScore(right.contentType, contentType) -
        mediaTypeMatchScore(left.contentType, contentType)
      );
    })[0];
}

function statusMatches(pattern: string, status: number): boolean {
  if (pattern === String(status) || pattern === "default") return true;
  return /^\dXX$/i.test(pattern) && Number(pattern[0]) === Math.floor(status / 100);
}

function statusMatchScore(pattern: string, status: number): number {
  if (pattern === String(status)) return 3;
  if (/^\dXX$/i.test(pattern) && Number(pattern[0]) === Math.floor(status / 100)) return 2;
  return pattern === "default" ? 1 : 0;
}

/** Matches a concrete media type against an OpenAPI media range. */
export function mediaTypeMatches(pattern: string, actual: string): boolean {
  const expected: string = pattern.split(";", 1)[0]?.trim().toLowerCase() ?? "";
  const received: string = actual.split(";", 1)[0]?.trim().toLowerCase() ?? "";
  if (expected === received || expected === "*/*") return true;
  const [expectedType, expectedSubtype]: string[] = expected.split("/", 2);
  const [receivedType, receivedSubtype]: string[] = received.split("/", 2);
  if (
    expectedType === undefined ||
    expectedSubtype === undefined ||
    receivedType === undefined ||
    receivedSubtype === undefined
  )
    return false;
  if (expectedType !== "*" && expectedType !== receivedType) return false;
  if (expectedSubtype === "*") return true;
  if (expectedSubtype.startsWith("*+")) return receivedSubtype.endsWith(expectedSubtype.slice(1));
  return false;
}

/** Ranks exact, suffix-wildcard and general media matches. */
export function mediaTypeMatchScore(pattern: string, actual: string | undefined): number {
  if (actual === undefined) return 0;
  const normalized: string = normalizeMediaType(pattern);
  if (normalized === normalizeMediaType(actual)) return 3;
  if (normalized.includes("*+")) return 2;
  if (normalized.includes("*")) return 1;
  return 0;
}

/** Validates and normalizes the request base URL without accepting query or fragment components. */
export function normalizeBaseURL(value: string): string {
  let url: URL;
  try {
    url = new URL(value);
  } catch {
    throw new TypeError("baseURL must be an absolute URL");
  }
  if ((url.protocol !== "http:" && url.protocol !== "https:") || url.search || url.hash) {
    throw new TypeError("baseURL must be an absolute http(s) URL without query or fragment");
  }
  url.pathname = url.pathname.replace(/\/+$/, "");
  return url.href.replace(/\/$/, "");
}

/** Combines request cancellation and timeout into request-local cleanup state. */
export function createAbortContext(
  signal: AbortSignal | undefined,
  timeoutMS: number | undefined,
  alwaysCreateSignal: boolean = false,
): AbortContext {
  if (timeoutMS !== undefined && (!Number.isFinite(timeoutMS) || timeoutMS <= 0)) {
    throw new TypeError("timeoutMS must be a positive finite number");
  }
  if (signal === undefined && timeoutMS === undefined && !alwaysCreateSignal) {
    return {
      signal: undefined,
      timedOut: (): false => false,
      aborted: (): false => false,
      cancel: (): undefined => undefined,
      cleanup: (): undefined => undefined,
    };
  }
  const controller: AbortController = new AbortController();
  let timeoutReached: boolean = false;
  const forwardAbort: () => void = (): void => controller.abort(signal?.reason);
  if (signal?.aborted) forwardAbort();
  else signal?.addEventListener("abort", forwardAbort, { once: true });
  const timer: ReturnType<typeof setTimeout> | undefined =
    timeoutMS === undefined
      ? undefined
      : setTimeout((): void => {
          timeoutReached = true;
          controller.abort();
        }, timeoutMS);
  return {
    signal: controller.signal,
    timedOut: (): boolean => timeoutReached,
    aborted: (): boolean => signal?.aborted === true,
    cancel: (reason?: unknown): void => controller.abort(reason),
    cleanup: (): void => {
      if (timer !== undefined) clearTimeout(timer);
      signal?.removeEventListener("abort", forwardAbort);
    },
  };
}

/** Returns the normalized media type from the response Content-Type header. */
export function responseContentType(response: Response): string | undefined {
  return response.headers.get("content-type")?.split(";", 1)[0]?.trim().toLowerCase();
}

function normalizeCodecs(
  codecs: Readonly<Record<string, MediaCodec<unknown>>> | undefined,
): ReadonlyMap<string, MediaCodec<unknown>> {
  const result: Map<string, MediaCodec<unknown>> = new Map<string, MediaCodec<unknown>>();
  for (const [contentType, codec] of Object.entries(codecs ?? {})) {
    const normalized: string = normalizeMediaType(contentType);
    if (normalized === "" || result.has(normalized))
      throw new TypeError(`duplicate or invalid media codec ${contentType}`);
    result.set(normalized, codec);
  }
  return result;
}

function normalizeStreamCodecs(
  codecs: Readonly<Record<string, StreamCodec>> | undefined,
): ReadonlyMap<string, StreamCodec> {
  const result: Map<string, StreamCodec<unknown, unknown>> = new Map<string, StreamCodec>();
  for (const [contentType, codec] of Object.entries(codecs ?? {})) {
    const normalized: string = normalizeMediaType(contentType);
    if (normalized === "" || result.has(normalized))
      throw new TypeError(`duplicate or invalid stream codec ${contentType}`);
    result.set(normalized, codec);
  }
  return result;
}

/** Normalizes a media type for matching, excluding parameters. */
export function normalizeMediaType(contentType: string): string {
  return contentType.split(";", 1)[0]?.trim().toLowerCase() ?? "";
}

/** Recognizes a thenable returned by asynchronous request services. */
export function isPromise<Value>(value: Value | Promise<Value>): value is Promise<Value> {
  return typeof (value as Promise<Value>)?.then === "function";
}

/** Recognizes a byte stream used as a Fetch request body. */
export function isReadableStream(value: unknown): value is ReadableStream<Uint8Array> {
  return (
    value !== null &&
    typeof value === "object" &&
    typeof (value as ReadableStream<Uint8Array>).getReader === "function"
  );
}

/** Preserves response body and metadata in an API failure. */
export function serverError(
  response: Response,
  request: RequestMetadata,
  body: unknown,
  contentType?: string,
): APIError {
  const envelope: unknown = isRecord(body) && isRecord(body["error"]) ? body["error"] : body;
  const error: Record<string, unknown> = isRecord(envelope) ? envelope : {};
  const code: string =
    typeof error["code"] === "string" ? error["code"] : `HTTP_${response.status}`;
  const message: string =
    typeof error["message"] === "string"
      ? error["message"]
      : typeof body === "string" && body.trim() !== ""
        ? body
        : `HTTP request failed with status ${response.status}`;
  return new APIError({
    code,
    message,
    request,
    status: response.status,
    details: error["details"] ?? error["fields"],
    fields: error["fields"],
    data: body,
    response,
    ...(contentType === undefined ? {} : { contentType }),
  });
}

/** Captures request identifiers exposed by the response headers. */
export function requestMetadata(response: Response): RequestMetadata {
  const id: string | null = response.headers.get("x-request-id");
  return id === null ? {} : { id };
}

/** Constructs a typed transport error preserving the underlying cause. */
export function transportError(
  code: TransportErrorCode,
  message: string,
  cause: unknown,
): TransportError {
  return new APIError({ code, message, cause });
}

/** Constructs a transport failure retaining available request and response metadata. */
export function transportErrorFromCause(
  code: TransportErrorCode,
  message: string,
  cause: unknown,
  responseMetadata?: ResponseFailureMetadata,
): TransportError {
  if (isAPIError(cause)) {
    return new APIError({
      code,
      message,
      cause,
      request: cause.request,
      ...(cause.status === undefined ? {} : { status: cause.status }),
      ...(cause.response === undefined ? {} : { response: cause.response }),
    });
  }
  if (responseMetadata !== undefined) {
    return new APIError({ code, message, cause, ...responseMetadata });
  }
  return transportError(code, message, cause);
}

/** Cancels an outstanding response body without replacing the original request error. */
export async function cancelResponseBody(
  response: Response | undefined,
  reason?: unknown,
): Promise<void> {
  const body: ReadableStream<Uint8Array<ArrayBuffer>> | null | undefined = response?.body;
  if (body === null || body === undefined || typeof body.cancel !== "function" || body.locked)
    return;
  await body.cancel(reason).catch((): undefined => undefined);
}

/** Stops waiting when the request aborts and removes cancellation listeners after settlement. */
export function awaitAbortable<Value>(
  value: Promise<Value>,
  signal: AbortSignal | undefined,
): Promise<Value> {
  if (signal === undefined) return value;
  if (signal.aborted) {
    void value.catch((): undefined => undefined);
    return Promise.reject(signal.reason);
  }
  return new Promise(
    (
      resolve: (value: Value | PromiseLike<Value>) => void,
      reject: (reason?: unknown) => void,
    ): void => {
      const onAbort: () => void = (): void => reject(signal.reason);
      signal.addEventListener("abort", onAbort, { once: true });
      value.then(
        (result: Value): void => {
          signal.removeEventListener("abort", onAbort);
          resolve(result);
        },
        (cause: unknown): void => {
          signal.removeEventListener("abort", onAbort);
          reject(cause);
        },
      );
    },
  );
}

/** Normalizes client defaults once; keeps request callbacks and mutable client state instance-local. */
export function createRequestContext(options: ClientOptions): RequestContext {
  const baseURL: string | undefined =
    options.baseURL === undefined ? undefined : normalizeBaseURL(options.baseURL);
  const fetchImplementation: RequestContext["fetchImplementation"] =
    options.transport?.fetch ?? options.fetch ?? globalThis.fetch;
  if (typeof fetchImplementation !== "function") {
    throw new TypeError("fetch is unavailable; pass ClientOptions.fetch");
  }
  const codecs: ReadonlyMap<string, MediaCodec<unknown>> = normalizeCodecs(options.codecs);
  const streamCodecs: ReadonlyMap<string, StreamCodec<unknown, unknown>> = normalizeStreamCodecs(
    options.streamCodecs,
  );

  return { options, baseURL, fetchImplementation, codecs, streamCodecs };
}

/** Available request and response metadata retained by transport failures. */
export interface ResponseFailureMetadata {
  request: RequestMetadata;
  status: number;
  response: Response;
}

/** No-auth composition still rejects explicit selection and missing compiled services. */
export function applyNoSecurity(
  _options: ClientOptions,
  operation: OperationDefinition,
  encoded: EncodedRequest,
  requestOptions: OperationRequestOptions,
  _credentials: RequestCredentials | undefined,
): EncodedRequest {
  if (
    requestOptions.securityRequirement !== undefined ||
    (operation.security?.some(
      (requirement: import("./security.js").SecurityRequirementDefinition): boolean =>
        requirement.schemes.length > 0,
    ) ??
      false)
  )
    throw transportError(
      TransportErrorCode.SECURITY_REQUIREMENT_INVALID,
      "The operation does not declare an OpenAPI security requirement or its security handler is missing",
      undefined,
    );
  return encoded;
}

/** Requires the implementation selected for a declared HTTP capability. */
export function requireHTTPHook<Hook>(hook: Hook | undefined): Hook {
  if (hook === undefined)
    throw new TypeError("Operation execution plan is missing a required HTTP implementation");
  return hook;
}

/** Validates the byte bound shared by request and response stream framing. */
export function resolveMaxStreamFrameBytes(value: number | undefined): number {
  const resolved: number = value ?? 1024 * 1024;
  if (!Number.isSafeInteger(resolved) || resolved <= 0)
    throw new TypeError("maxStreamFrameBytes must be a positive safe integer");
  return resolved;
}

/** Selects a request-local framing override before normalized client defaults. */
export function resolveStreamCodec(
  contentType: string,
  override: StreamCodec | undefined,
  defaults: ReadonlyMap<string, StreamCodec>,
): StreamCodec | undefined {
  return override ?? defaults.get(normalizeMediaType(contentType));
}
