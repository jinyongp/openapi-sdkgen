import type { StreamFraming } from "../stream/stream-protocol-types.js";
import type { WireSchema } from "../schema/wire-types.js";
import type { WireSchemas } from "../schema/wire-types.js";
import type { MediaCodec } from "./media-codec-types.js";
import type { WireEncodingDefinition } from "./media-contract-types.js";
import type { StreamCodec } from "../stream/stream-protocol-types.js";
import type { StreamContext } from "../stream/stream-protocol-types.js";
import type { WireBodyDefinition } from "./media-contract-types.js";

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

/** Canonical value shared by named, positional and streaming multipart encoders. */
export interface MultipartPartValue {
  readonly body: BlobPart;
  readonly contentType?: string;
  readonly filename?: string;
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
