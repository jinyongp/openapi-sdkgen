import { describe, expect, it, vi } from "vitest";
import { createAdvancedHTTPServices } from "../../../internal/target/typescript/runtime/compatibility/http-advanced.js";
import { jsonWireCodec } from "../../../internal/target/typescript/runtime/compatibility/wire-engine.js";
import type { HTTPStreamDecodeOptions } from "../../../internal/target/typescript/runtime/media/media-service-types.js";

const services = createAdvancedHTTPServices(() => {
  throw new Error("multipart codecs must not execute an HTTP request");
}, jsonWireCodec);
const encoder = new TextEncoder();
const options: HTTPStreamDecodeOptions = {
  contentType: 'multipart/mixed; boundary="part"',
  streamFraming: "multipart",
  itemSchema: {},
  prefixSchemas: undefined,
  schemas: {},
  codecs: new Map(),
  prefixEncoding: undefined,
  itemEncoding: undefined,
  maxFrameBytes: 1024,
  streamCodec: undefined,
  signal: undefined,
};

function chunked(bytes: Uint8Array, size: number) {
  let offset = 0;
  const cancel = vi.fn();
  const body = new ReadableStream<Uint8Array>(
    {
      pull(controller) {
        if (offset === bytes.length) {
          controller.close();
          return;
        }
        controller.enqueue(bytes.slice(offset, offset + size));
        offset = Math.min(offset + size, bytes.length);
      },
      cancel,
    },
    { highWaterMark: 0 },
  );
  return { body, cancel };
}
async function collect(values: AsyncIterable<unknown>) {
  const result: unknown[] = [];
  for await (const value of values) result.push(value);
  return result;
}
const part = (headers: string, value: string) =>
  `--part\r\n${headers}\r\n\r\n${value}\r\n--part--\r\n`;

describe("multipart stream boundaries", () => {
  it.each([1, 2, 7, 128])(
    "preserves nested, XML, custom and binary parts in %s-byte chunks",
    async (size) => {
      const bytes = new Uint8Array(
        await new Blob([
          "preamble\r\n--part\r\nContent-Type: application/xml\r\n\r\n<item><id>7</id></item>\r\n",
          "--part\r\nContent-Type: multipart/mixed; boundary=inner\r\n\r\n",
          "--inner\r\nContent-Type: text/plain\r\n\r\n한글🙂\r\n--inner--\r\n",
          "\r\n--part\r\nContent-Type: application/x-upper\r\n\r\ncustom\r\n",
          "--part\r\nContent-Type: application/octet-stream\r\n\r\n",
          new Uint8Array([0, 255, 10, 65]),
          "\r\n--part--\r\nepilogue",
        ]).arrayBuffer(),
      );
      const { body, cancel } = chunked(bytes, size);
      const decode = vi.fn(async (response: Response) => (await response.text()).toUpperCase());
      const result = await collect(
        services.decodeResponseStreamItems(body, {
          ...options,
          prefixSchemas: [
            {
              types: ["object"],
              xml: { name: "item" },
              properties: { id: { property: "id", schema: { types: ["integer"] } } },
            },
            { types: ["array"], items: { types: ["string"] } },
            { types: ["string"] },
            { contentEncoding: "binary" },
          ],
          codecs: new Map([["application/x-upper", { decode }]]),
        }),
      );
      expect(result.slice(0, 3)).toEqual([{ id: 7 }, ["한글🙂"], "CUSTOM"]);
      expect(new Uint8Array(result[3] as ArrayBuffer)).toEqual(new Uint8Array([0, 255, 10, 65]));
      expect(decode).toHaveBeenCalledOnce();
      expect(body.locked).toBe(false);
      expect(cancel).toHaveBeenCalledOnce();
    },
  );

  it.each([
    { name: "missing boundary", wire: "body", contentType: "multipart/mixed" },
    { name: "malformed opening", wire: "--part!!\r\n" },
    {
      name: "malformed separator",
      wire: "--part\r\nContent-Type: text/plain\r\n\r\nhello\r\n--part!!\r\n",
    },
    { name: "missing closing delimiter", wire: "--part\r\nContent-Type: text/plain\r\n\r\nhello" },
    {
      name: "missing header terminator",
      wire: "--part\r\nContent-Type: text/plain\r\nhello\r\n--part--\r\n",
    },
    { name: "malformed header", wire: part("not-a-header", "hello") },
    { name: "oversized frame", wire: part("Content-Type: text/plain", "12345"), maxFrameBytes: 4 },
    { name: "oversized headers", wire: part(`X-Large: ${"x".repeat(8192)}`, "a") },
    { name: "oversized preamble", wire: "x".repeat(8300), maxFrameBytes: 4 },
    { name: "missing part codec", wire: part("Content-Type: application/x-missing", "value") },
  ])("rejects $name and releases its reader", async ({ wire, contentType, maxFrameBytes }) => {
    const { body } = chunked(encoder.encode(wire), 23);
    await expect(
      collect(
        services.decodeResponseStreamItems(body, {
          ...options,
          contentType: contentType ?? options.contentType,
          maxFrameBytes: maxFrameBytes ?? options.maxFrameBytes,
        }),
      ),
    ).rejects.toThrow(TypeError);
    expect(body.locked).toBe(false);
  });

  it("accepts an empty multipart stream without decoding a phantom part", async () => {
    const { body } = chunked(encoder.encode("--part--\r\n"), 1);
    await expect(collect(services.decodeResponseStreamItems(body, options))).resolves.toEqual([]);
    expect(body.locked).toBe(false);
  });

  it("cancels the source when the consumer stops after the first part", async () => {
    const wire =
      "--part\r\nContent-Type: text/plain\r\n\r\nfirst\r\n" +
      "--part\r\nContent-Type: text/plain\r\n\r\nsecond\r\n--part--\r\n";
    const { body, cancel } = chunked(encoder.encode(wire), 1);
    for await (const value of services.decodeResponseStreamItems(body, options)) {
      expect(value).toBe("first");
      break;
    }
    expect(cancel).toHaveBeenCalledOnce();
    expect(body.locked).toBe(false);
  });

  it("cancels an awaiting source when its signal aborts", async () => {
    const cancel = vi.fn();
    const body = new ReadableStream<Uint8Array>({ cancel });
    const controller = new AbortController();
    const cause = new Error("consumer aborted");
    const pending = collect(
      services.decodeResponseStreamItems(body, { ...options, signal: controller.signal }),
    );
    const rejected = expect(pending).rejects.toBe(cause);
    controller.abort(cause);
    await rejected;
    expect(cancel).toHaveBeenCalledOnce();
    expect(body.locked).toBe(false);
  });
});
