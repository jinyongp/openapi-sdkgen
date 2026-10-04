import type {
  StreamProtocolEncodeOptions,
  EncodedStreamRequestBody,
} from "../../media/media-service-types.js";
import type { TextRequestEncoder } from "../../media/http-media-types.js";
export function createTextRequestEncoder(
  encodeItem: (value: unknown) => string,
): TextRequestEncoder {
  function encodeText(
    values: AsyncIterable<unknown>,
    options: StreamProtocolEncodeOptions,
  ): EncodedStreamRequestBody {
    const maxFrameBytes: number = options.context.maxFrameBytes;
    const iterator: AsyncIterator<unknown, unknown, unknown> = values[Symbol.asyncIterator]();
    const encoder: TextEncoder = new TextEncoder();
    const body: ReadableStream<Uint8Array> = new ReadableStream<Uint8Array>(
      {
        async pull(
          controller: ReadableStreamDefaultController<Uint8Array<ArrayBufferLike>>,
        ): Promise<void> {
          try {
            const next: IteratorResult<unknown, unknown> = await iterator.next();
            if (next.done) {
              controller.close();
              return;
            }
            const frame: Uint8Array<ArrayBuffer> = encoder.encode(encodeItem(next.value));
            if (frame.byteLength > maxFrameBytes)
              throw new TypeError(`stream frame exceeds ${maxFrameBytes} bytes`);
            controller.enqueue(frame);
          } catch (cause: unknown) {
            controller.error(cause);
            try {
              await iterator.return?.();
            } catch {
              /* original error wins */
            }
          }
        },
        async cancel(reason: unknown): Promise<void> {
          await iterator.return?.(reason);
        },
      },
      { highWaterMark: 0 },
    );
    return { body, contentType: options.contentType };
  }

  return encodeText;
}
