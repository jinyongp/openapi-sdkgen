import type { ServerSentEvent } from "./stream-types.js";
import type { Mutable } from "../shared/runtime-support.js";
import { awaitAbortable } from "./stream-abort.js";
import type { StreamDecodeOptions } from "./stream-types.js";
export function decodeSSEStreamFrames(
  body: ReadableStream<Uint8Array>,
  options: StreamDecodeOptions,
): AsyncIterable<ServerSentEvent> {
  return decodeSSEStreamItems(body, options.maxFrameBytes, options.signal);
}
export async function* decodeSSEStreamItems(
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
