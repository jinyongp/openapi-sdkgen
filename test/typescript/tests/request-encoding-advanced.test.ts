import { describe, expect, it, vi } from "vitest";

import { createRequest } from "../../../internal/target/typescript/runtime/internal/http.js";
import { TransportErrorCode } from "../../../internal/target/typescript/runtime/internal/errors.js";
import type { OperationDefinition } from "../../../internal/target/typescript/runtime/internal/operation.js";

describe("advanced request encoding", () => {
  it("awaits custom parameter codecs across every request location", async () => {
    const encodeParameter = vi.fn(async (value: unknown, _context: { contentType: string }) =>
      `encoded:${String(value)}`,
    );
    const seen: Array<{ url: URL; headers: Headers }> = [];
    const request = createRequest({
      codecs: {
        "application/x-param": { encodeParameter },
      },
      transport: {
        capabilities: { cookieJar: true },
        fetch: async (input, init) => {
          seen.push({ url: new URL(String(input)), headers: new Headers(init?.headers) });
          return new Response(null, { status: 204 });
        },
      },
      baseURL: "https://api.example.test",
    });
    const operation: OperationDefinition = {
      route: "GET /items/{id}",
      method: "GET",
      path: "/items/{id}",
      envelope: "",
      parameters: [
        {
          location: "path",
          name: "id",
          property: "id",
          style: "simple",
          explode: false,
          contentType: "application/x-param",
        },
        {
          location: "query",
          name: "q",
          property: "query",
          style: "form",
          explode: true,
          contentType: "application/x-param",
        },
        {
          location: "querystring",
          name: "raw",
          property: "raw",
          style: "form",
          explode: true,
          contentType: "application/x-param",
        },
        {
          location: "header",
          name: "X-Custom",
          property: "header",
          style: "simple",
          explode: false,
          contentType: "application/x-param",
        },
        {
          location: "cookie",
          name: "session",
          property: "cookie",
          style: "form",
          explode: true,
          contentType: "application/x-param",
        },
      ],
    };

    await expect(
      request(operation, {
        path: { id: "path value" },
        query: { query: "query value" },
        querystring: { raw: "raw value" },
        headerParams: { header: "header value" },
        cookieParams: { cookie: "cookie value" },
      }),
    ).resolves.toBeUndefined();

    expect(seen).toHaveLength(1);
    expect(seen[0]!.url.pathname).toBe("/items/encoded%3Apath%20value");
    expect(seen[0]!.url.search).toBe("?q=encoded%3Aquery%20value&encoded%3Araw%20value");
    expect(seen[0]!.headers.get("X-Custom")).toBe("encoded:header value");
    expect(seen[0]!.headers.get("Cookie")).toBe("session=encoded%3Acookie%20value");
    expect(encodeParameter).toHaveBeenCalledTimes(5);
    expect(
      encodeParameter.mock.calls.map(([, context]) => context),
    ).toEqual([
      { contentType: "application/x-param" },
      { contentType: "application/x-param" },
      { contentType: "application/x-param" },
      { contentType: "application/x-param" },
      { contentType: "application/x-param" },
    ]);
  });

  it("resolves relative OpenAPI servers against origin and expands selected variables", async () => {
    const seen: string[] = [];
    const operation: OperationDefinition = {
      route: "GET /items",
      method: "GET",
      path: "/items",
      envelope: "",
      servers: [
        {
          id: "regional",
          url: "/api/{region}/",
          variables: [
            {
              name: "region",
              defaultValue: "us",
              enumValues: ["us", "eu"],
            },
          ],
        },
        {
          id: "absolute",
          url: "https://absolute.example.test/v2/",
        },
      ],
    };

    const regional = createRequest({
      origin: "https://origin.example.test",
      server: { id: "regional", variables: { region: "eu" } },
      fetch: async (input) => {
        seen.push(String(input));
        return new Response(null, { status: 204 });
      },
    });
    await regional(operation);
    expect(seen).toEqual(["https://origin.example.test/api/eu/items"]);

    const absolute = createRequest({
      server: { id: "absolute" },
      fetch: async (input) => {
        seen.push(String(input));
        return new Response(null, { status: 204 });
      },
    });
    await absolute(operation);
    expect(seen[1]).toBe("https://absolute.example.test/v2/items");

    const invalidVariable = createRequest({
      origin: "https://origin.example.test",
      server: { id: "regional", variables: { region: "invalid" } },
      fetch: async () => {
        throw new Error("invalid server selection must not fetch");
      },
    });
    await expect(invalidVariable(operation)).rejects.toMatchObject({
      code: TransportErrorCode.REQUEST_ENCODE_FAILED,
    });

    const missingOrigin = createRequest({
      server: { id: "regional" },
      fetch: async () => {
        throw new Error("relative server without origin must not fetch");
      },
    });
    await expect(missingOrigin(operation)).rejects.toMatchObject({
      code: TransportErrorCode.REQUEST_ENCODE_FAILED,
    });
  });
});
