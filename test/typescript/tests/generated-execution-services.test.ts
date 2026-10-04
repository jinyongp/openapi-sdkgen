import { describe, expect, it } from "vitest";
import { bindBase } from "../fixtures/generated/lifecycle/internal/operations/inline/post.js";
import { createRequestCore } from "../fixtures/generated/lifecycle/internal/runtime/http/request/http-request-core.js";
import { createRequestContext } from "../fixtures/generated/lifecycle/internal/runtime/http/http-execution-support.js";
import { createBasicHTTPServices } from "../fixtures/generated/lifecycle/internal/runtime/http/request/http-basic.js";
import { createBasicProgramCodec } from "../fixtures/generated/lifecycle/internal/runtime/schema/program-basic.js";
import { encodeJSONBody } from "../fixtures/generated/lifecycle/internal/runtime/media/http-body-json.js";
import type { RequestExecutionServices } from "../../../internal/target/typescript/runtime/http/http-types.js";
const jsonRequestServices: RequestExecutionServices = createBasicHTTPServices(
  createBasicProgramCodec({}),
  { encodeRequestBody: encodeJSONBody },
);

describe("generated operation execution services", () => {
  it("binds an actual emitted definition to a buffered executor without a stream assertion", async () => {
    const requests: Array<{ url: string; body: unknown }> = [];
    const context = createRequestContext({
      baseURL: "https://example.test",
      fetch: async (url, init) => {
        const body: unknown = JSON.parse(String(init?.body));
        requests.push({ url: String(url), body });
        return Response.json(body);
      },
    });
    const request = createRequestCore(context, jsonRequestServices);
    const operation = bindBase(request);
    const body = { value: 42, nested: { flag: true } };
    expect(await operation({ body })).toEqual(body);
    expect((await operation.raw({ body })).data).toEqual(body);
    expect(requests).toEqual([
      { url: "https://example.test/inline", body },
      { url: "https://example.test/inline", body },
    ]);
    const before = requests.length;
    await expect(operation({ body: { value: -1 } })).rejects.toMatchObject({
      code: "REQUEST_ENCODE_FAILED",
    });
    expect(requests.length).toBe(before);
    expect("stream" in request).toBe(false);
  });
});

function generatedCallTypeWitness() {
  const request = createRequestCore(
    createRequestContext({ fetch: async () => Response.json({ value: 1 }) }),
    jsonRequestServices,
  );
  const operation = bindBase(request);
  void operation({ body: { value: 1 } });
  // @ts-expect-error The real generated input remains numeric; buffered binding does not erase it.
  void operation({ body: { value: "incorrect" } });
  // @ts-expect-error A non-streaming generated operation does not acquire a stream method.
  operation.stream;
}
void generatedCallTypeWitness;
