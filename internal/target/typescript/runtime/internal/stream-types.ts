import type { StreamCodec, StreamFraming } from "./wire-types.js";
export interface StreamDecodeOptions {
  readonly contentType: string;
  readonly streamFraming: StreamFraming | undefined;
  readonly maxFrameBytes: number;
  readonly streamCodec: StreamCodec | undefined;
  readonly signal: AbortSignal | undefined;
  readonly multipartFrames?: () => AsyncIterable<unknown>;
}

/** Decodes one response body through built-in or custom stream framing and adapters. */
export type StreamFrameDecoder = (
  body: ReadableStream<Uint8Array>,
  options: StreamDecodeOptions,
) => AsyncIterable<unknown>;
export type StreamFrameDecoders = Readonly<
  Partial<Record<Exclude<StreamFraming, "multipart" | "custom">, StreamFrameDecoder>>
>;
