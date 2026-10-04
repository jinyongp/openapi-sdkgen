import type { StreamCodec, StreamFraming } from "./stream-protocol-types.js";
/** Cancellation and frame limits for one stream decoder invocation. */
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
/** Selected framing algorithms keyed by their canonical protocol name. */
export type StreamFrameDecoders = Readonly<
  Partial<Record<Exclude<StreamFraming, "multipart" | "custom">, StreamFrameDecoder>>
>;
/** One parsed Server-Sent Event using the standard event-stream fields. */
export interface ServerSentEvent {
  readonly event?: string;
  readonly data: string;
  readonly id?: string;
  readonly retry?: number;
}
