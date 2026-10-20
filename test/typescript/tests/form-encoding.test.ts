import { expect, it } from "vitest";
import { encodeFormBody } from "../../../internal/target/typescript/runtime/media/http-body-form.js";
import type { WireEncodingDefinition } from "../../../internal/target/typescript/runtime/media/media-contract-types.js";
function encode(value: unknown, encoding?: readonly WireEncodingDefinition[]): string {
  return String(
    encodeFormBody(
      "application/x-www-form-urlencoded",
      value,
      new Map(),
      undefined,
      {},
      encoding === undefined
        ? undefined
        : { contentType: "application/x-www-form-urlencoded", schema: {}, encoding },
      undefined,
      undefined,
    ),
  );
}
it("honors form encoding modes and explicit field precedence", (): void => {
  expect(encode({ p: { x: 1, y: 2 } })).toBe("p=%7B%22x%22%3A1%2C%22y%22%3A2%7D");
  expect(encode({ p: ["a,b", "c"] }, [{ name: "p", explode: false }])).toBe("p=a%2Cb,c");
  expect(encode({ p: [1, 2] }, [{ name: "p", style: "pipeDelimited" }])).toBe("p=1%7C2");
  expect(encode({ p: [1, 2] }, [{ name: "p", style: "spaceDelimited" }])).toBe("p=1%202");
  expect(encode({ p: { x: 1, y: 2 } }, [{ name: "p", style: "deepObject", explode: true }])).toBe(
    "p%5Bx%5D=1&p%5By%5D=2",
  );
  expect(
    encode({ p: { x: 1 } }, [
      { name: "p", style: "form", explode: false, contentType: "application/json" },
    ]),
  ).toBe("p=x,1");
  expect(encode({ p: "a/b+c&d" }, [{ name: "p", allowReserved: true }])).toBe("p=a/b%2Bc%26d");
  expect(encode({ p: "" }, [{ name: "p", style: "form" }])).toBe("p=");
});
