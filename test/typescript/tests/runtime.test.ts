import { describe, expect, it, vi } from "vitest";

import {
  bindOperation,
  bindPathOperation,
  type RequestFunction,
} from "../fixtures/generated/client/internal/runtime/callables.js";
import {
  TransportErrorCode,
  getErrorCode,
  isAPIError,
  isErrorCode,
} from "../fixtures/generated/client/internal/runtime/errors.js";
import {
  transformWireValue,
  validateWireValue,
  type WireSchema,
} from "../fixtures/generated/client/internal/runtime/codecs.js";
import { createRequest } from "../fixtures/generated/client/internal/runtime/http.js";
import {
  mergeLinkInput,
  resolveLinkInput,
} from "../fixtures/generated/client/internal/runtime/links.js";
import type { OperationDefinition } from "../fixtures/generated/client/internal/runtime/operation.js";
import { createPaginator } from "../fixtures/generated/client/internal/runtime/pagination.js";
import type {
  RawResponse,
  RequestOptions,
} from "../fixtures/generated/client/internal/runtime/request.js";

const operation = (overrides: Partial<OperationDefinition> = {}): OperationDefinition => ({
  route: "POST /items/{itemID}",
  operationID: "runtimeTest",
  method: "POST",
  path: "/items/{itemID}",
  envelope: "",
  ...overrides,
});

const jsonResponse = (body: unknown, status = 200, headers: HeadersInit = {}): Response =>
  new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json", ...headers },
  });

const collect = async <Item>(items: AsyncIterable<Item>): Promise<Item[]> => {
  const result: Item[] = [];
  for await (const item of items) result.push(item);
  return result;
};

describe("generated runtime", () => {
  it("reuses deep subtree validation while transforming nested values", () => {
    const depth = 64;
    let value: unknown = { leaf: 1 };
    let schema: WireSchema = {
      types: ["object"],
      required: ["leaf"],
      properties: { leaf: { property: "leaf", schema: { types: ["integer"] } } },
      additionalProperties: false,
    };
    for (let index = 0; index < depth; index++) {
      value = { child: value };
      schema = {
        types: ["object"],
        required: ["child"],
        properties: { child: { property: "child", schema } },
        additionalProperties: false,
      };
    }

    const values = vi.spyOn(Object, "values");
    transformWireValue(value, schema, {}, "decode");
    const scans = values.mock.calls.length;
    values.mockRestore();
    expect(scans).toBeLessThan((depth + 1) * 4);
  });

  it("checks uniqueItems without pairwise structural comparisons", () => {
    let propertyReads = 0;
    const values = Array.from({ length: 512 }, (_, id) =>
      Object.defineProperty(Object.create(null), "id", {
        enumerable: true,
        get: () => {
          propertyReads++;
          return id;
        },
      }),
    );
    const schema: WireSchema = {
      types: ["array"],
      uniqueItems: true,
      items: { types: ["object"] },
    };

    validateWireValue(values, schema, {}, "decode");
    expect(propertyReads).toBeLessThan(values.length * 5);
    expect(() =>
      validateWireValue(
        [
          { first: 1, second: 2 },
          { second: 2, first: 1 },
        ],
        schema,
        {},
        "decode",
      ),
    ).toThrow("must contain unique items");
  });

  it("resolves OpenAPI Link runtime expressions into target operation input", () => {
    const response = {
      status: 201,
      data: { order: { id: "order-1" }, items: [{ id: "item-1" }] },
      headers: {},
      request: {},
      response: new Response(null, { headers: { "x-trace": "trace-1" } }),
    } as never;
    expect(
      resolveLinkInput(
        response,
        {
          parameters: [
            { location: "path", property: "orderID", value: "$response.body#/order/id" },
            { location: "query", property: "item", value: "$response.body#/items/0/id" },
            { location: "headerParams", property: "trace", value: "$response.header.X-Trace" },
            { location: "cookieParams", property: "status", value: "$response.statusCode" },
          ],
          requestBody: "$request.body#/payload",
        },
        { body: { payload: { linked: true } } },
      ),
    ).toEqual({
      path: { orderID: "order-1" },
      query: { item: "item-1" },
      headerParams: { trace: "trace-1" },
      cookieParams: { status: 201 },
      body: { linked: true },
    });
    expect(() => resolveLinkInput(response, { requestBody: "$method" })).toThrow(
      "unsupported OpenAPI Link runtime expression",
    );
  });
  it("lets an explicit Link target input override derived defaults by section", () => {
    expect(
      mergeLinkInput(
        { path: { orderID: "derived" }, query: { page: 1, size: 20 }, body: { state: "derived" } },
        { path: { orderID: "explicit" }, query: { size: 100 }, body: { state: "explicit" } },
      ),
    ).toEqual({
      path: { orderID: "explicit" },
      query: { page: 1, size: 100 },
      body: { state: "explicit" },
    });
  });
  it("serializes paths, query styles, headers, cookies, and wire names", async () => {
    const fetch = vi.fn<typeof globalThis.fetch>(async (input, init) => {
      const url = new URL(String(input));
      expect(url.pathname).toBe("/v1/items/;itemID=one%2Ftwo");
      expect(url.searchParams.getAll("tags")).toEqual(["one", "two"]);
      expect(url.searchParams.get("filter[name]")).toBe("widget");
      expect(url.searchParams.get("filter[active]")).toBe("true");
      expect(url.searchParams.get("spaces")).toBe("one two");
      expect(url.searchParams.get("pipes")).toBe("one|two");
      expect(url.searchParams.get("query")).toBe('{"scope":"all"}');
      const headers = new Headers(init?.headers);
      expect(headers.get("x-trace")).toBe("trace-1");
      expect(headers.get("cookie")).toBe("session=one; session=two");
      expect(headers.get("authorization")).toBe("Bearer client");
      expect(headers.get("x-request-id")).toBe("request-1");
      expect(init?.redirect).toBe("error");
      expect(init?.body).toBe('{"wire_name":"widget"}');
      return jsonResponse({ wire_name: "response" }, 200, { "x-request-id": "server-1" });
    });
    const request = createRequest({
      baseURL: "https://api.example.test/v1/",
      transport: { fetch, capabilities: { cookieJar: true } },
      authorization: "Bearer client",
    });

    const result = await request<{ displayName: string }>(
      operation({
        parameters: [
          { location: "path", name: "itemID", property: "itemID", style: "matrix", explode: false },
          { location: "query", name: "tags", property: "tags", style: "form", explode: true },
          {
            location: "query",
            name: "filter",
            property: "filter",
            style: "deepObject",
            explode: true,
          },
          {
            location: "query",
            name: "spaces",
            property: "spaces",
            style: "spaceDelimited",
            explode: false,
          },
          {
            location: "query",
            name: "pipes",
            property: "pipes",
            style: "pipeDelimited",
            explode: false,
          },
          {
            location: "query",
            name: "query",
            property: "query",
            style: "form",
            explode: true,
            contentType: "Application/JSON",
          },
          {
            location: "header",
            name: "X-Trace",
            property: "trace",
            style: "simple",
            explode: false,
          },
          {
            location: "cookie",
            name: "session",
            property: "session",
            style: "form",
            explode: true,
          },
        ],
        inputSchemas: {
          Body: { properties: { wire_name: { property: "displayName", schema: {} } } },
        },
        requestBodies: [{ contentType: "application/json", schema: { reference: "Body" } }],
        responses: [
          {
            status: "2XX",
            contentType: "application/json",
            schema: { properties: { wire_name: { property: "displayName", schema: {} } } },
          },
        ],
        outputSchemas: {},
      }),
      {
        path: { itemID: "one/two" },
        query: {
          tags: ["one", "two"],
          filter: { name: "widget", active: true },
          spaces: ["one", "two"],
          pipes: ["one", "two"],
          query: { scope: "all" },
        },
        headerParams: { trace: "trace-1" },
        cookieParams: { session: ["one", "two"] },
        body: { displayName: "widget" },
      },
      { requestID: "request-1" },
    );
    expect(result).toEqual({ displayName: "response" });
  });

  it("allows undeclared standard headers but protects declared header parameters", async () => {
    const seen: Headers[] = [];
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (_input, init) => {
        seen.push(new Headers(init?.headers));
        return new Response(null, { status: 204 });
      },
    });
    await request(operation({ path: "/headers" }), undefined, {
      headers: { "Idempotency-Key": "raw-idem", "If-Match": "raw-version" },
    });
    expect(seen[0]?.get("idempotency-key")).toBe("raw-idem");
    expect(seen[0]?.get("if-match")).toBe("raw-version");

    const declared = operation({
      path: "/headers",
      headerNames: ["Idempotency-Key", "If-Match"],
      parameters: [
        {
          location: "header",
          name: "Idempotency-Key",
          property: "idempotency",
          style: "simple",
          explode: false,
        },
        {
          location: "header",
          name: "If-Match",
          property: "version",
          style: "simple",
          explode: false,
        },
      ],
    });
    await request(declared, { headerParams: {} }, { headers: { "Idempotency-Key": "raw" } }).then(
      () => {
        throw new Error("declared raw header was accepted");
      },
      (error: unknown) => {
        expect(String((error as { cause?: unknown }).cause)).toContain("must use its typed option");
      },
    );
    await request(declared, {
      headerParams: { idempotency: "typed-idem", version: "typed-version" },
    });
    expect(seen[1]?.get("idempotency-key")).toBe("typed-idem");
    expect(seen[1]?.get("if-match")).toBe("typed-version");
  });

  it("forwards environment-controlled headers from typed and raw caller inputs", async () => {
    const seen: Headers[] = [];
    const fetch = async (_input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
      seen.push(new Headers(init?.headers));
      return new Response(null, { status: 204 });
    };
    const managed = operation({
      path: "/managed",
      parameters: [
        {
          location: "header",
          name: "Origin",
          property: "Origin",
          style: "simple",
          explode: false,
        },
        {
          location: "header",
          name: "Sec-Fetch-Site",
          property: "Sec-Fetch-Site",
          style: "simple",
          explode: false,
        },
      ],
    });
    const request = createRequest({ baseURL: "https://api.example.test", fetch });

    await request(managed, { headerParams: { Origin: "https://caller.example" } });
    expect(seen.at(-1)?.get("Origin")).toBe("https://caller.example");
    await request(managed, { headerParams: { "Sec-Fetch-Site": "same-origin" } });
    expect(seen.at(-1)?.get("Sec-Fetch-Site")).toBe("same-origin");
    await createRequest({
      baseURL: "https://api.example.test",
      fetch,
      headers: { Origin: "https://client.example" },
    })(operation({ path: "/raw-client" }));
    expect(seen.at(-1)?.get("Origin")).toBe("https://client.example");
    await request(operation({ path: "/raw-request" }), undefined, {
      headers: { "Proxy-Authorization": "secret" },
    });
    expect(seen.at(-1)?.get("Proxy-Authorization")).toBe("secret");
    for (const name of [
      "Accept-Charset",
      "Accept-Encoding",
      "Access-Control-Request-Headers",
      "Access-Control-Request-Method",
      "Connection",
      "Content-Length",
      "Cookie",
      "Cookie2",
      "Date",
      "DNT",
      "Expect",
      "Host",
      "Keep-Alive",
      "Origin",
      "Referer",
      "Set-Cookie",
      "TE",
      "Trailer",
      "Transfer-Encoding",
      "Upgrade",
      "Via",
      "Proxy-Future",
      "Sec-Future",
    ]) {
      await request(operation({ path: "/raw-fixed" }), undefined, {
        headers: [[name, "forwarded"]],
      });
      expect(seen.at(-1)?.get(name)).toBe("forwarded");
    }

    await request(operation({ path: "/raw-allowed" }), undefined, {
      headers: {
        "User-Agent": "openapi-sdkgen-test",
        "X-Origin": "caller-owned",
        "X-HTTP-Methods": "TRACE",
      },
    });
    expect(seen.at(-1)?.get("User-Agent")).toBe("openapi-sdkgen-test");
  });

  it("delegates method override values to Fetch", async () => {
    const seen: Headers[] = [];
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (_input, init) => {
        seen.push(new Headers(init?.headers));
        return new Response(null, { status: 204 });
      },
    });
    const override = operation({
      path: "/override",
      parameters: [
        {
          location: "header",
          name: "X-HTTP-Method-Override",
          property: "method",
          style: "simple",
          explode: false,
        },
      ],
    });

    await request(override, { headerParams: { method: "PATCH" } });
    expect(seen[0]?.get("x-http-method-override")).toBe("PATCH");
    await request(override, { headerParams: { method: `"PATCH, TRACE"` } });
    expect(seen[1]?.get("x-http-method-override")).toBe(`"PATCH, TRACE"`);
    await request(override, { headerParams: { method: "RECONNECT" } });
    expect(seen[2]?.get("x-http-method-override")).toBe("RECONNECT");
    await request(override, { headerParams: { method: "PATCH, TRACE" } });
    expect(seen[3]?.get("x-http-method-override")).toBe("PATCH, TRACE");
    await request(operation({ path: "/raw-override" }), undefined, {
      headers: { "X-Method-Override": "connect" },
    });
    expect(seen[4]?.get("x-method-override")).toBe("connect");
    expect(seen).toHaveLength(5);
  });

  it("lets a trusted custom transport inject an environment-controlled header after encoding", async () => {
    const dispatched: Headers[] = [];
    const request = createRequest({
      baseURL: "https://api.example.test",
      transport: {
        fetch: async (_input, init) => {
          const headers = new Headers(init?.headers);
          expect(headers.has("Origin")).toBe(false);
          headers.set("Origin", "https://transport.example");
          dispatched.push(headers);
          return new Response(null, { status: 204 });
        },
      },
    });
    await request(
      operation({
        path: "/managed",
        parameters: [
          {
            location: "header",
            name: "Origin",
            property: "Origin",
            style: "simple",
            explode: false,
          },
        ],
      }),
    );
    expect(dispatched[0]?.get("Origin")).toBe("https://transport.example");
  });

  it("encodes form, multipart, text, and binary bodies", async () => {
    const requests: RequestInit[] = [];
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (_input, init) => {
        requests.push(init ?? {});
        return new Response(null, { status: 204 });
      },
    });
    for (const [contentType, body] of [
      ["application/x-www-form-urlencoded", { tag: ["one", "two"], name: "widget" }],
      ["multipart/form-data", { name: "widget", file: new Uint8Array([1, 2, 3]) }],
      ["text/plain", "plain text"],
      ["application/octet-stream", new Uint8Array([1, 2])],
    ] as const) {
      await request(operation({ path: "/uploads", contentType }), { body });
    }
    expect(String(requests[0]?.body)).toBe("tag=one&tag=two&name=widget");
    expect(requests[1]?.body).toBeInstanceOf(FormData);
    expect((requests[1]?.body as FormData).get("name")).toBe("widget");
    const file = (requests[1]?.body as FormData).get("file");
    expect(file).toBeInstanceOf(Blob);
    expect([...new Uint8Array(await (file as Blob).arrayBuffer())]).toEqual([1, 2, 3]);
    expect(requests[2]?.body).toBe("plain text");
    expect(requests[3]?.body).toBeInstanceOf(Uint8Array);
  });

  it("normalizes raw output and transport failures", async () => {
    const raw = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        jsonResponse({ data: { id: "widget-1" } }, 201, { "x-request-id": "server-1" }),
    });
    await expect(
      raw.raw<{ id: string }>(operation({ path: "/health", envelope: "data" })),
    ).resolves.toMatchObject({
      status: 201,
      contentType: "application/json",
      data: { data: { id: "widget-1" } },
      request: { id: "server-1" },
    });

    const network = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () => {
        throw new Error("offline");
      },
    });
    const error = await network(operation({ path: "/health" })).catch((cause: unknown) => cause);
    expect(isErrorCode(error, TransportErrorCode.NETWORK_ERROR)).toBe(true);

    const decode = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        new Response("not json", { status: 200, headers: { "content-type": "application/json" } }),
    });
    const decodeError = await decode(operation({ path: "/health" })).catch(
      (cause: unknown) => cause,
    );
    expect(isErrorCode(decodeError, TransportErrorCode.RESPONSE_DECODE_FAILED)).toBe(true);

    const server = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        new Response("unavailable", { status: 503, headers: { "content-type": "text/plain" } }),
    });
    const serverError = await server(operation({ path: "/health" })).catch(
      (cause: unknown) => cause,
    );
    expect(isAPIError(serverError)).toBe(true);
    if (!isAPIError(serverError)) throw new Error("expected API error");
    expect(serverError.code).toBe("HTTP_503");
    expect(serverError.message).toBe("unavailable");
    expect(isErrorCode(serverError, TransportErrorCode.NETWORK_ERROR)).toBe(false);
    expect(isAPIError(new Error("plain"))).toBe(false);

    const controller = new AbortController();
    controller.abort("stop");
    const aborted = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () => {
        throw new Error("aborted");
      },
    });
    const abortedError = await aborted(operation({ path: "/health" }), undefined, {
      signal: controller.signal,
    }).catch((cause: unknown) => cause);
    expect(isErrorCode(abortedError, TransportErrorCode.REQUEST_ABORTED)).toBe(true);
  });

  it("rejects missing input and raw headers owned by the contract", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () => jsonResponse({}),
    });
    const missing = await request(operation(), {}).catch((cause: unknown) => cause);
    expect(isErrorCode(missing, TransportErrorCode.REQUEST_ENCODE_FAILED)).toBe(true);

    const reserved = await request(operation({ path: "/health" }), undefined, {
      headers: { "content-type": "application/json" },
    }).catch((cause: unknown) => cause);
    expect(isErrorCode(reserved, TransportErrorCode.REQUEST_ENCODE_FAILED)).toBe(true);

    const undefinedArray = await request(operation({ path: "/health" }), {
      query: { tags: ["valid", undefined] },
    }).catch((cause: unknown) => cause);
    expect(isErrorCode(undefinedArray, TransportErrorCode.REQUEST_ENCODE_FAILED)).toBe(true);
  });

  it("binds operations and path input without mutating caller input", async () => {
    const calls: unknown[][] = [];
    const request = (async <Output>(...args: unknown[]) => {
      calls.push(args);
      return { id: "widget-1" } as Output;
    }) as unknown as RequestFunction;
    request.raw = async <Output>() =>
      ({
        status: 200,
        contentType: "application/json",
        data: { id: "widget-1" } as Output,
        headers: Object.create(null) as Readonly<Record<string, unknown>>,
        request: {},
        response: new Response(),
      }) as RawResponse<Output>;
    const call = bindOperation<{ body: { name: string } }, { id: string }>(
      request,
      operation(),
      true,
    );
    const bound = bindPathOperation(call, { itemID: "widget-1" }, true);
    const input = { body: { name: "first" } };
    await expect(bound(input)).resolves.toEqual({ id: "widget-1" });
    expect(input).toEqual({ body: { name: "first" } });
    expect(calls).toEqual([
      [operation(), { body: { name: "first" }, path: { itemID: "widget-1" } }, undefined],
    ]);

    const noInput = bindOperation<never, { id: string }>(
      request,
      operation({ path: "/health" }),
      false,
    );
    await expect(noInput()).resolves.toEqual({ id: "widget-1" });
    await expect(noInput.raw()).resolves.toMatchObject({ data: { id: "widget-1" } });
  });

  it("splits sole options from optional generated input", async () => {
    const calls: Array<{ input: unknown; options: RequestOptions | undefined }> = [];
    const request = (async <Output>(
      _operation: OperationDefinition,
      input?: unknown,
      options?: RequestOptions,
    ) => {
      calls.push({ input, options });
      return { id: "widget-1" } as Output;
    }) as RequestFunction;
    request.raw = async <Output>(
      _operation: OperationDefinition,
      input?: unknown,
      options?: RequestOptions,
    ) => {
      calls.push({ input, options });
      return { data: { id: "widget-1" } as Output } as RawResponse<Output>;
    };
    type Input = { readonly query?: { readonly force?: boolean } };
    type OptionalCall = {
      (options?: RequestOptions): Promise<{ id: string }>;
      (input?: Input, options?: RequestOptions): Promise<{ id: string }>;
      raw(options?: RequestOptions): Promise<RawResponse<{ id: string }>>;
      raw(input?: Input, options?: RequestOptions): Promise<RawResponse<{ id: string }>>;
    };
    const optional = bindOperation<Input, { id: string }>(
      request,
      operation({ path: "/optional" }),
      true,
      true,
    ) as unknown as OptionalCall;

    await optional({ credentials: "include" });
    await optional({ query: { force: true } });
    await optional(undefined, { credentials: "omit" });
    await optional.raw({ credentials: "same-origin" });

    const full = bindOperation<
      { path: { accountID: string }; query?: { force?: boolean } },
      { id: string }
    >(request, operation({ path: "/accounts/{accountID}/phone" }), true);
    const resource = bindPathOperation(
      full,
      { accountID: "account-1" },
      true,
      true,
    ) as unknown as OptionalCall;
    await resource({ credentials: "include" });
    await resource({ query: { force: true } }, { credentials: "omit" });

    expect(calls).toEqual([
      { input: undefined, options: { credentials: "include" } },
      { input: { query: { force: true } }, options: undefined },
      { input: undefined, options: { credentials: "omit" } },
      { input: undefined, options: { credentials: "same-origin" } },
      { input: { path: { accountID: "account-1" } }, options: { credentials: "include" } },
      {
        input: { path: { accountID: "account-1" }, query: { force: true } },
        options: { credentials: "omit" },
      },
    ]);
  });

  it("paginates cursor and offset profiles and rejects invalid modes", async () => {
    const cursorInputs: Array<{ query: { cursor?: string } }> = [];
    const cursor = createPaginator<string, { query: { cursor?: string } }, unknown>(
      async (input) => {
        cursorInputs.push(input);
        return input.query.cursor === undefined
          ? { items: ["one"], pagination: { nextCursor: "next" } }
          : { items: ["two"], pagination: { nextCursor: "" } };
      },
      {
        mode: "cursor",
        request: { cursor: "cursor" },
        response: { items: ["items"], nextCursor: ["pagination", "nextCursor"] },
      },
    );
    await expect(collect(cursor({ query: {} }))).resolves.toEqual(["one", "two"]);
    expect(cursorInputs).toEqual([{ query: {} }, { query: { cursor: "next" } }]);

    const offset = createPaginator<string, { query: { offset?: number; limit?: number } }, unknown>(
      async (input) => ({
        items: input.query.offset === 0 ? ["one", "two"] : ["three"],
        pagination: { offset: input.query.offset, limit: 2, total: 3 },
      }),
      {
        mode: "offset",
        request: { offset: "offset", limit: "limit" },
        response: {
          items: ["items"],
          offset: ["pagination", "offset"],
          limit: ["pagination", "limit"],
          total: ["pagination", "total"],
        },
      },
    );
    await expect(collect(offset({ query: { offset: 0, limit: 2 } }))).resolves.toEqual([
      "one",
      "two",
      "three",
    ]);

    const both = createPaginator<string, { query: {} }, unknown>(
      async () => ({ data: { items: ["nested"], pagination: { nextCursor: "" } } }),
      {
        mode: "both",
        request: { cursor: "cursor", offset: "offset" },
        response: { items: ["data", "items"], nextCursor: ["data", "pagination", "nextCursor"] },
      },
    );
    await expect(collect(both({ mode: "cursor", query: {} } as never))).resolves.toEqual([
      "nested",
    ]);
    const error = await collect(both({ query: {} } as never)).catch((cause: unknown) => cause);
    expect(error).toBeInstanceOf(TypeError);
    expect(isAPIError(error)).toBe(false);

    const invalidCursor = await collect(
      both({ mode: "cursor", query: { offset: 1 } } as never),
    ).catch((cause: unknown) => cause);
    expect(invalidCursor).toBeInstanceOf(TypeError);
  });

  it("stops cursor pagination when a cursor repeats after more than one page", async () => {
    const cursors: string[] = [];
    const paginate = createPaginator<string, { query: { cursor?: string } }, unknown>(
      async (input) => {
        const cursor = input.query.cursor ?? "start";
        cursors.push(cursor);
        return {
          items: [cursor],
          pagination: { nextCursor: cursor === "start" ? "a" : cursor === "a" ? "b" : "a" },
        };
      },
      {
        mode: "cursor",
        request: { cursor: "cursor" },
        response: { items: ["items"], nextCursor: ["pagination", "nextCursor"] },
      },
    );
    await expect(collect(paginate({ query: {} }))).resolves.toEqual(["start", "a", "b"]);
    expect(cursors).toEqual(["start", "a", "b"]);
  });

  it("uses exact prototype-safe pointers and request offset fallbacks", async () => {
    const cursor = createPaginator<string, { query: { cursor?: string } }, unknown>(
      async () =>
        JSON.parse('{"__proto__":{"items":["safe"]},"constructor":{"nextCursor":null}}') as unknown,
      {
        mode: "cursor",
        request: { cursor: "cursor" },
        response: {
          items: ["__proto__", "items"],
          nextCursor: ["constructor", "nextCursor"],
        },
      },
    );
    await expect(collect(cursor({ query: {} }))).resolves.toEqual(["safe"]);

    const offsets: number[] = [];
    const offset = createPaginator<string, { query: { offset?: number; limit?: number } }, unknown>(
      async (input) => {
        const current = input.query.offset ?? 0;
        offsets.push(current);
        return { rows: current === 2 ? ["one", "two"] : [] };
      },
      {
        mode: "offset",
        request: { offset: "offset", limit: "limit" },
        response: { items: ["rows"] },
      },
    );
    await expect(collect(offset({ query: { offset: 2, limit: 2 } }))).resolves.toEqual([
      "one",
      "two",
    ]);
    expect(offsets).toEqual([2, 4]);
  });

  it("keeps ordinary bodies with contentType and value fields intact", async () => {
    const fetch = vi.fn<typeof globalThis.fetch>(async (_input, init) => {
      expect(init?.body).toBe('{"contentType":"business","value":"payload"}');
      return jsonResponse({ ok: true });
    });
    const request = createRequest({ baseURL: "https://api.example.test", fetch });
    await expect(
      request(
        operation({
          path: "/single-body",
          requestBodies: [{ contentType: "application/json", schema: {} }],
        }),
        { body: { contentType: "business", value: "payload" } },
      ),
    ).resolves.toEqual({ ok: true });
  });

  it("serializes ordinary limit and sort query parameters", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (input) => {
        const url = new URL(String(input));
        expect(url.searchParams.get("limit")).toBe("25");
        expect(url.searchParams.get("sort")).toBe("createdAt");
        return jsonResponse({ ok: true });
      },
    });
    await expect(
      request(
        operation({
          path: "/search",
          parameters: [
            { location: "query", name: "limit", property: "limit", style: "form", explode: true },
            { location: "query", name: "sort", property: "sort", style: "form", explode: true },
          ],
        }),
        { query: { limit: 25, sort: "createdAt" } },
      ),
    ).resolves.toEqual({ ok: true });
  });

  it("serializes OpenAPI style variants for paths, query objects, headers, and cookies", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      transport: {
        capabilities: { cookieJar: true },
        fetch: async (input, init) => {
          const url = new URL(String(input));
          expect(url.pathname).toBe("/styles/.one.two/;first=1;second=two");
          expect(url.searchParams.get("filter")).toBe("first,1,second,two");
          const headers = new Headers(init?.headers);
          expect(headers.get("x-context")).toBe('{"scope":"all"}');
          expect(headers.get("cookie")).toBe("flags=one%2Ctwo");
          return jsonResponse({ ok: true });
        },
      },
    });
    await expect(
      request(
        operation({
          path: "/styles/{label}/{matrix}",
          parameters: [
            { location: "path", name: "label", property: "label", style: "label", explode: true },
            {
              location: "path",
              name: "matrix",
              property: "matrix",
              style: "matrix",
              explode: true,
            },
            {
              location: "query",
              name: "filter",
              property: "filter",
              style: "form",
              explode: false,
            },
            {
              location: "header",
              name: "X-Context",
              property: "context",
              style: "simple",
              explode: false,
              contentType: "application/json",
            },
            { location: "cookie", name: "flags", property: "flags", style: "form", explode: false },
          ],
        }),
        {
          path: { label: ["one", "two"], matrix: { first: 1, second: "two" } },
          query: { filter: { first: 1, second: "two" } },
          headerParams: { context: { scope: "all" } },
          cookieParams: { flags: ["one", "two"] },
        },
      ),
    ).resolves.toEqual({ ok: true });
  });

  it("rejects serialized path parameters that would become URL dot-segments", async () => {
    const paths: string[] = [];
    const request = createRequest({
      baseURL: "https://api.example.test",
      codecs: { "application/x-dot": { encodeParameter: async () => ".." } },
      fetch: async (input) => {
        paths.push(new URL(String(input)).pathname);
        return jsonResponse({ ok: true });
      },
    });
    const simple = operation({
      path: "/root/{id}/child",
      parameters: [
        { location: "path", name: "id", property: "id", style: "simple", explode: false },
      ],
    });
    for (const value of [".", ".."]) {
      const error = await request(simple, { path: { id: value } }).catch((cause: unknown) => cause);
      expect(isErrorCode(error, TransportErrorCode.REQUEST_ENCODE_FAILED)).toBe(true);
      expect(String((error as { cause?: unknown }).cause)).toContain("URL dot-segment");
    }
    const label = operation({
      path: "/root/{id}/child",
      parameters: [
        { location: "path", name: "id", property: "id", style: "label", explode: false },
      ],
    });
    const labelError = await request(label, { path: { id: "." } }).catch((cause: unknown) => cause);
    expect(isErrorCode(labelError, TransportErrorCode.REQUEST_ENCODE_FAILED)).toBe(true);
    const custom = operation({
      path: "/root/{id}/child",
      parameters: [
        {
          location: "path",
          name: "id",
          property: "id",
          style: "simple",
          explode: false,
          contentType: "application/x-dot",
        },
      ],
    });
    const customError = await request(custom, { path: { id: "codec" } }).catch(
      (cause: unknown) => cause,
    );
    expect(isErrorCode(customError, TransportErrorCode.REQUEST_ENCODE_FAILED)).toBe(true);
    await expect(request(simple, { path: { id: "a/../b" } })).resolves.toEqual({ ok: true });
    expect(paths).toEqual(["/root/a%2F..%2Fb/child"]);
  });

  it("applies Encoding Object rules to urlencoded request-body properties", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (_input, init) => {
        expect(new Headers(init?.headers).get("content-type")).toBe(
          "application/x-www-form-urlencoded",
        );
        expect(String(init?.body)).toBe(
          "tags=one%2Ctwo&filter=first%2C1%2Csecond%2Ctwo&payload=%7B%22ok%22%3Atrue%7D",
        );
        return jsonResponse({ ok: true });
      },
    });
    await expect(
      request(
        operation({
          path: "/form",
          requestBodies: [
            {
              contentType: "application/x-www-form-urlencoded",
              schema: {},
              encoding: [
                { name: "tags", explode: false },
                { name: "filter", style: "form", explode: false },
                { name: "payload", contentType: "application/json" },
              ],
            },
          ],
          contentType: "application/x-www-form-urlencoded",
        }),
        {
          body: {
            tags: ["one", "two"],
            filter: { first: 1, second: "two" },
            payload: { ok: true },
          },
        },
      ),
    ).resolves.toEqual({ ok: true });
  });

  it("writes declared multipart part media types and headers without allowing undeclared headers", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (_input, init) => {
        const contentType = new Headers(init?.headers).get("content-type");
        expect(contentType).toMatch(/^multipart\/form-data; boundary=----openapi-sdkgen-/);
        const body = init?.body;
        expect(body).toBeInstanceOf(Blob);
        const text = await (body as Blob).text();
        expect(text).toContain('Content-Disposition: form-data; name="metadata"');
        expect(text).toContain("Content-Type: application/json");
        expect(text).toContain("x-part-id: part-42");
        expect(text).toContain('{"title":"hello"}');
        return jsonResponse({ ok: true });
      },
    });
    const multipart = operation({
      path: "/multipart",
      contentType: "multipart/form-data",
      requestBodies: [
        {
          contentType: "multipart/form-data",
          schema: {},
          encoding: [
            {
              name: "metadata",
              contentType: "application/json",
              headers: [{ name: "X-Part-ID", required: true, schema: { types: ["string"] } }],
            },
          ],
        },
      ],
    });
    await expect(
      request(
        multipart,
        {
          body: { metadata: { title: "hello" } },
        },
        { multipartHeaders: { metadata: { "X-Part-ID": "part-42" } } },
      ),
    ).resolves.toEqual({ ok: true });
    await expect(
      request(multipart, { body: { metadata: { title: "hello" } } }),
    ).rejects.toMatchObject({
      code: TransportErrorCode.REQUEST_ENCODE_FAILED,
    });
    await expect(
      request(
        multipart,
        { body: { metadata: { title: "hello" } } },
        { multipartHeaders: { metadata: { "X-Unknown": "no" } } },
      ),
    ).rejects.toMatchObject({ code: TransportErrorCode.REQUEST_ENCODE_FAILED });
  });

  it("selects and transforms declared multi-representation request bodies", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (_input, init) => {
        expect(new Headers(init?.headers).get("content-type")).toBe("application/json");
        expect(init?.body).toBe('{"wire_name":"widget"}');
        return new Response(null, { status: 204 });
      },
    });
    await expect(
      request(
        operation({
          path: "/multi-body",
          requestBodies: [
            {
              contentType: "application/json",
              schema: { properties: { wire_name: { property: "displayName", schema: {} } } },
            },
            { contentType: "text/plain", schema: {} },
          ],
          inputSchemas: {},
        }),
        { body: { contentType: "application/json", value: { displayName: "widget" } } },
      ),
    ).resolves.toBeUndefined();
  });

  it("applies reference, tuple, and composed schemas to request and response values", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (_input, init) => {
        expect(init?.body).toBe('[{"wire_name":"request"},{"wire_name":"extra"}]');
        return jsonResponse([{ wire_name: "response" }, { wire_name: "extra-response" }]);
      },
    });
    const itemSchema = { properties: { wire_name: { property: "displayName", schema: {} } } };
    await expect(
      request<Array<{ displayName: string }>>(
        operation({
          path: "/tuple",
          requestBodies: [{ contentType: "application/json", schema: { reference: "Tuple" } }],
          responses: [
            { status: "200", contentType: "application/json", schema: { reference: "Tuple" } },
          ],
          inputSchemas: { Tuple: { prefixItems: [itemSchema], items: itemSchema } },
          outputSchemas: { Tuple: { prefixItems: [itemSchema], items: itemSchema } },
        }),
        { body: [{ displayName: "request" }, { displayName: "extra" }] },
      ),
    ).resolves.toEqual([{ displayName: "response" }, { displayName: "extra-response" }]);
  });

  it("applies every declared composition branch to wire-name transforms", async () => {
    const composed = {
      allOf: [{ properties: { wire_first: { property: "first", schema: {} } } }],
      oneOf: [{ properties: { wire_second: { property: "second", schema: {} } } }],
      anyOf: [{ properties: { wire_third: { property: "third", schema: {} } } }],
    };
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (_input, init) => {
        expect(init?.body).toBe('{"wire_first":"one","wire_second":"two","wire_third":"three"}');
        return jsonResponse({
          wire_first: "response-one",
          wire_second: "response-two",
          wire_third: "response-three",
        });
      },
    });
    await expect(
      request<{ first: string; second: string; third: string }>(
        operation({
          path: "/composed",
          requestBodies: [{ contentType: "application/json", schema: composed }],
          responses: [{ status: "2XX", contentType: "application/json", schema: composed }],
          inputSchemas: {},
          outputSchemas: {},
        }),
        { body: { first: "one", second: "two", third: "three" } },
      ),
    ).resolves.toEqual({
      first: "response-one",
      second: "response-two",
      third: "response-three",
    });
  });

  it("times out when a fetch implementation ignores its AbortSignal", async () => {
    vi.useFakeTimers();
    try {
      const request = createRequest({
        baseURL: "https://api.example.test",
        fetch: async () => new Promise<Response>(() => undefined),
      });
      const pending = request(operation({ path: "/timeout" }), undefined, { timeoutMS: 5 });
      const result = pending.catch((cause: unknown) => cause);
      await vi.advanceTimersByTimeAsync(5);
      const error = await result;
      expect(isErrorCode(error, TransportErrorCode.REQUEST_TIMEOUT)).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });

  it("applies abort and timeout while preparing encoded and secured requests", async () => {
    vi.useFakeTimers();
    try {
      const secureOperation = operation({
        path: "/secure",
        security: [
          {
            id: "api-key",
            schemes: [
              { name: "api-key", type: "apiKey", location: "header", parameterName: "X-API-Key" },
            ],
          },
        ],
      });
      let providerCalls = 0;
      let fetchCalls = 0;
      const aborted = new AbortController();
      aborted.abort(new Error("stop"));
      const alreadyAborted = createRequest({
        baseURL: "https://api.example.test",
        securityProvider: () => {
          providerCalls++;
          return { "api-key": { kind: "api-key", value: "secret" } };
        },
        fetch: async () => {
          fetchCalls++;
          return jsonResponse({ ok: true });
        },
      });
      const abortedError = await alreadyAborted(secureOperation, undefined, {
        signal: aborted.signal,
      }).catch((cause: unknown) => cause);
      expect(isErrorCode(abortedError, TransportErrorCode.REQUEST_ABORTED)).toBe(true);
      expect(providerCalls).toBe(0);
      expect(fetchCalls).toBe(0);

      let providerSignal: AbortSignal | undefined;
      const slowSecurity = createRequest({
        baseURL: "https://api.example.test",
        securityProvider: ({ signal }) => {
          providerCalls++;
          providerSignal = signal;
          return new Promise((_, reject) =>
            signal?.addEventListener("abort", () => reject(signal.reason), { once: true }),
          );
        },
        fetch: async () => {
          fetchCalls++;
          return jsonResponse({ ok: true });
        },
      });
      const securityResult = slowSecurity(secureOperation, undefined, { timeoutMS: 5 }).catch(
        (cause: unknown) => cause,
      );
      await vi.advanceTimersByTimeAsync(5);
      expect(isErrorCode(await securityResult, TransportErrorCode.REQUEST_TIMEOUT)).toBe(true);
      expect(providerSignal?.aborted).toBe(true);
      expect(fetchCalls).toBe(0);

      const slowEncoder = createRequest({
        baseURL: "https://api.example.test",
        codecs: {
          "application/x-slow": {
            encodeParameter: async () => new Promise<string>(() => undefined),
          },
        },
        fetch: async () => {
          fetchCalls++;
          return jsonResponse({ ok: true });
        },
      });
      const encoderOperation = operation({
        path: "/items/{id}",
        parameters: [
          {
            location: "path",
            name: "id",
            property: "id",
            style: "simple",
            explode: false,
            contentType: "application/x-slow",
          },
        ],
      });
      const encoderResult = slowEncoder(
        encoderOperation,
        { path: { id: "one" } },
        { timeoutMS: 5 },
      ).catch((cause: unknown) => cause);
      await vi.advanceTimersByTimeAsync(5);
      expect(isErrorCode(await encoderResult, TransportErrorCode.REQUEST_TIMEOUT)).toBe(true);
      expect(fetchCalls).toBe(0);
    } finally {
      vi.useRealTimers();
    }
  });

  it("times out response-header decoding and cancels an unconsumed raw stream", async () => {
    vi.useFakeTimers();
    try {
      let cancelCount = 0;
      const body = new ReadableStream<Uint8Array>({
        cancel() {
          cancelCount++;
        },
      });
      const request = createRequest({
        baseURL: "https://api.example.test",
        codecs: {
          "application/x-slow-header": {
            decodeParameter: async () => new Promise<unknown>(() => undefined),
          },
        },
        fetch: async () =>
          new Response(body, {
            status: 200,
            headers: {
              "content-type": "application/x-ndjson",
              "x-slow": "value",
            },
          }),
      });
      const pending = request
        .raw(
          operation({
            path: "/raw-stream",
            responses: [
              {
                status: "200",
                contentType: "application/x-ndjson",
                schema: {},
                itemSchema: { types: ["object"] },
                headers: [
                  {
                    name: "X-Slow",
                    property: "slow",
                    contentType: "application/x-slow-header",
                    schema: {},
                  },
                ],
              },
            ],
          }),
          undefined,
          { timeoutMS: 5 },
        )
        .catch((cause: unknown) => cause);
      await vi.advanceTimersByTimeAsync(5);
      expect(isErrorCode(await pending, TransportErrorCode.REQUEST_TIMEOUT)).toBe(true);
      expect(cancelCount).toBe(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("times out generated stream reads and cancels the response reader", async () => {
    vi.useFakeTimers();
    try {
      let cancelCount = 0;
      const request = createRequest({
        baseURL: "https://api.example.test",
        fetch: async () =>
          new Response(
            new ReadableStream<Uint8Array>({
              cancel() {
                cancelCount++;
              },
            }),
            { headers: { "content-type": "application/x-ndjson" } },
          ),
      });
      const pending = collect(
        request.stream(
          operation({
            method: "GET",
            path: "/slow-stream",
            responses: [
              {
                status: "200",
                contentType: "application/x-ndjson",
                schema: {},
                itemSchema: { types: ["object"] },
              },
            ],
          }),
          undefined,
          { timeoutMS: 5 },
        ),
      ).catch((cause: unknown) => cause);
      await vi.advanceTimersByTimeAsync(5);
      expect(isErrorCode(await pending, TransportErrorCode.REQUEST_TIMEOUT)).toBe(true);
      expect(cancelCount).toBe(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("times out a custom stream codec even when its iterator ignores the signal", async () => {
    vi.useFakeTimers();
    try {
      let cancelCount = 0;
      const request = createRequest({
        baseURL: "https://api.example.test",
        codecs: {
          "application/x-slow-stream": {
            async *decodeStream() {
              await new Promise(() => undefined);
              yield { ok: true };
            },
          },
        },
        fetch: async () =>
          new Response(
            new ReadableStream<Uint8Array>({
              cancel() {
                cancelCount++;
              },
            }),
            { headers: { "content-type": "application/x-slow-stream" } },
          ),
      });
      const pending = collect(
        request.stream(
          operation({
            method: "GET",
            path: "/slow-custom-stream",
            responses: [
              {
                status: "200",
                contentType: "application/x-slow-stream",
                schema: {},
                itemSchema: { types: ["object"] },
              },
            ],
          }),
          undefined,
          { timeoutMS: 5 },
        ),
      ).catch((cause: unknown) => cause);
      await vi.advanceTimersByTimeAsync(5);
      expect(isErrorCode(await pending, TransportErrorCode.REQUEST_TIMEOUT)).toBe(true);
      expect(cancelCount).toBe(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("decodes documented text and JSON media types and preserves empty raw responses", async () => {
    const text = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        new Response("ready", { status: 200, headers: { "content-type": "text/plain" } }),
    });
    await expect(text<string>(operation({ path: "/text" }))).resolves.toBe("ready");

    const problem = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        new Response('{"title":"invalid"}', {
          status: 200,
          headers: { "content-type": "application/problem+json" },
        }),
    });
    await expect(problem<{ title: string }>(operation({ path: "/problem" }))).resolves.toEqual({
      title: "invalid",
    });

    const empty = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () => new Response(null, { status: 204 }),
    });
    await expect(empty.raw(operation({ path: "/empty" }))).resolves.toMatchObject({
      status: 204,
      data: undefined,
    });
  });

  it("returns binary response streams with raw response metadata", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        new Response(new Uint8Array([4, 5, 6]), {
          status: 200,
          headers: { "content-type": "application/octet-stream" },
        }),
    });
    const raw = await request.raw<ReadableStream<Uint8Array>>(operation({ path: "/binary" }));
    expect(raw.contentType).toBe("application/octet-stream");
    expect(raw.data).toBeInstanceOf(ReadableStream);
    expect([...new Uint8Array(await new Response(raw.data).arrayBuffer())]).toEqual([4, 5, 6]);
  });

  it("stops offset pagination at zero limit and leaves caller input unchanged", async () => {
    const calls: Array<{ query: { offset?: number; limit?: number } }> = [];
    const paginate = createPaginator<
      string,
      { query: { offset?: number; limit?: number } },
      unknown
    >(
      async (input) => {
        calls.push(input);
        return { items: ["first"], pagination: { offset: 0, limit: 0, total: 100 } };
      },
      {
        mode: "offset",
        request: { offset: "offset", limit: "limit" },
        response: {
          items: ["items"],
          offset: ["pagination", "offset"],
          limit: ["pagination", "limit"],
          total: ["pagination", "total"],
        },
      },
    );
    const input = { query: { offset: 0, limit: 0 } };
    await expect(collect(paginate(input))).resolves.toEqual(["first"]);
    expect(calls).toEqual([{ query: { offset: 0, limit: 0 } }]);
    expect(input).toEqual({ query: { offset: 0, limit: 0 } });
  });

  it("stops pagination after an empty page returned through meta pagination", async () => {
    const calls: unknown[] = [];
    const paginate = createPaginator<
      string,
      { query: { offset?: number; limit?: number } },
      unknown
    >(
      async (input) => {
        calls.push(input);
        return { items: [], meta: { pagination: { offset: 10, limit: 5 } } };
      },
      {
        mode: "offset",
        request: { offset: "offset", limit: "limit" },
        response: {
          items: ["items"],
          offset: ["meta", "pagination", "offset"],
          limit: ["meta", "pagination", "limit"],
        },
      },
    );
    await expect(collect(paginate({ query: { offset: 10, limit: 5 } }))).resolves.toEqual([]);
    expect(calls).toHaveLength(1);
  });

  it("rejects invalid base URLs and keeps plain errors unclassified", () => {
    expect(() => createRequest({ baseURL: "/relative" })).toThrow("absolute URL");
    expect(() => createRequest({ baseURL: "ftp://api.example.test" })).toThrow("http(s)");
    expect(getErrorCode(new Error("plain"))).toBeUndefined();
  });

  it("gives explicit base URLs precedence over operation servers and applies request-level transport options", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test/v1",
      credentials: "include",
      headers: { "x-client-version": "one" },
      fetch: async (input, init) => {
        expect(String(input)).toBe("https://api.example.test/v1/health");
        expect(init?.credentials).toBe("omit");
        expect(new Headers(init?.headers).get("x-client-version")).toBe("one");
        return jsonResponse({ ok: true });
      },
    });
    await expect(
      request(
        operation({
          path: "health",
          servers: [
            { id: "#/paths/~1health/get/servers/0", url: "https://alternate.example.test/v2/" },
          ],
        }),
        undefined,
        { credentials: "omit" },
      ),
    ).resolves.toEqual({ ok: true });
  });

  it("uses global fetch only when no client transport is supplied", async () => {
    const fetch = vi.fn<typeof globalThis.fetch>(async () => jsonResponse({ ok: true }));
    vi.stubGlobal("fetch", fetch);
    try {
      const request = createRequest({ baseURL: "https://api.example.test" });
      await expect(request(operation({ path: "/global" }))).resolves.toEqual({ ok: true });
      expect(fetch).toHaveBeenCalledOnce();
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("keeps structured server error details and fields", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        jsonResponse(
          {
            error: {
              code: "invalid",
              message: "bad input",
              details: { reason: "name" },
              fields: { name: "required" },
            },
          },
          422,
        ),
    });
    const error = await request(operation({ path: "/invalid" })).catch((cause: unknown) => cause);
    expect(isAPIError(error)).toBe(true);
    if (!isAPIError(error)) throw new Error("expected API error");
    expect(error.code).toBe("invalid");
    expect(error.details).toEqual({ reason: "name" });
    expect(error.fields).toEqual({ name: "required" });
  });

  it("maps additional-properties values on both request and response", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (_input, init) => {
        expect(init?.body).toBe('{"first":{"wire_name":"request"}}');
        return jsonResponse({ first: { wire_name: "response" } });
      },
    });
    await expect(
      request<{ first: { displayName: string } }>(
        operation({
          path: "/maps",
          requestBodies: [
            {
              contentType: "application/json",
              schema: {
                additionalProperties: {
                  properties: { wire_name: { property: "displayName", schema: {} } },
                },
              },
            },
          ],
          responses: [
            {
              status: "200",
              contentType: "application/json",
              schema: {
                additionalProperties: {
                  properties: { wire_name: { property: "displayName", schema: {} } },
                },
              },
            },
          ],
          inputSchemas: {},
          outputSchemas: {},
        }),
        { body: { first: { displayName: "request" } } },
      ),
    ).resolves.toEqual({ first: { displayName: "response" } });
  });

  it("prefers strict response schema branches before tolerant fallback", async () => {
    const branchA = {
      types: ["object"],
      properties: { a: { property: "alpha", schema: { types: ["number"] } } },
      additionalProperties: false,
    } as const;
    const branchB = {
      types: ["object"],
      properties: { b: { property: "beta", schema: { types: ["number"] } } },
      additionalProperties: false,
    } as const;
    const oneOfRequest = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () => jsonResponse({ a: 1 }),
    });
    await expect(
      oneOfRequest(
        operation({
          path: "/strict-one-of",
          responses: [
            {
              status: "200",
              contentType: "application/json",
              schema: { oneOf: [branchA, branchB] },
            },
          ],
          outputSchemas: {},
        }),
      ),
    ).resolves.toEqual({ alpha: 1 });

    const anyOfRequest = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () => jsonResponse({ a: 1 }),
    });
    await expect(
      anyOfRequest(
        operation({
          path: "/strict-any-of",
          responses: [
            {
              status: "200",
              contentType: "application/json",
              schema: { anyOf: [branchB, branchA] },
            },
          ],
          outputSchemas: {},
        }),
      ),
    ).resolves.toEqual({ alpha: 1 });
  });

  it("preserves unknown response properties through refs, variants, and nested schemas", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        jsonResponse({
          wire_name: "response",
          futureRoot: true,
          nested: { wire_nested: "nested", futureNested: 1 },
          choice: { kind: "known", wire_value: "variant", futureChoice: ["new"] },
          composed: { wire_detail: "detail", futureUnevaluated: { enabled: true } },
        }),
    });
    const outputSchemas = {
      Payload: {
        types: ["object"],
        required: ["wire_name", "nested", "choice", "composed"],
        properties: {
          wire_name: { property: "displayName", schema: { types: ["string"] } },
          nested: { property: "nested", schema: { reference: "Nested" } },
          choice: {
            property: "choice",
            schema: {
              oneOf: [
                {
                  types: ["object"],
                  required: ["kind", "wire_value"],
                  properties: {
                    kind: { property: "kind", schema: { constValue: "known" } },
                    wire_value: { property: "value", schema: { types: ["string"] } },
                  },
                  additionalProperties: false,
                },
                {
                  types: ["object"],
                  required: ["kind", "wire_value"],
                  properties: {
                    kind: { property: "kind", schema: { constValue: "other" } },
                    wire_value: { property: "value", schema: { types: ["number"] } },
                  },
                  additionalProperties: false,
                },
              ],
            },
          },
          composed: {
            property: "composed",
            schema: {
              types: ["object"],
              allOf: [
                {
                  properties: {
                    wire_detail: { property: "detail", schema: { types: ["string"] } },
                  },
                },
              ],
              unevaluatedProperties: false,
            },
          },
        },
        additionalProperties: false,
      },
      Nested: {
        types: ["object"],
        required: ["wire_nested"],
        properties: {
          wire_nested: { property: "nestedName", schema: { types: ["string"] } },
        },
        additionalProperties: false,
      },
    } as const;

    await expect(
      request(
        operation({
          path: "/tolerant",
          responses: [
            {
              status: "200",
              contentType: "application/json",
              schema: { reference: "Payload" },
            },
          ],
          outputSchemas,
        }),
      ),
    ).resolves.toEqual({
      displayName: "response",
      futureRoot: true,
      nested: { nestedName: "nested", futureNested: 1 },
      choice: { kind: "known", value: "variant", futureChoice: ["new"] },
      composed: { detail: "detail", futureUnevaluated: { enabled: true } },
    });
  });

  it("keeps known response fields validated while preserving unknown fields", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () => jsonResponse({ wire_name: 42, future: true }),
    });
    const error = await request(
      operation({
        path: "/invalid-known-response",
        responses: [
          {
            status: "200",
            contentType: "application/json",
            schema: {
              types: ["object"],
              required: ["wire_name"],
              properties: {
                wire_name: { property: "displayName", schema: { types: ["string"] } },
              },
              additionalProperties: false,
            },
          },
        ],
        outputSchemas: {},
      }),
    ).catch((cause: unknown) => cause);

    expect(isAPIError(error)).toBe(true);
    if (!isAPIError(error)) throw new Error("expected API error");
    expect(error.code).toBe(TransportErrorCode.RESPONSE_DECODE_FAILED);
    expect(String(error.cause)).toContain("property wire_name: expected string");
  });

  it("preserves unknown properties in ordinary and streaming error responses", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        jsonResponse(
          {
            error: { code: "future_error", message: "failed", futureDetail: true },
            futureRoot: "kept",
          },
          422,
        ),
    });
    const errorOperation = operation({
      path: "/tolerant-error",
      responses: [
        {
          status: "422",
          contentType: "application/json",
          schema: {
            types: ["object"],
            required: ["error"],
            properties: {
              error: {
                property: "error",
                schema: {
                  types: ["object"],
                  required: ["code", "message"],
                  properties: {
                    code: { property: "code", schema: { types: ["string"] } },
                    message: { property: "message", schema: { types: ["string"] } },
                  },
                  additionalProperties: false,
                },
              },
            },
            additionalProperties: false,
          },
        },
      ],
      outputSchemas: {},
    });
    const failures = [
      await request(errorOperation).catch((cause: unknown) => cause),
      await collect(request.stream(errorOperation)).catch((cause: unknown) => cause),
    ];

    for (const error of failures) {
      expect(isAPIError(error)).toBe(true);
      if (!isAPIError(error)) throw new Error("expected API error");
      expect(error.code).toBe("future_error");
      expect(error.data).toEqual({
        error: { code: "future_error", message: "failed", futureDetail: true },
        futureRoot: "kept",
      });
    }
  });

  it("releases built-in and custom stream resources exactly once on early break", async () => {
    const encoder = new TextEncoder();

    let builtInCancels = 0;
    let builtInSignal: AbortSignal | undefined;
    const builtIn = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (_input, init) => {
        builtInSignal = init?.signal as AbortSignal | undefined;
        return new Response(
          new ReadableStream<Uint8Array>({
            start(controller) {
              controller.enqueue(encoder.encode('{"value":1}\n'));
            },
            cancel() {
              builtInCancels++;
            },
          }),
          { headers: { "content-type": "application/x-ndjson" } },
        );
      },
    });
    for await (const item of builtIn.stream<{ value: number }>(
      operation({
        method: "GET",
        path: "/events",
        responses: [
          {
            status: "200",
            contentType: "application/x-ndjson",
            schema: {},
            itemSchema: {
              types: ["object"],
              required: ["value"],
              properties: { value: { property: "value", schema: { types: ["integer"] } } },
            },
          },
        ],
        outputSchemas: {},
      }),
    )) {
      expect(item).toEqual({ value: 1 });
      break;
    }
    expect(builtInCancels).toBe(1);
    expect(builtInSignal?.aborted).toBe(true);

    let customCancels = 0;
    let customReturns = 0;
    let customSignal: AbortSignal | undefined;
    const custom = createRequest({
      baseURL: "https://api.example.test",
      codecs: {
        "application/x-custom-stream": {
          decodeStream(reader) {
            let yielded = false;
            return {
              [Symbol.asyncIterator]() {
                return {
                  async next() {
                    if (yielded) return new Promise<IteratorResult<unknown>>(() => undefined);
                    yielded = true;
                    await reader.read(1);
                    return { done: false, value: { value: 1 } };
                  },
                  async return() {
                    customReturns++;
                    return { done: true, value: undefined };
                  },
                };
              },
            };
          },
        },
      },
      fetch: async (_input, init) => {
        customSignal = init?.signal as AbortSignal | undefined;
        return new Response(
          new ReadableStream<Uint8Array>({
            start(controller) {
              controller.enqueue(encoder.encode("x"));
            },
            cancel() {
              customCancels++;
            },
          }),
          { headers: { "content-type": "application/x-custom-stream" } },
        );
      },
    });
    for await (const item of custom.stream<{ value: number }>(
      operation({
        method: "GET",
        path: "/events",
        responses: [
          {
            status: "200",
            contentType: "application/x-custom-stream",
            schema: {},
            itemSchema: {
              types: ["object"],
              required: ["value"],
              properties: { value: { property: "value", schema: { types: ["integer"] } } },
            },
          },
        ],
        outputSchemas: {},
      }),
    )) {
      expect(item).toEqual({ value: 1 });
      break;
    }
    expect(customCancels).toBe(1);
    expect(customReturns).toBe(1);
    expect(customSignal?.aborted).toBe(true);
  });

  it("releases stream resources once on framing, schema, and codec failures without leaking payloads", async () => {
    const encoder = new TextEncoder();
    const secret = "TOP_SECRET_STREAM_PAYLOAD";
    const cases: Array<{
      contentType: string;
      wire: string;
      itemSchema: WireSchema;
      codecs?: Parameters<typeof createRequest>[0]["codecs"];
      cause: string;
    }> = [
      {
        contentType: "application/x-ndjson",
        wire: `${secret}\n`,
        itemSchema: { types: ["object"] },
        cause: "stream item is not valid JSON",
      },
      {
        contentType: "application/x-ndjson",
        wire: '{"wire_name":42}\n',
        itemSchema: {
          types: ["object"],
          required: ["wire_name"],
          properties: {
            wire_name: { property: "displayName", schema: { types: ["string"] } },
          },
        },
        cause: "expected string",
      },
      {
        contentType: "application/x-codec-failure",
        wire: "x",
        itemSchema: { types: ["object"] },
        codecs: {
          "application/x-codec-failure": {
            decodeStream: async function* (reader) {
              await reader.read(1);
              throw new TypeError("codec framing failed");
            },
          },
        },
        cause: "codec framing failed",
      },
    ];

    for (const test of cases) {
      let cancels = 0;
      const request = createRequest({
        baseURL: "https://api.example.test",
        ...(test.codecs === undefined ? {} : { codecs: test.codecs }),
        fetch: async () =>
          new Response(
            new ReadableStream<Uint8Array>({
              start(controller) {
                controller.enqueue(encoder.encode(test.wire));
              },
              cancel() {
                cancels++;
              },
            }),
            { headers: { "content-type": test.contentType } },
          ),
      });
      const error = await collect(
        request.stream(
          operation({
            method: "GET",
            path: "/events",
            responses: [
              {
                status: "200",
                contentType: test.contentType,
                schema: {},
                itemSchema: test.itemSchema,
              },
            ],
            outputSchemas: {},
          }),
        ),
      ).catch((cause: unknown) => cause);

      expect(isAPIError(error)).toBe(true);
      if (!isAPIError(error)) throw new Error("expected API error");
      expect(error.code).toBe(TransportErrorCode.RESPONSE_DECODE_FAILED);
      expect(error.message).toContain("runtimeTest");
      expect(error.message).toContain(test.contentType);
      expect(String(error.cause)).toContain(test.cause);
      expect(error.message).not.toContain(secret);
      expect(String(error.cause)).not.toContain(secret);
      expect(cancels).toBe(1);
    }
  });

  it("cancels a pending response reader exactly once on external stream abort", async () => {
    let cancels = 0;
    let fetched = false;
    let receivedSignal: AbortSignal | undefined;
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (_input, init) => {
        fetched = true;
        receivedSignal = init?.signal as AbortSignal | undefined;
        return new Response(
          new ReadableStream<Uint8Array>({
            cancel() {
              cancels++;
            },
          }),
          { headers: { "content-type": "application/x-ndjson" } },
        );
      },
    });
    const controller = new AbortController();
    const iterator = request
      .stream(
        operation({
          method: "GET",
          path: "/events",
          responses: [
            {
              status: "200",
              contentType: "application/x-ndjson",
              schema: {},
              itemSchema: { types: ["object"] },
            },
          ],
          outputSchemas: {},
        }),
        undefined,
        { signal: controller.signal },
      )
      [Symbol.asyncIterator]();
    const pending = iterator.next().catch((cause: unknown) => cause);
    await vi.waitFor(() => expect(fetched).toBe(true));
    controller.abort("stop");
    const error = await pending;
    expect(isErrorCode(error, TransportErrorCode.REQUEST_ABORTED)).toBe(true);
    expect(receivedSignal?.aborted).toBe(true);
    expect(cancels).toBe(1);
  });

  it("enforces generated stream item limits independently of chunk boundaries", async () => {
    const itemSchema = { types: ["object"] } as const;
    const streamOperation = (contentType: string): OperationDefinition =>
      operation({
        method: "GET",
        path: "/bounded-stream",
        responses: [{ status: "200", contentType, schema: {}, itemSchema }],
        outputSchemas: {},
      });
    const payload = JSON.stringify({ value: "x".repeat(64) });
    const cases = [
      { contentType: "application/x-ndjson", wire: `${payload}\n` },
      { contentType: "application/json-seq", wire: `\u001e${payload}\n\u001e` },
      { contentType: "text/event-stream", wire: `data: ${payload}\r\r` },
      {
        contentType: "multipart/mixed; boundary=frame",
        declaredContentType: "multipart/mixed",
        wire: `--frame\r\nContent-Type: application/json\r\n\r\n${payload}\r\n--frame--\r\n`,
      },
    ];
    for (const item of cases) {
      for (const chunks of [
        [item.wire],
        [
          item.wire.slice(0, Math.floor(item.wire.length / 2)),
          item.wire.slice(Math.floor(item.wire.length / 2)),
        ],
      ]) {
        const request = createRequest({
          baseURL: "https://api.example.test",
          maxStreamItemBytes: 32,
          fetch: async () =>
            new Response(
              new ReadableStream({
                start(controller) {
                  const encoder = new TextEncoder();
                  for (const chunk of chunks) controller.enqueue(encoder.encode(chunk));
                  controller.close();
                },
              }),
              { headers: { "content-type": item.contentType } },
            ),
        });
        const error = await collect(
          request.stream(
            streamOperation(item.declaredContentType ?? item.contentType.split(";", 1)[0]!),
          ),
        ).catch((cause: unknown) => cause);
        expect(isErrorCode(error, TransportErrorCode.RESPONSE_DECODE_FAILED)).toBe(true);
        expect(String((error as { cause?: unknown }).cause)).toContain("exceeds 32 bytes");
      }
    }
  });

  it("accepts CR-only SSE framing and discards an incomplete event at EOF", async () => {
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        new Response('data: {"value":1}\r\rdata: {"value":2}', {
          headers: { "content-type": "text/event-stream" },
        }),
    });
    await expect(
      collect(
        request.stream(
          operation({
            method: "GET",
            path: "/events",
            responses: [
              {
                status: "200",
                contentType: "text/event-stream",
                schema: {},
                itemSchema: { types: ["object"] },
              },
            ],
            outputSchemas: {},
          }),
        ),
      ),
    ).resolves.toEqual([{ data: '{"value":1}' }]);
  });

  it("parses SSE event objects across arbitrary UTF-8 and line boundaries", async () => {
    const wire =
      "\uFEFF: ignored comment\r\n" +
      "event: update\r\n" +
      "data: hello 🌍\r\n" +
      "data:  second line\r\n" +
      "id: event-1\r\n" +
      "retry: 42\r\n" +
      "unknownField: ignored\r\n" +
      "\r\n" +
      "data\r" +
      "event\r" +
      "id: kept\r" +
      "id: bad\u0000value\r" +
      "retry: 7\r" +
      "retry: nope\r" +
      "\r" +
      "data: [DONE]\n\n" +
      "data: incomplete";
    const bytes = new TextEncoder().encode(wire);
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        new Response(
          new ReadableStream({
            start(controller) {
              for (const byte of bytes) controller.enqueue(new Uint8Array([byte]));
              controller.close();
            },
          }),
          { headers: { "content-type": "text/event-stream" } },
        ),
    });

    await expect(
      collect(
        request.stream(
          operation({
            method: "GET",
            path: "/events",
            responses: [
              {
                status: "200",
                contentType: "text/event-stream",
                schema: {},
                itemSchema: { types: ["object"] },
              },
            ],
            outputSchemas: {},
          }),
        ),
      ),
    ).resolves.toEqual([
      {
        data: "hello 🌍\n second line",
        event: "update",
        id: "event-1",
        retry: 42,
      },
      { data: "", event: "", id: "kept", retry: 7 },
      { data: "[DONE]" },
    ]);
  });

  it("applies item-schema transformation to parsed SSE event objects", async () => {
    const itemSchema = {
      types: ["object"],
      required: ["data"],
      properties: {
        data: { property: "payload", schema: { types: ["string"] } },
        event: { property: "eventType", schema: { types: ["string"] } },
        retry: { property: "retryAfter", schema: { types: ["integer"] } },
      },
    } as const;
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        new Response('event: delta\ndata: {"token":"a"}\nretry: 5\n\n', {
          headers: { "content-type": "text/event-stream" },
        }),
    });

    await expect(
      collect(
        request.stream(
          operation({
            method: "GET",
            path: "/events",
            responses: [
              {
                status: "200",
                contentType: "text/event-stream",
                schema: {},
                itemSchema,
              },
            ],
            outputSchemas: {},
          }),
        ),
      ),
    ).resolves.toEqual([{ payload: '{"token":"a"}', eventType: "delta", retryAfter: 5 }]);
  });

  it("lets stream codecs override built-in framing in both directions", async () => {
    const itemSchema = {
      types: ["object"],
      required: ["wire_name"],
      properties: {
        wire_name: { property: "displayName", schema: { types: ["string"] } },
      },
      additionalProperties: false,
    } as const;
    let responseMaxFrameBytes = 0;
    let responseWire = "";
    const decoder = new TextDecoder();
    const encoder = new TextEncoder();
    const codec = {
      decodeStream: async function* (
        reader: {
          read(maxBytes: number): Promise<Uint8Array | null>;
        },
        context: { maxFrameBytes: number },
      ) {
        responseMaxFrameBytes = context.maxFrameBytes;
        for (;;) {
          const bytes = await reader.read(context.maxFrameBytes);
          if (bytes === null) break;
          responseWire += decoder.decode(bytes);
        }
        yield { wire_name: "decoded" };
      },
      encodeStream: (items: AsyncIterable<unknown>) => {
        const iterator = items[Symbol.asyncIterator]();
        return new ReadableStream<Uint8Array>({
          async pull(controller) {
            const next = await iterator.next();
            if (next.done) {
              controller.close();
              return;
            }
            const item = next.value as { wire_name: string };
            controller.enqueue(encoder.encode(`override:${item.wire_name}\n`));
          },
          async cancel(reason) {
            await iterator.return?.(reason);
          },
        });
      },
    };

    const responseRequest = createRequest({
      baseURL: "https://api.example.test",
      maxStreamItemBytes: 4,
      codecs: { "text/event-stream": codec },
      fetch: async () =>
        new Response("not-sse-framing", {
          headers: { "content-type": "text/event-stream" },
        }),
    });
    await expect(
      collect(
        responseRequest.stream(
          operation({
            method: "GET",
            path: "/events",
            responses: [
              {
                status: "200",
                contentType: "text/event-stream",
                schema: {},
                itemSchema,
              },
            ],
            outputSchemas: {},
          }),
        ),
      ),
    ).resolves.toEqual([{ displayName: "decoded" }]);
    expect(responseMaxFrameBytes).toBe(4);
    expect(responseWire).toBe("not-sse-framing");

    let requestWire = "";
    const requestRequest = createRequest({
      baseURL: "https://api.example.test",
      codecs: { "text/event-stream": codec },
      fetch: async (_input, init) => {
        requestWire = await new Response(init?.body).text();
        return jsonResponse({ ok: true });
      },
    });
    async function* items() {
      yield { displayName: "encoded" };
    }
    await expect(
      requestRequest(
        operation({
          path: "/events",
          contentType: "text/event-stream",
          requestBodyRequired: true,
          requestBodies: [
            {
              contentType: "text/event-stream",
              schema: {},
              itemSchema,
            },
          ],
          inputSchemas: {},
        }),
        { body: items() },
      ),
    ).resolves.toEqual({ ok: true });
    expect(requestWire).toBe("override:encoded\n");
  });

  it("preserves unknown properties in generated and custom stream items", async () => {
    const itemSchema = {
      types: ["object"],
      required: ["wire_name"],
      properties: {
        wire_name: { property: "displayName", schema: { types: ["string"] } },
      },
      additionalProperties: false,
    } as const;
    const streamOperation = (contentType: string): OperationDefinition =>
      operation({
        path: "/tolerant-stream",
        responses: [{ status: "200", contentType, schema: {}, itemSchema }],
        outputSchemas: {},
      });
    const generated = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        new Response('{"wire_name":"generated","future":true}\n', {
          headers: { "content-type": "application/x-ndjson" },
        }),
    });
    const custom = createRequest({
      baseURL: "https://api.example.test",
      codecs: {
        "application/x-tolerant-stream": {
          decodeStream: async function* () {
            yield { wire_name: "custom", future: true };
          },
        },
      },
      fetch: async () =>
        new Response("stream", {
          headers: { "content-type": "application/x-tolerant-stream" },
        }),
    });

    await expect(
      collect(generated.stream(streamOperation("application/x-ndjson"))),
    ).resolves.toEqual([{ displayName: "generated", future: true }]);
    await expect(
      collect(custom.stream(streamOperation("application/x-tolerant-stream"))),
    ).resolves.toEqual([{ displayName: "custom", future: true }]);
  });

  it("encodes standard SSE request event objects and rejects non-stream bodies before fetch", async () => {
    const bodies: string[] = [];
    const fetch = vi.fn<typeof globalThis.fetch>(async (_input, init) => {
      bodies.push(await new Response(init?.body).text());
      return jsonResponse({ ok: true });
    });
    const request = createRequest({ baseURL: "https://api.example.test", fetch });
    const streamOperation = operation({
      path: "/events",
      contentType: "text/event-stream",
      requestBodyRequired: true,
      requestBodies: [
        {
          contentType: "text/event-stream",
          schema: {},
          itemSchema: { types: ["object"] },
        },
      ],
      inputSchemas: {},
    });
    async function* events() {
      yield {
        event: "delta",
        data: "first\n second",
        id: "event-1",
        retry: 25,
      };
      yield { data: "" };
    }

    await expect(request(streamOperation, { body: events() })).resolves.toEqual({ ok: true });
    expect(bodies).toEqual([
      "event: delta\ndata: first\ndata:  second\nid: event-1\nretry: 25\n\ndata: \n\n",
    ]);

    const error = await request(streamOperation, { body: { data: "not-a-stream" } }).catch(
      (cause: unknown) => cause,
    );
    expect(isErrorCode(error, TransportErrorCode.REQUEST_ENCODE_FAILED)).toBe(true);
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("validates SSE request fields while preserving request-stream cancellation", async () => {
    let iteratorReturnCount = 0;
    const streamOperation = operation({
      path: "/events",
      contentType: "text/event-stream",
      requestBodyRequired: true,
      requestBodies: [
        {
          contentType: "text/event-stream",
          schema: {},
          itemSchema: { types: ["object"] },
        },
      ],
      inputSchemas: {},
    });

    const invalidCases = [
      { data: 1 },
      { data: "ok", event: "bad\nname" },
      { data: "ok", id: "bad\u0000id" },
      { data: "ok", retry: -1 },
      { data: "ok", extra: true },
    ];
    for (const invalid of invalidCases) {
      const request = createRequest({
        baseURL: "https://api.example.test",
        fetch: async (_input, init) => {
          await new Response(init?.body).text();
          return jsonResponse({ ok: true });
        },
      });
      async function* values() {
        yield invalid;
      }
      const error = await request(streamOperation, { body: values() }).catch(
        (cause: unknown) => cause,
      );
      expect(isErrorCode(error, TransportErrorCode.REQUEST_ENCODE_FAILED)).toBe(true);
    }

    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (_input, init) => {
        const reader = (init?.body as ReadableStream<Uint8Array>).getReader();
        const first = await reader.read();
        expect(new TextDecoder().decode(first.value)).toBe("data: first\n\n");
        await reader.cancel("stop");
        return jsonResponse({ ok: true });
      },
    });
    async function* cancellable() {
      try {
        yield { data: "first" };
        yield { data: "second" };
      } finally {
        iteratorReturnCount++;
      }
    }
    await expect(request(streamOperation, { body: cancellable() })).resolves.toEqual({ ok: true });
    expect(iteratorReturnCount).toBe(1);
  });

  it("keeps request encoding strict for closed objects", async () => {
    const fetch = vi.fn<typeof globalThis.fetch>(async () => jsonResponse({ ok: true }));
    const request = createRequest({ baseURL: "https://api.example.test", fetch });
    const error = await request(
      operation({
        path: "/strict-request",
        requestBodies: [
          {
            contentType: "application/json",
            schema: {
              types: ["object"],
              properties: { known: { property: "known", schema: { types: ["string"] } } },
              additionalProperties: false,
            },
          },
        ],
        inputSchemas: {},
      }),
      { body: { known: "value", future: true } },
    ).catch((cause: unknown) => cause);

    expect(isAPIError(error)).toBe(true);
    if (!isAPIError(error)) throw new Error("expected API error");
    expect(error.code).toBe(TransportErrorCode.REQUEST_ENCODE_FAILED);
    expect(fetch).not.toHaveBeenCalled();
  });

  it("honors cancellation even when fetch ignores its AbortSignal", async () => {
    let receivedSignal: AbortSignal | undefined;
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async (_input, init) => {
        receivedSignal = init?.signal as AbortSignal | undefined;
        return new Promise<Response>(() => undefined);
      },
    });
    const controller = new AbortController();
    const pending = request(operation({ path: "/slow" }), undefined, { signal: controller.signal });
    controller.abort("cancelled");
    const error = await pending.catch((cause: unknown) => cause);
    expect(receivedSignal).toBeDefined();
    expect(isErrorCode(error, TransportErrorCode.REQUEST_ABORTED)).toBe(true);
  });

  it("preserves response metadata when cancellation interrupts decoding", async () => {
    let release: (() => void) | undefined;
    let resolveFetch: ((response: Response) => void) | undefined;
    const response = jsonResponse({ ok: true }, 200, { "x-request-id": "server-1" });
    vi.spyOn(response, "json").mockImplementation(
      () =>
        new Promise<unknown>((resolve) => {
          release = () => resolve({ ok: true });
        }),
    );
    const request = createRequest({
      baseURL: "https://api.example.test",
      fetch: async () =>
        new Promise<Response>((resolve) => {
          resolveFetch = resolve;
        }),
    });
    const controller = new AbortController();
    const pending = request(operation({ path: "/decode" }), undefined, {
      signal: controller.signal,
    });
    await Promise.resolve();
    resolveFetch?.(response);
    await vi.waitFor(() => expect(release).toBeTypeOf("function"));
    controller.abort();
    const error = await pending.catch((cause: unknown) => cause);
    release?.();
    expect(isAPIError(error)).toBe(true);
    if (!isAPIError(error)) throw new Error("expected API error");
    expect(error.code).toBe(TransportErrorCode.REQUEST_ABORTED);
    expect(error.status).toBe(200);
    expect(error.request).toEqual({ id: "server-1" });
  });
});
