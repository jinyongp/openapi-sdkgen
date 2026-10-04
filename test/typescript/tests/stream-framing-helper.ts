import { vi } from "vitest";
import type { Mock } from "vitest";
import { createAdvancedHTTPServices } from "../../../internal/target/typescript/runtime/compatibility/http-advanced.js";
import { jsonWireCodec } from "../../../internal/target/typescript/runtime/compatibility/wire-engine.js";
import type { StreamFraming } from "../../../internal/target/typescript/runtime/stream/stream-protocol-types.js";
import { decodeInboundBody } from "../../../internal/target/typescript/runtime/server/runtime.js";

export type TestedFraming = Extract<
  StreamFraming,
  "line-delimited-json" | "json-sequence" | "multipart"
>;
export interface ChunkedBody {
  readonly body: ReadableStream<Uint8Array>;
  readonly cancel: Mock<(reason?: unknown) => void>;
}

const services: ReturnType<typeof createAdvancedHTTPServices> = createAdvancedHTTPServices(
  (): never => {
    throw new Error("framing must not execute a request");
  },
  jsonWireCodec,
);

export function framingContentType(framing: TestedFraming): string {
  return framing === "multipart"
    ? "multipart/mixed; boundary=ababab"
    : framing === "json-sequence"
      ? "application/json-seq"
      : "application/x-ndjson";
}

export function framingSource(framing: TestedFraming, items: readonly unknown[]): string {
  const values: string[] = items.map((item: unknown): string => JSON.stringify(item));
  if (framing === "multipart")
    return (
      "--ababab\r\nContent-Type: application/json\r\n\r\n" +
      values.join("\r\n--ababab\r\nContent-Type: application/json\r\n\r\n") +
      "\r\n--ababab--\r\n"
    );
  return values
    .map((value: string): string => (framing === "json-sequence" ? "\u001e" : "") + value + "\r\n")
    .join("");
}

export function chunkedFramingBody(bytes: Uint8Array, size: number): ChunkedBody {
  let offset: number = 0;
  const cancel: Mock<(reason?: unknown) => void> = vi.fn();
  const body: ReadableStream<Uint8Array> = new ReadableStream<Uint8Array>(
    {
      pull(controller: ReadableStreamDefaultController<Uint8Array>): void {
        if (offset >= bytes.length) {
          controller.close();
          return;
        }
        controller.enqueue(bytes.subarray(offset, offset + size));
        offset += size;
      },
      cancel,
    },
    { highWaterMark: 0 },
  );
  return { body, cancel };
}

export async function framingItems(
  side: "client" | "server",
  framing: TestedFraming,
  body: ReadableStream<Uint8Array>,
  maximum: number,
): Promise<AsyncIterable<unknown>> {
  const contentType: string = framingContentType(framing);
  if (side === "client")
    return services.decodeResponseStreamItems(body, {
      contentType,
      streamFraming: framing,
      itemSchema: {},
      prefixSchemas: undefined,
      schemas: {},
      codecs: new Map(),
      prefixEncoding: undefined,
      itemEncoding: undefined,
      maxFrameBytes: maximum,
      streamCodec: undefined,
      signal: undefined,
    });
  const request: Request = new Request("https://host.test/stream", {
    method: "POST",
    headers: { "content-type": contentType },
    body,
    duplex: "half",
  } as RequestInit);
  return (await decodeInboundBody(request, {
    required: true,
    schemas: {},
    wireSchema: {},
    wireSchemas: {},
    maxStreamFrameBytes: maximum,
    plans: [
      {
        contentType: contentType.split(";", 1)[0]!,
        stream: true,
        binary: false,
        streamFraming: framing,
        schema: {},
      },
    ],
  })) as AsyncIterable<unknown>;
}

export async function collectFraming(items: AsyncIterable<unknown>): Promise<unknown[]> {
  const values: unknown[] = [];
  for await (const item of items) values.push(item);
  return values;
}
