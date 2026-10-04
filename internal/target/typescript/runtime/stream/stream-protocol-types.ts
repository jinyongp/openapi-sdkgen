/** Bounded, cancellable byte reader supplied to custom stream protocols. */
export interface StreamReader {
  /** Reads at most `maxBytes`, which must not exceed the configured frame limit. */
  read(maxBytes: number): Promise<Uint8Array | null>;
  /** Cancels the source body and releases its reader lock. */
  cancel(reason?: unknown): Promise<void>;
}

/** Context shared by stream protocols and adapters. */
export interface StreamContext {
  readonly contentType: string;
  readonly maxFrameBytes: number;
  readonly signal?: AbortSignal | undefined;
}

/** Byte framing for one sequential media protocol. */
export interface StreamProtocol<Frame> {
  decode(reader: StreamReader, context: StreamContext): AsyncIterable<Frame>;
  encode(
    frames: AsyncIterable<Frame>,
    context: StreamContext,
  ): ReadableStream<Uint8Array> | Promise<ReadableStream<Uint8Array>>;
}

/** Application-level transform layered over a stream protocol. */
export interface StreamAdapter<Frame, Item> {
  decode(frames: AsyncIterable<Frame>, context: StreamContext): AsyncIterable<Item>;
  encode(items: AsyncIterable<Item>, context: StreamContext): AsyncIterable<Frame>;
}

/** Optional protocol and application adapter for one sequential media type. */
export interface StreamCodec<Frame = unknown, Item = unknown> {
  readonly protocol?: StreamProtocol<Frame>;
  readonly adapter?: StreamAdapter<Frame, Item>;
}

/** Compiler-owned sequential framing identity carried into generated runtime metadata. */
export type StreamFraming =
  | "line-delimited-json"
  | "json-sequence"
  | "sse"
  | "multipart"
  | "custom";
