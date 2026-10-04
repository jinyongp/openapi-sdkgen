import { describe, expect, it } from "vitest";
import { jsonWireCodec } from "../../../internal/target/typescript/runtime/compatibility/wire-engine.js";
import type {
  WireProperty,
  WireSchema,
  WireSchemas,
} from "../../../internal/target/typescript/runtime/schema/wire-types.js";

const scalar: WireSchema = { types: ["string"] };
const entry: WireSchema = {
  types: ["object"],
  properties: { wire_name: { property: "name", schema: scalar } },
  required: ["wire_name"],
  additionalProperties: false,
};
interface DTOCase {
  readonly name: string;
  readonly value: unknown;
  readonly schema: WireSchema;
  readonly components?: WireSchemas;
  readonly expected: unknown;
}
const cases: readonly DTOCase[] = [
  {
    name: "property mapping",
    value: { wire_name: "one" },
    schema: entry,
    expected: { name: "one" },
  },
  {
    name: "nested records and arrays",
    value: { box: [{ wire_name: "one" }] },
    schema: {
      types: ["object"],
      properties: { box: { property: "box", schema: { types: ["array"], items: entry } } },
    },
    expected: { box: [{ name: "one" }] },
  },
  {
    name: "additional properties",
    value: { extra: { wire_name: "one" } },
    schema: { types: ["object"], additionalProperties: entry },
    expected: { extra: { name: "one" } },
  },
  {
    name: "intersected mappings",
    value: { first_name: "one", last_name: "two" },
    schema: {
      allOf: [
        { properties: { first_name: { property: "firstName", schema: scalar } } },
        { properties: { last_name: { property: "lastName", schema: scalar } } },
      ],
    },
    expected: { firstName: "one", lastName: "two" },
  },
  {
    name: "all valid anyOf branches",
    value: { first_name: "one", last_name: "two" },
    schema: {
      anyOf: [
        { properties: { first_name: { property: "firstName", schema: scalar } } },
        { properties: { last_name: { property: "lastName", schema: scalar } } },
      ],
    },
    expected: { firstName: "one", lastName: "two" },
  },
  {
    name: "recursive reference",
    value: { wire_name: "root", next: { wire_name: "leaf" } },
    schema: { reference: "Node" },
    components: {
      Node: {
        types: ["object"],
        properties: {
          wire_name: { property: "name", schema: scalar },
          next: { property: "next", schema: { reference: "Node" } },
        },
        required: ["wire_name"],
      },
    },
    expected: { name: "root", next: { name: "leaf" } },
  },
];

function assertPlain(value: unknown): void {
  if (typeof value !== "object" || value === null) return;
  if (Array.isArray(value)) {
    expect(Object.getPrototypeOf(value)).toBe(Array.prototype);
  } else {
    expect(Object.getPrototypeOf(value)).toBe(Object.prototype);
    expect(value instanceof Object).toBe(true);
  }
  for (const item of Object.values(value)) assertPlain(item);
}

describe("public schema DTO records", (): void => {
  it.each(cases)("builds ordinary objects for $name", (test: DTOCase): void => {
    const value: unknown = jsonWireCodec.decodeWireValue(
      test.value,
      test.schema,
      test.components ?? {},
    );
    assertPlain(value);
    expect(value).toStrictEqual(test.expected);
    expect(jsonWireCodec.encodeWireValue(value, test.schema, test.components ?? {})).toEqual(
      test.value,
    );
  });
  it("preserves sensitive names as own data properties", (): void => {
    const value: unknown = JSON.parse(
      '{"__proto__":{"wire_name":"safe"},"constructor":"data","prototype":"data","hasOwnProperty":"own"}',
    );
    const schema: WireSchema = {
      types: ["object"],
      properties: Object.fromEntries([
        ["__proto__", { property: "__proto__", schema: entry }],
        ...["constructor", "prototype", "hasOwnProperty"].map(
          (name: string): [string, WireProperty] => [name, { property: name, schema: scalar }],
        ),
      ]),
      required: ["__proto__", "constructor", "prototype", "hasOwnProperty"],
      additionalProperties: false,
    };
    const decoded: unknown = jsonWireCodec.decodeWireValue(value, schema, {});
    assertPlain(decoded);
    expect(Object.hasOwn(decoded as object, "__proto__")).toBe(true);
    expect((decoded as Record<string, unknown>)["__proto__"]).toStrictEqual({ name: "safe" });
    expect(({} as Record<string, unknown>)["name"]).toBeUndefined();
  });
  it("rejects inherited required fields", (): void => {
    const inherited: unknown = Object.create({ wire_name: "inherited" });
    expect((): unknown => jsonWireCodec.decodeWireValue(inherited, entry, {})).toThrow(
      /missing required/,
    );
    expect((): unknown => jsonWireCodec.decodeWireValue({ wire_name: 4 }, entry, {})).toThrow();
  });
  it("keeps opaque and native objects by identity", (): void => {
    for (const value of [
      { arbitrary: { deep: "unchanged" } },
      Object.create(null),
      new Date(),
      new Blob(["body"]),
      new Uint8Array([1]),
      new ArrayBuffer(2),
      new ReadableStream<Uint8Array>(),
    ]) {
      expect(jsonWireCodec.decodeWireValue(value, {}, {})).toBe(value);
    }
    const opaque: unknown = Object.create(null);
    const value: unknown = jsonWireCodec.decodeWireValue(
      { wire_name: "one", opaque },
      { ...entry, additionalProperties: {} },
      {},
    );
    expect((value as Record<string, unknown>)["opaque"]).toBe(opaque);
  });
});
