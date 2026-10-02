import { createSelectedClient } from "./selected-client.js";
import { operationLoaderABI, OperationPreparationError } from "./operation-loader.js";
import type { OperationExecutionProvider } from "./operation-loader.js";
import type { ClientOptions } from "./configuration.js";

/** Shares prepared code while binding request state separately for every instance. */
export function createNamedClientFactory(
  providers: readonly OperationExecutionProvider[],
  generation: string,
): (options: ClientOptions) => object {
  const loads = new Map<string, Promise<OperationExecutionProvider>>();
  function validate(
    provider: OperationExecutionProvider,
    route: string,
  ): OperationExecutionProvider {
    if (
      provider.abi !== operationLoaderABI ||
      provider.generation !== generation ||
      provider.route !== route
    ) {
      throw new OperationPreparationError("IDENTITY", "Generated client provider identity differs");
    }
    return provider;
  }
  for (const provider of providers) {
    loads.set(provider.route, Promise.resolve(validate(provider, provider.route)));
  }
  return (options) =>
    createSelectedClient(options, providers, (route, load) => {
      const existing = loads.get(route);
      if (existing !== undefined) return existing;
      if (load === undefined) {
        return Promise.reject(
          new OperationPreparationError("MODULE_LOAD", "Missing Link target provider"),
        );
      }
      const pending = Promise.resolve()
        .then(load)
        .then((provider) => validate(provider, route));
      loads.set(route, pending);
      void pending.catch(() => {
        if (loads.get(route) === pending) loads.delete(route);
      });
      return pending;
    });
}
