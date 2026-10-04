import type {
  HTTPCodecExtensions,
  EncodedStreamRequestBody,
  StreamProtocolEncodeOptions,
} from "./http-types.js";
import type { MediaCodec, WireSchema, WireSchemas } from "./wire-types.js";
/** Request body encoder derived from the canonical HTTP extension contract. */
export type HTTPBodyEncoder = HTTPCodecExtensions["encodeRequestBody"];
/** Decodes a named header with its whole schema and declared representation. */
export type HTTPHeaderDecoder = (
  name: string,
  value: string,
  schema: WireSchema,
  contentType: string | undefined,
  explode: boolean | undefined,
  schemas: WireSchemas,
  codecs: ReadonlyMap<string, MediaCodec<unknown>>,
  role?: "response" | "multipart",
) => Promise<unknown>;
/** Produces a framed text request stream with shared cleanup and backpressure. */
export type TextRequestEncoder = (
  values: AsyncIterable<unknown>,
  options: StreamProtocolEncodeOptions,
) => EncodedStreamRequestBody;
/** Encodes multipart frames through the same request stream contract. */
export type MultipartRequestEncoder = TextRequestEncoder;
/** Buffered and streamed multipart encoders sharing one part implementation. */
export interface MultipartRequestServices {
  readonly encodeBody: HTTPBodyEncoder;
  readonly encodeStream: MultipartRequestEncoder;
}
/** Complete and incremental multipart response decoding hooks. */
export type MultipartResponseServices = Required<
  Pick<HTTPCodecExtensions, "decodeMultipartResponse">
> & {
  readonly decodeStreamItems: (
    body: ReadableStream<Uint8Array>,
    options: import("./http-types.js").HTTPStreamDecodeOptions,
  ) => AsyncIterable<unknown>;
};
/** Canonical hooks for incremental and complete sequential request bodies. */
export type RequestStreamServices = Required<
  Pick<
    HTTPCodecExtensions,
    "encodeIncrementalStreamRequestBody" | "encodeCompleteSequentialRequestBody"
  >
>;
/** Optional body representation handlers selected during generator preparation. */
export type BodyEncoders = Readonly<
  Partial<
    Record<"json" | "xml" | "form" | "multipart" | "text" | "binary" | "custom", HTTPBodyEncoder>
  >
>;
