import { describe, expect, it } from "vitest";
import { createPreparedQueryEncoder } from "../../../internal/target/typescript/runtime/http/request/http-query-core.js";
import { decodeInboundParameters } from "../../../internal/target/typescript/runtime/server/runtime-parameters.js";
import { jsonWireCodec } from "../../../internal/target/typescript/runtime/compatibility/wire-engine.js";
import type { OperationDefinition } from "../../../internal/target/typescript/runtime/http/operation.js";
import type { InboundParameterDefinition } from "../../../internal/target/typescript/runtime/server/runtime-types.js";
import type { WireSchema } from "../../../internal/target/typescript/runtime/schema/wire-types.js";
describe("query data boundaries", (): void => {
  it("encodes form elements before joining structural commas", (): void => {
    const encode: ReturnType<typeof createPreparedQueryEncoder> = createPreparedQueryEncoder(
      (_operation, _parameter, value: unknown): unknown => value,
    );
    const operation: OperationDefinition = {
      route: "GET /",
      method: "GET",
      path: "/",
      envelope: "",
      parameters: [{ location: "query", name: "p", property: "p", style: "form", explode: false }],
    };
    expect(encode({ p: ["a,b", "c"] }, operation, "query")).toBe("p=a%2Cb,c");
    expect(encode({ p: ["a", "b,c"] }, operation, "query")).toBe("p=a,b%2Cc");
    expect(encode({ p: { "a,b": "c,d" } }, operation, "query")).toBe("p=a%2Cb,c%2Cd");
  });
  it.each([
    ["form", true, "p=a%2Cb&p=c", ["a,b", "c"]],
    ["form", false, "p=a%2Cb,c", ["a,b", "c"]],
    ["pipeDelimited", false, "p=a%7Cb", ["a", "b"]],
    ["spaceDelimited", false, "p=a%20b", ["a", "b"]],
  ] as const)(
    "decodes %s explode=%s",
    async (
      style: string,
      explode: boolean,
      query: string,
      want: readonly string[],
    ): Promise<void> => {
      const schema: WireSchema = { types: ["array"], items: { types: ["string"] } };
      const definition: InboundParameterDefinition = {
        location: "query",
        name: "p",
        property: "p",
        style,
        explode,
        required: false,
        allowReserved: false,
        schema: { type: "array", items: { type: "string" } },
        wireSchema: schema,
      };
      const result: Awaited<ReturnType<typeof decodeInboundParameters>> =
        await decodeInboundParameters(
          { wire: jsonWireCodec },
          new Request("https://example.test/?" + query),
          [definition],
          {},
          {},
        );
      expect(result.query.p).toEqual(want);
    },
  );
});
