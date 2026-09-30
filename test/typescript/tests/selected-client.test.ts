import { describe, expect, it, vi } from "vitest";
import { createSelectedClient } from "../../../internal/target/typescript/runtime/internal/selected-client.js";
import type {
  OperationExecutionProvider,
  OperationProviderResolver,
} from "../../../internal/target/typescript/runtime/internal/operation-loader.js";

type RuntimeCall = ((...args: unknown[]) => Promise<unknown>) & {
  raw: (...args: unknown[]) => Promise<unknown>;
  paginate?: (...args: unknown[]) => unknown;
};

function runtimeCall(label: string, calls: unknown[][] = []): RuntimeCall {
  const call = async (...args: unknown[]) => {
    calls.push(args);
    return { label, args };
  };
  return Object.assign(call, {
    raw: async (...args: unknown[]) => ({ label: label + ":raw", args }),
  });
}

function provider(
  route: string,
  call: RuntimeCall,
  options: Partial<OperationExecutionProvider> = {},
): OperationExecutionProvider {
  return {
    abi: 1,
    generation: "selected-client-test",
    route,
    bind: () => call,
    ...options,
  };
}

describe("selected client assembly", () => {
  it("binds parameter resources and exposes pagination placements", async () => {
    const calls: unknown[][] = [];
    const item = runtimeCall("item", calls);
    const paginate = vi.fn(() => "page");
    item.paginate = paginate;

    const getItem = provider("GET /items/{itemID}", item, {
      operationID: "getItem",
      resources: [
        {
          path: ["items", null],
          member: "get",
          pathParameters: ["itemID"],
        },
        {
          path: ["items"],
          member: "paginate",
          pagination: true,
        },
      ],
    });

    const client = createSelectedClient({}, [getItem], async () => getItem) as {
      readonly items: ((itemID: string) => {
        readonly get: () => Promise<unknown>;
      }) & { readonly paginate: typeof paginate };
      readonly $routes: Record<string, RuntimeCall>;
      readonly $operations: Record<string, RuntimeCall>;
    };

    await expect(client.items("a/b").get()).resolves.toEqual({
      label: "item",
      args: [{ path: { itemID: "a/b" } }],
    });
    expect(calls).toEqual([[{ path: { itemID: "a/b" } }]]);
    expect(client.items.paginate()).toBe("page");
    expect(client.$routes["GET /items/{itemID}"]).toBe(item);
    expect(client.$operations.getItem).toBe(item);
  });

  it("deduplicates repeated routes before binding or publishing aliases", () => {
    const firstCall = runtimeCall("first");
    const secondCall = runtimeCall("second");
    const firstBind = vi.fn(() => firstCall);
    const secondBind = vi.fn(() => secondCall);
    const first = { ...provider("GET /items", firstCall), operationID: "first", bind: firstBind };
    const duplicate = {
      ...provider("GET /items", secondCall),
      operationID: "duplicate",
      bind: secondBind,
    };

    const client = createSelectedClient({}, [first, duplicate], async () => first) as {
      readonly $routes: Record<string, RuntimeCall>;
      readonly $operations: Record<string, RuntimeCall>;
    };

    expect(firstBind).toHaveBeenCalledTimes(1);
    expect(secondBind).not.toHaveBeenCalled();
    expect(client.$routes["GET /items"]).toBe(firstCall);
    expect(client.$operations.first).toBe(firstCall);
    expect(Object.hasOwn(client.$operations, "duplicate")).toBe(false);
  });

  it("reuses selected Link targets and resolves only missing targets lazily", async () => {
    const sourceCall = runtimeCall("source");
    const selectedTargetCall = runtimeCall("selected-target");
    const lazyTargetCall = runtimeCall("lazy-target");
    let lazyTarget: OperationExecutionProvider;
    const lazyLoader = vi.fn(async () => lazyTarget);

    const selectedTarget = provider("GET /selected", selectedTargetCall);
    lazyTarget = provider("GET /lazy", lazyTargetCall);
    const source = provider("GET /source", sourceCall, {
      operationID: "source",
      linkTargets: { "GET /lazy": lazyLoader },
      bindLinks: (invoke) => ({
        selected: (...args: unknown[]) => invoke("GET /selected", args),
        lazy: (...args: unknown[]) => invoke("GET /lazy", args),
      }),
    });
    const resolve = vi.fn<OperationProviderResolver>(async (route, loadProvider) => {
      expect(route).toBe("GET /lazy");
      expect(loadProvider).toBe(lazyLoader);
      return (await loadProvider?.()) ?? lazyTarget;
    });

    const client = createSelectedClient({}, [source, selectedTarget], resolve) as {
      readonly $links: {
        readonly source: {
          selected: (...args: unknown[]) => Promise<unknown>;
          lazy: (...args: unknown[]) => Promise<unknown>;
        };
      };
      readonly $operations: {
        readonly source: RuntimeCall & {
          readonly links: {
            selected: (...args: unknown[]) => Promise<unknown>;
            lazy: (...args: unknown[]) => Promise<unknown>;
          };
        };
      };
    };

    await expect(client.$links.source.selected("one")).resolves.toEqual({
      label: "selected-target",
      args: ["one"],
    });
    expect(resolve).not.toHaveBeenCalled();

    await expect(client.$operations.source.links.lazy("two")).resolves.toEqual({
      label: "lazy-target",
      args: ["two"],
    });
    expect(resolve).toHaveBeenCalledTimes(1);
    expect(lazyLoader).toHaveBeenCalledTimes(1);
    expect(client.$operations.source.links).toBe(client.$links.source);
  });
});
