import { describe, expect, it, vi } from "vitest";
import {
  createWebhookRouter,
  type WebhookHandlers,
  type WebhookRoutes,
} from "../fixtures/generated/collisions/server/webhooks.js";

// The fixture has an optional exact "constructor" member. TypeScript also
// models Object.prototype.constructor, so empty ordinary maps cross an explicit
// JS host-input boundary here. This tests runtime lookup, not a wider public type.
function routerForHostMaps(
  handlers: object,
  options: Omit<Parameters<typeof createWebhookRouter>[1], "routes"> & { routes: object },
) {
  return createWebhookRouter(handlers as WebhookHandlers, {
    ...options,
    routes: options.routes as WebhookRoutes,
  });
}

const names = ["event-hook", "event_hook", "__proto__", "constructor"] as const;
const post = () => ({ status: 204 as const });
const request = (path: string, method = "POST") =>
  new Request(`https://host.example.test${path}`, { method });

function registeredHandlers(): object {
  return Object.fromEntries(names.map((name) => [name, { POST: post }]));
}

function registeredRoutes(): object {
  return Object.fromEntries(names.map((name, index) => [name, `/hooks/${index}`]));
}

describe("Webhook own-property host maps", () => {
  it("accepts empty ordinary maps without reading prototype values as paths", async () => {
    const router = routerForHostMaps({}, { routes: {} });
    expect((await router.fetch(request("/unhandled"))).status).toBe(404);
    expect((await router.fetch(request("/unhandled", "GET"))).status).toBe(404);
  });

  it("does not invoke inherited route getters during registration or dispatch", async () => {
    const get = vi.fn(() => {
      throw new Error("inherited route getter must not run");
    });
    const prototype = Object.create(null);
    for (const name of names) Object.defineProperty(prototype, name, { get });
    const routes: object = Object.create(prototype);
    const empty = routerForHostMaps({}, { routes });
    expect((await empty.fetch(request("/unhandled"))).status).toBe(404);
    expect(() => routerForHostMaps(registeredHandlers(), { routes })).toThrow(
      /must be an absolute path without query or fragment/,
    );
    expect(get).not.toHaveBeenCalled();
  });

  it("ignores inherited handlers and inherited HTTP method getters", async () => {
    const get = vi.fn(() => {
      throw new Error("inherited handler getter must not run");
    });
    const prototype = Object.create(null);
    for (const name of names) Object.defineProperty(prototype, name, { get });
    const inherited: object = Object.create(prototype);
    const inheritedRouter = routerForHostMaps(inherited, { routes: registeredRoutes() });
    expect((await inheritedRouter.fetch(request("/hooks/0"))).status).toBe(404);

    const methodPrototype = Object.defineProperty({}, "POST", { get });
    const ownGroups: object = Object.fromEntries(
      names.map((name) => [name, Object.create(methodPrototype)]),
    );
    const methodRouter = routerForHostMaps(ownGroups, { routes: registeredRoutes() });
    expect((await methodRouter.fetch(request("/hooks/0"))).status).toBe(404);
    expect(get).not.toHaveBeenCalled();
  });

  it("dispatches every exact own name and preserves authentication contexts", async () => {
    const authenticate = vi.fn<
      import("../fixtures/generated/collisions/server/runtime.js").Authenticate
    >(() => undefined);
    const router = routerForHostMaps(registeredHandlers(), {
      routes: registeredRoutes(),
      authenticate,
    });
    for (let index = 0; index < names.length; index++) {
      expect((await router.fetch(request(`/hooks/${index}`))).status).toBe(204);
      expect((await router.fetch(request(`/hooks/${index}`, "GET"))).status).toBe(404);
    }
    expect(authenticate).toHaveBeenCalledTimes(4);
    expect(authenticate.mock.calls.map(([context]) => context.operationID)).toEqual([
      "event-hook-delivery",
      "event_hook_delivery",
      "prototype-delivery",
      "constructor-delivery",
    ]);
  });

  it("retains missing/invalid-path and duplicate-registration diagnostics", () => {
    const handlers: object = { "event-hook": { POST: post } };
    for (const path of [undefined, "relative", "/hook?query=1", "/hook#fragment"]) {
      const routes = path === undefined ? {} : { "event-hook": path };
      expect(() => routerForHostMaps(handlers, { routes })).toThrow(
        "Webhook route for event-hook must be an absolute path without query or fragment",
      );
    }
    expect(() =>
      routerForHostMaps(
        { "event-hook": { POST: post }, event_hook: { POST: post } },
        { routes: { "event-hook": "/same", event_hook: "/same" } },
      ),
    ).toThrow("Duplicate generated Webhook route: POST /same");
  });

  it("keeps authentication denial ahead of the handler", async () => {
    const handler = vi.fn(post);
    const handlers: object = { "event-hook": { POST: handler } };
    const routes = { "event-hook": "/secured" };
    expect((await routerForHostMaps(handlers, { routes }).fetch(request("/secured"))).status).toBe(
      401,
    );
    const denied = routerForHostMaps(handlers, {
      routes,
      authenticate: () => new Response(null, { status: 403 }),
    });
    expect((await denied.fetch(request("/secured"))).status).toBe(403);
    const failed = routerForHostMaps(handlers, {
      routes,
      authenticate: () => {
        throw new Error("host auth error");
      },
    });
    expect((await failed.fetch(request("/secured"))).status).toBe(500);
    expect(handler).not.toHaveBeenCalled();
  });

  it("does not snapshot mutable own route and handler maps", async () => {
    const first = vi.fn(post);
    const second = vi.fn(post);
    const methods = { POST: first };
    const handlers = { "event-hook": methods };
    const routes = { "event-hook": "/before" };
    const router = routerForHostMaps(handlers, { routes, authenticate: () => undefined });
    expect((await router.fetch(request("/before"))).status).toBe(204);
    methods.POST = second;
    routes["event-hook"] = "/after";
    expect((await router.fetch(request("/before"))).status).toBe(404);
    expect((await router.fetch(request("/after"))).status).toBe(204);
    expect(first).toHaveBeenCalledTimes(1);
    expect(second).toHaveBeenCalledTimes(1);
  });
});
