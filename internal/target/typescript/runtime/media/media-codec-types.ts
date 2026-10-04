import type { StreamContext } from "../stream/stream-protocol-types.js";

type MediaCodecContext = Pick<StreamContext, "contentType">;

/** Host-owned encoder/decoder for one complete declared media value. */
export interface MediaCodec<Value> {
  readonly encode?: (value: Value, context: MediaCodecContext) => BodyInit | Promise<BodyInit>;
  readonly decode?: (response: Response, context: MediaCodecContext) => Value | Promise<Value>;
  /** Serializes one Parameter Object `content` value into its required string representation. */
  readonly encodeParameter?: (value: Value, context: MediaCodecContext) => string | Promise<string>;
  /** Decodes a Parameter Object or response Header Object `content` string. */
  readonly decodeParameter?: (value: string, context: MediaCodecContext) => Value | Promise<Value>;
  /** Decodes one non-streaming inbound server request for a declared custom media type. */
  readonly decodeInbound?: (request: Request, context: MediaCodecContext) => Value | Promise<Value>;
}
