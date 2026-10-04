import { describe, expect, it } from "vitest";

import { createRequest } from "../../../internal/target/typescript/runtime/internal/http.js";
import { TransportErrorCode } from "../../../internal/target/typescript/runtime/internal/errors.js";
import type { OperationDefinition } from "../../../internal/target/typescript/runtime/internal/operation.js";

const operation: OperationDefinition = {
  route: "GET /headers",
  method: "GET",
  path: "/headers",
  envelope: "",
  outputSchemas: {
    Count: { types: ["integer"] },
  },
  responses: [
    {
      status: "200",
      contentType: "application/json",
      schema: { types: ["object"] },
      headers: [
        { name: "X-Count", property: "count", required: true, schema: { reference: "Count" } },
        { name: "X-Enabled", property: "enabled", schema: { types: ["boolean"] } },
        {
          name: "X-List",
          property: "list",
          schema: { types: ["array"], items: { types: ["integer"] } },
        },
        {
          name: "X-Meta",
          property: "meta",
          explode: true,
          schema: {
            types: ["object"],
            properties: {
              id: { property: "id", schema: { types: ["integer"] } },
              active: { property: "active", schema: { types: ["boolean"] } },
            },
          },
        },
        {
          name: "X-Pair",
          property: "pair",
          explode: false,
          schema: {
            types: ["object"],
            properties: {
              id: { property: "id", schema: { types: ["integer"] } },
              active: { property: "active", schema: { types: ["boolean"] } },
            },
          },
        },
        {
          name: "X-JSON",
          property: "json",
          contentType: "application/json",
          schema: {
            types: ["object"],
            properties: {
              value: { property: "value", schema: { types: ["integer"] } },
            },
          },
        },
        {
          name: "X-Form",
          property: "form",
          contentType: "application/x-www-form-urlencoded; charset=utf-8",
          schema: {},
        },
        {
          name: "X-Custom",
          property: "custom",
          contentType: "application/x-header",
          schema: {},
        },
      ],
    },
  ],
};

describe("response header decoding", () => {
  it("decodes schema, simple-style and content-based response headers", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      codecs: {
        "application/x-header": {
          decodeParameter: (value) => ({ decoded: value }),
        },
      },
      fetch: async () =>
        Response.json(
          {},
          {
            status: 200,
            headers: {
              "X-Count": "42",
              "X-Enabled": "true",
              "X-List": "1,2,3",
              "X-Meta": "id=7,active=false",
              "X-Pair": "id,8,active,true",
              "X-JSON": JSON.stringify({ value: 9 }),
              "X-Form": "a=1&a=2&b=x",
              "X-Custom": "custom-value",
            },
          },
        ),
    });

    const raw = await request.raw(operation);
    expect(raw.headers).toEqual({
      count: 42,
      enabled: true,
      list: [1, 2, 3],
      meta: { id: 7, active: false },
      pair: { id: 8, active: true },
      json: { value: 9 },
      form: { a: ["1", "2"], b: "x" },
      custom: { decoded: "custom-value" },
    });
  });

  it("normalizes missing required and malformed scalar headers as decode failures", async () => {
    for (const headers of [{}, { "X-Count": "not-an-integer" }]) {
      const request = createRequest({
        baseURL: "https://api.example.test",
        fetch: async () => Response.json({}, { status: 200, headers }),
      });
      await expect(request.raw(operation)).rejects.toMatchObject({
        code: TransportErrorCode.RESPONSE_DECODE_FAILED,
      });
    }
  });
});
