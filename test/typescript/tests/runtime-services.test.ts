import { describe, expect, it } from "vitest";
import * as templateHTTP from "../../../internal/target/typescript/runtime/compatibility/http.js";
import * as templateCore from "../../../internal/target/typescript/runtime/compatibility/http-core.js";
import * as templateContext from "../../../internal/target/typescript/runtime/compatibility/http-core.js";
import * as templateJSON from "../../../internal/target/typescript/runtime/compatibility/http-json.js";
import * as templateFull from "../../../internal/target/typescript/runtime/compatibility/http-codecs.js";
import * as templateWire from "../../../internal/target/typescript/runtime/compatibility/wire-engine.js";
import * as templateCodecs from "../../../internal/target/typescript/runtime/compatibility/codecs.js";
import * as templateBinder from "../../../internal/target/typescript/runtime/client/callables.js";
import type { OperationDefinition } from "../../../internal/target/typescript/runtime/http/operation.js";
import type { WireSchema } from "../../../internal/target/typescript/runtime/schema/wire-types.js";

const jsonOperation: OperationDefinition = {
  route: "GET /items/{id}",
  method: "GET",
  path: "/items/{id}",
  envelope: "",
  parameters: [
    {
      location: "path",
      name: "id",
      property: "id",
      required: true,
      style: "simple",
      explode: false,
      schema: { types: ["string"] },
    },
  ],
  responses: [
    {
      status: "200",
      contentType: "application/json",
      schemaDeclared: true,
      schema: {
        types: ["object"],
        required: ["item_id"],
        properties: { item_id: { property: "itemId", schema: { types: ["string"] } } },
      },
    },
  ],
};
const xmlOperation: OperationDefinition = {
  route: "GET /xml",
  method: "GET",
  path: "/xml",
  envelope: "",
  responses: [
    {
      status: "200",
      contentType: "application/xml",
      schemaDeclared: true,
      schema: {
        types: ["object"],
        xml: { name: "item" },
        properties: { title: { property: "title", schema: { types: ["string"] } } },
      },
    },
  ],
};

function textOperation(schema: WireSchema, contentType = "text/plain"): OperationDefinition {
  return {
    route: "GET /users/$count",
    method: "GET",
    path: "/users/$count",
    envelope: "",
    outputSchemas: { Count: { types: ["integer"], minimum: 0 } },
    responses: [{ status: "2XX", contentType, schemaDeclared: true, schema }],
  };
}

for (const [label, http, core, contexts, json, full, wire, codecs, binders] of [
  [
    "template",
    templateHTTP,
    templateCore,
    templateContext,
    templateJSON,
    templateFull,
    templateWire,
    templateCodecs,
    templateBinder,
  ],
] as const) {
  describe(`${label} execution services`, () => {
    it.each([
      ["reference", { reference: "Count" }, "3", 3],
      ["integer", { types: ["integer"] }, " 3\r\n", 3],
      ["number", { types: ["number"] }, "1.25e2", 125],
      ["boolean", { types: ["boolean"] }, "false", false],
      ["nullable", { types: ["integer", "null"] }, "null", null],
      ["allOf", { allOf: [{ reference: "Count" }, { maximum: 10 }] }, "4", 4],
      ["oneOf", { oneOf: [{ types: ["integer"] }, { types: ["boolean"] }] }, "true", true],
      ["string", { types: ["string"] }, "003", "003"],
      ["string union", { types: ["string", "integer"] }, "3", "3"],
      ["anyOf string", { anyOf: [{ types: ["string"] }, { types: ["number"] }] }, "3", "3"],
      ["untyped", {}, "3", "3"],
    ] satisfies readonly (readonly [string, WireSchema, string, unknown])[])(
      "decodes text %s values according to the declared schema",
      async (_name, schema, body, expected) => {
        const request = http.createRequest({
          baseURL: "https://example.test",
          fetch: async () =>
            new Response(body, { headers: { "content-type": "Text/Plain; charset=utf-8" } }),
        });
        expect(await request(textOperation(schema))).toEqual(expected);
        expect((await request.raw(textOperation(schema))).data).toEqual(expected);
      },
    );

    it.each(["", " ", "3items", "3.5", "NaN", "Infinity", "1e999", "0x10", "-1", "{}", "[]"])(
      "rejects invalid text integer response %j",
      async (body) => {
        const request = http.createRequest({
          baseURL: "https://example.test",
          fetch: async () => new Response(body, { headers: { "content-type": "text/plain" } }),
        });
        await expect(request(textOperation({ reference: "Count" }))).rejects.toMatchObject({
          code: "RESPONSE_DECODE_FAILED",
          status: 200,
        });
      },
    );

    it("keeps JSON and custom codec string values subject to their own schemas", async () => {
      for (const contentType of ["application/json", "application/vnd.example.count"]) {
        const request = http.createRequest({
          baseURL: "https://example.test",
          codecs: { "application/vnd.example.count": { decode: async () => "3" } },
          fetch: async () => new Response('"3"', { headers: { "content-type": contentType } }),
        });
        await expect(
          request(textOperation({ reference: "Count" }, contentType)),
        ).rejects.toMatchObject({
          code: "RESPONSE_DECODE_FAILED",
        });
      }
    });

    it("shares normalized client settings across buffered and streaming-capable executors", async () => {
      let codecReads = 0;
      const calls: string[] = [];
      const context = contexts.createRequestContext({
        baseURL: "https://example.test",
        authorization: "Bearer same-client",
        get codecs() {
          codecReads++;
          return {};
        },
        fetch: async (url, init) => {
          calls.push(new Headers(init?.headers).get("Authorization") ?? "");
          return String(url).endsWith("/xml")
            ? new Response("<item><title>XML</title></item>", {
                headers: { "content-type": "application/xml" },
              })
            : Response.json({ item_id: "id-1" });
        },
      });
      const buffered = core.createRequestCore(context, json.jsonRequestServices);
      const general = core.createRequestCore(context, full.fullRequestServices);
      for (let index = 0; index < 100; index++)
        core.createRequestCore(context, json.jsonRequestServices);
      expect(codecReads).toBe(1);
      expect("stream" in buffered).toBe(false);
      expect(typeof general.stream).toBe("function");
      const [one, two] = await Promise.all([
        buffered<{ itemId: string }>(jsonOperation, { path: { id: "x/y" } }),
        general<{ title: string }>(xmlOperation),
      ]);
      expect(one).toEqual({ itemId: "id-1" });
      expect(two).toEqual({ title: "XML" });
      expect(calls).toEqual(["Bearer same-client", "Bearer same-client"]);
      expect(codecReads).toBe(1);
      const callable = binders.bindOperation<{ path: { id: string } }, { itemId: string }>(
        buffered,
        jsonOperation,
        true,
      );
      expect((await callable.raw({ path: { id: "1" } })).data).toEqual({ itemId: "id-1" });
    });

    it("keeps credentials and fetch state on each client despite shared service objects", async () => {
      const seen: string[] = [];
      const make = (token: string) =>
        core.createRequestCore(
          contexts.createRequestContext({
            baseURL: "https://example.test",
            authorization: token,
            fetch: async (_url, init) => {
              await Promise.resolve();
              const auth = new Headers(init?.headers).get("authorization")!;
              seen.push(auth);
              return Response.json({ item_id: auth });
            },
          }),
          json.jsonRequestServices,
        );
      const left = make("Bearer left");
      const right = make("Bearer right");
      const [a, b] = await Promise.all([
        left(jsonOperation, { path: { id: "1" } }),
        right(jsonOperation, { path: { id: "2" } }),
      ]);
      expect(a).toEqual({ itemId: "Bearer left" });
      expect(b).toEqual({ itemId: "Bearer right" });
      expect(seen.sort()).toEqual(["Bearer left", "Bearer right"]);
    });

    it("retains path, response validation, error fallback, raw and abort behavior in JSON services", async () => {
      const traces: string[] = [];
      const options = {
        baseURL: "https://example.test",
        fetch: async (url: RequestInfo | URL) => {
          traces.push(String(url));
          return String(url).endsWith("/bad")
            ? new Response("unavailable", {
                status: 503,
                headers: { "content-type": "text/plain" },
              })
            : Response.json({ item_id: "valid" });
        },
      };
      const request = json.createJSONRequest(options);
      await expect(request(jsonOperation, { path: {} })).rejects.toMatchObject({
        code: "REQUEST_ENCODE_FAILED",
      });
      await expect(request(jsonOperation, { path: { id: ".." } })).rejects.toMatchObject({
        code: "REQUEST_ENCODE_FAILED",
      });
      await expect(
        request(jsonOperation, { path: { id: "a" } }, { signal: AbortSignal.abort() }),
      ).rejects.toMatchObject({ code: "REQUEST_ABORTED" });
      expect(traces).toEqual([]);
      const result = await request.raw<{ itemId: string }>(jsonOperation, { path: { id: "a/b" } });
      expect(traces[0]).toBe("https://example.test/items/a%2Fb");
      expect(result.status).toBe(200);
      expect(result.data).toEqual({ itemId: "valid" });
      await expect(request(jsonOperation, { path: { id: "bad" } })).rejects.toMatchObject({
        status: 503,
        message: "unavailable",
      });
      const badResponse = json.createJSONRequest({
        baseURL: "https://example.test",
        fetch: async () => Response.json({ item_id: 123 }),
      });
      await expect(badResponse(jsonOperation, { path: { id: "1" } })).rejects.toMatchObject({
        code: "RESPONSE_DECODE_FAILED",
      });
      const fullResponse = await http.createRequest(options)(jsonOperation, {
        path: { id: "a/b" },
      });
      expect(fullResponse).toEqual(result.data);
    });

    it("shares validation algorithms without sharing decoder context across reentrant calls", () => {
      const schema: WireSchema = {
        types: ["string"],
        contentMediaType: "application/x-fixture",
        contentSchema: { types: ["string"], constValue: "left" },
      };
      const rightSchema: WireSchema = {
        ...schema,
        contentSchema: { types: ["string"], constValue: "right" },
      };
      const order: string[] = [];
      const right = wire.createWireCodec(() => {
        order.push("right");
        return "right";
      });
      const left = wire.createWireCodec(() => {
        order.push("left");
        right.validateWireValue("raw", rightSchema, {}, "decode");
        return "left";
      });
      left.validateWireValue("raw", schema, {}, "decode");
      expect(order).toEqual(["left", "right"]);
      expect(() => right.validateWireValue("raw", schema, {}, "decode")).toThrow();
      expect(() => left.validateWireValue("raw", schema, {}, "decode")).not.toThrow();
    });

    it("preserves nested XML content validation in the full compatibility facade", () => {
      const schema: WireSchema = {
        types: ["string"],
        contentMediaType: "application/xml",
        contentSchema: {
          types: ["object"],
          xml: { name: "item" },
          required: ["value"],
          properties: {
            value: { property: "value", schema: { types: ["string"], constValue: "ok" } },
          },
        },
      };
      const valid = "<item><value>ok</value></item>";
      expect(codecs.decodeWireValue(valid, schema, {})).toBe(valid);
      expect(() => codecs.decodeWireValue("<item><value>bad</value></item>", schema, {})).toThrow();
    });
  });
}

function executorTypeContract() {
  const context = templateContext.createRequestContext({ fetch: async () => Response.json({}) });
  const buffered = templateCore.createRequestCore(context, templateJSON.jsonRequestServices);
  // @ts-expect-error A buffered executor does not expose an unimplemented stream function.
  buffered.stream;
  templateBinder.bindGeneratedOperation(buffered, jsonOperation, true);
  // @ts-expect-error Stream binding requires the streaming service contract.
  templateBinder.bindStreamOperation(buffered, jsonOperation, true);
  const full = templateCore.createRequestCore(context, templateFull.fullRequestServices);
  templateBinder.bindStreamOperation(full, jsonOperation, true);
}
void executorTypeContract;
