import { mkdirSync, writeFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { jsonWireCodec } from "../../../internal/target/typescript/runtime/internal/wire-engine.js";
import { fragmentedSSE } from "./runtime-quality-helper.js";

const perfIt = process.env.OPENAPI_SDKGEN_QUALITY_PERF === "1" ? it : it.skip;
const report: Record<string, unknown> = { node: process.version };

function record(key: string, value: unknown): void {
  report[key] = value;
  const output = resolve(import.meta.dirname, "../../..", ".tmp/runtime-quality-performance.json");
  mkdirSync(dirname(output), { recursive: true });
  writeFileSync(output, JSON.stringify(report, null, 2) + "\n");
}

describe("runtime quality performance", () => {
  perfIt("measures fragmented SSE across frame and chunk sizes", async () => {
    const results = [];
    for (const size of [32, 64, 128, 256].map((kib) => kib * 1024)) {
      for (const chunkSize of [64, 1024, size + 16]) {
        const run = async () => {
          const start = performance.now();
          const result = await fragmentedSSE(size, chunkSize);
          const elapsed = performance.now() - start;
          expect(result).toEqual([{ data: "x".repeat(size) }]);
          return elapsed;
        };
        await run();
        const samples = [await run(), await run(), await run()].sort((a, b) => a - b);
        results.push({ size, chunkSize, samplesMS: samples, medianMS: samples[1] });
      }
    }
    record("sse", results);
  });
  perfIt("measures exact decimal validation, including bounded extreme scales", () => {
    const values = [
      [0.3, 0.1],
      [3000000000000000, 1],
      [Number.MAX_VALUE, Number.MIN_VALUE],
    ] as const;
    const results = values.map(([value, multipleOf]) => {
      const schema = { types: ["number"], multipleOf };
      const run = () => {
        const start = performance.now();
        for (let index = 0; index < 10000; index++)
          jsonWireCodec.validateWireValue(value, schema, {}, "decode");
        return performance.now() - start;
      };
      run();
      const samples = [run(), run(), run()].sort((a, b) => a - b);
      expect(samples.every(Number.isFinite)).toBe(true);
      return { value, multipleOf, iterations: 10000, samplesMS: samples, medianMS: samples[1] };
    });
    record("numbers", results);
  });
});
