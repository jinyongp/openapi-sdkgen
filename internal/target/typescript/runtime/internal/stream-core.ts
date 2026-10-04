import type { StreamContext, StreamProtocol, StreamReader } from "./wire-types.js";
import type {
  StreamDecodeOptions,
  StreamFrameDecoder,
  StreamFrameDecoders,
} from "./stream-types.js";
import { awaitAbortable } from "./stream-abort.js";
export function createStreamDecoder(protocols: StreamFrameDecoders): StreamFrameDecoder {
  async function* decodeResponseStreamItems(
    body: ReadableStream<Uint8Array>,
    options: StreamDecodeOptions,
  ): AsyncIterable<unknown> {
    const context: StreamContext = {
      contentType: options.contentType,
      maxFrameBytes: options.maxFrameBytes,
      ...(options.signal === undefined ? {} : { signal: options.signal }),
    };
    const frames: AsyncIterable<unknown> =
      options.streamCodec?.protocol === undefined
        ? decodeBuiltInStreamFrames(body, options)
        : decodeCustomStreamProtocol(body, options.streamCodec.protocol, context);
    const items: AsyncIterable<unknown> = decodeStreamApplicationItems(frames, options, context);
    const iterator: AsyncIterator<unknown, unknown, unknown> = items[Symbol.asyncIterator]();
    try {
      while (true) {
        const next: IteratorResult<unknown, unknown> = await awaitAbortable(
          Promise.resolve(iterator.next()),
          options.signal,
        );
        if (next.done) return;
        yield next.value;
      }
    } finally {
      if (iterator.return !== undefined) {
        const close: Promise<IteratorResult<unknown, unknown>> = Promise.resolve(iterator.return());
        if (options.signal?.aborted) void close.catch((): undefined => undefined);
        else await close.catch((): undefined => undefined);
      }
    }
  }

  function decodeStreamApplicationItems(
    frames: AsyncIterable<unknown>,
    options: StreamDecodeOptions,
    context: StreamContext,
  ): AsyncIterable<unknown> {
    if (options.streamCodec?.adapter !== undefined)
      return options.streamCodec.adapter.decode(frames, context);
    return frames;
  }

  async function* decodeBuiltInStreamFrames(
    body: ReadableStream<Uint8Array>,
    options: StreamDecodeOptions,
  ): AsyncIterable<unknown> {
    if (options.streamFraming === undefined || options.streamFraming === "custom")
      throw new TypeError(`missing stream protocol for ${options.contentType}`);
    if (options.streamFraming === "multipart") {
      if (options.multipartFrames === undefined)
        throw new TypeError(`missing multipart stream decoder for ${options.contentType}`);
      yield* options.multipartFrames();
      return;
    }
    const decoder: StreamFrameDecoder | undefined = protocols[options.streamFraming];
    if (decoder === undefined)
      throw new TypeError(`missing stream protocol for ${options.contentType}`);
    yield* decoder(body, options);
  }

  async function* decodeCustomStreamProtocol(
    body: ReadableStream<Uint8Array>,
    protocol: StreamProtocol<unknown>,
    context: StreamContext,
  ): AsyncIterable<unknown> {
    const reader: StreamReader = createMediaStreamReader(
      body,
      context.maxFrameBytes,
      context.signal,
    );
    const frames: AsyncIterable<unknown> = protocol.decode(reader, context);
    const iterator: AsyncIterator<unknown, unknown, unknown> = frames[Symbol.asyncIterator]();
    try {
      while (true) {
        const next: IteratorResult<unknown, unknown> = await awaitAbortable(
          Promise.resolve(iterator.next()),
          context.signal,
        );
        if (next.done) return;
        yield next.value;
      }
    } finally {
      await reader.cancel(context.signal?.reason);
      if (iterator.return !== undefined) {
        const close: Promise<IteratorResult<unknown, unknown>> = Promise.resolve(iterator.return());
        if (context.signal?.aborted) void close.catch((): undefined => undefined);
        else await close.catch((): undefined => undefined);
      }
    }
  }

  function createMediaStreamReader(
    body: ReadableStream<Uint8Array>,
    maxFrameBytes: number,
    signal?: AbortSignal,
  ): StreamReader {
    if (!Number.isSafeInteger(maxFrameBytes) || maxFrameBytes <= 0)
      throw new TypeError("maxStreamFrameBytes must be a positive safe integer");
    const reader: ReadableStreamDefaultReader<Uint8Array<ArrayBufferLike>> = body.getReader();
    let pending: Uint8Array<ArrayBufferLike> = new Uint8Array();
    let done: boolean = false;
    let released: boolean = false;
    const cancel: (reason?: unknown) => Promise<void> = async (reason?: unknown): Promise<void> => {
      if (released) return;
      released = true;
      try {
        await reader.cancel(reason);
      } finally {
        reader.releaseLock();
      }
    };
    return {
      async read(maxBytes: number): Promise<Uint8Array | null> {
        if (!Number.isSafeInteger(maxBytes) || maxBytes <= 0 || maxBytes > maxFrameBytes)
          throw new TypeError(
            `stream read size must be a positive safe integer at most ${maxFrameBytes}`,
          );
        if (released) return null;
        while (pending.length === 0 && !done) {
          const next: ReadableStreamReadResult<Uint8Array<ArrayBufferLike>> = await awaitAbortable(
            reader.read(),
            signal,
          );
          done = next.done;
          if (next.value !== undefined) pending = next.value;
        }
        if (pending.length === 0) {
          if (!released) {
            released = true;
            reader.releaseLock();
          }
          return null;
        }
        const result: Uint8Array<ArrayBuffer> = pending.slice(0, maxBytes);
        pending = pending.subarray(result.length);
        return result;
      },
      cancel,
    };
  }

  return decodeResponseStreamItems;
}
