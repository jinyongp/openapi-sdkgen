import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  jsonWireCodec,
  type WireSchema,
} from "../../../internal/target/typescript/runtime/internal/wire-engine.js";

type Schema = boolean | Record<string, unknown>;
type Suite = {
  description: string;
  schema: Schema;
  tests: { description: string; data: unknown; valid: boolean }[];
}[];

// Translate the pinned standard cases into wire descriptors without calculating
// validity. Expected results come directly from the independent upstream suite.
function descriptor(
  value: Schema,
  root: Schema = value,
  components: Record<string, WireSchema> = {},
): WireSchema {
  if (typeof value === "boolean") return { boolean: value };
  const result: Record<string, unknown> = {};
  for (const [key, item] of Object.entries(value)) {
    if (key === "$schema" || key === "$defs") continue;
    if (key === "$ref") {
      const reference = String(item);
      if (!reference.startsWith("#/")) throw new Error("suite reference must be local");
      if (!Object.hasOwn(components, reference)) {
        let target: unknown = root;
        for (const part of reference.slice(2).split("/"))
          target = (target as Record<string, unknown>)[
            part.replaceAll("~1", "/").replaceAll("~0", "~")
          ];
        if (target === undefined) throw new Error("suite reference target is missing");
        components[reference] = {};
        Object.assign(components[reference]!, descriptor(target as Schema, root, components));
      }
      result.reference = reference;
      continue;
    }
    if (key === "type") result.types = Array.isArray(item) ? item : [item];
    else if (key === "const") result.constValue = item;
    else if (key === "enum") result.enumValues = item;
    else if (["allOf", "anyOf", "oneOf", "prefixItems"].includes(key))
      result[key] = (item as Schema[]).map((schema) => descriptor(schema, root, components));
    else if (
      [
        "if",
        "then",
        "else",
        "not",
        "items",
        "contains",
        "additionalProperties",
        "unevaluatedProperties",
        "unevaluatedItems",
      ].includes(key)
    )
      result[key] =
        item === false &&
        ["additionalProperties", "unevaluatedProperties", "unevaluatedItems"].includes(key)
          ? false
          : descriptor(item as Schema, root, components);
    else if (key === "properties")
      result[key] = Object.fromEntries(
        Object.entries(item as Record<string, Schema>).map(([property, schema]) => [
          property,
          { property, schema: descriptor(schema, root, components) },
        ]),
      );
    else if (["patternProperties", "dependentSchemas"].includes(key))
      result[key] = Object.fromEntries(
        Object.entries(item as Record<string, Schema>).map(([property, schema]) => [
          property,
          descriptor(schema, root, components),
        ]),
      );
    else result[key] = item;
  }
  return result as WireSchema;
}

describe("JSON Schema evaluation annotations", () => {
  for (const keyword of ["unevaluatedProperties", "unevaluatedItems"]) {
    const suite = JSON.parse(
      readFileSync(new URL(`../fixtures/json-schema-${keyword}.json`, import.meta.url), "utf8"),
    ) as Suite;
    for (const group of suite) {
      const components: Record<string, WireSchema> = {};
      const schema = descriptor(group.schema, group.schema, components);
      it.each(group.tests)(`${keyword}: ${group.description}: $description`, ({ data, valid }) => {
        for (const apply of [
          () => jsonWireCodec.validateWireValue(data, schema, components, "decode"),
          () => jsonWireCodec.encodeWireValue(data, schema, components),
          () => jsonWireCodec.decodeWireValue(data, schema, components),
        ]) {
          if (valid) expect(apply).not.toThrow();
          else expect(apply).toThrow();
        }
      });
    }
  }

  it("does not leak annotations from failed anyOf branches", () => {
    const schema: WireSchema = {
      anyOf: [
        {
          properties: { good: { property: "good", schema: { types: ["string"] } } },
          required: ["good"],
        },
        { properties: { leaked: { property: "leaked", schema: {} } }, required: ["missing"] },
      ],
      unevaluatedProperties: false,
    };
    expect(() => jsonWireCodec.decodeWireValue({ good: "ok" }, schema, {})).not.toThrow();
    expect(() => jsonWireCodec.decodeWireValue({ good: "ok", leaked: 1 }, schema, {})).toThrow();
  });

  it("keeps preserve policy separate from strict control-flow evaluations", () => {
    const schema: WireSchema = {
      if: {
        properties: { tag: { property: "tag", schema: { constValue: "a" } } },
        required: ["tag"],
      },
      then: { properties: { value: { property: "value", schema: { types: ["string"] } } } },
      unevaluatedProperties: false,
    };
    const value = { tag: "a", value: "ok", extra: 1 };
    expect(() => jsonWireCodec.decodeWireValue(value, schema, {})).toThrow();
    expect(
      jsonWireCodec.transformWireValue(value, schema, {}, "decode", {
        unknownProperties: "preserve",
      }),
    ).toEqual(value);
  });
});
