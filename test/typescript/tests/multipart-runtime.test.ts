import { describe, expect, it, vi } from "vitest";
import type { Mock } from "vitest";
import { createAdvancedHTTPServices } from "../../../internal/target/typescript/runtime/compatibility/http-advanced.js";
import { jsonWireCodec } from "../../../internal/target/typescript/runtime/compatibility/wire-engine.js";
import type { MediaCodec } from "../../../internal/target/typescript/runtime/media/media-codec-types.js";
import type { WireBodyDefinition } from "../../../internal/target/typescript/runtime/media/media-contract-types.js";
import type { WireSchema } from "../../../internal/target/typescript/runtime/schema/wire-types.js";

const services = createAdvancedHTTPServices(() => {
  throw new Error("base HTTP services are not used by multipart codec tests");
}, jsonWireCodec);

describe("advanced multipart runtime", () => {
  it.each(["hello", 3, true, null, { id: 1 }, [1, 2]])(
    "uses identical JSON bytes for %j in every multipart path",
    async (value: unknown): Promise<void> => {
      const contentType: string = "Application/JSON; charset=utf-8";
      const field: WireSchema = {};
      const named: WireBodyDefinition = {
        contentType: "multipart/form-data; charset=utf-8",
        schema: { properties: { payload: { property: "payload", schema: field } } },
        encoding: [{ name: "payload", contentType }],
      };
      const plain: BodyInit = await services.encodeRequestBody(
        named.contentType,
        { payload: value },
        new Map(),
        named.schema,
        {},
        named,
        undefined,
        undefined,
      );
      expect(plain).toBeInstanceOf(FormData);
      const part: FormDataEntryValue | null = (plain as FormData).get("payload");
      expect(part).toBeInstanceOf(Blob);
      expect(await (part as Blob).text()).toBe(JSON.stringify(value));
      const headers: BodyInit = await services.encodeRequestBody(
        named.contentType,
        { payload: value },
        new Map(),
        named.schema,
        {},
        named,
        { payload: {} },
        undefined,
      );
      expect(await (headers as Blob).text()).toContain(`\r\n\r\n${JSON.stringify(value)}\r\n`);
      const positional: WireBodyDefinition = {
        contentType: "multipart/mixed",
        schema: { types: ["array"], items: field },
        prefixEncoding: [{ contentType }],
      };
      const ordered: BodyInit = await services.encodeRequestBody(
        positional.contentType,
        [value],
        new Map(),
        positional.schema,
        {},
        positional,
        undefined,
        undefined,
      );
      expect(await (ordered as Blob).text()).toContain(`\r\n\r\n${JSON.stringify(value)}\r\n`);
    },
  );

  it("shares XML and async custom codecs between FormData and explicit-header paths", async (): Promise<void> => {
    const components: Record<string, WireSchema> = {
      Payload: {
        types: ["object"],
        xml: { name: "payload" },
        properties: { count: { property: "count", schema: { types: ["integer"] } } },
      },
    };
    const xml: WireBodyDefinition = {
      contentType: "multipart/form-data",
      schema: {
        properties: { payload: { property: "payload", schema: { reference: "Payload" } } },
      },
      encoding: [{ name: "payload", contentType: "application/xml; charset=utf-8" }],
    };
    const encode: Mock<NonNullable<MediaCodec<unknown>["encode"]>> = vi.fn(
      async (
        _value: unknown,
        context: Parameters<NonNullable<MediaCodec<unknown>["encode"]>>[1],
      ): Promise<URLSearchParams> => new URLSearchParams({ type: context.contentType }),
    );
    const codecs: ReadonlyMap<string, MediaCodec<unknown>> = new Map([
      ["application/x-custom", { encode }],
    ]);
    for (const definition of [
      xml,
      { ...xml, encoding: [{ name: "payload", contentType: "application/x-custom; version=1" }] },
    ]) {
      const expected: string =
        definition === xml
          ? "<payload><count>2</count></payload>"
          : "type=application%2Fx-custom%3B+version%3D1";
      const form: BodyInit = await services.encodeRequestBody(
        definition.contentType,
        { payload: { count: 2 } },
        codecs,
        definition.schema,
        components,
        definition,
        undefined,
        undefined,
      );
      expect(await ((form as FormData).get("payload") as Blob).text()).toBe(expected);
      const body: BodyInit = await services.encodeRequestBody(
        definition.contentType,
        { payload: { count: 2 } },
        codecs,
        definition.schema,
        components,
        definition,
        { payload: {} },
        undefined,
      );
      expect(await (body as Blob).text()).toContain(`\r\n\r\n${expected}\r\n`);
    }
    expect(encode).toHaveBeenCalledTimes(2);
  });

  it("preserves supplied Blob and byte payloads across both named encoders", async (): Promise<void> => {
    const bytes: Uint8Array<ArrayBuffer> = new Uint8Array([0, 1, 127, 255]);
    const definition: WireBodyDefinition = {
      contentType: "multipart/form-data",
      schema: {},
      encoding: [{ name: "payload", contentType: "application/json" }],
    };
    for (const value of [
      new Blob([bytes]),
      new File([bytes], "payload.bin"),
      bytes,
      bytes.buffer,
    ]) {
      const form: BodyInit = await services.encodeRequestBody(
        definition.contentType,
        { payload: value },
        new Map(),
        undefined,
        {},
        definition,
        undefined,
        undefined,
      );
      expect(
        new Uint8Array(await ((form as FormData).get("payload") as Blob).arrayBuffer()),
      ).toEqual(bytes);
      const body: BodyInit = await services.encodeRequestBody(
        definition.contentType,
        { payload: value },
        new Map(),
        undefined,
        {},
        definition,
        { payload: {} },
        undefined,
      );
      const wire: Uint8Array<ArrayBuffer> = new Uint8Array(await (body as Blob).arrayBuffer());
      const separator: number = wire.findIndex(
        (_byte: number, index: number): boolean =>
          wire[index] === 13 &&
          wire[index + 1] === 10 &&
          wire[index + 2] === 13 &&
          wire[index + 3] === 10,
      );
      expect(wire.slice(separator + 4, separator + 4 + bytes.length)).toEqual(bytes);
      if (value instanceof File) {
        const parsed: FormData = await new Response(body).formData();
        expect((parsed.get("payload") as File).name).toBe(value.name);
      }
    }
  });
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
