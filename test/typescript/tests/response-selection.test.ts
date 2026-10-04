import { describe, expect, it } from "vitest";
import { createRequest } from "../../../internal/target/typescript/runtime/compatibility/http.js";
import { selectResponseDefinition } from "../../../internal/target/typescript/runtime/compatibility/http-core.js";
import type { OperationDefinition } from "../../../internal/target/typescript/runtime/http/operation.js";
import type { WireResponseDefinition } from "../../../internal/target/typescript/runtime/http/response/http-response-types.js";

const operation = (responses: readonly WireResponseDefinition[]): OperationDefinition => ({
  route: "GET /value",
  method: "GET",
  path: "/value",
  envelope: "",
  responses,
});
const definition = (status: string, contentType: string): WireResponseDefinition => ({
  status,
  contentType,
  schema: {},
});

describe("HTTP response selection", () => {
  it.each([false, true])(
    "prefers exact status, then status class, then default (reversed = %s)",
    (reverse) => {
      const fallback = definition("default", "application/json");
      const range = definition("2XX", "application/json");
      const exact = definition("201", "application/json");
      const declarations = [fallback, range, exact];
      if (reverse) declarations.reverse();
      for (const [status, expected] of [
        [201, exact],
        [202, range],
        [404, fallback],
      ] as const) {
        const response = new Response(null, {
          status,
          headers: { "Content-Type": "application/json" },
        });
        expect(selectResponseDefinition(operation(declarations), response, true)).toBe(expected);
      }
    },
  );

  it.each([
    "application/problem+json",
    " APPLICATION/PROBLEM+JSON ",
    "application/problem+json; charset=utf-8",
  ])("ranks exact %s ahead of suffix and broad media ranges", (mediaType) => {
    const exact = definition("200", mediaType);
    const suffix = definition("200", "application/*+json");
    const broad = definition("200", "*/*");
    for (const declarations of [
      [broad, suffix, exact],
      [exact, suffix, broad],
    ]) {
      const response = new Response(null, {
        headers: { "Content-Type": "Application/Problem+JSON; charset=utf-8" },
      });
      expect(selectResponseDefinition(operation(declarations), response, true)).toBe(exact);
    }
  });

  it("uses suffix wildcards before type wildcards and excludes unrelated status or media", () => {
    const suffix = definition("200", "application/*+json");
    const response = new Response(null, {
      headers: { "Content-Type": "application/problem+json" },
    });
    expect(
      selectResponseDefinition(
        operation([
          definition("200", "*/*"),
          definition("200", "application/*"),
          definition("201", "application/problem+json"),
          definition("200", "text/plain"),
          suffix,
        ]),
        response,
        true,
      ),
    ).toBe(suffix);
    expect(
      selectResponseDefinition(operation([definition("201", "*/*")]), response, true),
    ).toBeUndefined();
    expect(
      selectResponseDefinition(operation([definition("200", "text/plain")]), response, true),
    ).toBeUndefined();
  });

  it("selects a bodyless declaration when there is no content type", () => {
    const empty = definition("204", "");
    const response = new Response(null, { status: 204 });
    expect(
      selectResponseDefinition(
        operation([definition("default", "application/json"), empty]),
        response,
        true,
      ),
    ).toBe(empty);
  });

  it.each([false, true])(
    "uses the selected request schema with async codec = %s",
    async (customCodec) => {
      const captured: string[] = [];
      const request = createRequest({
        baseURL: "https://api.example.test",
        codecs: { "application/x-test": { encodeParameter: async (value) => String(value) } },
        fetch: async (_input, init) => {
          captured.push(String(init?.body));
          return new Response(null, { status: 204 });
        },
      });
      const schema = (name: string) => ({
        properties: { [name]: { property: "value", schema: { types: ["integer"] } } },
      });
      const target: OperationDefinition = {
        ...operation([]),
        method: "POST",
        route: "POST /value",
        parameters: [
          {
            location: "header",
            name: "X-Codec",
            property: "codec",
            style: "simple",
            explode: false,
            contentType: "application/x-test",
          },
        ],
        requestBodies: [
          { contentType: "application/*", schema: schema("wildcard_value") },
          { contentType: "application/json", schema: schema("exact_value") },
        ],
      };
      const extra = customCodec ? { headerParams: { codec: "active" } } : {};
      await request(target, {
        ...extra,
        body: { contentType: "Application/JSON; charset=utf-8", value: { value: 42 } },
      });
      await request(target, {
        ...extra,
        body: { contentType: "application/problem+json", value: { value: 17 } },
      });
      await expect(
        request(target, {
          ...extra,
          body: { contentType: "application/json; charset=utf-8", value: { value: "invalid" } },
        }),
      ).rejects.toMatchObject({ code: "REQUEST_ENCODE_FAILED" });
      expect(captured).toEqual(['{"exact_value":42}', '{"wildcard_value":17}']);
    },
  );

  it("uses the selected schema to map the actual decoded response", async () => {
    const schema = (property: string) => ({
      properties: { wire_value: { property, schema: { types: ["integer"] } } },
    });
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () => Response.json({ wire_value: 42 }),
    });
    await expect(
      request(
        operation([
          { status: "default", contentType: "*/*", schema: schema("fallback") },
          { status: "2XX", contentType: "application/json", schema: schema("range") },
          { status: "200", contentType: "application/*", schema: schema("wildcard") },
          {
            status: "200",
            contentType: "application/json; charset=utf-8",
            schema: schema("value"),
          },
        ]),
      ),
    ).resolves.toEqual({ value: 42 });
  });
});
