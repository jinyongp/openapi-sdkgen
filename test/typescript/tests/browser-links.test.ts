import { describe, expect, it } from "vitest";
import { APIError } from "../fixtures/generated/selection-links/internal/runtime/errors.js";
import * as publicAPI from "../fixtures/generated/selection-links/browser/index.js";
import {
  createOperationLoader,
  staticOperationReference,
} from "../fixtures/generated/selection-links/internal/runtime/operation-loader.js";
import { createSelectedClient } from "../fixtures/generated/selection-links/internal/runtime/selected-client.js";
import { provider as sourceProvider } from "../fixtures/generated/selection-links/internal/executions/source/get.js";
import { provider as itemProvider } from "../fixtures/generated/selection-links/internal/executions/items/by-id/get.js";
import { operation as sourceReference } from "../fixtures/generated/selection-links/browser/operations/source/get.js";
import { operation as itemReference } from "../fixtures/generated/selection-links/browser/operations/items/by-id/get.js";
import type { OperationExecutionProvider } from "../fixtures/generated/selection-links/internal/runtime/operation-loader.js";
import type {
  BaseCall,
  Links,
} from "../fixtures/generated/selection-links/internal/operations/source/get.js";
import type { ClientOptions } from "../fixtures/generated/selection-links/internal/runtime/configuration.js";

type SourceClient = {
  readonly $routes: Record<string, unknown>;
  readonly $operations: { readonly getSource: BaseCall & { readonly links: Links } };
};
function setup(loadTarget?: () => Promise<OperationExecutionProvider>) {
  let loads = 0;
  let binds = 0;
  const target = {
    ...itemProvider,
    bind(context: Parameters<typeof itemProvider.bind>[0]) {
      binds++;
      return itemProvider.bind(context);
    },
  };
  const source = {
    ...sourceProvider,
    linkTargets: {
      ...sourceProvider.linkTargets,
      "GET /items/{id}": () => {
        loads++;
        return loadTarget ? loadTarget() : Promise.resolve(target);
      },
    },
  };
  const loader = createOperationLoader({
    generation: source.generation,
    baseURL: new URL("https://assets.test/browser/"),
    loadClient: async () => ({ createSelectedClient }),
    importModule: async () => {
      throw Error("Static Link reference unexpectedly requested a lookup");
    },
  });
  const ref = staticOperationReference(source);
  const requests: Array<{ url: string; authorization: string | null }> = [];
  const fetch: typeof globalThis.fetch = async (url, init) => {
    requests.push({
      url: String(url),
      authorization: new Headers(init?.headers).get("authorization"),
    });
    return Response.json({ id: "a/b" });
  };
  return {
    loader,
    ref,
    requests,
    loads: () => loads,
    binds: () => binds,
    async client(options: ClientOptions = {}) {
      const prepared = await loader.loadOperations([ref]);
      // Public source/d.ts tests independently verify the facade; this test counts internal loading and binding.
      return loader.createClient({
        baseURL: "https://api.test",
        fetch,
        ...options,
        operations: prepared,
      }) as SourceClient;
    },
  };
}

describe("selected client lazy response links", () => {
  it("does not load or bind private targets during preparation", async () => {
    const test = setup();
    const api = await test.client();
    expect(test.loads()).toBe(0);
    expect(test.binds()).toBe(0);
    expect(test.requests).toEqual([]);
    const response = await api.$operations.getSource.raw();
    expect(test.loads()).toBe(0);
    expect((await api.$operations.getSource.links.follow(response)).id).toBe("a/b");
    expect(test.loads()).toBe(1);
    expect(test.binds()).toBe(1);
    expect(test.requests[1]?.url).toBe("https://api.test/items/a%2Fb");
    expect(Object.keys(api.$routes)).toEqual(["GET /source"]);
    expect(Object.keys(api.$operations)).toEqual(["getSource"]);
    await api.$operations.getSource.links.follow(response);
    expect(test.loads()).toBe(1);
    expect(test.binds()).toBe(1);
  });

  it("shares in-flight code, not credentials or target bindings", async () => {
    const test = setup();
    const first = await test.client({
      baseURL: "https://first.test",
      authorization: "Bearer first",
    });
    const second = await test.client({
      baseURL: "https://second.test",
      authorization: "Bearer second",
    });
    const [one, two] = await Promise.all([
      first.$operations.getSource.raw(),
      second.$operations.getSource.raw(),
    ]);
    await Promise.all([
      first.$operations.getSource.links.follow(one),
      second.$operations.getSource.links.follow(two),
    ]);
    expect(test.loads()).toBe(1);
    expect(test.binds()).toBe(2);
    expect(test.requests.slice(2).sort((a, b) => a.url.localeCompare(b.url))).toEqual([
      { url: "https://first.test/items/a%2Fb", authorization: "Bearer first" },
      { url: "https://second.test/items/a%2Fb", authorization: "Bearer second" },
    ]);
  });

  it("does not send a target request after abort during code loading", async () => {
    let release!: (provider: OperationExecutionProvider) => void;
    const gate = new Promise<OperationExecutionProvider>((resolve) => {
      release = resolve;
    });
    const test = setup(() => gate);
    const api = await test.client();
    const response = await api.$operations.getSource.raw();
    const controller = new AbortController();
    const pending = api.$operations.getSource.links.follow(response, {
      options: { signal: controller.signal },
    });
    expect(test.loads()).toBe(1);
    controller.abort();
    release(itemProvider);
    await expect(pending).rejects.toMatchObject({ code: "REQUEST_ABORTED" });
    expect(test.requests).toHaveLength(1);
  });

  it("keeps synchronous Link-input failures before loading and target request timeouts", async () => {
    const test = setup();
    const api = await test.client();
    const invalid = new APIError({ code: "REQUEST_ABORTED", message: "No HTTP response" });
    expect(() => api.$operations.getSource.links.follow.byStatus.status200(invalid)).toThrow(
      "HTTP response",
    );
    expect(test.loads()).toBe(0);
    const timed = await test.client({
      fetch: async (url) =>
        String(url).endsWith("/source")
          ? Response.json({ id: "a/b" })
          : new Promise<Response>(() => undefined),
    });
    const raw = await timed.$operations.getSource.raw();
    await expect(
      timed.$operations.getSource.links.follow(raw, { options: { timeoutMS: 5 } }),
    ).rejects.toMatchObject({ code: "REQUEST_TIMEOUT" });
  });

  it("retains load causes, rejects wrong providers, and preserves API errors", async () => {
    const cause = new Error("fixture module unavailable");
    const failed = setup(async () => {
      throw cause;
    });
    const api = await failed.client();
    const response = await api.$operations.getSource.raw();
    await expect(api.$operations.getSource.links.follow(response)).rejects.toMatchObject({
      stage: "MODULE_LOAD",
      cause,
    });
    expect(failed.requests).toHaveLength(1);
    const mismatch = setup(async () => ({ ...itemProvider, generation: "wrong" }));
    const other = await mismatch.client();
    await expect(
      other.$operations.getSource.links.follow(await other.$operations.getSource.raw()),
    ).rejects.toMatchObject({ stage: "IDENTITY" });
    expect(mismatch.requests).toHaveLength(1);
    const http = setup();
    const errorAPI = await http.client({
      fetch: async (url) =>
        String(url).endsWith("/source")
          ? Response.json({ id: "a/b" })
          : new Response("denied", { status: 403 }),
    });
    await expect(
      errorAPI.$operations.getSource.links.follow(await errorAPI.$operations.getSource.raw()),
    ).rejects.toMatchObject({ status: 403 });
  });

  it("uses public helpers for ready cyclic targets and ID-less XML targets", async () => {
    const requests: string[] = [];
    const api = publicAPI.createClient({
      operations: await publicAPI.loadOperations([sourceReference, itemReference]),
      baseURL: "https://cycle.test",
      fetch: async (url) => {
        requests.push(String(url));
        return String(url).endsWith("/xml")
          ? new Response("<item><id>xml</id></item>", {
              headers: { "content-type": "application/xml" },
            })
          : Response.json({ id: "a/b" });
      },
    });
    const raw = await api.$operations.getSource.raw();
    expect(api.$links.getSource).toBe(api.$operations.getSource.links);
    expect(api.source.get.links).toBe(api.$links.getSource);
    expect((await api.$links.getSource.follow(raw)).id).toBe("a/b");
    const targetRaw = await api.$operations.getItem.raw({ path: { id: "a/b" } });
    expect((await api.$operations.getItem.links.back(targetRaw)).id).toBe("a/b");
    expect((await api.$operations.getSource.links.xml(raw)).id).toBe("xml");
    expect(Object.hasOwn(api.$routes, "GET /xml")).toBe(false);
    expect(requests).toHaveLength(5);
  });
});
