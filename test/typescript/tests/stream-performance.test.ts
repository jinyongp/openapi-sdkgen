import { describe, expect, it } from "vitest";

import type { WireSchema } from "../fixtures/generated/client/internal/runtime/codecs.js";
import { createRequest } from "../fixtures/generated/client/internal/runtime/http.js";
import type { OperationDefinition } from "../fixtures/generated/client/internal/runtime/operation.js";

const perfIt = process.env.OPENAPI_SDKGEN_STREAM_PERF === "1" ? it : it.skip;

const operation = (contentType: string, itemSchema: WireSchema): OperationDefinition => ({
  route: "GET /events",
  operationID: "streamPerformance",
  method: "GET",
  path: "/events",
  envelope: "",
  responses: [{ status: "200", contentType, schema: {}, itemSchema }],
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

const expectWithin = (name: string, milliseconds: number, limit = 10_000): void => {
  expect(milliseconds, `${name} took ${milliseconds.toFixed(1)}ms`).toBeLessThan(limit);
};

const objectItemSchema = {
  types: ["object"],
  required: ["value"],
  properties: {
    value: { property: "value", schema: { types: ["integer"] } },
  },
  additionalProperties: false,
} as const;

const sseItemSchema = {
  types: ["object"],
  required: ["data"],
  properties: {
    data: { property: "data", schema: { types: ["string"] } },
    event: { property: "event", schema: { types: ["string"] } },
  },
  additionalProperties: false,
} as const;

describe("stream runtime performance acceptance", () => {
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

    const ndjsonMilliseconds = await measure(async () => {
      expect(
        await collect(ndjsonRequest.stream(operation("application/x-ndjson", objectItemSchema))),
      ).toBe(count);
    });
    const sseMilliseconds = await measure(async () => {
      expect(await collect(sseRequest.stream(operation("text/event-stream", sseItemSchema)))).toBe(
        count,
      );
    });

    expectWithin("many-small-ndjson", ndjsonMilliseconds);
    expectWithin("many-small-sse", sseMilliseconds);
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

    const milliseconds = await measure(async () => {
      expect(
        await collect(request.stream(operation("application/x-ndjson", objectItemSchema))),
      ).toBe(count);
    });

    expectWithin("one-byte-chunks", milliseconds);
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

    const milliseconds = await measure(async () => {
      expect(await collect(request.stream(operation("application/x-ndjson", itemSchema)))).toBe(
        count,
      );
    });

    expectWithin("near-limit-frames", milliseconds);
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

    const plainMilliseconds = await measure(async () => {
      expect(await collect(plain.stream(definition))).toBe(count);
    });
    const adaptedMilliseconds = await measure(async () => {
      expect(await collect(adapted.stream(definition))).toBe(count);
    });

    expectWithin("adapter-pass-through", adaptedMilliseconds);
    expect(
      adaptedMilliseconds,
      `adapter ${adaptedMilliseconds.toFixed(1)}ms vs plain ${plainMilliseconds.toFixed(1)}ms`,
    ).toBeLessThan(plainMilliseconds * 10 + 1_000);
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

    const iterationMilliseconds = await measure(async () => {
      expect(await collect(request.stream(definition))).toBe(count);
    });
    const readableMilliseconds = await measure(async () => {
      const reader = request.stream(definition).toReadableStream().getReader();
      let received = 0;
      while (true) {
        const next = await reader.read();
        if (next.done) break;
        received++;
      }
      expect(received).toBe(count);
    });

    expectWithin("operation-stream-readable", readableMilliseconds);
    expect(
      readableMilliseconds,
      `readable ${readableMilliseconds.toFixed(1)}ms vs iteration ${iterationMilliseconds.toFixed(1)}ms`,
    ).toBeLessThan(iterationMilliseconds * 10 + 1_000);
  });
});
