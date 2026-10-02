import type {
  StreamCodec,
  StreamContext,
  StreamFraming,
  StreamProtocol,
  StreamReader,
} from "./wire-engine.js";
import type { ServerSentEvent } from "./request.js";
import type { Mutable } from "./runtime-support.js";

/** Internal response-stream decoding options shared by HTTP framing implementations. */
export interface StreamDecodeOptions {
  readonly contentType: string;
  readonly streamFraming: StreamFraming | undefined;
  readonly maxFrameBytes: number;
  readonly streamCodec: StreamCodec | undefined;
  readonly signal: AbortSignal | undefined;
  readonly multipartFrames?: () => AsyncIterable<unknown>;
}

/** Decodes one response body through built-in or custom stream framing and adapters. */
export async function* decodeResponseStreamItems(
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
  yield* decodeStreamItems(body, options);
}

async function* decodeCustomStreamProtocol(
  body: ReadableStream<Uint8Array>,
  protocol: StreamProtocol<unknown>,
  context: StreamContext,
): AsyncIterable<unknown> {
  const reader: StreamReader = createMediaStreamReader(body, context.maxFrameBytes, context.signal);
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
      pending = pending.slice(result.length);
      return result;
    },
    cancel,
  };
}

async function* decodeStreamItems(
  body: ReadableStream<Uint8Array>,
  options: StreamDecodeOptions,
): AsyncIterable<unknown> {
  const { contentType, streamFraming, maxFrameBytes, signal }: StreamDecodeOptions = options;
  if (streamFraming === "sse") {
    yield* decodeSSEStreamItems(body, maxFrameBytes, signal);
    return;
  }
  if (streamFraming !== "line-delimited-json" && streamFraming !== "json-sequence")
    throw new TypeError(`missing stream protocol for ${contentType}`);
  const decoder: TextDecoder = new TextDecoder();
  const encoder: TextEncoder = new TextEncoder();
  const assertFrameBytes: (source: string) => void = (source: string): void => {
    if (encoder.encode(source).byteLength > maxFrameBytes)
      throw new TypeError(`stream frame exceeds ${maxFrameBytes} bytes`);
  };
  let pending: string = "";
  const reader: ReadableStreamDefaultReader<Uint8Array<ArrayBufferLike>> = body.getReader();
  try {
    while (true) {
      const { done, value }: ReadableStreamReadResult<Uint8Array<ArrayBufferLike>> =
        await awaitAbortable(reader.read(), signal);
      pending += decoder.decode(value, { stream: !done });
      if (streamFraming === "json-sequence") {
        const records: string[] = pending.split("\u001e");
        pending = records.pop() ?? "";
        for (const record of records) {
          assertFrameBytes(record);
          if (record.trim() !== "") yield parseStreamJSON(record.trim());
        }
      } else {
        let newline: number;
        while ((newline = pending.indexOf("\n")) >= 0) {
          const rawLine: string = pending.slice(0, newline);
          pending = pending.slice(newline + 1);
          assertFrameBytes(rawLine);
          const line: string = rawLine.replace(/\r$/, "");
          if (line.trim() !== "") yield parseStreamJSON(line);
        }
      }
      assertFrameBytes(pending);
      if (done) break;
    }
    if (pending.trim() !== "") {
      assertFrameBytes(pending);
      yield parseStreamJSON(pending.trim().replace(/^\u001e/, ""));
    }
  } finally {
    try {
      await reader.cancel();
    } finally {
      reader.releaseLock();
    }
  }
}

async function* decodeSSEStreamItems(
  body: ReadableStream<Uint8Array>,
  maxFrameBytes: number,
  signal?: AbortSignal,
): AsyncIterable<ServerSentEvent> {
  const decoder: TextDecoder = new TextDecoder("utf-8", { ignoreBOM: true });
  const encoder: TextEncoder = new TextEncoder();
  const reader: ReadableStreamDefaultReader<Uint8Array<ArrayBufferLike>> = body.getReader();
  let pending: string[] = [];
  let pendingBytes: number = 0;
  let afterCR: boolean = false;
  let countLF: boolean = false;
  let frameBytes: number = 0;
  let firstText: boolean = true;
  let data: string = "";
  let hasData: boolean = false;
  let event: string | undefined;
  let lastEventID: string | undefined;
  let retry: number | undefined;

  const assertFrameBytes: (byteLength: number) => void = (byteLength: number): void => {
    if (byteLength > maxFrameBytes)
      throw new TypeError(`stream frame exceeds ${maxFrameBytes} bytes`);
  };
  const resetEvent: () => void = (): void => {
    data = "";
    hasData = false;
    event = undefined;
    retry = undefined;
  };
  const dispatchEvent: () => ServerSentEvent | undefined = (): ServerSentEvent | undefined => {
    if (!hasData) {
      resetEvent();
      return undefined;
    }
    const item: Mutable<ServerSentEvent> = {
      data: data.slice(0, -1),
    };
    if (event !== undefined) item.event = event;
    if (lastEventID !== undefined) item.id = lastEventID;
    if (retry !== undefined) item.retry = retry;
    resetEvent();
    return item;
  };
  const processLine: (line: string) => void = (line: string): void => {
    if (line.startsWith(":")) return;
    const separator: number = line.indexOf(":");
    const field: string = separator < 0 ? line : line.slice(0, separator);
    let value: string = separator < 0 ? "" : line.slice(separator + 1);
    if (value.startsWith(" ")) value = value.slice(1);
    switch (field) {
      case "data":
        data += value + "\n";
        hasData = true;
        break;
      case "event":
        event = value;
        break;
      case "id":
        if (!value.includes("\u0000")) lastEventID = value;
        break;
      case "retry":
        if (/^[0-9]+$/.test(value)) {
          const parsed: number = Number(value);
          if (Number.isFinite(parsed)) retry = parsed;
        }
        break;
    }
  };

  try {
    while (true) {
      const { done, value }: ReadableStreamReadResult<Uint8Array<ArrayBufferLike>> =
        await awaitAbortable(reader.read(), signal);
      let decoded: string = decoder.decode(value, { stream: !done });
      if (firstText && decoded !== "") {
        if (decoded.startsWith("\uFEFF")) decoded = decoded.slice(1);
        firstText = false;
      }
      let start: number = 0;
      for (let index: number = 0; index < decoded.length; index++) {
        const code: number = decoded.charCodeAt(index);
        if (afterCR) {
          afterCR = false;
          if (code === 10) {
            if (countLF) {
              frameBytes++;
              assertFrameBytes(frameBytes);
            }
            start = index + 1;
            continue;
          }
        }
        if (code !== 10 && code !== 13) continue;
        const part: string = decoded.slice(start, index);
        pending.push(part);
        pendingBytes += encoder.encode(part).byteLength;
        const line: string = pending.join("");
        const lineBytes: number = pendingBytes + 1;
        pending = [];
        pendingBytes = 0;
        start = index + 1;
        afterCR = code === 13;
        countLF = line !== "";
        if (line === "") {
          const item: ServerSentEvent | undefined = dispatchEvent();
          frameBytes = 0;
          if (item !== undefined) yield item;
          continue;
        }
        frameBytes += lineBytes;
        assertFrameBytes(frameBytes);
        processLine(line);
      }
      if (start < decoded.length) {
        const part: string = decoded.slice(start);
        pending.push(part);
        pendingBytes += encoder.encode(part).byteLength;
      }
      assertFrameBytes(frameBytes + pendingBytes);
      if (done) break;
    }
  } finally {
    try {
      await reader.cancel();
    } finally {
      reader.releaseLock();
    }
  }
}

/** Parses one JSON stream item and normalizes invalid input as a TypeError. */
export function parseStreamJSON(value: string): unknown {
  try {
    return JSON.parse(value);
  } catch {
    throw new TypeError("stream item is not valid JSON");
  }
}

function awaitAbortable<Value>(
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
