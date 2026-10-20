import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { jsonWireCodec } from "../../../internal/target/typescript/runtime/compatibility/wire-engine.js";
import type { WireSchema } from "../../../internal/target/typescript/runtime/schema/wire-types.js";
import { matchesWireFormat } from "../../../internal/target/typescript/runtime/schema/assertions/wire-format.js";
interface FormatCase {
  readonly description: string;
  readonly data: unknown;
  readonly valid: boolean;
}
interface FormatGroup {
  readonly description: string;
  readonly tests: readonly FormatCase[];
}
for (const name of [
  "hostname",
  "idn-hostname",
  "uri",
  "uri-reference",
  "iri",
  "iri-reference",
  "email",
  "idn-email",
]) {
  const groups: readonly FormatGroup[] = JSON.parse(
    readFileSync(new URL(`../fixtures/json-schema-format-${name}.json`, import.meta.url), "utf8"),
  ) as readonly FormatGroup[];
  describe(`official JSON Schema ${name}`, (): void => {
    for (const group of groups)
      for (const test of group.tests)
        it(`${group.description}: ${test.description}`, (): void => {
          const schema: WireSchema = { format: name, formatAssertion: true };
          const validate: () => void = (): void =>
            jsonWireCodec.validateWireValue(test.data, schema, {}, "decode");
          if (test.valid) expect(validate).not.toThrow();
          else expect(validate).toThrow();
        });
  });
}
it("rejects URL components passed as a hostname", (): void => {
  for (const value of [
    "user@example.com",
    "example.com/path",
    "example.com:443",
    "example.com?x=1",
    "example.com#fragment",
  ])
    expect(matchesWireFormat(value, "idn-hostname")).toBe(false);
});

it("checks IDNA hyphen positions as Unicode code points", (): void => {
  expect(matchesWireFormat("é--a", "idn-hostname")).toBe(true);
  for (const value of ["-é", "é-", "éa--b"])
    expect(matchesWireFormat(value, "idn-hostname")).toBe(false);
});

it("keeps URI schemes and IPvFuture literals ASCII even for IRIs", (): void => {
  for (const format of ["uri", "uri-reference", "iri", "iri-reference"]) {
    for (const value of ["K://example.test", "ſ://example.test", "http://[v1.K]", "http://[v1.ſ]"])
      expect(matchesWireFormat(value, format)).toBe(false);
    for (const value of ["HTTPS://example.test", "http://[Vf.X:1]"])
      expect(matchesWireFormat(value, format)).toBe(true);
  }
});
