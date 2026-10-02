import { createSelectedClient } from "./selected-client.js";
import { operationLoaderABI, OperationPreparationError } from "./operation-loader.js";
import type { OperationExecutionProvider } from "./operation-loader.js";
import type { ClientOptions } from "./configuration.js";

/** Shares prepared code while binding request state separately for every instance. */
export function createNamedClientFactory(
  providers: readonly OperationExecutionProvider[],
  generation: string,
): (options: ClientOptions) => object {
  const loads: Map<string, Promise<OperationExecutionProvider>> = new Map<
    string,
    Promise<OperationExecutionProvider>
  >();
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
  return (options: ClientOptions): object =>
    createSelectedClient(
      options,
      providers,
      (
        route: string,
        load: (() => Promise<OperationExecutionProvider>) | undefined,
      ): Promise<OperationExecutionProvider> => {
        const existing: Promise<OperationExecutionProvider> | undefined = loads.get(route);
        if (existing !== undefined) return existing;
        if (load === undefined) {
          return Promise.reject(
            new OperationPreparationError("MODULE_LOAD", "Missing Link target provider"),
          );
        }
        const pending: Promise<OperationExecutionProvider> = Promise.resolve()
          .then(load)
          .then((provider: OperationExecutionProvider): OperationExecutionProvider =>
            validate(provider, route),
          );
        loads.set(route, pending);
        void pending.catch((): void => {
          if (loads.get(route) === pending) loads.delete(route);
        });
        return pending;
      },
    );
}
