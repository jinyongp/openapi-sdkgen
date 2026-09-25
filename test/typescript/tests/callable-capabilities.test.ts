import { describe, expect, it } from "vitest";
import { assignCallableProperties } from "../../../internal/target/typescript/runtime/internal/callables.js";
import { createClient } from "../fixtures/generated/lifecycle/index.js";

// These checks must not execute: the generated stream-only surface deliberately
// hides its runtime buffered-call signature.
function checkPublicTypes(api: ReturnType<typeof createClient>) {
  const stream = assignCallableProperties(api.$operations.events, { marker: "stream" as const });
  const marker: "stream" = stream.marker;
  void marker;
  // @ts-expect-error Decorating capabilities must not expose a buffered call.
  stream();
  // @ts-expect-error The original public stream-only operation is not callable.
  api.$operations.events();
  // @ts-expect-error Resource shortcuts retain the same stream-only restriction.
  api.events.get();
  // @ts-expect-error Primitive targets cannot hold generated namespace members.
  assignCallableProperties(1, { marker: true });
  // @ts-expect-error A null target is invalid even when no members are added.
  assignCallableProperties(null, {});
  // @ts-expect-error Namespace members must be an object.
  assignCallableProperties(() => 1, "invalid");
}
void checkPublicTypes;

describe("internal callable capability boundary", () => {
  it("decorates a generated value whose public type exposes capabilities only", async () => {
    const raw = async () => ({ status: 200, data: "ok" });
    const callable = Object.assign(() => "buffered", { raw });
    const publicSurface: { readonly raw: typeof raw } = callable;
    const stream = () => "stream";
    const name = callable.name;
    const length = callable.length;
    const decorated = assignCallableProperties(publicSurface, { stream });

    expect(decorated).toBe(callable);
    expect(decorated.raw).toBe(raw);
    expect(decorated.stream).toBe(stream);
    expect(await decorated.raw()).toEqual({ status: 200, data: "ok" });
    expect(callable.name).toBe(name);
    expect(callable.length).toBe(length);
    expect(Object.getOwnPropertyDescriptor(decorated, "stream")).toEqual({
      value: stream,
      enumerable: true,
      configurable: true,
      writable: true,
    });
  });

  it("preserves ordinary callable inference and the original function object", () => {
    const call = (input: number) => input + 1;
    const decorated = assignCallableProperties(call, { marker: "normal" as const });
    const value: number = decorated(4);
    const marker: "normal" = decorated.marker;
    expect(value).toBe(5);
    expect(marker).toBe("normal");
    expect(decorated).toBe(call);
  });

  it("installs prototype-sensitive members as own data properties", () => {
    const call = () => 1;
    const proto = Object.getPrototypeOf(call);
    const value = { exact: true };
    const decorated = assignCallableProperties(call, Object.fromEntries([["__proto__", value]]));
    expect(Object.getPrototypeOf(decorated)).toBe(proto);
    expect(Object.getOwnPropertyDescriptor(decorated, "__proto__")).toEqual({
      value,
      enumerable: true,
      configurable: true,
      writable: true,
    });
  });
});
