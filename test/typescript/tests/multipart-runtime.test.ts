import { describe, expect, it, vi } from "vitest";

import { createAdvancedHTTPServices } from "../../../internal/target/typescript/runtime/internal/http-advanced.js";
import { jsonWireCodec } from "../../../internal/target/typescript/runtime/internal/wire-engine.js";
import type {
  MediaCodec,
  WireBodyDefinition,
} from "../../../internal/target/typescript/runtime/internal/wire-engine.js";

const services = createAdvancedHTTPServices(() => {
  throw new Error("base HTTP services are not used by multipart codec tests");
}, jsonWireCodec);

describe("advanced multipart runtime", () => {
  it("round-trips positional multipart JSON, text and binary parts", async () => {
    const definition: WireBodyDefinition = {
      contentType: "multipart/mixed",
      schema: {
        types: ["array"],
        prefixItems: [{ types: ["object"] }, { types: ["string"] }, { contentEncoding: "binary" }],
      },
      prefixEncoding: [
        { contentType: "application/json" },
        { contentType: "text/plain" },
        { contentType: "application/octet-stream" },
      ],
    };
    const binary = new Uint8Array([1, 2, 3, 4]).buffer;

    const encoded = await services.encodeRequestBody(
      definition.contentType,
      [{ id: 7 }, "hello", binary],
      new Map(),
      definition.schema,
      {},
      definition,
      undefined,
      undefined,
    );
    expect(encoded).toBeInstanceOf(Blob);
    const blob = encoded as Blob;
    expect(blob.type).toMatch(/^multipart\/mixed; boundary=----openapi-sdkgen-/);
    const text = await blob.text();
    expect(text).toContain("Content-Type: application/json");
    expect(text).toContain('{"id":7}');
    expect(text).toContain("Content-Type: text/plain");
    expect(text).toContain("hello");
    expect(text).toContain("Content-Type: application/octet-stream");

    const decoded = await services.decodeMultipartResponse(
      blob.stream(),
      blob.type,
      definition,
      {},
      new Map(),
    );
    expect(decoded[0]).toEqual({ id: 7 });
    expect(decoded[1]).toBe("hello");
    expect(Array.from(new Uint8Array(decoded[2] as ArrayBuffer))).toEqual([1, 2, 3, 4]);
  });

  it("uses custom codecs for declared multipart part media types", async () => {
    const encode = vi.fn(async () => new URLSearchParams({ first: "one", second: "two" }));
    const codecs = new Map<string, MediaCodec<unknown>>([["application/x-custom", { encode }]]);
    const definition: WireBodyDefinition = {
      contentType: "multipart/form-data",
      schema: {
        types: ["object"],
        properties: {
          payload: { property: "payload", schema: { types: ["object"] } },
        },
      },
      encoding: [
        {
          name: "payload",
          contentType: "application/x-custom",
        },
      ],
    };

    const encoded = await services.encodeRequestBody(
      definition.contentType,
      { payload: { id: 1 } },
      codecs,
      definition.schema,
      {},
      definition,
      { payload: {} },
      undefined,
    );
    expect(encoded).toBeInstanceOf(Blob);
    const blob = encoded as Blob;
    const text = await blob.text();
    expect(text).toContain('Content-Disposition: form-data; name="payload"');
    expect(text).toContain("Content-Type: application/x-custom");
    expect(text).toContain("first=one&second=two");
    expect(encode).toHaveBeenCalledWith({ id: 1 }, { contentType: "application/x-custom" });
  });
});
