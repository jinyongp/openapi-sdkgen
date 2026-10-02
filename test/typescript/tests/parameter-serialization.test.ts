import { describe, expect, it, vi } from "vitest";

import { createRequest } from "../../../internal/target/typescript/runtime/internal/http.js";
import { TransportErrorCode } from "../../../internal/target/typescript/runtime/internal/errors.js";
import type {
  OperationDefinition,
  ParameterDefinition,
} from "../../../internal/target/typescript/runtime/internal/operation.js";

type ParameterCase = {
  name: string;
  parameter: Partial<ParameterDefinition> & Pick<ParameterDefinition, "location">;
  value: unknown;
  search?: string;
  path?: string;
  header?: string;
};

const cases: ParameterCase[] = [
  ...(["path", "query", "querystring", "header", "cookie"] as const).map(
    (location): ParameterCase => ({
      name: `required nullable JSON ${location}`,
      parameter: {
        location,
        required: true,
        contentType: "application/json",
        schema: { types: ["string", "null"] },
      },
      value: null,
      ...(location === "path" ? { path: "/items/null" } : {}),
      ...(location === "query" ? { search: "?value=null" } : {}),
      ...(location === "querystring" ? { search: "?null" } : {}),
      ...(location === "header" ? { header: "null" } : {}),
      ...(location === "cookie" ? { header: "value=null" } : {}),
    }),
  ),
  {
    name: "text path",
    parameter: { location: "path", contentType: "text/plain" },
    value: "a/b",
    path: "/items/a%2Fb",
  },
  {
    name: "text header",
    parameter: { location: "header", contentType: "text/plain" },
    value: "hello world",
    header: "hello world",
  },
  {
    name: "text cookie",
    parameter: { location: "cookie", contentType: "text/plain" },
    value: "a/b",
    header: "value=a%2Fb",
  },
  {
    name: "exploded array",
    parameter: { location: "query" },
    value: ["a", "b"],
    search: "?value=a&value=b",
  },
  {
    name: "joined array",
    parameter: { location: "query", explode: false },
    value: ["a", "b"],
    search: "?value=a%2Cb",
  },
  {
    name: "space array",
    parameter: { location: "query", style: "spaceDelimited" },
    value: ["a", "b"],
    search: "?value=a%20b",
  },
  {
    name: "pipe array",
    parameter: { location: "query", style: "pipeDelimited" },
    value: ["a", "b"],
    search: "?value=a%7Cb",
  },
  {
    name: "deep object",
    parameter: { location: "query", style: "deepObject" },
    value: { id: 1, unused: undefined },
    search: "?value%5Bid%5D=1",
  },
  {
    name: "exploded form object",
    parameter: { location: "query" },
    value: { id: 1, active: true, unused: undefined },
    search: "?id=1&active=true",
  },
  {
    name: "joined form object",
    parameter: { location: "query", explode: false },
    value: { id: 1, active: true },
    search: "?value=id%2C1%2Cactive%2Ctrue",
  },
  {
    name: "exploded space object",
    parameter: { location: "query", style: "spaceDelimited" },
    value: { id: 1, active: true },
    search: "?value=id%3D1%20active%3Dtrue",
  },
  {
    name: "joined pipe object",
    parameter: { location: "query", style: "pipeDelimited", explode: false },
    value: { id: 1, active: true },
    search: "?value=id%7C1%7Cactive%7Ctrue",
  },
  {
    name: "reserved value without query injection",
    parameter: { location: "query", allowReserved: true },
    value: "https://host/a?b=c&d=e#f",
    search: "?value=https://host/a?b=c%26d=e%23f",
  },
  { name: "omitted parameter", parameter: { location: "query" }, value: undefined, search: "" },
  {
    name: "explicit empty parameter",
    parameter: { location: "query", allowEmptyValue: true },
    value: "",
    search: "?value=",
  },
  {
    name: "JSON parameter",
    parameter: { location: "query", contentType: "application/json" },
    value: { id: 1 },
    search: "?value=%7B%22id%22%3A1%7D",
  },
  {
    name: "form content parameter",
    parameter: { location: "query", contentType: "application/x-www-form-urlencoded" },
    value: { tag: ["a", "b"], id: 1 },
    search: "?value=tag%3Da%26tag%3Db%26id%3D1",
  },
  {
    name: "text content parameter",
    parameter: { location: "query", contentType: "text/plain" },
    value: "hello world",
    search: "?value=hello%20world",
  },
  {
    name: "whole form querystring",
    parameter: { location: "querystring", contentType: "application/x-www-form-urlencoded" },
    value: { tag: ["a", "b"], id: 1, unused: undefined },
    search: "?tag=a&tag=b&id=1",
  },
  {
    name: "whole JSON querystring",
    parameter: { location: "querystring", contentType: "application/json" },
    value: { id: 1 },
    search: "?%7B%22id%22%3A1%7D",
  },
  {
    name: "whole text querystring",
    parameter: { location: "querystring", contentType: "text/plain" },
    value: "a=b&c=d",
    search: "?a%3Db%26c%3Dd",
  },
  {
    name: "simple path",
    parameter: { location: "path", explode: false },
    value: "a/b",
    path: "/items/a%2Fb",
  },
  {
    name: "label path",
    parameter: { location: "path", style: "label" },
    value: ["a", "b"],
    path: "/items/.a.b",
  },
  {
    name: "exploded matrix array",
    parameter: { location: "path", style: "matrix" },
    value: ["a", "b"],
    path: "/items/;value=a;value=b",
  },
  {
    name: "exploded matrix object",
    parameter: { location: "path", style: "matrix" },
    value: { id: 1, active: true },
    path: "/items/;id=1;active=true",
  },
  {
    name: "joined matrix object",
    parameter: { location: "path", style: "matrix", explode: false },
    value: { id: 1, active: true },
    path: "/items/;value=id,1,active,true",
  },
  {
    name: "JSON path",
    parameter: { location: "path", contentType: "application/json" },
    value: { id: 1 },
    path: "/items/%7B%22id%22%3A1%7D",
  },
  { name: "header array", parameter: { location: "header" }, value: ["a", "b"], header: "a,b" },
  {
    name: "exploded header object",
    parameter: { location: "header" },
    value: { id: 1, active: true },
    header: "id=1,active=true",
  },
  {
    name: "joined header object",
    parameter: { location: "header", explode: false },
    value: { id: 1, active: true },
    header: "id,1,active,true",
  },
  {
    name: "exploded cookie array",
    parameter: { location: "cookie" },
    value: ["a", "b"],
    header: "value=a; value=b",
  },
  {
    name: "joined cookie array",
    parameter: { location: "cookie", explode: false },
    value: ["a", "b"],
    header: "value=a%2Cb",
  },
  {
    name: "exploded cookie object",
    parameter: { location: "cookie" },
    value: { id: 1, active: true, unused: undefined },
    header: "id=1; active=true",
  },
  {
    name: "joined cookie object",
    parameter: { location: "cookie", explode: false },
    value: { id: 1, active: true },
    header: "value=id%2C1%2Cactive%2Ctrue",
  },
  {
    name: "JSON cookie",
    parameter: { location: "cookie", contentType: "application/json" },
    value: { id: 1 },
    header: "value=%7B%22id%22%3A1%7D",
  },
];

const sections = {
  path: "path",
  query: "query",
  querystring: "querystring",
  header: "headerParams",
  cookie: "cookieParams",
} as const;

function setup(test: ParameterCase, customCodec: boolean) {
  const fetch = vi.fn<typeof globalThis.fetch>(async () => new Response(null, { status: 204 }));
  const request = createRequest({
    baseURL: "https://api.example.test",
    transport: { fetch, capabilities: { cookieJar: true } },
    codecs: { "application/x-test": { encodeParameter: async (value) => String(value) } },
  });
  const parameter: ParameterDefinition = {
    name: "value",
    property: "value",
    style:
      test.parameter.location === "path" || test.parameter.location === "header"
        ? "simple"
        : "form",
    explode: true,
    ...test.parameter,
  };
  // A custom header codec puts otherwise identical input through async encoding.
  // Its presence must not change any other parameter's wire representation.
  const parameters: ParameterDefinition[] = [parameter];
  const input: Record<string, Record<string, unknown>> = {
    [sections[parameter.location]]: { value: test.value },
  };
  if (customCodec) {
    parameters.push({
      location: "header",
      name: "X-Codec",
      property: "codec",
      style: "simple",
      explode: false,
      contentType: "application/x-test",
    });
    input.headerParams = { ...input.headerParams, codec: "active" };
  }
  const path = parameter.location === "path" ? "/items/{value}" : "/items";
  const operation: OperationDefinition = {
    route: `GET ${path}`,
    method: "GET",
    path,
    envelope: "",
    parameters,
  };
  return { request, fetch, operation, input };
}

describe.each([false, true])("parameter serialization with custom codec = %s", (customCodec) => {
  it.each(["path", "query", "querystring", "header", "cookie"] as const)(
    "rejects absent and disallowed null %s parameters",
    async (location) => {
      const test: ParameterCase = {
        name: "required",
        parameter: {
          location,
          required: true,
          contentType: "application/json",
          schema: { types: ["string"] },
        },
        value: null,
      };
      const { request, fetch, operation, input } = setup(test, customCodec);
      await expect(request(operation, input)).rejects.toThrow();
      input[sections[location]] = { value: undefined };
      await expect(request(operation, input)).rejects.toThrow();
      input[sections[location]] = Object.create({ value: "inherited" }) as Record<string, unknown>;
      await expect(request(operation, input)).rejects.toThrow();
      expect(fetch).not.toHaveBeenCalled();
    },
  );
  it.each(cases)("preserves $name on the wire", async (test) => {
    const { request, fetch, operation, input } = setup(test, customCodec);
    await request(operation, input);
    expect(fetch).toHaveBeenCalledOnce();
    const [url, options] = fetch.mock.calls[0]!;
    const parsed = new URL(String(url));
    expect(parsed.pathname).toBe(test.path ?? "/items");
    expect(parsed.search).toBe(test.search ?? "");
    if (test.header !== undefined) {
      const header = test.parameter.location === "cookie" ? "Cookie" : "value";
      expect(new Headers(options?.headers).get(header)).toBe(test.header);
    }
  });

  it.each<ParameterCase>([
    {
      name: "missing required query",
      parameter: { location: "query", required: true },
      value: undefined,
    },
    { name: "disallowed empty query", parameter: { location: "query" }, value: "" },
    { name: "undefined array entry", parameter: { location: "query" }, value: ["a", undefined] },
    {
      name: "non-object form querystring",
      parameter: { location: "querystring", contentType: "application/x-www-form-urlencoded" },
      value: "not an object",
    },
    { name: "missing path value", parameter: { location: "path" }, value: undefined },
  ])("rejects $name without fetching", async (test) => {
    const { request, fetch, operation, input } = setup(test, customCodec);
    await expect(request(operation, input)).rejects.toMatchObject({
      code: TransportErrorCode.REQUEST_ENCODE_FAILED,
    });
    expect(fetch).not.toHaveBeenCalled();
  });
});
