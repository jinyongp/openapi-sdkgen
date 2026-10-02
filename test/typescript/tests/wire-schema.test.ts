import { describe, expect, it } from "vitest";

import {
  jsonWireCodec,
  type WireSchema,
  type WireSchemas,
} from "../../../internal/target/typescript/runtime/internal/wire-engine.js";

const { validateWireValue, encodeWireValue, decodeWireValue } = jsonWireCodec;
const validate = (value: unknown, schema: WireSchema) =>
  validateWireValue(value, schema, {}, "decode");

describe("wire schema constraints", () => {
  it.each<[number, number, boolean]>([
    [3000000000000000.5, 1, false],
    [3000000000000000, 1, true],
    [0.3, 0.1, true],
    [0.35, 0.1, false],
    [0.1 + 0.2, 0.1, false],
    [-0.3, 0.1, true],
    [0, 0.1, true],
    [-0, 2, true],
    [1e30, 1e-300, true],
    [Number.MAX_VALUE, Number.MIN_VALUE, true],
    [Number.MAX_VALUE, Number.MIN_VALUE * 3, false],
    [Number.MIN_VALUE, Number.MIN_VALUE, true],
    [1e-323, Number.MIN_VALUE, true],
    [0.00000007, 0.00000001, true],
    [1, 0, false],
    [1, -1, false],
    [1, Infinity, false],
  ])("checks exact wire decimal multiple %s / %s", (value, divisor, valid) => {
    for (const apply of [encodeWireValue, decodeWireValue]) {
      const call = () => apply(value, { types: ["number"], multipleOf: divisor }, {});
      if (valid) expect(call).not.toThrow();
      else expect(call).toThrow();
    }
  });
  it("uses every matching pattern and excludes those properties from additional schemas", () => {
    const schema: WireSchema = {
      types: ["object"],
      patternProperties: { "^s": { types: ["string"] }, r$: { minLength: 2 } },
      additionalProperties: { types: ["number"] },
    };
    for (const apply of [encodeWireValue, decodeWireValue]) {
      expect(apply({ str: "ok", num: 1 }, schema, {})).toEqual({ str: "ok", num: 1 });
      expect(() => apply({ str: "x", num: 1 }, schema, {})).toThrow();
      expect(() => apply({ str: 1 }, schema, {})).toThrow();
      expect(() => apply({ num: "x" }, schema, {})).toThrow();
    }
  });

  it("matches wire names after classifying renamed declared properties", () => {
    const schema: WireSchema = {
      properties: { wire_value: { property: "wireValue", schema: { types: ["string"] } } },
      patternProperties: { "^wire_": { minLength: 2 } },
      additionalProperties: false,
      propertyNames: { pattern: "^wire_" },
    };
    expect(encodeWireValue({ wireValue: "ok" }, schema, {})).toEqual({ wire_value: "ok" });
    expect(decodeWireValue({ wire_value: "ok" }, schema, {})).toEqual({ wireValue: "ok" });
    expect(() => encodeWireValue({ wireValue: "x" }, schema, {})).toThrow();
  });

  it("transforms pattern property values while preserving each allOf additional scope", () => {
    const child: WireSchema = {
      properties: { wire_value: { property: "wireValue", schema: { types: ["string"] } } },
    };
    const schema: WireSchema = { patternProperties: { "^s": child }, additionalProperties: false };
    expect(encodeWireValue({ sample: { wireValue: "ok" } }, schema, {})).toEqual({
      sample: { wire_value: "ok" },
    });
    expect(decodeWireValue({ sample: { wire_value: "ok" } }, schema, {})).toEqual({
      sample: { wireValue: "ok" },
    });
    expect(() =>
      validate(
        { a: 1, b: 2 },
        {
          allOf: [
            { properties: { a: { property: "a", schema: {} } }, additionalProperties: false },
            { properties: { b: { property: "b", schema: {} } } },
          ],
        },
      ),
    ).toThrow();
  });
  it.each<{ name: string; schema: WireSchema; valid: unknown; invalid: unknown }>([
    { name: "decimal multiples", schema: { multipleOf: 0.1 }, valid: 0.3, invalid: 0.35 },
    { name: "minimum", schema: { minimum: 2 }, valid: 2, invalid: 1 },
    { name: "maximum", schema: { maximum: 2 }, valid: 2, invalid: 3 },
    { name: "exclusive minimum", schema: { exclusiveMinimum: 2 }, valid: 3, invalid: 2 },
    { name: "exclusive maximum", schema: { exclusiveMaximum: 2 }, valid: 1, invalid: 2 },
    { name: "Unicode minimum length", schema: { minLength: 2 }, valid: "😀a", invalid: "😀" },
    { name: "Unicode maximum length", schema: { maxLength: 1 }, valid: "😀", invalid: "😀a" },
    { name: "pattern", schema: { pattern: "^[a-z]+$" }, valid: "abc", invalid: "abc1" },
    { name: "minimum array size", schema: { minItems: 1 }, valid: [1], invalid: [] },
    { name: "maximum array size", schema: { maxItems: 1 }, valid: [1], invalid: [1, 2] },
    { name: "minimum object size", schema: { minProperties: 1 }, valid: { a: 1 }, invalid: {} },
    {
      name: "maximum object size",
      schema: { maxProperties: 1 },
      valid: { a: 1 },
      invalid: { a: 1, b: 2 },
    },
  ])("enforces $name at its boundary", ({ schema, valid, invalid }) => {
    expect(() => validate(valid, schema)).not.toThrow();
    expect(() => validate(invalid, schema)).toThrow(TypeError);
  });

  it("compares nested const, enum and unique items by value, not object key order", () => {
    const first = { a: [1, { b: true }], c: null };
    const reordered = { c: null, a: [1, { b: true }] };
    const different = { c: null, a: [1, { b: false }] };
    expect(() => validate(reordered, { constValue: first })).not.toThrow();
    expect(() => validate(different, { constValue: first })).toThrow(TypeError);
    expect(() => validate(reordered, { enumValues: [first, "other"] })).not.toThrow();
    expect(() => validate(different, { enumValues: [first, "other"] })).toThrow(TypeError);
    expect(() => validate([first, different], { uniqueItems: true })).not.toThrow();
    expect(() => validate([first, reordered], { uniqueItems: true })).toThrow(TypeError);
  });

  it("requires exactly one oneOf branch and at least one anyOf branch", () => {
    const alternatives = [{ types: ["integer"] }, { types: ["number"] }];
    expect(() => validate(2, { oneOf: alternatives })).toThrow(TypeError);
    expect(() => validate(2.5, { oneOf: alternatives })).not.toThrow();
    expect(() => validate(2, { anyOf: alternatives })).not.toThrow();
    expect(() => validate("2", { anyOf: alternatives })).toThrow(TypeError);
    expect(() => validate(2, { not: { constValue: 2 } })).toThrow(TypeError);
    expect(() => validate(3, { not: { constValue: 2 } })).not.toThrow();
  });

  it("applies only the selected conditional branch when validating and transforming", () => {
    const schema: WireSchema = {
      types: ["object"],
      if: {
        required: ["kind"],
        properties: { kind: { property: "kind", schema: { constValue: "number" } } },
      },
      then: {
        required: ["value"],
        properties: { value: { property: "value", schema: { types: ["integer"] } } },
      },
      else: {
        required: ["label"],
        properties: { label: { property: "label", schema: { types: ["string"] } } },
      },
    };
    expect(decodeWireValue({ kind: "number", value: 2 }, schema, {})).toEqual({
      kind: "number",
      value: 2,
    });
    expect(encodeWireValue({ label: "text" }, schema, {})).toEqual({ label: "text" });
    expect(() => validate({ kind: "number", value: "2" }, schema)).toThrow(TypeError);
    expect(() => validate({ label: 2 }, schema)).toThrow(TypeError);
  });

  it("preserves dependency constraints across wire-name mapping", () => {
    const schema: WireSchema = {
      types: ["object"],
      properties: {
        billing_id: { property: "billingID", schema: { types: ["string"] } },
        country_code: { property: "countryCode", schema: { types: ["string"] } },
      },
      dependentRequired: { billing_id: ["country_code"] },
      dependentSchemas: { country_code: { minProperties: 2 } },
    };
    expect(encodeWireValue({ billingID: "b1", countryCode: "KR" }, schema, {})).toEqual({
      billing_id: "b1",
      country_code: "KR",
    });
    expect(decodeWireValue({ billing_id: "b1", country_code: "KR" }, schema, {})).toEqual({
      billingID: "b1",
      countryCode: "KR",
    });
    expect(() => encodeWireValue({ billingID: "b1" }, schema, {})).toThrow(TypeError);
    expect(() => validate({ country_code: "KR" }, schema)).toThrow(TypeError);
    expect(() => validate({}, schema)).not.toThrow();
  });

  it("counts properties evaluated by references, patterns and dependencies", () => {
    const components: WireSchemas = {
      Base: { properties: { id: { property: "id", schema: { types: ["integer"] } } } },
    };
    const schema: WireSchema = {
      allOf: [{ reference: "Base" }],
      patternProperties: { "^x-": { types: ["boolean"] } },
      dependentSchemas: {
        id: { properties: { label: { property: "label", schema: { types: ["string"] } } } },
      },
      unevaluatedProperties: false,
    };
    expect(() =>
      validateWireValue({ id: 1, label: "one", "x-active": true }, schema, components, "decode"),
    ).not.toThrow();
    expect(() => validateWireValue({ id: 1, extra: 2 }, schema, components, "decode")).toThrow(
      TypeError,
    );
    expect(() => validateWireValue({ "x-active": "yes" }, schema, components, "decode")).toThrow(
      TypeError,
    );
    expect(() =>
      validateWireValue({ id: 1, extra: 2 }, schema, components, "decode", {
        unknownProperties: "preserve",
      }),
    ).not.toThrow();
    expect(() => validate({ invalid: 1 }, { propertyNames: { pattern: "^x-" } })).toThrow(
      TypeError,
    );
    expect(() => validate({ "x-valid": 1 }, { propertyNames: { pattern: "^x-" } })).not.toThrow();
  });

  it("combines tuple and contains evaluations without accepting unrelated trailing items", () => {
    const schema: WireSchema = {
      allOf: [{ prefixItems: [{ types: ["string"] }] }],
      contains: { types: ["integer"] },
      minContains: 1,
      maxContains: 2,
      unevaluatedItems: false,
    };
    expect(() => validate(["label", 1, 2], schema)).not.toThrow();
    expect(() => validate(["label"], schema)).toThrow(TypeError);
    expect(() => validate(["label", 1, 2, 3], schema)).toThrow(TypeError);
    expect(() => validate(["label", 1, true], schema)).toThrow(TypeError);
    const trailing: WireSchema = {
      prefixItems: [{ types: ["string"] }],
      unevaluatedItems: { types: ["boolean"] },
    };
    expect(() => validate(["label", true, false], trailing)).not.toThrow();
    expect(() => validate(["label", 1], trailing)).toThrow(TypeError);
  });

  it("uses outer dynamic anchors for nested values and static fallback without an override", () => {
    const components: WireSchemas = {
      Base: {
        dynamicAnchor: "node",
        types: ["object"],
        properties: {
          value: { property: "value", schema: { types: ["integer"] } },
          children: {
            property: "children",
            schema: {
              types: ["array"],
              items: { dynamicReference: { anchor: "node", fallback: { reference: "Base" } } },
            },
          },
        },
      },
    };
    const strict: WireSchema = { dynamicAnchor: "node", reference: "Base", required: ["tag"] };
    const valid = { tag: "root", value: 1, children: [{ tag: "child", value: 2 }] };
    expect(decodeWireValue(valid, strict, components)).toEqual(valid);
    expect(() =>
      decodeWireValue({ ...valid, children: [{ value: 2 }] }, strict, components),
    ).toThrow(TypeError);
    const fallback: WireSchema = {
      dynamicReference: { anchor: "absent", fallback: { types: ["integer"] } },
    };
    expect(decodeWireValue(2, fallback, {})).toBe(2);
    expect(() => decodeWireValue("2", fallback, {})).toThrow(TypeError);
  });

  it("validates decoded content while preserving the outer encoded value", () => {
    const schema: WireSchema = {
      types: ["string"],
      contentEncoding: "base64",
      contentMediaType: "application/json",
      contentSchema: {
        types: ["object"],
        required: ["count"],
        properties: { count: { property: "count", schema: { types: ["integer"] } } },
      },
    };
    const encoded = btoa('{"count":2}');
    expect(decodeWireValue(encoded, schema, {})).toBe(encoded);
    expect(encodeWireValue(encoded, { ...schema, contentEncoding: "base64url" }, {})).toBe(encoded);
    for (const invalid of ["%%%", btoa("not JSON"), btoa('{"count":"two"}')]) {
      expect(() => validate(invalid, schema)).toThrow(TypeError);
    }
    expect(() => validate(encoded, { ...schema, contentEncoding: "unsupported" })).toThrow(
      TypeError,
    );
    expect(() =>
      validate(encoded, { ...schema, contentMediaType: "application/x-unknown" }),
    ).toThrow(TypeError);
    expect(
      decodeWireValue(
        "text",
        { contentMediaType: "text/plain", contentSchema: { types: ["string"] } },
        {},
      ),
    ).toBe("text");
  });
});
