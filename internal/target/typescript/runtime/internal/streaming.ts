import type {
  StreamCodec,
  StreamContext,
  StreamFraming,
  StreamProtocol,
  StreamReader,
} from "./wire-engine.js";
import type { ServerSentEvent } from "./request.js";

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
  const frames =
    options.streamCodec?.protocol === undefined
      ? decodeBuiltInStreamFrames(body, options)
      : decodeCustomStreamProtocol(body, options.streamCodec.protocol, context);
  const items = decodeStreamApplicationItems(frames, options, context);
  const iterator = items[Symbol.asyncIterator]();
  try {
    while (true) {
      const next = await awaitAbortable(Promise.resolve(iterator.next()), options.signal);
      if (next.done) return;
      yield next.value;
    }
  } finally {
    if (iterator.return !== undefined) {
      const close = Promise.resolve(iterator.return());
      if (options.signal?.aborted) void close.catch(() => undefined);
      else await close.catch(() => undefined);
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
  const reader = createMediaStreamReader(body, context.maxFrameBytes, context.signal);
  const frames = protocol.decode(reader, context);
  const iterator = frames[Symbol.asyncIterator]();
  try {
    while (true) {
      const next = await awaitAbortable(Promise.resolve(iterator.next()), context.signal);
      if (next.done) return;
      yield next.value;
    }
  } finally {
    await reader.cancel(context.signal?.reason);
    if (iterator.return !== undefined) {
      const close = Promise.resolve(iterator.return());
      if (context.signal?.aborted) void close.catch(() => undefined);
      else await close.catch(() => undefined);
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
  const reader = body.getReader();
  let pending: Uint8Array<ArrayBufferLike> = new Uint8Array();
  let done = false;
  let released = false;
  const cancel = async (reason?: unknown): Promise<void> => {
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
        const next = await awaitAbortable(reader.read(), signal);
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
      const result = pending.slice(0, maxBytes);
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
  const { contentType, streamFraming, maxFrameBytes, signal } = options;
  if (streamFraming === "sse") {
    yield* decodeSSEStreamItems(body, maxFrameBytes, signal);
    return;
  }
  if (streamFraming !== "line-delimited-json" && streamFraming !== "json-sequence")
    throw new TypeError(`missing stream protocol for ${contentType}`);
  const decoder = new TextDecoder();
  const encoder = new TextEncoder();
  const assertFrameBytes = (source: string): void => {
    if (encoder.encode(source).byteLength > maxFrameBytes)
      throw new TypeError(`stream frame exceeds ${maxFrameBytes} bytes`);
  };
  let pending = "";
  const reader = body.getReader();
  try {
    while (true) {
      const { done, value } = await awaitAbortable(reader.read(), signal);
      pending += decoder.decode(value, { stream: !done });
      if (streamFraming === "json-sequence") {
        const records = pending.split("\u001e");
        pending = records.pop() ?? "";
        for (const record of records) {
          assertFrameBytes(record);
          if (record.trim() !== "") yield parseStreamJSON(record.trim());
        }
      } else {
        let newline: number;
        while ((newline = pending.indexOf("\n")) >= 0) {
          const rawLine = pending.slice(0, newline);
          pending = pending.slice(newline + 1);
          assertFrameBytes(rawLine);
          const line = rawLine.replace(/\r$/, "");
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
  const decoder = new TextDecoder("utf-8", { ignoreBOM: true });
  const encoder = new TextEncoder();
  const reader = body.getReader();
  let pending: string[] = [];
  let pendingBytes = 0;
  let afterCR = false;
  let countLF = false;
  let frameBytes = 0;
  let firstText = true;
  let data = "";
  let hasData = false;
  let event: string | undefined;
  let lastEventID: string | undefined;
  let retry: number | undefined;

  const assertFrameBytes = (byteLength: number): void => {
    if (byteLength > maxFrameBytes)
      throw new TypeError(`stream frame exceeds ${maxFrameBytes} bytes`);
  };
  const resetEvent = (): void => {
    data = "";
    hasData = false;
    event = undefined;
    retry = undefined;
  };
  const dispatchEvent = (): ServerSentEvent | undefined => {
    if (!hasData) {
      resetEvent();
      return undefined;
    }
    const item: { data: string; event?: string; id?: string; retry?: number } = {
      data: data.slice(0, -1),
    };
    if (event !== undefined) item.event = event;
    if (lastEventID !== undefined) item.id = lastEventID;
    if (retry !== undefined) item.retry = retry;
    resetEvent();
    return item;
  };
  const processLine = (line: string): void => {
    if (line.startsWith(":")) return;
    const separator = line.indexOf(":");
    const field = separator < 0 ? line : line.slice(0, separator);
    let value = separator < 0 ? "" : line.slice(separator + 1);
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
          const parsed = Number(value);
          if (Number.isFinite(parsed)) retry = parsed;
        }
        break;
    }
  };

  try {
    while (true) {
      const { done, value } = await awaitAbortable(reader.read(), signal);
      let decoded = decoder.decode(value, { stream: !done });
      if (firstText && decoded !== "") {
        if (decoded.startsWith("\uFEFF")) decoded = decoded.slice(1);
        firstText = false;
      }
      let start = 0;
      for (let index = 0; index < decoded.length; index++) {
        const code = decoded.charCodeAt(index);
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
        const part = decoded.slice(start, index);
        pending.push(part);
        pendingBytes += encoder.encode(part).byteLength;
        const line = pending.join("");
        const lineBytes = pendingBytes + 1;
        pending = [];
        pendingBytes = 0;
        start = index + 1;
        afterCR = code === 13;
        countLF = line !== "";
        if (line === "") {
          const item = dispatchEvent();
          frameBytes = 0;
          if (item !== undefined) yield item;
          continue;
        }
        frameBytes += lineBytes;
        assertFrameBytes(frameBytes);
        processLine(line);
      }
      if (start < decoded.length) {
        const part = decoded.slice(start);
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
    void value.catch(() => undefined);
    return Promise.reject(signal.reason);
  }
  return new Promise((resolve, reject) => {
    const onAbort = (): void => reject(signal.reason);
    signal.addEventListener("abort", onAbort, { once: true });
    value.then(
      (result) => {
        signal.removeEventListener("abort", onAbort);
        resolve(result);
      },
      (cause) => {
        signal.removeEventListener("abort", onAbort);
        reject(cause);
      },
    );
  });
}
