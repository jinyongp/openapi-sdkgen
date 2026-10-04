import { describe, expect, it } from "vitest";
import {
  decodeSimpleWireHeader,
  wireSchemaTypes,
  wirePropertySchema,
} from "../../../internal/target/typescript/runtime/schema/schema-query.js";
import { jsonWireCodec } from "../../../internal/target/typescript/runtime/compatibility/wire-engine.js";
import type {
  WireSchema,
  WireSchemas,
} from "../../../internal/target/typescript/runtime/schema/wire-types.js";

function decode(
  value: string,
  schema: WireSchema,
  schemas: WireSchemas = {},
  preference: "converted" | "string" = "converted",
  explode: boolean = true,
): unknown {
  return decodeSimpleWireHeader(
    value,
    schema,
    schemas,
    explode,
    (candidate: unknown, contract: WireSchema): void =>
      jsonWireCodec.validateWireValue(candidate, contract, schemas, "decode"),
    preference,
  );
}

describe("shared simple header schema queries", (): void => {
  it("keeps client numeric preference and server lossless string preference", (): void => {
    const schema: WireSchema = { anyOf: [{ types: ["integer"] }, { types: ["string"] }] };
    expect(decode("2", schema)).toBe(2);
    expect(decode("2", schema, {}, "string")).toBe("2");
    expect(decode("hello", schema)).toBe("hello");
  });

  it("terminates current-node ref cycles and rejects malformed object tokens", (): void => {
    const schemas: WireSchemas = {
      Recursive: {
        types: ["object"],
        properties: {
          count: { property: "count", schema: { types: ["integer"] } },
          next: { property: "next", schema: { reference: "Recursive" } },
        },
        required: ["count"],
      },
      Cycle: {
        reference: "Cycle",
        types: ["object"],
        properties: { count: { property: "count", schema: { types: ["integer"] } } },
      },
    };
    expect(wireSchemaTypes({ reference: "Cycle" }, schemas)).toContain("object");
    expect(wirePropertySchema({ reference: "Cycle" }, "count", schemas)).toEqual({
      types: ["integer"],
    });
    expect(decode("count=2", { reference: "Recursive" }, schemas)).toEqual({ count: 2 });
    expect(() => decode("count=bad", { reference: "Recursive" }, schemas)).toThrow();
    expect(() => decode("count", { types: ["object"], properties: {} })).toThrow("name=value");
    expect(() => decode("count,2,orphan", { types: ["object"] }, {}, "converted", false)).toThrow(
      "name,value",
    );
  });

  it("selects conditional branches and intersects direct and matching patterns", (): void => {
    const schema: WireSchema = {
      types: ["object"],
      properties: { flag: { property: "flag", schema: { types: ["boolean"] } } },
      if: {
        properties: { flag: { property: "flag", schema: { constValue: true } } },
        required: ["flag"],
      },
      then: {
        properties: { count: { property: "count", schema: { types: ["integer"] } } },
        required: ["count"],
      },
      else: {
        properties: { label: { property: "label", schema: { types: ["string"] } } },
        required: ["label"],
      },
    };
    expect(decode("flag=false,label=hello", schema)).toEqual({ flag: false, label: "hello" });
    const patterns: WireSchema = {
      types: ["object"],
      properties: { count: { property: "count", schema: { types: ["integer"] } } },
      patternProperties: { "^count$": { minimum: 2 } },
      additionalProperties: { types: ["boolean"] },
    };
    expect(decode("count=2,flag=true", patterns)).toEqual({ count: 2, flag: true });
    expect(() => decode("count=1,flag=true", patterns)).toThrow();
  });
});
