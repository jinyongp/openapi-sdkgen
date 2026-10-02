import { createRequestContext } from "./http-core.js";
import { bindPathOperation } from "./callables.js";
import { defineOwnDataProperty } from "./runtime-support.js";
import { OperationPreparationError } from "./operation-loader.js";
import type { ClientOptions } from "./configuration.js";
import type { RequestContext } from "./http-types.js";
import type {
  OperationExecutionProvider,
  OperationResourcePlacement,
  OperationProviderResolver,
} from "./operation-loader.js";

type ExactCall = ((...args: unknown[]) => Promise<unknown>) & {
  readonly raw: (...args: unknown[]) => Promise<unknown>;
  readonly paginate?: unknown;
};

type ResourceNode = {
  readonly children: Map<string, ResourceNode>;
  parameter?: ResourceNode;
  operation?: { readonly call: ExactCall; readonly placement: OperationResourcePlacement };
};

function node(): ResourceNode {
  return { children: new Map() };
}

function define(target: object, key: string, value: unknown): void {
  defineOwnDataProperty(target as Record<string, unknown>, key, value);
}

function buildResource(node: ResourceNode, bound: readonly unknown[]): object {
  let value: object = Object.create(null) as object;
  if (node.parameter !== undefined) {
    const child: ResourceNode = node.parameter;
    value = (parameter: unknown): object => buildResource(child, [...bound, parameter]);
  } else if (node.operation !== undefined) {
    const { call, placement }: SelectedOperation = node.operation;
    if (placement.pagination) {
      if (call.paginate === undefined)
        throw new OperationPreparationError(
          "BINDING",
          "Generated pagination placement has no callable",
        );
      value = call.paginate as object;
    } else if (placement.pathParameters?.length) {
      const path: Record<string, unknown> = Object.create(null) as Record<string, unknown>;
      for (let index: number = 0; index < placement.pathParameters.length; index++) {
        const name: string | undefined = placement.pathParameters[index];
        if (name !== undefined) defineOwnDataProperty(path, name, bound[index]);
      }
      // The compiler guarantees every exact operation has the same runtime
      // call/raw shape. Its generated declaration retains the precise overloads.
      value = bindPathOperation(
        call,
        path,
        placement.hasInput === true,
        placement.inputOptional === true,
      );
    } else value = call;
  }
  for (const [name, child] of [...node.children].sort(
    ([left]: [string, ResourceNode], [right]: [string, ResourceNode]): 0 | 1 | -1 =>
      left < right ? -1 : left > right ? 1 : 0,
  )) {
    define(value, name, buildResource(child, bound));
  }
  return value;
}

/** Builds only compiler-resolved resource paths belonging to the selected providers. */
export function createSelectedClient(
  options: ClientOptions,
  providers: readonly OperationExecutionProvider[],
  resolve: OperationProviderResolver,
): object {
  const context: RequestContext = createRequestContext(options);
  const routes: Record<string, ExactCall> = Object.create(null) as Record<string, ExactCall>;
  const operations: Record<string, ExactCall> = Object.create(null) as Record<string, ExactCall>;
  const links: Record<string, object> = Object.create(null) as Record<string, object>;
  const resources: ResourceNode = node();
  // Code is shared by the loader. Even helper-private callables are owned by
  // this client, so no credential or mutable request state crosses instances.
  const boundCalls: Map<string, ExactCall> = new Map<string, ExactCall>();
  function bind(provider: OperationExecutionProvider): ExactCall {
    const existing: ExactCall | undefined = boundCalls.get(provider.route);
    if (existing !== undefined) return existing;
    const bound: object = provider.bind(context);
    if (typeof bound !== "function" || typeof Reflect.get(bound, "raw") !== "function") {
      throw new OperationPreparationError(
        "BINDING",
        "Generated execution provider did not bind a call/raw operation",
      );
    }
    const call: ExactCall = bound as ExactCall;
    boundCalls.set(provider.route, call);
    return call;
  }
  for (const provider of providers) {
    if (Object.hasOwn(routes, provider.route)) continue;
    const call: ExactCall = bind(provider);
    defineOwnDataProperty(routes, provider.route, call);
    if (provider.operationID !== undefined)
      defineOwnDataProperty(operations, provider.operationID, call);
    for (const placement of provider.resources ?? []) {
      let current: ResourceNode = resources;
      for (const segment of placement.path) {
        if (segment === null) {
          current.parameter ??= node();
          current = current.parameter;
        } else {
          let child: ResourceNode | undefined = current.children.get(segment);
          if (child === undefined) {
            child = node();
            current.children.set(segment, child);
          }
          current = child;
        }
      }
      let terminal: ResourceNode | undefined = current.children.get(placement.member);
      if (terminal === undefined) {
        terminal = node();
        current.children.set(placement.member, terminal);
      }
      if (terminal.operation !== undefined && terminal.operation.call !== call) {
        throw new OperationPreparationError("BINDING", "Generated resource placements conflict");
      }
      terminal.operation = { call, placement };
    }
  }
  for (const provider of providers) {
    if (provider.bindLinks === undefined) continue;
    const helpers: object = provider.bindLinks(
      (route: string, args: readonly unknown[]): Promise<unknown> => {
        const existing: ExactCall | undefined = boundCalls.get(route);
        if (existing !== undefined) return existing(...args);
        return resolve(route, provider.linkTargets?.[route]).then(
          (target: OperationExecutionProvider): Promise<unknown> => bind(target)(...args),
        );
      },
    );
    define(bind(provider), "links", helpers);
    if (provider.operationID !== undefined)
      defineOwnDataProperty(links, provider.operationID, helpers);
  }
  const client: object = buildResource(resources, []);
  define(client, "$routes", routes);
  define(client, "$operations", operations);
  if (Object.keys(links).length !== 0) define(client, "$links", links);
  return client;
}

type SelectedOperation = {
  readonly call: ExactCall;
  readonly placement: OperationResourcePlacement;
};
