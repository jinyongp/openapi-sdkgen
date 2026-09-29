// Compile-only public API contracts, also checked from emitted declarations alone.
import {
  createClient,
  loadOperations,
  operations,
  routes,
} from "../fixtures/generated/lifecycle/browser/index.js";
import * as contract from "../fixtures/generated/client/browser/index.js";
import * as exact from "../fixtures/generated/selection-public/browser/index.js";
import type {
  OperationReference,
  OperationSelection,
} from "../fixtures/generated/lifecycle/browser/index.js";
import type { OperationInput } from "../fixtures/generated/client/index.js";

type Equal<A, B> =
  (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;
type Assert<T extends true> = T;

async function publicSelectionTypeChecks(flag: boolean) {
  const fixed = createClient({
    operations: await loadOperations([operations.echoInline]),
    baseURL: "https://example.test",
  });
  const value: number = (await fixed.inline.post({ body: { value: 1 } })).value;
  const raw: number = (await fixed.$routes["POST /inline"].raw({ body: { value: 1 } })).data.value;
  void [value, raw];
  // @ts-expect-error Unselected operation is not on the exact map.
  fixed.$operations.events;
  // @ts-expect-error Unselected resource namespace is not present.
  fixed.events;
  // @ts-expect-error Original input type is preserved.
  fixed.inline.post({ body: { value: "wrong" } });
  // @ts-expect-error A JSON operation does not acquire stream.
  fixed.inline.post.stream;
  const group = {
    get selected() {
      return [operations.echoInline] as const;
    },
  };
  const grouped = createClient({
    operations: await loadOperations([group, [routes["GET /events"]]]),
  });
  const stream = grouped.events.get.stream();
  for await (const item of stream) {
    const n: number = item.value;
    void n;
  }
  grouped.inline.post({ body: { value: 1 } });

  const candidate: OperationReference<"GET /events">[] = [];
  const mixed = createClient({
    operations: await loadOperations([operations.echoInline, candidate]),
  });
  mixed.inline.post({ body: { value: 1 } });
  // @ts-expect-error Candidate arrays may be empty.
  mixed.events.get.stream();
  mixed.events?.get?.stream();
  const union = flag ? ([operations.echoInline] as const) : ([operations.events] as const);
  const selected = createClient({ operations: await loadOperations(union) });
  // @ts-expect-error Different union branches are not all guaranteed.
  selected.$operations.echoInline({ body: { value: 1 } });
  selected.$operations.echoInline?.({ body: { value: 1 } });
  // @ts-expect-error The optional callable still has its original input contract.
  selected.$operations.echoInline?.({ body: { value: "wrong" } });
  const empty = createClient({ operations: await loadOperations([]) });
  // @ts-expect-error Empty selection does not expose a resource.
  empty.inline;
  // @ts-expect-error Unknown IDs are not fabricated by development-time declarations.
  operations.missing;
  // @ts-expect-error Routes are exact METHOD/path strings.
  routes["get /events"];
  // @ts-expect-error Preparation, rather than an unresolved selection, is required.
  createClient({ operations: [operations.echoInline] });
  // @ts-expect-error Invalid selection leaves are rejected.
  loadOperations({ selected: 7 });

  const nested = contract.createClient({
    operations: await contract.loadOperations([
      contract.operations.getCustomerWidget,
      contract.operations.createTask,
    ]),
  });
  nested.customers("customer").widgets("widget").get();
  nested.projects("project").tasks.create({ body: { title: "Write", priority: "HIGH" } });
  // @ts-expect-error Original path parameter type is preserved.
  nested.customers(7);
  nested.projects("project").tasks.create({
    // @ts-expect-error Path-bound methods no longer accept a path input section.
    path: { projectID: "wrong" },
    body: { title: "Write", priority: "HIGH" },
  });
  type BoundInput = OperationInput<ReturnType<typeof nested.projects>["tasks"]["create"]>;
  type NoPath = Assert<Equal<"path" extends keyof BoundInput ? true : false, false>>;
  void (null as unknown as NoPath);
  // @ts-expect-error The omitted top-level resource is not resurrected.
  nested.widgets;

  const special = exact.createClient({
    operations: await exact.loadOperations([
      exact.operations.then,
      exact.operations["__proto__"],
      exact.operations.constructor,
      exact.routes["GET /idless"],
    ]),
  });
  special.$operations.then();
  special.$operations["__proto__"]();
  special.$operations.constructor();
  special.$routes["GET /idless"]();
  // @ts-expect-error ID-less operations have no synthetic operation ID.
  special.$operations.idless;
}
void publicSelectionTypeChecks;

async function widenedSelectionTypeChecks(selection: OperationSelection) {
  const api = createClient({ operations: await loadOperations(selection) });
  const result: number | undefined = (await api.$operations.echoInline?.({ body: { value: 1 } }))
    ?.value;
  void result;
  api.$operations.echoInline?.raw({ body: { value: 1 } });
  api.events?.get?.stream();
  // @ts-expect-error A widened recursive selection guarantees no specific operation.
  api.$operations.echoInline({ body: { value: 1 } });
  // @ts-expect-error Possible methods retain the actual operation's input type.
  api.$operations.echoInline?.({ body: { value: "wrong" } });
  const mixed = createClient({
    operations: await loadOperations([operations.echoInline, selection]),
  });
  mixed.$operations.echoInline({ body: { value: 1 } });
  // @ts-expect-error The widened tail does not guarantee this operation.
  mixed.$operations.events.stream();
}
void widenedSelectionTypeChecks;

import * as linked from "../fixtures/generated/selection-links/browser/index.js";
async function linkedTypeChecks() {
  const api = linked.createClient({
    operations: await linked.loadOperations([linked.operations.getSource]),
  });
  const response = await api.source.get.raw();
  const id: string = (
    await api.source.get.links.follow(response, {
      input: { path: { id: "override" } },
      options: { timeoutMS: 100 },
    })
  ).id;
  const xml: string = (await api.source.get.links.xml(response)).id;
  const alias: string = (await api.$links.getSource.follow(response)).id;
  void [id, xml, alias];
  // @ts-expect-error Unselected Link sources do not acquire a root helper alias.
  api.$links.getItem;
  // @ts-expect-error Helper-private targets do not become selected public methods.
  api.$operations.getItem;
  // @ts-expect-error Helper-private target resources are not published.
  api.items;
  // @ts-expect-error ID-less helper target is not automatically on the public route map.
  api.$routes["GET /xml"];
  // @ts-expect-error Target input types survive the lazy invocation boundary.
  api.source.get.links.follow(response, { input: { path: { id: 3 } } });
  // @ts-expect-error Target request options retain their type.
  api.source.get.links.follow(response, { options: { timeoutMS: "later" } });
}
void linkedTypeChecks;
