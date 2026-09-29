import type {
  MediaCodec,
  StreamCodec,
  StreamContext,
  StreamFraming,
  WireBodyDefinition,
  WireEncodingDefinition,
  WireSchema,
  WireSchemas,
} from "./wire-engine.js";
import type { ClientOptions } from "./configuration.js";
import type { OperationDefinition } from "./operation.js";
import type { OperationStream, RequestMetadata, RequestOptions } from "./request.js";

/** Internal operation options after generated security selection is attached. */
export interface OperationRequestOptions extends RequestOptions {
  readonly securityRequirement?: string;
}

/** Operation-specific schemas and codecs required to decode a framed response stream. */
export interface HTTPStreamDecodeOptions {
  readonly contentType: string;
  readonly streamFraming: StreamFraming | undefined;
  readonly itemSchema: WireSchema;
  readonly prefixSchemas: readonly WireSchema[] | undefined;
  readonly schemas: WireSchemas;
  readonly codecs: ReadonlyMap<string, MediaCodec<unknown>>;
  readonly prefixEncoding: readonly WireEncodingDefinition[] | undefined;
  readonly itemEncoding: WireEncodingDefinition | undefined;
  readonly maxFrameBytes: number;
  readonly streamCodec: StreamCodec | undefined;
  readonly signal: AbortSignal | undefined;
}

/** A decoded multipart part with its headers and raw bytes. */
export interface MultipartStreamPart {
  readonly headers: Headers;
  readonly bytes: Uint8Array;
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

/** An encoded streaming request body and its actual Content-Type. */
export interface EncodedStreamRequestBody {
  readonly body: ReadableStream<Uint8Array>;
  readonly contentType: string;
}

/** Operation and request settings for incrementally encoded request items. */
export interface IncrementalStreamRequestOptions {
  readonly contentType: string;
  readonly streamFraming: StreamFraming | undefined;
  readonly itemSchema: WireSchema;
  readonly schemas: WireSchemas;
  readonly streamCodec: StreamCodec | undefined;
  readonly maxFrameBytes: number;
  readonly signal: AbortSignal | undefined;
  readonly itemEncoding: WireEncodingDefinition | undefined;
  readonly suppliedHeaders: Readonly<Record<string, HeadersInit>> | undefined;
  readonly suppliedContentTypes: Readonly<Record<string, string>> | undefined;
  readonly codecs: ReadonlyMap<string, MediaCodec<unknown>>;
}

/** Settings for encoding an already complete sequential request value. */
export interface CompleteSequentialRequestOptions extends Omit<
  IncrementalStreamRequestOptions,
  "itemSchema"
> {
  readonly schema: WireSchema;
  readonly prefixEncoding: readonly WireEncodingDefinition[] | undefined;
}

/** Framing, multipart and cancellation context passed to a request stream encoder. */
export interface StreamProtocolEncodeOptions {
  readonly contentType: string;
  readonly streamFraming: StreamFraming | undefined;
  readonly streamCodec: StreamCodec | undefined;
  readonly context: StreamContext;
  readonly frameSchema: WireSchema;
  readonly prefixSchemas: readonly WireSchema[] | undefined;
  readonly schemas: WireSchemas;
  readonly prefixEncoding: readonly WireEncodingDefinition[] | undefined;
  readonly itemEncoding: WireEncodingDefinition | undefined;
  readonly suppliedHeaders: Readonly<Record<string, HeadersInit>> | undefined;
  readonly suppliedContentTypes: Readonly<Record<string, string>> | undefined;
  readonly codecs: ReadonlyMap<string, MediaCodec<unknown>>;
}

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
export interface RequestExecutionServices {
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

/** Media-specific operations required only by plans that can reach those code paths. */
export interface HTTPCodecExtensions {
  encodeRequestBody(
    contentType: string,
    value: unknown,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
    schema: WireSchema | undefined,
    schemas: WireSchemas,
    definition: WireBodyDefinition | undefined,
    multipartHeaders: Readonly<Record<string, HeadersInit>> | undefined,
    multipartContentTypes: Readonly<Record<string, string>> | undefined,
  ): BodyInit | Promise<BodyInit>;
  encodeIncrementalStreamRequestBody?(
    values: AsyncIterable<unknown>,
    options: IncrementalStreamRequestOptions,
  ): EncodedStreamRequestBody | Promise<EncodedStreamRequestBody>;
  encodeCompleteSequentialRequestBody?(
    value: unknown,
    options: CompleteSequentialRequestOptions,
  ): EncodedStreamRequestBody | Promise<EncodedStreamRequestBody>;
  decodeResponseStreamItems?(
    body: ReadableStream<Uint8Array>,
    options: HTTPStreamDecodeOptions,
  ): AsyncIterable<unknown>;
  decodeMultipartResponse?(
    body: ReadableStream<Uint8Array>,
    contentType: string,
    definition: WireBodyDefinition,
    schemas: WireSchemas,
    codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  ): Promise<unknown[]>;
  encodeXML?(value: unknown, schema: WireSchema, components: WireSchemas): string;
  decodeXML?(value: string, schema: WireSchema, components: WireSchemas): unknown;
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
