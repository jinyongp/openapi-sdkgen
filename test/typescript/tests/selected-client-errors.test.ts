import { describe, expect, it } from "vitest";
import { createSelectedClient } from "../../../internal/target/typescript/runtime/client/selected-client.js";
import { OperationPreparationError } from "../../../internal/target/typescript/runtime/client/operation-loader.js";
import type { OperationExecutionProvider } from "../../../internal/target/typescript/runtime/client/operation-loader.js";

const callable = () => Object.assign(async () => undefined, { raw: async () => undefined });
const provider = (route: string): OperationExecutionProvider => ({
  abi: 1,
  generation: "binding-test",
  route,
  bind: callable,
});
const resolve = async () => {
  throw new Error("Binding must not load unrelated code");
};
function thrown(run: () => unknown): unknown {
  try {
    run();
  } catch (error) {
    return error;
  }
  throw new Error("Expected binding to fail");
}

describe("selected client binding diagnostics", () => {
  it("classifies an invalid compiler-owned call/raw surface as BINDING", () => {
    const invalid = { ...provider("GET /a"), bind: () => ({}) };
    const error = thrown(() => createSelectedClient({}, [invalid], resolve));
    expect(error).toBeInstanceOf(OperationPreparationError);
    expect(error).toMatchObject({ stage: "BINDING" });
  });

  it("rejects conflicting resource placements rather than overwriting a method", () => {
    const resources = [{ path: [], member: "get" }] as const;
    const error = thrown(() =>
      createSelectedClient(
        {},
        [
          { ...provider("GET /a"), resources },
          { ...provider("GET /b"), resources },
        ],
        resolve,
      ),
    );
    expect(error).toMatchObject({ stage: "BINDING" });
  });

  it("rejects a pagination placement without its corresponding capability", () => {
    const invalid = {
      ...provider("GET /a"),
      resources: [{ path: [], member: "paginate", pagination: true }] as const,
    };
    expect(thrown(() => createSelectedClient({}, [invalid], resolve))).toMatchObject({
      stage: "BINDING",
    });
  });

  it("does not wrap an original binding exception as a different error", () => {
    const original = new Error("Application callback failure");
    const throwing = {
      ...provider("GET /a"),
      bind() {
        throw original;
      },
    };
    expect(thrown(() => createSelectedClient({}, [throwing], resolve))).toBe(original);
  });
});
