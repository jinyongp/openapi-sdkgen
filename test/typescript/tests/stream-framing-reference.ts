import type { TestedFraming } from "./stream-framing-helper.js";

/** Independent linear reference for known JSON fixtures, never a product codec. */
export async function referenceFramingItems(
  body: ReadableStream<Uint8Array>,
  framing: TestedFraming,
): Promise<unknown[]> {
  const reader: ReadableStreamDefaultReader<Uint8Array> = body.getReader();
  const chunks: Uint8Array[] = [];
  let size: number = 0;
  try {
    while (true) {
      const next: ReadableStreamReadResult<Uint8Array> = await reader.read();
      if (next.done) break;
      chunks.push(next.value);
      size += next.value.length;
    }
  } finally {
    await reader.cancel();
    reader.releaseLock();
  }
  // The reference gathers the one-frame benchmark fixture and parses it once.
  // It provides an independent linear cost; product tests separately exercise
  // incremental delivery, limits, cancellation and arbitrary chunk boundaries.
  const bytes: Uint8Array<ArrayBuffer> = new Uint8Array(size);
  let offset: number = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.length;
  }
  const source: string = new TextDecoder().decode(bytes);
  if (framing === "multipart")
    return source
      .split("--ababab")
      .slice(1, -1)
      .map((part: string): unknown => {
        const start: number = part.indexOf("\r\n\r\n");
        if (start < 0) throw new Error("invalid reference fixture headers");
        return JSON.parse(part.slice(start + 4).trim());
      });
  return source
    .split(framing === "json-sequence" ? "\u001e" : "\n")
    .filter((record: string): boolean => record.trim() !== "")
    .map((record: string): unknown => JSON.parse(record));
}
