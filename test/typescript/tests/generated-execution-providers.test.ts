import { describe, expect, it } from "vitest";
import { provider as jsonProvider } from "../fixtures/generated/lifecycle/internal/executions/inline/post.js";
import { provider as streamProvider } from "../fixtures/generated/lifecycle/internal/executions/events/get.js";
import { createClient } from "../fixtures/generated/lifecycle/index.js";
import { createRequestContext } from "../fixtures/generated/lifecycle/internal/runtime/http/http-execution-support.js";

describe("compiler-emitted execution providers", () => {
  it("automatically selects and binds JSON services with the real input and raw contract", async () => {
    const requests: Array<{ url: string; body: unknown; authorization: string | null }> = [];
    const options = {
      baseURL: "https://example.test",
      authorization: "Bearer isolated",
      fetch: async (url: RequestInfo | URL, init?: RequestInit) => {
        const body: unknown = JSON.parse(String(init?.body));
        requests.push({
          url: String(url),
          body,
          authorization: new Headers(init?.headers).get("authorization"),
        });
        return Response.json(body, { headers: { "x-request-id": "request-1" } });
      },
    };
    const operation = jsonProvider.bind(createRequestContext(options));
    expect(requests).toHaveLength(0);
    expect(jsonProvider.profile).toBe("json");
    expect(jsonProvider.route).toBe("POST /inline");
    expect(jsonProvider.operationID).toBe("echoInline");
    const body = { value: 42, nested: { flag: true } };
    expect(await operation({ body })).toEqual(body);
    expect((await operation.raw({ body })).data).toEqual(body);
    expect(await createClient(options).$operations.echoInline({ body })).toEqual(body);
    expect(requests).toEqual(
      Array.from({ length: 3 }, () => ({
        url: "https://example.test/inline",
        body,
        authorization: "Bearer isolated",
      })),
    );
    expect("stream" in operation).toBe(false);
  });

  it("preserves schema rejection and abort before fetch", async () => {
    let requests = 0;
    const operation = jsonProvider.bind(
      createRequestContext({
        baseURL: "https://example.test",
        fetch: async () => {
          requests++;
          return Response.json({ value: 1 });
        },
      }),
    );
    await expect(operation({ body: { value: -1 } })).rejects.toMatchObject({
      code: "REQUEST_ENCODE_FAILED",
    });
    await expect(
      operation({ body: { value: 1 } }, { signal: AbortSignal.abort() }),
    ).rejects.toMatchObject({ code: "REQUEST_ABORTED" });
    expect(requests).toBe(0);
  });

  it("generates streaming services without changing the synchronous stream handle", async () => {
    let requests = 0;
    const operation = streamProvider.bind(
      createRequestContext({
        baseURL: "https://example.test",
        fetch: async () => {
          requests++;
          return new Response('{"value":1}\n{"value":2}\n', {
            headers: { "content-type": "application/x-ndjson" },
          });
        },
      }),
    );
    expect(streamProvider.profile).toBe("json-response-stream");
    const stream = operation.stream();
    expect(typeof stream[Symbol.asyncIterator]).toBe("function");
    expect("then" in stream).toBe(false);
    expect(requests).toBe(0);
    const values = [];
    for await (const item of stream) values.push(item.value);
    expect(values).toEqual([1, 2]);
    expect(requests).toBe(1);
  });

  it("shares the provider code without sharing client credentials or base URL", async () => {
    const requests: string[] = [];
    const fetch: typeof globalThis.fetch = async (url, init) => {
      requests.push(String(url) + " " + new Headers(init?.headers).get("authorization"));
      return Response.json({ value: 1 });
    };
    const first = jsonProvider.bind(
      createRequestContext({ baseURL: "https://first.test", authorization: "Bearer first", fetch }),
    );
    const second = jsonProvider.bind(
      createRequestContext({
        baseURL: "https://second.test",
        authorization: "Bearer second",
        fetch,
      }),
    );
    expect(first).not.toBe(second);
    await Promise.all([first({ body: { value: 1 } }), second({ body: { value: 1 } })]);
    expect(requests.sort()).toEqual([
      "https://first.test/inline Bearer first",
      "https://second.test/inline Bearer second",
    ]);
  });
});

function compilerProviderTypeWitness() {
  const context = createRequestContext({
    baseURL: "https://example.test",
    fetch: async () => Response.json({ value: 1 }),
  });
  const json = jsonProvider.bind(context);
  void json({ body: { value: 1 } });
  // @ts-expect-error The provider preserves the generated input, rather than returning an untyped callable.
  void json({ body: { value: "wrong" } });
  // @ts-expect-error JSON preparation does not create an unimplemented stream method.
  json.stream;
  const stream = streamProvider.bind(context).stream();
  void stream;
}
void compilerProviderTypeWitness;
