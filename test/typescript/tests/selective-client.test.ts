import { describe, expect, it } from "vitest";
import * as lifecycle from "../fixtures/generated/lifecycle/selective/index.js";
import { operation as echo } from "../fixtures/generated/lifecycle/selective/operations/inline/post.js";
import { operation as events } from "../fixtures/generated/lifecycle/selective/operations/events/get.js";
import * as contract from "../fixtures/generated/client/selective/index.js";
import {
  operations as allOperations,
  routes as allRoutes,
} from "../fixtures/generated/client/selective/all.js";
import { operation as widget } from "../fixtures/generated/client/selective/operations/customers/by-customer-id/widgets/by-widget-id/get.js";
import { operation as createTask } from "../fixtures/generated/client/selective/operations/projects/by-project-id/tasks/post.js";
import { operation as health } from "../fixtures/generated/client/selective/operations/health/get.js";
import { operation as tasks } from "../fixtures/generated/client/selective/operations/tasks/get.js";
import { createClient as fullClient } from "../fixtures/generated/client/index.js";

// Static operation references exercise the same public preparation/assembly API
// without asking Vitest's transform loader to serve native .js lookup assets.
describe("generated selective client", () => {
  it("enumerates names without replacing the default operation references", () => {
    expect(Object.keys(allOperations).sort()).toEqual([
      "createTask",
      "createWidget",
      "getCustomerWidget",
      "listTasks",
      "uploadWidget",
    ]);
    expect(Object.keys(allRoutes).sort()).toEqual([
      "GET /customers/{customerID}/widgets/{widgetID}",
      "GET /health",
      "GET /tasks",
      "POST /projects/{projectID}/tasks",
      "POST /uploads",
      "POST /widgets",
    ]);
    expect(allOperations.listTasks).toBe(contract.operations.listTasks);
    expect(allRoutes["GET /health"]).toBe(contract.routes["GET /health"]);
  });

  it("composes features by value and prepares code without sending API requests", async () => {
    let reads = 0;
    const feature = {
      get selected() {
        reads++;
        return [echo] as const;
      },
    };
    const prepared = await lifecycle.loadOperations([feature, [events], feature]);
    expect(reads).toBe(1);
    let requests = 0;
    const api = lifecycle.createClient({
      operations: prepared,
      baseURL: "https://api.test",
      fetch: async () => {
        requests++;
        return Response.json({ value: 7 });
      },
    });
    expect(requests).toBe(0);
    expect(Object.keys(api.$routes)).toEqual(["POST /inline", "GET /events"]);
    expect(api.inline.post).toBe(api.$operations.echoInline);
    expect(api.events.get).toBe(api.$operations.events);
    expect(await api.inline.post({ body: { value: 7 } })).toEqual({ value: 7 });
    expect(requests).toBe(1);
    expect(Reflect.get(prepared, "then")).toBeUndefined();
  });

  it("reuses preparation without sharing credentials, defaults, or bound callables", async () => {
    const prepared = await lifecycle.loadOperations([echo]);
    const requests: string[] = [];
    const fetch: typeof globalThis.fetch = async (url, init) => {
      requests.push(`${url} ${new Headers(init?.headers).get("authorization")}`);
      return Response.json({ value: 1 });
    };
    const first = lifecycle.createClient({
      operations: prepared,
      baseURL: "https://first.test",
      authorization: "Bearer first",
      fetch,
    });
    const second = lifecycle.createClient({
      operations: prepared,
      baseURL: "https://second.test",
      authorization: "Bearer second",
      fetch,
    });
    expect(first.$operations.echoInline).not.toBe(second.$operations.echoInline);
    await Promise.all([
      first.inline.post({ body: { value: 1 } }),
      second.inline.post.raw({ body: { value: 1 } }),
    ]);
    expect(requests.sort()).toEqual([
      "https://first.test/inline Bearer first",
      "https://second.test/inline Bearer second",
    ]);
    expect(Object.hasOwn(first, "events")).toBe(false);
    expect(Object.hasOwn(first.$operations, "events")).toBe(false);
  });

  it("keeps ID-less routes and empty clients without bringing unselected resources", async () => {
    let requests = 0;
    const prepared = await contract.loadOperations([health]);
    const api = contract.createClient({
      operations: prepared,
      baseURL: "https://api.test",
      fetch: async () => {
        requests++;
        return new Response(null, { status: 204 });
      },
    });
    expect(Object.keys(api.$operations)).toEqual([]);
    expect(api.health.get).toBe(api.$routes["GET /health"]);
    await api.health.get();
    expect(requests).toBe(1);
    const empty = contract.createClient({
      operations: await contract.loadOperations([]),
      fetch: async () => {
        throw Error("empty client must not request");
      },
    });
    expect(Object.keys(empty)).toEqual(["$routes", "$operations"]);
    expect(Object.keys(empty.$routes)).toEqual([]);
  });

  it("preserves nested parameter binding and the full client's request/response contract", async () => {
    const requests: Array<{ url: string; method: string | undefined; body: unknown }> = [];
    const fetch: typeof globalThis.fetch = async (url, init) => {
      requests.push({ url: String(url), method: init?.method, body: init?.body });
      return Response.json({ data: { id: "widget/2", name: "nested" } });
    };
    const options = { baseURL: "https://api.test/api", fetch };
    const api = contract.createClient({
      ...options,
      operations: await contract.loadOperations([widget]),
    });
    const full = fullClient(options);
    expect(await api.customers("customer/1").widgets("widget/2").get()).toEqual(
      await full.customers("customer/1").widgets("widget/2").get(),
    );
    expect(requests[0]).toEqual(requests[1]);
    expect(requests[0]?.url).toBe("https://api.test/api/customers/customer%2F1/widgets/widget%2F2");
    expect(Object.hasOwn(api, "projects")).toBe(false);
    expect(Object.hasOwn(api, "widgets")).toBe(false);
  });

  it("preserves path-bound body calls, pre-fetch validation and abort", async () => {
    const bodies: string[] = [];
    const api = contract.createClient({
      operations: await contract.loadOperations([createTask]),
      baseURL: "https://api.test",
      fetch: async (url, init) => {
        expect(String(url)).toBe("https://api.test/projects/project%2F1/tasks");
        expect(init?.method).toBe("POST");
        bodies.push(String(init?.body));
        return Response.json(
          { id: "task-1", projectID: "project/1", title: "Write", priority: "HIGH" },
          { status: 201 },
        );
      },
    });
    const call = api.projects("project/1").tasks.create;
    expect((await call({ body: { title: "Write", priority: "HIGH" } })).id).toBe("task-1");
    expect(JSON.parse(bodies[0]!)).toEqual({ title: "Write", priority: "HIGH" });
    await expect(
      call({ body: { title: "Write", priority: "HIGH" } }, { signal: AbortSignal.abort() }),
    ).rejects.toMatchObject({ code: "REQUEST_ABORTED" });
    expect(bodies).toHaveLength(1);
  });

  it("provides a synchronous stream handle and shares pagination aliases", async () => {
    const streaming = lifecycle.createClient({
      operations: await lifecycle.loadOperations([events]),
      baseURL: "https://api.test",
      fetch: async () =>
        new Response('{"value":1}\n{"value":2}\n', {
          headers: { "content-type": "application/x-ndjson" },
        }),
    });
    const stream = streaming.events.get.stream();
    expect(Reflect.get(stream, "then")).toBeUndefined();
    const values: number[] = [];
    for await (const value of stream) values.push(value.value);
    expect(values).toEqual([1, 2]);
    const api = contract.createClient({
      operations: await contract.loadOperations([tasks]),
      fetch: async () => Response.json({ data: [], nextCursor: null }),
    });
    expect(api.tasks.paginate).toBe(api.tasks.list.paginate);
    expect(api.tasks.list).toBe(api.$operations.listTasks);
  });
});
