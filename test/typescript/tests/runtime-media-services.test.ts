import { describe, expect, it } from "vitest";
import {
  createRequestCore,
  createRequestContext,
} from "../../../internal/target/typescript/runtime/compatibility/http-core.js";
import { bufferedXMLRequestServices } from "../../../internal/target/typescript/runtime/compatibility/http-buffered.js";
import { jsonResponseStreamServices } from "../../../internal/target/typescript/runtime/compatibility/http-json-stream.js";
import type { OperationDefinition } from "../../../internal/target/typescript/runtime/http/operation.js";

const streamOperation: OperationDefinition = {
  route: "GET /items",
  method: "GET",
  path: "/items",
  envelope: "",
  responses: [
    {
      status: "200",
      contentType: "application/x-ndjson",
      streamFraming: "line-delimited-json",
      schemaDeclared: true,
      schema: { types: ["array"], items: { types: ["object"] } },
      itemSchema: {
        types: ["object"],
        properties: { item_id: { property: "itemId", schema: { types: ["string"] } } },
      },
    },
  ],
};
const xmlOperation: OperationDefinition = {
  route: "POST /xml",
  method: "POST",
  path: "/xml",
  envelope: "",
  contentType: "application/xml",
  requestBodyRequired: true,
  requestBodies: [
    {
      contentType: "application/xml",
      schemaDeclared: true,
      schema: {
        types: ["object"],
        xml: { name: "item" },
        properties: { item_id: { property: "itemId", schema: { types: ["string"] } } },
      },
    },
  ],
  responses: [
    {
      status: "200",
      contentType: "application/xml",
      schemaDeclared: true,
      schema: {
        types: ["object"],
        xml: { name: "item" },
        properties: { item_id: { property: "itemId", schema: { types: ["string"] } } },
      },
    },
  ],
};

for (const [label, core, context, xmlServices, streamServices] of [
  [
    "template",
    createRequestCore,
    createRequestContext,
    bufferedXMLRequestServices,
    jsonResponseStreamServices,
  ],
] as const) {
  describe(`${label} media-specific services`, () => {
    it("preserves XML input and response mapping without stream services", async () => {
      const bodies: unknown[] = [];
      const request = core(
        context({
          baseURL: "https://example.test",
          fetch: async (_url, init) => {
            bodies.push(init?.body);
            return new Response("<item><item_id>result</item_id></item>", {
              headers: { "content-type": "application/xml" },
            });
          },
        }),
        xmlServices,
      );
      expect(await request(xmlOperation, { body: { itemId: "input" } })).toEqual({
        itemId: "result",
      });
      expect(bodies).toEqual(["<item><item_id>input</item_id></item>"]);
      expect("stream" in request).toBe(false);
    });

    it("keeps the stream handle synchronous and validates/maps individual items", async () => {
      const request = core(
        context({
          baseURL: "https://example.test",
          fetch: async () =>
            new Response('{"item_id":"a"}\n{"item_id":"b"}\n', {
              headers: { "content-type": "application/x-ndjson" },
            }),
        }),
        streamServices,
      );
      const stream = request.stream(streamOperation);
      expect("then" in stream).toBe(false);
      const items: unknown[] = [];
      for await (const item of stream) items.push(item);
      expect(items).toEqual([{ itemId: "a" }, { itemId: "b" }]);
      expect((await stream.response).status).toBe(200);
      const raw = await request.raw(streamOperation);
      expect(raw.status).toBe(200);
      expect(raw.data).toBeUndefined();
      expect(raw.response.bodyUsed).toBe(false);
      expect(await raw.response.text()).toBe('{"item_id":"a"}\n{"item_id":"b"}\n');
      expect(await request(streamOperation)).toEqual([{ item_id: "a" }, { item_id: "b" }]);
    });

    it("does not fetch for pre-aborted streams", async () => {
      let calls = 0;
      const request = core(
        context({
          baseURL: "https://example.test",
          fetch: async () => {
            calls++;
            return new Response();
          },
        }),
        streamServices,
      );
      const stream = request.stream(streamOperation, undefined, {
        signal: AbortSignal.abort(new Error("cancel")),
      });
      await expect(stream[Symbol.asyncIterator]().next()).rejects.toMatchObject({
        code: "REQUEST_ABORTED",
      });
      expect(calls).toBe(0);
    });

    it("cancels an unfinished response when the consumer stops early", async () => {
      let cancelled = 0;
      const body = new ReadableStream<Uint8Array>({
        start(controller) {
          controller.enqueue(new TextEncoder().encode('{"item_id":"first"}\n'));
        },
        cancel() {
          cancelled++;
        },
      });
      const request = core(
        context({
          baseURL: "https://example.test",
          fetch: async () =>
            new Response(body, { headers: { "content-type": "application/x-ndjson" } }),
        }),
        streamServices,
      );
      for await (const item of request.stream<{ itemId: string }>(streamOperation)) {
        expect(item.itemId).toBe("first");
        break;
      }
      expect(cancelled).toBe(1);
      expect(body.locked).toBe(false);
    });

    it("preserves decoding and HTTP error failures", async () => {
      const invalid = core(
        context({
          baseURL: "https://example.test",
          fetch: async () =>
            new Response("not-json\n", { headers: { "content-type": "application/x-ndjson" } }),
        }),
        streamServices,
      );
      await expect(
        invalid.stream(streamOperation)[Symbol.asyncIterator]().next(),
      ).rejects.toMatchObject({ code: "RESPONSE_DECODE_FAILED" });
      const unavailable = core(
        context({
          baseURL: "https://example.test",
          fetch: async () =>
            new Response("unavailable", { status: 503, headers: { "content-type": "text/plain" } }),
        }),
        streamServices,
      );
      await expect(
        unavailable.stream(streamOperation)[Symbol.asyncIterator]().next(),
      ).rejects.toMatchObject({ status: 503, message: "unavailable" });
    });
  });
}
