import { describe, expect, it, vi } from "vitest";
import {
  createClient,
  type Components,
  type OperationInput,
} from "../fixtures/generated/representation/index.js";

const body: Components["Node"]["input"] = {
  label: "root",
  constructor: "own",
  left: { value: 0 },
  right: { value: 1 },
  next: { label: "child", constructor: "own", left: { value: 2 }, right: { value: 3 } },
};
const input: OperationInput<"echo-record"> = { path: { id: "record/one" }, body };
// @ts-expect-error A required path section must not disappear during section refactoring.
const missingPath: OperationInput<"echo-record"> = { body };
const invalidLeaf: Components["Node"]["input"] = {
  label: "root",
  constructor: "own",
  // @ts-expect-error Node.left is a Leaf, not a scalar.
  left: "bad",
  right: { value: 0 },
};
void [missingPath, invalidLeaf];

describe("reusable representation fixture", () => {
  it("preserves recursive schemas and exact property names through transport", async () => {
    const fetch = vi.fn<typeof globalThis.fetch>(async (url, options) => {
      expect(new URL(String(url)).pathname).toBe("/records/record%2Fone");
      return new Response(String(options?.body), {
        headers: { "content-type": "application/json" },
      });
    });
    const api = createClient({ baseURL: "https://api.example.test", fetch });
    // Spread creates own data properties, including an exact __proto__ key.
    const exact = {
      ...body,
      ...JSON.parse('{"__proto__":"own","foo-bar":"hyphen","foo_bar":"underscore"}'),
    };
    const result = await api.$operations["echo-record"]({ ...input, body: exact });
    expect(JSON.parse(JSON.stringify(result))).toEqual(JSON.parse(JSON.stringify(exact)));
    expect(Object.hasOwn(result, "__proto__")).toBe(true);
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("keeps request validation before fetch", async () => {
    const fetch = vi.fn<typeof globalThis.fetch>();
    const api = createClient({ baseURL: "https://api.example.test", fetch });
    await expect(
      api.$operations["echo-record"]({ ...input, body: { ...body, left: { value: -1 } } }),
    ).rejects.toBeDefined();
    expect(fetch).not.toHaveBeenCalled();
  });

  it("supports no-input and empty-object operations without changing the public surface", async () => {
    const api = createClient({
      baseURL: "https://api.example.test",
      fetch: async (url) =>
        new URL(String(url)).pathname === "/ping"
          ? new Response(null, { status: 204 })
          : Response.json({}),
    });
    await expect(api.$operations.ping()).resolves.toBeUndefined();
    await expect(api.$operations.empty()).resolves.toEqual({});
    expect(api.$operations["echo-record"]).toBe(api.$routes["POST /records/{id}"]);
  });
});
