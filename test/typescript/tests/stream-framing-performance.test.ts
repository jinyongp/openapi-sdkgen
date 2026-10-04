import { describe, expect, it } from "vitest";
import { appendFileSync } from "node:fs";
import {
  chunkedFramingBody,
  collectFraming,
  framingItems,
  framingSource,
} from "./stream-framing-helper.js";
import type { TestedFraming } from "./stream-framing-helper.js";
import { referenceFramingItems } from "./stream-framing-reference.js";

const perfIt: typeof it.skip = process.env.OPENAPI_SDKGEN_STREAM_PERF === "1" ? it : it.skip;
const encoder: TextEncoder = new TextEncoder();
const framings: readonly TestedFraming[] = ["line-delimited-json", "json-sequence", "multipart"];

async function timing(run: () => Promise<unknown>): Promise<number> {
  const started: number = performance.now();
  await run();
  return performance.now() - started;
}
function median(values: readonly number[]): number {
  return [...values].sort((left: number, right: number): number => left - right)[2]!;
}

describe("fragmented frame performance acceptance", (): void => {
  it("cross-checks the independent reference on UTF-8 fixture chunk boundaries", async (): Promise<void> => {
    const items: readonly unknown[] = [{ data: "한글😀\r\ntext" }, { data: "next" }];
    for (const framing of framings)
      for (const size of [1, 2, 3, 64, 65536]) {
        const bytes: Uint8Array<ArrayBuffer> = encoder.encode(framingSource(framing, items));
        expect(await referenceFramingItems(chunkedFramingBody(bytes, size).body, framing)).toEqual(
          items,
        );
      }
  });

  for (const side of ["client", "server"] as const)
    for (const framing of framings) {
      perfIt.each([64, 128, 256])(
        `${side} ${framing} %s KiB / 64-byte chunks`,
        async (kib: number): Promise<void> => {
          const item: unknown = { data: "x".repeat(kib * 1024) };
          const bytes: Uint8Array<ArrayBuffer> = encoder.encode(framingSource(framing, [item]));
          const candidate: () => Promise<unknown[]> = async (): Promise<unknown[]> =>
            collectFraming(
              await framingItems(side, framing, chunkedFramingBody(bytes, 64).body, bytes.length),
            );
          const reference: () => Promise<unknown[]> = (): Promise<unknown[]> =>
            referenceFramingItems(chunkedFramingBody(bytes, 64).body, framing);
          expect(await candidate()).toEqual([item]);
          expect(await reference()).toEqual([item]);
          const candidates: number[] = [],
            references: number[] = [];
          for (let sample: number = 0; sample < 5; sample++) {
            // Alternate order in the same process to avoid giving either path all
            // cold runs or all later runs. Existing baseline gates stay unchanged.
            if (sample % 2 === 0) {
              candidates.push(await timing(candidate));
              references.push(await timing(reference));
            } else {
              references.push(await timing(reference));
              candidates.push(await timing(candidate));
            }
          }
          const candidateMs: number = median(candidates),
            referenceMs: number = median(references);
          const limitMs: number = referenceMs * 2.5 + 5;
          const evidence: string | undefined = process.env.OPENAPI_SDKGEN_STREAM_PERF_EVIDENCE;
          if (evidence !== undefined)
            appendFileSync(
              evidence,
              JSON.stringify({
                side,
                framing,
                payloadKiB: kib,
                chunkBytes: 64,
                candidateSamplesMs: candidates,
                referenceSamplesMs: references,
                candidateMs,
                referenceMs,
                limitMs,
              }) + "\n",
            );
          console.log(
            `STREAM_FRAME_PERF side=${side} framing=${framing} payload_kib=${kib} chunk_bytes=64 samples=5 median_ms=${candidateMs.toFixed(3)} reference_ms=${referenceMs.toFixed(3)} limit_ms=${limitMs.toFixed(3)}`,
          );
          expect(candidateMs).toBeLessThanOrEqual(limitMs);
        },
      );
    }
});
