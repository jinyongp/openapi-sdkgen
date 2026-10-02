import { decodeResponseStreamItems } from "../../../internal/target/typescript/runtime/internal/streaming.js";
export async function fragmentedSSE(
  size: number,
  chunkSize: number,
  source = `data: ${"x".repeat(size)}\r\n\r\n`,
): Promise<unknown[]> {
  const bytes = new TextEncoder().encode(source);
  let offset = 0;
  const body = new ReadableStream<Uint8Array>({
    pull(controller) {
      if (offset === bytes.length) {
        controller.close();
        return;
      }
      const end = Math.min(bytes.length, offset + chunkSize);
      controller.enqueue(bytes.slice(offset, end));
      offset = end;
    },
  });
  const result: unknown[] = [];
  for await (const value of decodeResponseStreamItems(body, {
    contentType: "text/event-stream",
    streamFraming: "sse",
    maxFrameBytes: size + 32,
    streamCodec: undefined,
    signal: undefined,
  }))
    result.push(value);
  return result;
}
