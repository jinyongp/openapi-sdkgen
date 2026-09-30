import { describe, expect, it, vi } from "vitest";

import { createRequest } from "../../../internal/target/typescript/runtime/internal/http.js";
import { TransportErrorCode } from "../../../internal/target/typescript/runtime/internal/errors.js";
import type { OperationDefinition } from "../../../internal/target/typescript/runtime/internal/operation.js";
import type {
  MediaCodec,
  WireMultipartHeaderDefinition,
  WireSchemas,
} from "../../../internal/target/typescript/runtime/internal/wire-engine.js";

type HeaderCase = {
  name: string;
  header: Omit<WireMultipartHeaderDefinition, "name" | "required">;
  valid: string;
  invalid: string;
};
const schemas: WireSchemas = {
  Count: { types: ["integer"] },
  Metadata: {
    types: ["object"],
    required: ["id"],
    properties: { id: { property: "id", schema: { reference: "Count" } } },
  },
};
const objectSchema = {
  types: ["object"],
  required: ["id", "flag"],
  properties: {
    id: { property: "id", schema: { types: ["integer"] } },
    flag: { property: "flag", schema: { types: ["boolean"] } },
  },
};
const codecs: Readonly<Record<string, MediaCodec<unknown>>> = {
  "application/x-count": { decodeParameter: async (value) => Number(value) },
};
const cases: HeaderCase[] = [
  {
    name: "referenced exploded object",
    header: { schema: { reference: "Metadata" }, explode: true },
    valid: "id=42",
    invalid: "id=wrong",
  },
  {
    name: "referenced joined object",
    header: { schema: { reference: "Metadata" }, explode: false },
    valid: "id,42",
    invalid: "id,wrong",
  },
  { name: "integer", header: { schema: { types: ["integer"] } }, valid: "42", invalid: "1.5" },
  { name: "number", header: { schema: { types: ["number"] } }, valid: "1.25", invalid: "Infinity" },
  { name: "boolean", header: { schema: { types: ["boolean"] } }, valid: "false", invalid: "yes" },
  {
    name: "integer reference",
    header: { schema: { reference: "Count" } },
    valid: "42",
    invalid: "wrong",
  },
  {
    name: "array with references",
    header: { schema: { types: ["array"], items: { reference: "Count" } } },
    valid: "1,2",
    invalid: "1,wrong",
  },
  {
    name: "exploded object",
    header: { schema: objectSchema, explode: true },
    valid: "id=1,flag=true",
    invalid: "id=bad,flag=true",
  },
  {
    name: "joined object",
    header: { schema: objectSchema, explode: false },
    valid: "id,2,flag,false",
    invalid: "id,2,flag,bad",
  },
  {
    name: "JSON",
    header: { schema: objectSchema, contentType: "application/json" },
    valid: '{"id":1,"flag":true}',
    invalid: "not JSON",
  },
  {
    name: "form",
    header: {
      schema: {
        types: ["object"],
        required: ["tag"],
        properties: {
          tag: { property: "tag", schema: { types: ["array"], items: { types: ["string"] } } },
        },
      },
      contentType: "application/x-www-form-urlencoded",
    },
    valid: "tag=a&tag=b&tag=c",
    invalid: "other=1",
  },
  {
    name: "XML",
    header: {
      schema: { types: ["integer"], xml: { name: "count" } },
      contentType: "application/xml",
    },
    valid: "<count>42</count>",
    invalid: "<count>wrong</count>",
  },
  {
    name: "custom codec",
    header: { schema: { types: ["integer"] }, contentType: "application/x-count" },
    valid: "42",
    invalid: "wrong",
  },
  {
    name: "text with schema",
    header: { schema: { types: ["integer"] }, contentType: "text/plain" },
    valid: "42",
    invalid: "wrong",
  },
];

function operations(test: HeaderCase) {
  const headers = [{ name: "X-Metadata", required: true, ...test.header }];
  const upload: OperationDefinition = {
    route: "POST /parts",
    method: "POST",
    path: "/parts",
    envelope: "",
    contentType: "multipart/form-data",
    inputSchemas: schemas,
    requestBodies: [
      {
        contentType: "multipart/form-data",
        schema: { types: ["object"] },
        encoding: [{ name: "payload", contentType: "text/plain", headers }],
      },
    ],
  };
  const download: OperationDefinition = {
    route: "GET /parts",
    method: "GET",
    path: "/parts",
    envelope: "",
    outputSchemas: schemas,
    responses: [
      {
        status: "200",
        contentType: "multipart/mixed",
        schema: { types: ["array"], items: { types: ["string"] } },
        itemEncoding: { contentType: "text/plain", headers },
      },
    ],
  };
  return { upload, download };
}

function partResponse(value: string | undefined): Response {
  const header = value === undefined ? "" : `X-Metadata: ${value}\r\n`;
  return new Response(
    `--part\r\nContent-Type: text/plain\r\n${header}\r\npayload\r\n--part--\r\n`,
    {
      headers: { "Content-Type": 'multipart/mixed; boundary="part"' },
    },
  );
}

describe("multipart header contracts", () => {
  it.each(cases)("accepts a valid $name request header", async (test) => {
    const fetch = vi.fn<typeof globalThis.fetch>(async () => new Response(null, { status: 204 }));
    const request = createRequest({ baseURL: "https://api.example.test", fetch, codecs });
    await request(
      operations(test).upload,
      { body: { payload: "hello" } },
      {
        multipartHeaders: { payload: { "X-Metadata": test.valid } },
      },
    );
    expect(fetch).toHaveBeenCalledOnce();
    const body = fetch.mock.calls[0]?.[1]?.body;
    expect(body).toBeInstanceOf(Blob);
    expect(await (body as Blob).text()).toContain(`x-metadata: ${test.valid}\r\n`);
  });

  it.each(cases)("rejects invalid or missing $name request headers before fetch", async (test) => {
    const fetch = vi.fn<typeof globalThis.fetch>(async () => new Response(null, { status: 204 }));
    const request = createRequest({ baseURL: "https://api.example.test", fetch, codecs });
    for (const supplied of [{ "X-Metadata": test.invalid }, {}]) {
      await expect(
        request(
          operations(test).upload,
          { body: { payload: "hello" } },
          {
            multipartHeaders: { payload: supplied },
          },
        ),
      ).rejects.toMatchObject({ code: TransportErrorCode.REQUEST_ENCODE_FAILED });
    }
    expect(fetch).not.toHaveBeenCalled();
  });

  it.each(cases)("decodes parts with valid $name response headers", async (test) => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      codecs,
      fetch: async () => partResponse(test.valid),
    });
    await expect(request(operations(test).download)).resolves.toEqual(["payload"]);
  });

  it.each(cases)("rejects parts with invalid or missing $name response headers", async (test) => {
    for (const value of [test.invalid, undefined]) {
      const request = createRequest({
        baseURL: "https://api.example.test",
        codecs,
        fetch: async () => partResponse(value),
      });
      await expect(request(operations(test).download)).rejects.toMatchObject({
        code: TransportErrorCode.RESPONSE_DECODE_FAILED,
      });
    }
  });
});
