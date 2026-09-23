import { describe, expect, it } from "vitest";

import type { WireSchema } from "../fixtures/generated/client/internal/runtime/codecs.js";
import { createRequest } from "../fixtures/generated/client/internal/runtime/http.js";
import type { OperationDefinition } from "../fixtures/generated/client/internal/runtime/operation.js";

import {
  streamPerformanceBaseline,
  type StreamPerformanceMetric,
} from "./stream-performance-baseline.js";

const perfIt = process.env.OPENAPI_SDKGEN_STREAM_PERF === "1" ? it : it.skip;

const operation = (contentType: string, itemSchema: WireSchema): OperationDefinition => ({
  route: "GET /events",
  operationID: "streamPerformance",
  method: "GET",
  path: "/events",
  envelope: "",
  responses: [
    {
      status: "200",
      contentType,
      schema: {},
      itemSchema,
      streamFraming: contentType === "text/event-stream" ? "sse" : "line-delimited-json",
    },
  ],
  outputSchemas: {},
});

const collect = async (items: AsyncIterable<unknown>): Promise<number> => {
  let count = 0;
  for await (const _item of items) count++;
  return count;
};

const measure = async (run: () => Promise<void>): Promise<number> => {
  const started = performance.now();
  await run();
  return performance.now() - started;
};

const median = (values: readonly number[]): number => {
  const sorted = [...values].sort((left, right) => left - right);
  return sorted[Math.floor(sorted.length / 2)]!;
};

const measureMedian = async (run: () => Promise<void>): Promise<number> => {
  await run();
  const samples: number[] = [];
  for (let index = 0; index < streamPerformanceBaseline.sampleCount; index += 1) {
    samples.push(await measure(run));
  }
  return median(samples);
};

const expectWithinBaseline = async (
  name: StreamPerformanceMetric,
  run: () => Promise<void>,
): Promise<number> => {
  const milliseconds = await measureMedian(run);
  const baseline = streamPerformanceBaseline.workloads[name].wallMilliseconds;
  const limit = baseline * (1 + streamPerformanceBaseline.regressionThresholdPercent / 100);
  const status =
    milliseconds <= limit && milliseconds <= streamPerformanceBaseline.catastrophicLimitMilliseconds
      ? "pass"
      : "fail";
  console.log(
    `STREAM_PERF name=${name} baseline_ms=${baseline.toFixed(1)} median_ms=${milliseconds.toFixed(1)} limit_ms=${limit.toFixed(1)} catastrophic_ms=${streamPerformanceBaseline.catastrophicLimitMilliseconds.toFixed(1)} status=${status}`,
  );
  expect(milliseconds, `${name} took ${milliseconds.toFixed(1)}ms`).toBeLessThanOrEqual(limit);
  expect(
    milliseconds,
    `${name} exceeded catastrophic limit ${streamPerformanceBaseline.catastrophicLimitMilliseconds}ms`,
  ).toBeLessThanOrEqual(streamPerformanceBaseline.catastrophicLimitMilliseconds);
  return milliseconds;
};

const expectRelativeOverhead = (
  name: string,
  candidateMilliseconds: number,
  baselineMilliseconds: number,
): void => {
  const limit =
    baselineMilliseconds * streamPerformanceBaseline.relativeOverhead.multiplier +
    streamPerformanceBaseline.relativeOverhead.slackMilliseconds;
  expect(
    candidateMilliseconds,
    `${name} ${candidateMilliseconds.toFixed(1)}ms vs baseline ${baselineMilliseconds.toFixed(1)}ms`,
  ).toBeLessThanOrEqual(limit);
};

const objectItemSchema = {
  types: ["object"],
  required: ["value"],
  properties: {
    value: { property: "value", schema: { types: ["integer"] } },
  },
  additionalProperties: false,
} as const;

const integerItemSchema = {
  types: ["integer"],
} as const;

describe("stream runtime performance acceptance", () => {
  it("defines a complete checked-in performance policy", () => {
    const expectedMetrics: StreamPerformanceMetric[] = [
      "adapter-pass-through",
      "many-small-ndjson",
      "many-small-sse",
      "near-limit-frames",
      "one-byte-chunks",
      "operation-stream-readable",
    ];
    expect(Object.keys(streamPerformanceBaseline.workloads).sort()).toEqual(expectedMetrics.sort());
    expect(streamPerformanceBaseline.sampleCount).toBeGreaterThanOrEqual(3);
    expect(streamPerformanceBaseline.sampleCount % 2).toBe(1);
    expect(streamPerformanceBaseline.regressionThresholdPercent).toBeGreaterThan(0);
    expect(streamPerformanceBaseline.catastrophicLimitMilliseconds).toBeGreaterThan(0);
    expect(streamPerformanceBaseline.relativeOverhead.multiplier).toBeGreaterThan(1);
    expect(streamPerformanceBaseline.relativeOverhead.slackMilliseconds).toBeGreaterThanOrEqual(0);
  });

  perfIt("decodes many small NDJSON and SSE frames", async () => {
    const count = 2_000;
    const ndjson = Array.from({ length: count }, (_, index) => `{"value":${index}}\n`).join("");
    const sse = Array.from({ length: count }, (_, index) => `event: todo\ndata: ${index}\n\n`).join(
      "",
    );

    const ndjsonRequest = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        new Response(ndjson, { headers: { "content-type": "application/x-ndjson" } }),
    });
    const sseRequest = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () => new Response(sse, { headers: { "content-type": "text/event-stream" } }),
    });

    await expectWithinBaseline("many-small-ndjson", async () => {
      expect(
        await collect(ndjsonRequest.stream(operation("application/x-ndjson", objectItemSchema))),
      ).toBe(count);
    });
    await expectWithinBaseline("many-small-sse", async () => {
      expect(
        await collect(sseRequest.stream(operation("text/event-stream", integerItemSchema))),
      ).toBe(count);
    });
  });

  perfIt("handles adversarial one-byte transport chunks", async () => {
    const count = 512;
    const wire = Array.from({ length: count }, (_, index) => `{"value":${index}}\n`).join("");
    const bytes = new TextEncoder().encode(wire);
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        new Response(
          new ReadableStream<Uint8Array>({
            start(controller) {
              for (const byte of bytes) controller.enqueue(Uint8Array.of(byte));
              controller.close();
            },
          }),
          { headers: { "content-type": "application/x-ndjson" } },
        ),
    });

    await expectWithinBaseline("one-byte-chunks", async () => {
      expect(
        await collect(request.stream(operation("application/x-ndjson", objectItemSchema))),
      ).toBe(count);
    });
  });

  perfIt("keeps near-limit frames bounded", async () => {
    const count = 64;
    const payload = "x".repeat(7_900);
    const itemSchema = {
      types: ["object"],
      required: ["payload"],
      properties: {
        payload: { property: "payload", schema: { types: ["string"] } },
      },
      additionalProperties: false,
    } as const;
    const wire = Array.from({ length: count }, () => JSON.stringify({ payload }) + "\n").join("");
    const request = createRequest({
      baseURL: "https://api.example.test",
      maxStreamFrameBytes: 8_192,
      fetch: async () =>
        new Response(wire, { headers: { "content-type": "application/x-ndjson" } }),
    });

    await expectWithinBaseline("near-limit-frames", async () => {
      expect(await collect(request.stream(operation("application/x-ndjson", itemSchema)))).toBe(
        count,
      );
    });
  });

  perfIt("bounds adapter pass-through overhead", async () => {
    const count = 2_000;
    const wire = Array.from({ length: count }, (_, index) => `{"value":${index}}\n`).join("");
    const definition = operation("application/x-ndjson", objectItemSchema);
    const makeFetch = () => async () =>
      new Response(wire, { headers: { "content-type": "application/x-ndjson" } });

    const plain = createRequest({ baseURL: "https://api.example.test", fetch: makeFetch() });
    const adapted = createRequest({
      baseURL: "https://api.example.test",
      streamCodecs: {
        "application/x-ndjson": {
          adapter: {
            async *decode(frames) {
              yield* frames;
            },
            async *encode(items) {
              yield* items;
            },
          },
        },
      },
      fetch: makeFetch(),
    });

    const plainMilliseconds = await measureMedian(async () => {
      expect(await collect(plain.stream(definition))).toBe(count);
    });
    const adaptedMilliseconds = await expectWithinBaseline("adapter-pass-through", async () => {
      expect(await collect(adapted.stream(definition))).toBe(count);
    });

    expectRelativeOverhead("adapter", adaptedMilliseconds, plainMilliseconds);
  });

  perfIt("bounds OperationStream Web ReadableStream interop overhead", async () => {
    const count = 2_000;
    const wire = Array.from({ length: count }, (_, index) => `{"value":${index}}\n`).join("");
    const definition = operation("application/x-ndjson", objectItemSchema);
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        new Response(wire, { headers: { "content-type": "application/x-ndjson" } }),
    });

    const iterationMilliseconds = await measureMedian(async () => {
      expect(await collect(request.stream(definition))).toBe(count);
    });
    const readableMilliseconds = await expectWithinBaseline(
      "operation-stream-readable",
      async () => {
        const reader = request.stream(definition).toReadableStream().getReader();
        let received = 0;
        while (true) {
          const next = await reader.read();
          if (next.done) break;
          received++;
        }
        expect(received).toBe(count);
      },
    );

    expectRelativeOverhead("readable", readableMilliseconds, iterationMilliseconds);
  });
});
