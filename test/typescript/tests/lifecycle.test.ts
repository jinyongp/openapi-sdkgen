import { describe, expect, it, vi } from "vitest";
import { createClient } from "../fixtures/generated/lifecycle/index.js";
import type { OperationInput } from "../fixtures/generated/lifecycle/index.js";

const input: OperationInput<"echoInline"> = { body: { value: 0, nested: { flag: false } } };
// @ts-expect-error Inline value must remain a number.
const invalid: OperationInput<"echoInline"> = { body: { value: "wrong" } };
void invalid;

describe("inline descriptor lifecycle fixture", () => {
  it("preserves nested inputs, response values and stream frames", async () => {
    const fetch = vi.fn<typeof globalThis.fetch>(async (url, options) => {
      if (new URL(String(url)).pathname === "/events") {
        return new Response('{"value":1}\n{"value":2}\n', {
          headers: { "content-type": "application/x-ndjson" },
        });
      }
      return new Response(String(options?.body), {
        headers: { "content-type": "application/json" },
      });
    });
    const api = createClient({ baseURL: "https://example.test", fetch });
    expect(await api.$operations.echoInline(input)).toEqual(input.body);
    const frames = [];
    for await (const value of api.$operations.events.stream()) frames.push(value);
    expect(frames).toEqual([{ value: 1 }, { value: 2 }]);
    await expect(api.$operations.echoInline({ body: { value: -1 } })).rejects.toBeDefined();
    expect(fetch).toHaveBeenCalledTimes(2);
  });
});
