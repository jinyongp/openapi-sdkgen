import { describe, expect, it } from "vitest";
import { wireProperties } from "../../../internal/target/typescript/runtime/schema/wire-properties.js";
import { createWireProperties } from "../../../internal/target/typescript/runtime/client/callables.js";
import type { WireSchema } from "../../../internal/target/typescript/runtime/schema/wire-types.js";
import { wireProperties as emitted } from "../fixtures/generated/lifecycle/internal/runtime/schema/wire-properties.js";
import { createWireProperties as emittedFacade } from "../fixtures/generated/lifecycle/internal/runtime/client/operation-bind-input.js";

function checkTypes() {
  // @ts-expect-error Keys remain strings rather than arbitrary JSON values.
  wireProperties([1], [{}]);
  // @ts-expect-error Children remain WireSchema descriptors.
  wireProperties(["x"], ["schema"]);
}
void checkTypes;

for (const [label, build] of [
  ["template", wireProperties],
  ["emitted", emitted],
] as const) {
  describe(`${label} wire property construction`, () => {
    it("matches explicit wrapper objects for exact keys and duplicate ordering", () => {
      const keys = [
        "z",
        "__proto__",
        "constructor",
        "prototype",
        "1",
        "01",
        "",
        "é",
        "e\u0301",
        "a\0b",
        "z",
      ];
      const schemas: WireSchema[] = keys.map((_, index) => ({ constValue: index }));
      const expected = Object.fromEntries(
        keys.map((key, index) => [key, { property: key, schema: schemas[index] }]),
      );
      const result = build(keys, schemas);
      expect(Object.getPrototypeOf(result)).toBe(Object.prototype);
      expect(Reflect.ownKeys(result)).toEqual(Reflect.ownKeys(expected));
      expect(Object.getOwnPropertyDescriptors(result)).toEqual(
        Object.getOwnPropertyDescriptors(expected),
      );
      expect(Object.getOwnPropertyDescriptor(result, "__proto__")?.value.schema).toBe(schemas[1]);
      expect(Object.getOwnPropertyDescriptor(result, "z")?.value.schema).toBe(schemas.at(-1));
      expect(build([], [])).toEqual({});
      expect(Object.getPrototypeOf(build([], []))).toBe(Object.prototype);
    });

    it("keeps distinct wrappers around shared frozen children without mutating inputs", () => {
      const opaque = JSON.parse(
        '{"property":"schema","schema":"__sdkgen_Properties","__proto__":null,"p":false,"s":0}',
      );
      const child: WireSchema = Object.freeze({ constValue: opaque, enumValues: [opaque] });
      const keys = Object.freeze(["left", "right"]);
      const children = Object.freeze([child, child]);
      const result = build(keys, children);
      expect(result.left).not.toBe(result.right);
      expect(result.left?.schema).toBe(child);
      expect(result.right?.schema).toBe(child);
      expect(result.left?.schema.constValue).toBe(opaque);
      expect(Object.hasOwn(child, "property")).toBe(false);
      expect(Object.getOwnPropertyDescriptor(opaque, "__proto__")?.value).toBeNull();
      expect(keys).toEqual(["left", "right"]);
      expect(children).toEqual([child, child]);
    });

    it("does not traverse recursive schema graphs", () => {
      const child: {
        properties?: Readonly<Record<string, { property: string; schema: WireSchema }>>;
      } = {};
      const result = build(["self"], [child]);
      child.properties = result;
      expect(result.self?.schema).toBe(child);
      expect(result.self?.schema.properties).toBe(result);
    });

    it("preserves explicit values rather than applying truthiness defaults", () => {
      const values = [undefined, null, false, 0, "", [], {}];
      const schemas = values.map((constValue) => ({ constValue }));
      const result = build(
        values.map((_, index) => String(index)),
        schemas,
      );
      for (let index = 0; index < values.length; index++) {
        expect(result[String(index)]?.schema).toBe(schemas[index]);
        expect(Object.hasOwn(result[String(index)]!.schema, "constValue")).toBe(true);
        expect(result[String(index)]?.schema.constValue).toBe(values[index]);
      }
    });

    it("rejects either mismatched array length without reading child elements", () => {
      const children: WireSchema[] = [];
      Object.defineProperty(children, 0, {
        get() {
          throw new Error("must not read");
        },
      });
      expect(() => build([], children)).toThrow("wire property/schema count mismatch");
      expect(() => build(["x"], [])).toThrow("wire property/schema count mismatch");
    });
  });
}

it("uses direct facade re-exports of the same function object", () => {
  expect(createWireProperties).toBe(wireProperties);
  expect(emittedFacade).toBe(emitted);
  expect(wireProperties.name).toBe("wireProperties");
  expect(wireProperties.length).toBe(2);
});
