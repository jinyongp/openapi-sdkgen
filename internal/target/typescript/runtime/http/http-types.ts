import type { HTTPCodecExtensions } from "../media/media-service-types.js";

import type { MediaCodec } from "../media/media-codec-types.js";
import type { StreamCodec } from "../stream/stream-protocol-types.js";
import type { ClientOptions } from "./configuration.js";
import type { OperationDefinition } from "./operation.js";
import type { OperationStream, RequestOptions } from "./request.js";
import type { RequestMetadata } from "../shared/runtime-support.js";
import type { APIError } from "../shared/runtime-support.js";

/** Internal operation options after generated security selection is attached. */
export interface OperationRequestOptions extends RequestOptions {
  readonly securityRequirement?: string;
}

/** Fetch request settings with the streaming-body extension used by Node. */
export interface StreamingRequestInit extends RequestInit {
  duplex?: "half";
}

/** The available credential source and collision state for one security scheme. */
export type SDKSecuritySource =
  | { readonly state: "none" }
  | { readonly state: "conflict"; readonly location: string }
  | {
      readonly state: "satisfied";
      readonly kind: "header";
      readonly name: string;
      readonly value: string;
    }
  | { readonly state: "satisfied"; readonly kind: "cookie" | "mutualTLS" };

/** A serialized request together with optional streaming body cleanup hooks. */
export interface EncodedRequest {
  readonly url: string;
  readonly headers: Headers;
  readonly body?: BodyInit | ReadableStream<Uint8Array>;
  readonly bodyFailure?: () => unknown;
  readonly bodyCancel?: (reason?: unknown) => Promise<void>;
  readonly redirect?: RequestRedirect;
}

/** Encodes typed query values using their OpenAPI escaping and serialization rules. */
export type QueryEncoder = (
  query: Readonly<Record<string, unknown>>,
  operation: OperationDefinition,
  location: "query" | "querystring",
) => string;

/** One query parameter value with its OpenAPI escaping and serialization rules. */
export interface QueryPart {
  readonly name?: string;
  readonly value?: string;
  readonly allowReserved?: boolean;
  readonly raw?: string;
}

/** Request-local cancellation, timeout observation and listener cleanup. */
export interface AbortContext {
  readonly signal: AbortSignal | undefined;
  readonly timedOut: () => boolean;
  readonly aborted: () => boolean;
  readonly cancel: (reason?: unknown) => void;
  readonly cleanup: () => void;
}

/** Codec and cancellation settings used to decode an HTTP response. */
export interface ResponseDecodeOptions {
  readonly codecs: ReadonlyMap<string, MediaCodec<unknown>>;
  readonly streamCodecs: ReadonlyMap<string, StreamCodec>;
  readonly streamCodec?: StreamCodec;
  readonly maxStreamFrameBytes?: number | undefined;
  readonly signal?: AbortSignal;
}

/** Client-wide normalized transport state, reused by each operation execution plan. */
export interface RequestContext {
  readonly options: ClientOptions;
  readonly baseURL: string | undefined;
  readonly fetchImplementation: typeof globalThis.fetch;
  readonly codecs: ReadonlyMap<string, MediaCodec<unknown>>;
  readonly streamCodecs: ReadonlyMap<string, StreamCodec>;
}

/** Concrete operation implementations selected by the generator; no runtime feature registry. */
export type OperationSecurity = (
  options: ClientOptions,
  operation: OperationDefinition,
  encoded: EncodedRequest,
  requestOptions: OperationRequestOptions,
  credentials: RequestCredentials | undefined,
) => EncodedRequest | Promise<EncodedRequest>;

/** Canonical buffered execution hooks shared by generated provider compositions. */
export interface RequestExecutionServices {
  readonly applyOperationSecurity?: OperationSecurity;
  encodeRequest(
    baseURL: string | undefined,
    client: ClientOptions,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    streamCodecs: ReadonlyMap<string, StreamCodec>,
    operation: OperationDefinition,
    input: unknown,
    options: RequestOptions,
  ): EncodedRequest | Promise<EncodedRequest>;
  decodeResponse(
    operation: OperationDefinition,
    response: Response,
    request: RequestMetadata,
    options: ResponseDecodeOptions,
  ): Promise<unknown>;
  decodeResponseHeaders(
    operation: OperationDefinition,
    response: Response,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<Readonly<Record<string, unknown>>>;
  decodeResponseWireValue(
    operation: OperationDefinition,
    response: Response,
    value: unknown,
  ): unknown;
  createHTTPError(
    operation: OperationDefinition,
    response: Response,
    request: RequestMetadata,
    data: unknown,
  ): APIError;
}

/** Execution services for an operation exposing the existing synchronous stream handle. */
export interface StreamingRequestExecutionServices extends RequestExecutionServices {
  createOperationStream<Item>(
    baseURL: string | undefined,
    options: ClientOptions,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    streamCodecs: ReadonlyMap<string, StreamCodec>,
    fetchImplementation: typeof globalThis.fetch,
    operation: OperationDefinition,
    input: unknown,
    requestOptions: RequestOptions,
  ): OperationStream<Item>;
}

/** Complete extension set used by the compatibility request factory. */
export interface AdvancedHTTPServices extends Required<HTTPCodecExtensions> {
  createOperationStream<Item>(
    baseURL: string | undefined,
    options: ClientOptions,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    streamCodecs: ReadonlyMap<string, StreamCodec>,
    fetchImplementation: typeof globalThis.fetch,
    operation: OperationDefinition,
    input: unknown,
    requestOptions: RequestOptions,
  ): OperationStream<Item>;
}
