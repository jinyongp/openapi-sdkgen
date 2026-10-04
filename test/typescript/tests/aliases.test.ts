import { describe, expect, it, vi } from "vitest";
import { createClient } from "../fixtures/generated/aliases/index.js";
import type { Components, OperationInput } from "../fixtures/generated/aliases/index.js";

type EnvelopeInput = Components["Envelope"]["input"];
type EnvelopeOutput = Components["Envelope"]["output"];
const body: EnvelopeInput = {
  left: { kind: "hyphen", value: "left", writeValue: "input-left" },
  right: { kind: "hyphen", value: "right", writeValue: "input-right" },
  otherLeft: { kind: "underscore", value: 1 },
  otherRight: { kind: "underscore", value: 2 },
  composedLeft: "composed",
  decomposedLeft: 3,
  __sdkgen_t_d0: "__sdkgen_r_d0",
  opaque: { __sdkgen_t_d0: "__sdkgen_r_d0", ["__proto__"]: "exact", property: "schema" },
};
const input: OperationInput<"echoAliases"> = { path: { aliasID: "record/one" }, body };
// @ts-expect-error The normalized-looking underscore component still requires a number.
const wrongComponent: Components["foo_bar"]["input"]["value"] = "not a number";
// @ts-expect-error Input projection retains its write-only required property.
const missingInput: Components["foo-bar"]["input"] = { kind: "hyphen", value: "x" };
// @ts-expect-error Output projection retains its read-only required property.
const missingOutput: Components["foo-bar"]["output"] = { kind: "hyphen", value: "x" };
// @ts-expect-error Canonically equivalent Unicode names remain different schemas.
const wrongUnicode: EnvelopeInput["decomposedLeft"] = "not a number";
// @ts-expect-error A path is still required despite local input name reuse.
const missingPath: OperationInput<"echoAliases"> = { body };
void [wrongComponent, missingInput, missingOutput, wrongUnicode, missingPath];

function checkProjection(value: EnvelopeOutput) {
  const text: string = value.left.readValue;
  const numeric: number = value.otherLeft.value;
  // @ts-expect-error Write-only data is absent from the output type.
  const writeOnly = value.left.writeValue;
  void [text, numeric, writeOnly];
}
void checkProjection;

describe("artifact-owned aliases", () => {
  it("keeps distinct schemas, projections and private-looking data in transport", async () => {
    const fetch = vi.fn<typeof globalThis.fetch>(async (url, options) => {
      expect(new URL(String(url)).pathname).toBe("/aliases/record%2Fone");
      const received = JSON.parse(String(options?.body)) as Record<string, unknown>;
      expect(received).toEqual(body);
      return Response.json({
        ...body,
        left: { kind: "hyphen", value: "left", readValue: "left" },
        right: { kind: "hyphen", value: "right", readValue: "right" },
      });
    });
    const api = createClient({ baseURL: "https://example.test", fetch });
    const output = await api.$operations.echoAliases(input);
    expect(output.left.readValue).toBe("left");
    expect(output.otherLeft.value).toBe(1);
    expect(output.decomposedLeft).toBe(3);
    expect(output.__sdkgen_t_d0).toBe("__sdkgen_r_d0");
    expect(Object.hasOwn(output.opaque ?? {}, "__proto__")).toBe(true);
    expect(output.opaque?.__proto__).toBe("exact");
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("shares an owner between schema aliases, Link groups and streaming", async () => {
    const requests: string[] = [];
    const api = createClient({
      baseURL: "https://example.test",
      fetch: async (url, options) => {
        const request = new Request(url, options);
        const path = new URL(request.url).pathname;
        requests.push(path);
        if (path !== "/linked-events") return new Response(null, { status: 204 });
        if (request.headers.get("accept")?.includes("application/x-ndjson"))
          return new Response("1\n2\n", { headers: { "content-type": "application/x-ndjson" } });
        return Response.json([1, 2]);
      },
    });
    const raw = await api.$operations.linkedEvents.raw();
    await api.$operations.linkedEvents.links.follow(raw);
    const frames: number[] = [];
    for await (const frame of api.$operations.linkedEvents.stream()) frames.push(frame);
    expect(frames).toEqual([1, 2]);
    expect(requests).toEqual(["/linked-events", "/left/linked/details", "/linked-events"]);
  });

  it("resolves each resource builder to its own route", async () => {
    const requests: string[] = [];
    const api = createClient({
      baseURL: "https://example.test",
      fetch: async (url) => {
        requests.push(new URL(String(url)).pathname);
        return new Response(null, { status: 204 });
      },
    });
    await api.left("one").details.get();
    await api.right("two").details.get();
    expect(requests).toEqual(["/left/one/details", "/right/two/details"]);
    expect(api.$operations.leftDetails).toBe(api.$routes["GET /left/{id}/details"]);
  });
});
