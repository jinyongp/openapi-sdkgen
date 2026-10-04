import { describe, expect, it } from "vitest";
import {
  mergeLinkInput,
  resolveLinkInput,
} from "../../../internal/target/typescript/runtime/client/links.js";
import { APIError } from "../../../internal/target/typescript/runtime/shared/runtime-support.js";
import type { RawResponse } from "../../../internal/target/typescript/runtime/http/request.js";

function sourceResponse(): RawResponse<unknown> {
  const response = new Response(null, {
    status: 201,
    headers: { "X-Trace": "trace-1" },
  });
  Object.defineProperty(response, "url", {
    value: "https://api.example.test/source",
    configurable: true,
  });
  return {
    status: 201,
    headers: { trace: "trace-1" },
    contentType: "application/json",
    data: {
      nested: {
        "a/b": {
          "~key": ["zero", "one"],
        },
      },
      items: [{ id: "first" }],
    },
    request: {},
    response,
  };
}

describe("OpenAPI Link runtime expressions", () => {
  it("resolves response and request expressions across every input section", () => {
    const source = sourceResponse();
    const sourceInput = {
      path: { id: "item-1" },
      query: {
        filter: { nested: { value: "query-value" } },
        generated: { inner: ["first", "second"] },
      },
      headerParams: { trace: "request-trace" },
      cookieParams: { session: "cookie-value" },
      body: { nested: ["body-zero", "body-one"] },
    };

    const resolved = resolveLinkInput<{
      path: Record<string, unknown>;
      query: Record<string, unknown>;
      headerParams: Record<string, unknown>;
      cookieParams: Record<string, unknown>;
      body: unknown;
    }>(
      source,
      {
        parameters: [
          { location: "path", property: "literal", value: 42 },
          { location: "path", property: "url", value: "$url" },
          { location: "path", property: "status", value: "$statusCode" },
          { location: "path", property: "responseStatus", value: "$response.statusCode" },
          { location: "query", property: "responseBody", value: "$response.body" },
          {
            location: "query",
            property: "escapedPointer",
            value: "$response.body#/nested/a~1b/~0key/1",
          },
          {
            location: "headerParams",
            property: "responseTrace",
            value: "$response.header.X-Trace",
          },
          { location: "path", property: "requestPath", value: "$request.path.id" },
          {
            location: "query",
            property: "requestQuery",
            value: "$request.query.filter#/nested/value",
          },
          { location: "headerParams", property: "requestHeader", value: "$request.header.trace" },
          { location: "cookieParams", property: "requestCookie", value: "$request.cookie.session" },
          {
            location: "query",
            property: "generated",
            value: {
              "x-sdkgen-link-request-parameter": {
                section: "query",
                property: "generated",
                pointer: "/inner/0",
              },
            },
          },
        ],
        requestBody: "$request.body#/nested/1",
      },
      sourceInput,
    );

    expect(resolved).toEqual({
      path: {
        literal: 42,
        url: "https://api.example.test/source",
        status: 201,
        responseStatus: 201,
        requestPath: "item-1",
      },
      query: {
        responseBody: source.data,
        escapedPointer: "one",
        requestQuery: "query-value",
        generated: "first",
      },
      headerParams: {
        responseTrace: "trace-1",
        requestHeader: "request-trace",
      },
      cookieParams: {
        requestCookie: "cookie-value",
      },
      body: "body-one",
    });
  });

  it("normalizes APIError responses and rejects unusable or malformed sources", () => {
    const raw = sourceResponse();
    const error = new APIError({
      code: "DECLARED",
      message: "declared failure",
      status: 409,
      data: { conflict: { id: "conflict-1" } },
      response: raw.response,
      request: { id: "request-1" },
    });
    expect(
      resolveLinkInput<{ path: { id: string } }>(error, {
        parameters: [
          {
            location: "path",
            property: "id",
            value: "$response.body#/conflict/id",
          },
        ],
      }),
    ).toEqual({ path: { id: "conflict-1" } });

    expect(() =>
      resolveLinkInput(new APIError({ code: "NO_RESPONSE", message: "no response" }), {}),
    ).toThrow("Link requires an APIError with an HTTP response");

    expect(() =>
      resolveLinkInput(raw, {
        requestBody: "$response.body#/items/not-an-index",
      }),
    ).toThrow("JSON Pointer array token not-an-index is invalid");

    expect(() =>
      resolveLinkInput(
        raw,
        {
          requestBody: {
            "x-sdkgen-link-request-parameter": {
              section: "query",
              property: "generated",
            },
          },
        },
        { query: { generated: "value" } },
      ),
    ).toThrow("invalid generated Link request parameter expression");
  });

  it("merges object sections while replacing scalar target sections", () => {
    const defaults = {
      query: { page: 1, size: 20 },
      body: { state: "derived" },
      mode: "derived",
    };
    const override = {
      query: { size: 50 },
      body: { state: "explicit" },
      mode: "explicit",
    };
    const merged = mergeLinkInput(defaults, override);
    expect(merged).toEqual({
      query: { page: 1, size: 50 },
      body: { state: "explicit" },
      mode: "explicit",
    });
    expect(defaults).toEqual({
      query: { page: 1, size: 20 },
      body: { state: "derived" },
      mode: "derived",
    });

    expect(mergeLinkInput("default", undefined)).toBe("default");
    const mergePrimitive = mergeLinkInput as unknown as (
      defaults: string,
      override: string,
    ) => string;
    expect(mergePrimitive("default", "override")).toBe("override");
  });
});
