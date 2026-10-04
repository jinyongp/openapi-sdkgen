import type { RequestOptions } from "../http/request.js";
import {
  operationOptionsArguments,
  splitOptionalOperationArguments,
  type OptionalOperationArguments,
} from "./callable-arguments.js";
import {
  bindResourceMethods,
  createResourceInput,
  type ResourceInputMerger,
  type ResourceOperation,
  type ResourcePath,
} from "./resource-binding-support.js";

/** Binds a resource with optional remaining input and options-only calls. */
export function bindResourceOptionalInput(operation: object, path: ResourcePath): unknown {
  const callable: ResourceOperation = operation as ResourceOperation;
  const merge: ResourceInputMerger = createResourceInput(path);
  return bindResourceMethods(
    (...args: readonly unknown[]): Promise<unknown> => {
      const [input, options]: OptionalOperationArguments<unknown, RequestOptions> =
        splitOptionalOperationArguments<unknown, RequestOptions>(args);
      return callable(merge(input), ...operationOptionsArguments(options));
    },
    (...args: readonly unknown[]): Promise<unknown> => {
      const [input, options]: OptionalOperationArguments<unknown, RequestOptions> =
        splitOptionalOperationArguments<unknown, RequestOptions>(args);
      return callable.raw(merge(input), ...operationOptionsArguments(options));
    },
    operation,
  );
}
