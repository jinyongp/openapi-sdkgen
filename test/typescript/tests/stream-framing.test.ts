import { describe, expect, it, vi } from "vitest";
import type { MockInstance } from "vitest";
import { InboundRequestError } from "../../../internal/target/typescript/runtime/server/runtime.js";
import {
  chunkedFramingBody,
  collectFraming,
  framingItems,
  framingSource,
  type ChunkedBody,
  type TestedFraming,
} from "./stream-framing-helper.js";
const encoder: TextEncoder = new TextEncoder();
const framings: readonly TestedFraming[] = ["line-delimited-json", "json-sequence", "multipart"];

describe("incremental client and inbound framing", (): void => {
  for (const side of ["client", "server"] as const)
    for (const framing of framings) {
      it.each([1, 2, 3, 64, 65536])(
        `${side} ${framing} preserves UTF-8 and delimiters in %s-byte chunks`,
        async (size: number): Promise<void> => {
          const items: readonly unknown[] = [
            { data: "한글😀\r\ntext" },
            { data: "\r\n--ababaX" },
            { data: "last" },
          ];
          const { body, cancel }: ChunkedBody = chunkedFramingBody(
            encoder.encode(framingSource(framing, items)),
            size,
          );
          expect(await collectFraming(await framingItems(side, framing, body, 1024))).toEqual(
            items,
          );
          expect(body.locked).toBe(false);
          if (framing === "multipart") expect(cancel).toHaveBeenCalledOnce();
        },
      );

      it(`${side} ${framing} rejects UTF-8 byte overflow and releases the reader`, async (): Promise<void> => {
        const { body }: ChunkedBody = chunkedFramingBody(
          encoder.encode(framingSource(framing, [{ data: "😀".repeat(40) }])),
          3,
        );
        try {
          await collectFraming(await framingItems(side, framing, body, 100));
          throw new Error("oversized frame accepted");
        } catch (error: unknown) {
          if (side === "server") {
            expect(error).toBeInstanceOf(InboundRequestError);
            expect((error as InboundRequestError).response.status).toBe(400);
          } else expect(error).toBeInstanceOf(TypeError);
        }
        expect(body.locked).toBe(false);
      });

      it(`${side} ${framing} cancels when the consumer stops`, async (): Promise<void> => {
        const { body, cancel }: ChunkedBody = chunkedFramingBody(
          encoder.encode(framingSource(framing, [1, 2, 3])),
          1,
        );
        for await (const item of await framingItems(side, framing, body, 1024)) {
          expect(item).toBe(1);
          break;
        }
        expect(cancel).toHaveBeenCalledOnce();
        expect(body.locked).toBe(false);
      });

      it.each([64, 128, 256])(
        `${side} ${framing} does linear encoding and copying for a %s KiB frame`,
        async (kib: number): Promise<void> => {
          const item: unknown = { data: "x".repeat(kib * 1024) };
          const bytes: Uint8Array<ArrayBuffer> = encoder.encode(framingSource(framing, [item]));
          const { body }: ChunkedBody = chunkedFramingBody(bytes, 64);
          let encoded: number = 0,
            copied: number = 0;
          const encode: typeof TextEncoder.prototype.encode = TextEncoder.prototype.encode;
          const set: typeof Uint8Array.prototype.set = Uint8Array.prototype.set;
          const encodeSpy: MockInstance<typeof encode> = vi
            .spyOn(TextEncoder.prototype, "encode")
            .mockImplementation(function (
              this: TextEncoder,
              source?: string,
            ): Uint8Array<ArrayBuffer> {
              encoded += source?.length ?? 0;
              return encode.call(this, source);
            });
          const setSpy: MockInstance<typeof set> = vi
            .spyOn(Uint8Array.prototype, "set")
            .mockImplementation(function (
              this: Uint8Array,
              source: ArrayLike<number>,
              offset?: number,
            ): void {
              copied += source.length;
              set.call(this, source, offset);
            });
          try {
            expect(
              await collectFraming(await framingItems(side, framing, body, bytes.length)),
            ).toEqual([item]);
          } finally {
            encodeSpy.mockRestore();
            setSpy.mockRestore();
          }
          expect(encoded).toBeLessThanOrEqual(bytes.length * 2);
          expect(copied).toBeLessThanOrEqual(bytes.length * 2);
        },
      );
    }

  for (const side of ["client", "server"] as const)
    for (const framing of ["line-delimited-json", "json-sequence"] as const) {
      it(`${side} ${framing} handles a leading BOM, empty records and EOF`, async (): Promise<void> => {
        const source: string =
          framing === "json-sequence" ? "\uFEFF\u001e\u001e1\r\n\u001e2" : "\uFEFF\r\n1\r\n2";
        const { body }: ChunkedBody = chunkedFramingBody(encoder.encode(source), 1);
        expect(await collectFraming(await framingItems(side, framing, body, 10))).toEqual([1, 2]);
      });
    }
});
